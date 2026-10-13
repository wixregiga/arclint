package cmlvocab

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	"github.com/wixregiga/arclint/internal/infrastructure/vocab/cml/syntax"
)

func reserveExistingNames(a *nameAllocator, file *syntax.File) {
	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		if v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
			if !v.IsNil() {
				walk(v.Elem())
			}
			return
		}
		switch v.Kind() {
		default:
			return
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				f := v.Type().Field(i)
				if !f.IsExported() {
					continue
				}
				if f.Name == "Name" && v.Field(i).Kind() == reflect.String {
					a.reserve(v.Field(i).String())
				} else if f.Name != nodeField {
					walk(v.Field(i))
				}
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i))
			}
		}
	}
	walk(reflect.ValueOf(file))
}

// removeDeclaration removes a declaration from whichever grammar slice owns it.
// It never traverses parser-private derivations or touches sibling declarations.
func removeDeclaration(owner any, target any) {
	needle := reflect.ValueOf(target)
	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		if v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
			if !v.IsNil() {
				walk(v.Elem())
			}
			return
		}
		switch v.Kind() {
		default:
			return
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() && v.Type().Field(i).Name != nodeField {
					walk(v.Field(i))
				}
			}
		case reflect.Slice:
			if !v.CanSet() {
				return
			}
			for i := 0; i < v.Len(); {
				item := v.Index(i)
				actual := item
				if actual.Kind() == reflect.Interface && !actual.IsNil() {
					actual = actual.Elem()
				}
				if actual.IsValid() && actual.Type() == needle.Type() && actual.Kind() == reflect.Pointer && actual.Pointer() == needle.Pointer() {
					v.Set(reflect.AppendSlice(v.Slice(0, i), v.Slice(i+1, v.Len())))
					continue
				}
				walk(item)
				i++
			}
		}
	}
	walk(reflect.ValueOf(owner))
}

func replaceDirectives(comments []syntax.Comment, kind string, replacements []syntax.Comment) []syntax.Comment {
	var out []syntax.Comment
	inserted := false
	for _, c := range comments {
		ins, ok, err := readDirective(c)
		if err == nil && ok && ins.kind == kind {
			if !inserted {
				out = append(out, replacements...)
				inserted = true
			}
			continue
		}
		out = append(out, c)
	}
	if !inserted {
		out = append(out, replacements...)
	}
	return out
}

func syncName(n *syntax.Node, handle, exact string) {
	var comments []syntax.Comment
	if handle != exact {
		comments = []syntax.Comment{directive(nameDirective, exact)}
	}
	n.LeadingComments = replaceDirectives(n.LeadingComments, nameDirective, comments)
}

func syncFile(p *projection, old, want vocab.UbiquitousLanguage) error {
	references := bindReferences(p.file)
	desired := freshBoundFile(want, p)
	for i, c := range want.Contexts {
		for j, a := range c.Aggregates {
			root := p.roots[binding(c.Name, a.Name)]
			if root != nil && root.Repository != nil && a.Repository != "" && desired.BoundedContexts[i].Aggregates[j].DomainObjects[0].(*syntax.Entity).Repository == nil && repositoryHasBody(root.Repository) {
				return fmt.Errorf("%s:%d: Repository %q cannot become %q: a CML Repository requires an identifier ending in Repository, unique model-wide; its authored body cannot be carried by a hint", p.path, root.Repository.Pos.Line, root.Repository.Name, a.Repository)
			}
		}
	}
	if len(p.file.Domains) > 0 {
		bindInitialNameComment(p.file, &p.file.Domains[0].Node)
	} else if p.file.Map != nil {
		bindInitialNameComment(p.file, &p.file.Map.Node)
	}
	for _, bc := range p.file.BoundedContexts {
		bindInitialNameComment(p.file, &bc.Node)
	}
	syncProject(p, old, want, desired)
	for _, c := range old.Contexts {
		if _, ok := want.Context(c.Name); !ok {
			removeDeclaration(p.file, p.contexts[c.Name])
		}
	}
	for i, c := range want.Contexts {
		fresh := desired.BoundedContexts[i]
		bc := p.contexts[c.Name]
		if bc == nil {
			p.file.BoundedContexts = append(p.file.BoundedContexts, fresh)
			continue
		}
		previous, _ := old.Context(c.Name)
		if err := syncContext(p, previous, c, bc, fresh); err != nil {
			return err
		}
	}
	// Context order is part of the value. TEAM declarations retain their own
	// slots; modeled declarations fill their existing slots in requested order.
	contexts := make([]*syntax.BoundedContext, 0, len(want.Contexts))
	for _, c := range want.Contexts {
		bc := p.contexts[c.Name]
		if bc == nil {
			for _, candidate := range p.file.BoundedContexts {
				if exactName(candidate.Name, &candidate.Node) == c.Name {
					bc = candidate
				}
			}
		}
		contexts = append(contexts, bc)
	}
	p.file.BoundedContexts = orderContextSlots(p.file.BoundedContexts, contexts)
	syncMap(p, old, want, desired)
	return repairReferences(p.file, references, p.path)
}

func repositoryHasBody(r *syntax.Repository) bool {
	return r.Doc != "" || r.Present["doc"] || r.GapClass || r.NoGapClass || r.Hint != "" || r.Subscribe != nil || len(r.Dependencies)+len(r.Operations)+len(r.TrailingComments) > 0 || r.Present["hint"]
}

func orderContextSlots(existing, modeled []*syntax.BoundedContext) []*syntax.BoundedContext {
	var out []*syntax.BoundedContext
	i := 0
	for _, bc := range existing {
		if bc.Type == teamContext {
			out = append(out, bc)
		} else if i < len(modeled) {
			out = append(out, modeled[i])
			i++
		}
	}
	out = append(out, modeled[i:]...)
	return out
}

func syncContext(p *projection, old, want vocab.BoundedContext, bc, fresh *syntax.BoundedContext) error {
	if old.Definition != want.Definition {
		bc.DomainVisionStatement = want.Definition
		delete(bc.Present, "domainVisionStatement")
	}
	module := func() *syntax.SculptorModule { return contextModule(bc) }
	kept, wasOccupied := ownModule(bc)
	for _, a := range old.Aggregates {
		if _, ok := want.Aggregate(a.Name); !ok {
			container := p.containers[binding(want.Name, a.Name)]
			// Surviving declarations keep their authored bodies when their old
			// aggregate container disappears. Entities are attached below.
			for _, obj := range container.DomainObjects {
				switch o := obj.(type) {
				case *syntax.ValueObject:
					name := exactName(o.Name, &o.Node)
					_, value := want.ValueObject(name)
					_, spec := want.Specification(name)
					if value || spec {
						module().DomainObjects = append(module().DomainObjects, o)
					}
				case *syntax.DomainEvent:
					if _, keep := want.Event(exactName(o.Name, &o.Node)); keep {
						module().DomainObjects = append(module().DomainObjects, o)
					}
				}
			}
			for _, service := range container.Services {
				if _, keep := want.Service(exactName(service.Name, &service.Node)); keep {
					bc.DomainServices = append(bc.DomainServices, service)
				}
			}
			removeDeclaration(bc, container)
		}
	}
	for _, a := range old.Aggregates {
		for _, e := range a.Entities {
			if _, _, ok := want.Entity(e.Name); !ok {
				removeDeclaration(bc, p.entities[binding(want.Name, e.Name)])
			}
		}
	}
	for i, a := range want.Aggregates {
		var previous vocab.Aggregate
		previous, _ = old.Aggregate(a.Name)
		root := p.roots[binding(want.Name, a.Name)]
		container := p.containers[binding(want.Name, a.Name)]
		freshAggregate := fresh.Aggregates[i]
		freshRoot := freshAggregate.DomainObjects[0].(*syntax.Entity)
		freshMembers := append([]syntax.SimpleDomainObject(nil), freshAggregate.DomainObjects[1:]...)
		if root == nil {
			container, root = freshAggregate, freshRoot
			// Existing member bindings take precedence over fresh declarations.
			container.DomainObjects = container.DomainObjects[:1]
			bc.Aggregates = append(bc.Aggregates, container)
		}
		if previous.Definition != a.Definition {
			root.Doc = a.Definition
			delete(root.Present, "doc")
		}
		if !slices.Equal(previous.Aliases, a.Aliases) {
			root.Hint = replaceHint(root.Hint, "alias", a.Aliases)
			delete(root.Present, "hint")
		}
		if previous.Identity != a.Identity {
			syncIdentity(root, freshRoot.Attributes[0])
		}
		root.Operations = syncOperations(root.Operations, a.Invariants, a.Assertions)
		if previous.Repository != a.Repository || root.Repository != nil && freshRoot.Repository == nil {
			syncRepository(root, freshRoot, a.Repository)
		}
		if previous.Factory != a.Factory && root != freshRoot {
			syncFactory(p.factories[binding(want.Name, a.Name)], container, freshAggregate, a.Factory)
		}

		for j, e := range a.Entities {
			entity := p.entities[binding(want.Name, e.Name)]
			freshEntity := freshMembers[j].(*syntax.Entity)
			if entity == nil {
				container.DomainObjects = append(container.DomainObjects, freshEntity)
				continue
			}
			prev, prevAggregate, _ := old.Entity(e.Name)
			if prevAggregate.Name != a.Name {
				removeDeclaration(bc, entity)
				container.DomainObjects = append(container.DomainObjects, entity)
			}
			if prev.Definition != e.Definition {
				entity.Doc = e.Definition
				delete(entity.Present, "doc")
			}
			if !slices.Equal(prev.Aliases, e.Aliases) {
				entity.Hint = replaceHint(entity.Hint, "alias", e.Aliases)
				delete(entity.Present, "hint")
			}
			if prev.Identity != e.Identity {
				var attr *syntax.Attribute
				if len(freshEntity.Attributes) > 0 {
					attr = freshEntity.Attributes[0]
				}
				syncIdentity(entity, attr)
			}
		}
	}
	for _, v := range old.ValueObjects {
		if _, ok := want.ValueObject(v.Name); !ok {
			if _, spec := want.Specification(v.Name); !spec {
				removeDeclaration(bc, p.values[binding(want.Name, v.Name)])
			}
		}
	}
	for _, s := range old.Specifications {
		if _, ok := want.Specification(s.Name); !ok {
			if _, value := want.ValueObject(s.Name); !value {
				removeDeclaration(bc, p.values[binding(want.Name, s.Name)])
			}
		}
	}

	if len(fresh.Modules) > 0 {
		for _, obj := range fresh.Modules[0].DomainObjects {
			switch f := obj.(type) {
			case *syntax.ValueObject:
				syncValue(p, old, want, f, module)
			case *syntax.DomainEvent:
				syncEvent(p, old, want, bc, f, module)
			}
		}
	}

	for _, e := range old.Events {
		if _, ok := want.Event(e.Name); !ok {
			removeDeclaration(bc, p.events[binding(want.Name, e.Name)])
		}
	}
	for _, s := range old.Services {
		if _, ok := want.Service(s.Name); !ok {
			removeDeclaration(bc, p.services[binding(want.Name, s.Name)])
		}
	}
	for i, s := range want.Services {
		service := p.services[binding(want.Name, s.Name)]
		if service == nil {
			bc.DomainServices = append(bc.DomainServices, fresh.DomainServices[i])
			continue
		}
		previous, _ := old.Service(s.Name)
		if previous.Definition != s.Definition {
			service.Doc = s.Definition
			delete(service.Present, "doc")
		}
		service.Operations = syncServiceOperations(service.Operations, s.Assertions)
	}
	syncQuestions(bc, want)
	syncServiceBlock(p, bc, want)
	if kept != nil && wasOccupied && !moduleOccupied(kept) && moduleBare(kept) {
		removeDeclaration(bc, kept)
	}
	return syncContextOrder(bc, want)
}

// ownModule finds the Module arclint keeps a context's value objects and
// events in, the one named after the context, and whether it holds anything.
func ownModule(bc *syntax.BoundedContext) (*syntax.SculptorModule, bool) {
	for _, m := range bc.Modules {
		if m.Name == bc.Name {
			return m, moduleOccupied(m)
		}
	}
	return nil, false
}

func moduleOccupied(m *syntax.SculptorModule) bool {
	return len(m.DomainObjects)+len(m.Aggregates)+len(m.Services)+len(m.Resources)+len(m.Consumers) > 0
}

// A Module appears with its first member and goes with its last, so an edit and
// its inverse leave the file as it was. A Module that was already empty, or that
// carries a doc, a hint or comments, is authored structure and stays.
func moduleBare(m *syntax.SculptorModule) bool {
	return m.Doc == "" && m.Hint == "" && !m.External && m.BasePackage == "" && len(m.LeadingComments) == 0 && len(m.TrailingComments) == 0
}

// A direct services block precedes every Module and Aggregate when documented.
// Preserve the requested service order by carrying earlier modeled services
// into that block too, with their complete authored bodies and comments. Later
// nested services keep their homes. Application services and factories stay put.
func syncServiceBlock(p *projection, bc *syntax.BoundedContext, want vocab.BoundedContext) {
	direct := map[string]*syntax.Service{}
	documented := false
	for _, service := range bc.DomainServices {
		direct[exactName(service.Name, &service.Node)] = service
		documented = documented || service.Doc != "" || service.Present["doc"]
	}
	if !documented {
		return
	}
	last := -1
	for i, service := range want.Services {
		if direct[service.Name] != nil {
			last = i
		}
	}
	for _, service := range want.Services[:last+1] {
		if direct[service.Name] != nil {
			continue
		}
		node := p.services[binding(want.Name, service.Name)]
		removeDeclaration(bc, node)
		bc.DomainServices = append(bc.DomainServices, node)
	}
}

func satisfaction(s string) bool {
	return s == satisfactionOperation || s == "SatisfiedBy" || s == "satisfied_by" || s == "isSatisfiedBy"
}

func allAggregates(bc *syntax.BoundedContext) []*syntax.Aggregate {
	out := append([]*syntax.Aggregate(nil), bc.Aggregates...)
	for _, m := range bc.Modules {
		out = append(out, m.Aggregates...)
	}
	return out
}

func syncIdentity(entity *syntax.Entity, fresh *syntax.Attribute) {
	type property struct {
		attr   *syntax.Attribute
		ref    *syntax.Reference
		offset int
	}
	var first *property
	for _, a := range entity.Attributes {
		if a.Key && (!builtinTypes[a.Type] || strings.HasPrefix(a.Spellings["type"], "^")) && identifier.MatchString(a.Type) {
			item := property{attr: a, offset: a.Pos.Offset}
			if first == nil || item.offset < first.offset {
				first = &item
			}
		}
	}
	for _, r := range entity.References {
		if r.Key && identifier.MatchString(r.DomainObjectType.Name) {
			item := property{ref: r, offset: r.Pos.Offset}
			if first == nil || item.offset < first.offset {
				first = &item
			}
		}
	}
	if first == nil {
		if fresh != nil {
			entity.Attributes = append(entity.Attributes, fresh)
		}
		return
	}
	if fresh == nil {
		if first.attr != nil {
			removeDeclaration(entity, first.attr)
		} else {
			removeDeclaration(entity, first.ref)
		}
		return
	}
	if first.attr != nil {
		first.attr.Type = fresh.Type
		delete(first.attr.Spellings, "type")
		if spelling := fresh.Spellings["type"]; spelling != "" {
			if first.attr.Spellings == nil {
				first.attr.Spellings = map[string]string{}
			}
			first.attr.Spellings["type"] = spelling
		}
		syncName(&first.attr.Node, fresh.Type, exactName(fresh.Type, &fresh.Node))
	} else {
		first.ref.DomainObjectType.Name = fresh.Type
		syncName(&first.ref.Node, fresh.Type, exactName(fresh.Type, &fresh.Node))
	}
}

func syncOperations(existing []*syntax.DomainObjectOperation, inv []vocab.Invariant, assertions []vocab.Assertion) []*syntax.DomainObjectOperation {
	desired := freshOperations(inv, assertions)
	owned := map[string]*syntax.DomainObjectOperation{}
	for _, op := range desired {
		if _, _, ok := ownedOperation(op.Name, op.Doc, op.Hint, &op.Node); ok {
			owned[op.Name] = op
		}
	}
	var out []*syntax.DomainObjectOperation
	declared := map[string]bool{}
	for _, op := range existing {
		if _, _, ok := ownedOperation(op.Name, op.Doc, op.Hint, &op.Node); ok {
			fresh := owned[op.Name]
			if fresh == nil {
				continue
			}
			op.Doc = fresh.Doc
			if hintOne(op.Hint, "on") != hintOne(fresh.Hint, "on") {
				op.Hint = scalarHint(op.Hint, "on", hintOne(fresh.Hint, "on"))
			}
			delete(op.Present, "doc")
			delete(op.Present, "hint")
			delete(owned, op.Name)
		}
		out = append(out, op)
		declared[op.Name] = true
	}
	name := func(op *syntax.DomainObjectOperation) string { return op.Name }
	isOwned := func(op *syntax.DomainObjectOperation) bool {
		_, _, ok := ownedOperation(op.Name, op.Doc, op.Hint, &op.Node)
		return ok
	}
	placed := map[string]bool{}
	for i, op := range desired {
		if _, ok := owned[op.Name]; ok || !declared[op.Name] {
			out = insertBesideNeighbors(out, desired, i, name, isOwned, placed)
			declared[op.Name] = true
		}
	}
	return orderOperations(out, desired, name, isOwned)
}

func syncServiceOperations(existing []*syntax.ServiceOperation, assertions []vocab.Assertion) []*syntax.ServiceOperation {
	desired := freshServiceOperations(assertions)
	owned := map[string]*syntax.ServiceOperation{}
	for _, op := range desired {
		if kind, _, ok := ownedOperation(op.Name, op.Doc, op.Hint, &op.Node); ok && kind == assertionPrefix {
			owned[op.Name] = op
		}
	}
	var out []*syntax.ServiceOperation
	declared := map[string]bool{}
	for _, op := range existing {
		if kind, _, ok := ownedOperation(op.Name, op.Doc, op.Hint, &op.Node); ok && kind == assertionPrefix {
			fresh := owned[op.Name]
			if fresh == nil {
				continue
			}
			op.Doc = fresh.Doc
			if hintOne(op.Hint, "on") != hintOne(fresh.Hint, "on") {
				op.Hint = scalarHint(op.Hint, "on", hintOne(fresh.Hint, "on"))
			}
			delete(op.Present, "doc")
			delete(op.Present, "hint")
			delete(owned, op.Name)
		}
		out = append(out, op)
		declared[op.Name] = true
	}
	name := func(op *syntax.ServiceOperation) string { return op.Name }
	isOwned := func(op *syntax.ServiceOperation) bool {
		kind, _, ok := ownedOperation(op.Name, op.Doc, op.Hint, &op.Node)
		return ok && kind == assertionPrefix
	}
	placed := map[string]bool{}
	for i, op := range desired {
		if _, ok := owned[op.Name]; ok || !declared[op.Name] {
			out = insertBesideNeighbors(out, desired, i, name, isOwned, placed)
			declared[op.Name] = true
		}
	}
	return orderOperations(out, desired, name, isOwned)
}

// insertBesideNeighbors writes desired[i], an operation the owner does not
// declare yet, next to the operation that precedes it in the requested order,
// or before the one that follows when it leads, and not at the end of the
// owner. Only operations the vocabulary owns (and the ones inserted here)
// anchor it: the owner's other operations are foreign content with no order of
// their own to keep. Filling the owned slots in order from the end would carry
// every later owned operation one slot on, past the foreign operations between.
func insertBesideNeighbors[T any](out, desired []T, i int, nameOf func(T) string, owned func(T) bool, placed map[string]bool) []T {
	find := func(name string) int {
		for at, op := range out {
			if nameOf(op) == name && (placed[name] || owned(op)) {
				return at
			}
		}
		return -1
	}
	defer func() { placed[nameOf(desired[i])] = true }()
	for j := i - 1; j >= 0; j-- {
		if at := find(nameOf(desired[j])); at >= 0 {
			return slices.Insert(out, at+1, desired[i])
		}
	}
	for j := i + 1; j < len(desired); j++ {
		if at := find(nameOf(desired[j])); at >= 0 {
			return slices.Insert(out, at, desired[i])
		}
	}
	return append(out, desired[i])
}

func relationPair(from, to string) string {
	if from > to {
		from, to = to, from
	}
	return from + "\x00" + to
}

func syncMap(p *projection, old, want vocab.UbiquitousLanguage, desired *syntax.File) {
	if p.file.Map == nil {
		if len(want.Relations) > 0 {
			p.file.Map = desired.Map
		}
		return
	}
	m := p.file.Map
	handles := map[string]string{}
	names := map[string]string{}
	for _, bc := range p.file.BoundedContexts {
		if bc.Type != teamContext {
			n := exactName(bc.Name, &bc.Node)
			if !contextPattern.MatchString(n) {
				n = snakeCase(n)
			}
			handles[n] = bc.Name
			names[bc.Name] = n
		}
	}
	// Preserve every foreign contains entry (for example a TEAM), remove deleted
	// contexts and put modeled entries in the requested order.
	var contains []syntax.Ref
	for _, c := range want.Contexts {
		contains = append(contains, syntax.Ref{Name: handles[c.Name]})
	}
	for _, ref := range m.BoundedContexts {
		for _, bc := range p.file.BoundedContexts {
			if ref.Name == bc.Name && bc.Type == teamContext {
				contains = append(contains, ref)
			}
		}
	}
	teamHandles := map[string]bool{}
	for _, bc := range p.file.BoundedContexts {
		if bc.Type == teamContext {
			teamHandles[bc.Name] = true
		}
	}
	modeledContains := contains[:len(want.Contexts)]
	m.BoundedContexts = fillModeledSlots(m.BoundedContexts, modeledContains, func(ref syntax.Ref) bool { return teamHandles[ref.Name] })
	originalRelationships := append([]syntax.Relationship(nil), m.Relationships...)
	existing := map[string]syntax.Relationship{}
	var foreign []syntax.Relationship
	for _, r := range m.Relationships {
		from, to, _, _ := relationshipValue(r)
		a, b := names[from], names[to]
		if a != "" && b != "" {
			existing[relationPair(a, b)] = r
		} else {
			deleted := false
			for _, c := range old.Contexts {
				if (p.contexts[c.Name].Name == from || p.contexts[c.Name].Name == to) && handles[c.Name] == "" {
					deleted = true
				}
			}
			if !deleted {
				foreign = append(foreign, r)
			}
		}
	}
	visitNodes(m, func(n *syntax.Node) {
		n.LeadingComments = replaceDirectives(n.LeadingComments, separateWaysDirective, nil)
		n.TrailingComments = replaceDirectives(n.TrailingComments, separateWaysDirective, nil)
	})
	var relationships []syntax.Relationship
	var pending []syntax.Comment
	for _, r := range want.Relations {
		if r.Kind == vocab.RelationSeparateWays {
			if carrier := existing[relationPair(r.From, r.To)]; carrier != nil {
				_, _, _, n := relationshipValue(carrier)
				comments := replaceDirectives(n.LeadingComments, "description", nil)
				pending = append(pending, comments...)
				pending = append(pending, n.TrailingComments...)
			}
			pending = append(pending, directive(separateWaysDirective, r.From, r.To, r.Description))
			continue
		}
		relation := existing[relationPair(r.From, r.To)]
		previous, ok := old.Relation(r.From, r.To)
		if relation == nil || !ok || previous.From != r.From || previous.To != r.To || previous.Kind != r.Kind {
			canonical := freshRelation(r, handles)
			if relation != nil {
				copyRelationExtras(canonical, relation)
			}
			relation = canonical
		}
		_, _, _, n := relationshipValue(relation)
		if !ok || previous.Kind == vocab.RelationSeparateWays || previous.Description != r.Description {
			var descriptions []syntax.Comment
			if r.Description != "" {
				descriptions = []syntax.Comment{directive("description", r.Description)}
			}
			n.LeadingComments = replaceDirectives(n.LeadingComments, "description", descriptions)
		}
		n.LeadingComments = append(pending, n.LeadingComments...)
		pending = nil
		relationships = append(relationships, relation)
	}
	foreignRelations := map[syntax.Relationship]bool{}
	for _, relation := range foreign {
		foreignRelations[relation] = true
	}
	// Deleted TEAM relationships have no declaration to retain. Other foreign
	// relationships keep their slots among the reordered modeled relationships.
	var retained []syntax.Relationship
	for _, relation := range originalRelationships {
		from, to, _, _ := relationshipValue(relation)
		if foreignRelations[relation] || names[from] != "" && names[to] != "" {
			retained = append(retained, relation)
		}
	}
	m.Relationships = fillModeledSlots(retained, relationships, func(relation syntax.Relationship) bool { return foreignRelations[relation] })
	m.TrailingComments = append(m.TrailingComments, pending...)
}

func copyRelationExtras(target, source syntax.Relationship) {
	dst, src := reflect.ValueOf(target).Elem(), reflect.ValueOf(source).Elem()
	for _, field := range []string{"Name", "ImplementationTechnology", "UpstreamExposedAggregates", "ExposedAggregatesComment", "DownstreamGovernanceRights"} {
		to, from := dst.FieldByName(field), src.FieldByName(field)
		if to.IsValid() && from.IsValid() && to.Type() == from.Type() {
			to.Set(from)
		}
	}
	// The parsed form describes the old relationship grammar, and so do the
	// notation choices it retained. Keep provenance, comments and the authored
	// body braces, and let the new relation print in its canonical notation.
	_, _, _, to := relationshipValue(target)
	_, _, _, from := relationshipValue(source)
	to.Pos, to.NamePos, to.ExplicitBody = from.Pos, from.NamePos, from.ExplicitBody
	to.LeadingComments, to.TrailingComments = from.LeadingComments, from.TrailingComments
}

// Compile-time assertion keeps the adapter aligned with the inward-owned port.
var _ vocab.Repository = Repository{}

func syncProject(p *projection, old, want vocab.UbiquitousLanguage, desired *syntax.File) {
	if len(p.file.Domains) > 0 {
		d, fresh := p.file.Domains[0], desired.Domains[0]
		if old.Project != want.Project {
			d.Name = fresh.Name
			syncName(&d.Node, d.Name, want.Project)
		}
		if old.Description != want.Description {
			d.DomainVisionStatement = want.Description
			delete(d.Present, "domainVisionStatement")
		}
		return
	}
	if want.Description != "" {
		domain := desired.Domains[0]
		domain.Pos = syntax.Pos{Offset: -1, Line: 1, Column: 1}
		p.file.Domains = append([]*syntax.Domain{domain}, p.file.Domains...)
		if p.file.Map != nil {
			p.file.Map.Name = ""
			delete(p.file.Map.Present, "name")
			p.file.Map.LeadingComments = replaceDirectives(p.file.Map.LeadingComments, nameDirective, nil)
		}
		return
	}
	if p.file.Map != nil && old.Project != want.Project {
		p.file.Map.Name = desired.Domains[0].Name
		syncName(&p.file.Map.Node, p.file.Map.Name, want.Project)
	}
}

func syncRepository(root, fresh *syntax.Entity, name string) {
	if fresh.Repository == nil {
		if name != "" && root.Repository != nil {
			root.TrailingComments = append(root.TrailingComments, replaceDirectives(root.Repository.LeadingComments, nameDirective, nil)...)
		}
		root.Repository = nil
		root.Hint = scalarHint(root.Hint, "repository", name)
		delete(root.Present, "hint")
		return
	}
	if root.Repository == nil {
		root.Repository = fresh.Repository
	} else {
		root.Repository.Name = fresh.Repository.Name
		syncName(&root.Repository.Node, root.Repository.Name, name)
	}
	root.Hint = replaceHint(root.Hint, "repository", nil)
	delete(root.Present, "hint")
}

func syncValue(p *projection, old, want vocab.BoundedContext, fresh *syntax.ValueObject, module func() *syntax.SculptorModule) {
	name := exactName(fresh.Name, &fresh.Node)
	value := p.values[binding(want.Name, name)]
	if value == nil {
		m := module()
		m.DomainObjects = append(m.DomainObjects, fresh)
		return
	}
	previousDefinition := ""
	if previous, ok := old.ValueObject(name); ok {
		previousDefinition = previous.Definition
	} else if previous, ok := old.Specification(name); ok {
		previousDefinition = previous.Definition
	}
	if previousDefinition != fresh.Doc {
		value.Doc = fresh.Doc
		delete(value.Present, "doc")
	}
	if v, ok := want.ValueObject(name); ok {
		previous, _ := old.ValueObject(name)
		if !slices.Equal(previous.Aliases, v.Aliases) {
			value.Hint = replaceHint(value.Hint, "alias", v.Aliases)
		}
		value.Operations = syncOperations(value.Operations, v.Invariants, nil)
		if _, wasSpec := old.Specification(name); wasSpec {
			value.Operations = withoutSatisfaction(value.Operations)
		}
	} else {
		syncSatisfaction(value, fresh)
	}

	delete(value.Present, "hint")
}

func syncEvent(p *projection, old, want vocab.BoundedContext, bc *syntax.BoundedContext, fresh *syntax.DomainEvent, module func() *syntax.SculptorModule) {
	name := exactName(fresh.Name, &fresh.Node)
	event := p.events[binding(want.Name, name)]
	if event == nil {
		m := module()
		m.DomainObjects = append(m.DomainObjects, fresh)
		return
	}
	previous, _ := old.Event(name)
	next, _ := want.Event(name)
	if previous.Definition != next.Definition {
		event.Doc = next.Definition
		delete(event.Present, "doc")
	}
	if previous.RaisedBy == next.RaisedBy {
		return
	}
	event.BelongsTo = fresh.BelongsTo
	for _, a := range allAggregates(bc) {
		found := false
		rootName := ""
		for _, child := range a.DomainObjects {
			if child == event {
				found = true
			}
			if root, ok := child.(*syntax.Entity); ok && root.AggregateRoot {
				rootName = exactName(root.Name, &root.Node)
			}
		}
		if found && rootName != next.RaisedBy {
			removeDeclaration(a, event)
			m := module()
			m.DomainObjects = append(m.DomainObjects, event)
			return
		}
	}
}

func withoutSatisfaction(existing []*syntax.DomainObjectOperation) []*syntax.DomainObjectOperation {
	out := make([]*syntax.DomainObjectOperation, 0, len(existing))
	for _, op := range existing {
		if !satisfaction(op.Name) {
			out = append(out, op)
		}
	}
	return out
}

func syncSatisfaction(value, fresh *syntax.ValueObject) {
	value.Operations = syncOperations(value.Operations, nil, nil)
	for _, op := range value.Operations {
		if satisfaction(op.Name) {
			return
		}
	}
	value.Operations = append(value.Operations, fresh.Operations[0])
}

func syncFactory(existing *syntax.Service, owner, fresh *syntax.Aggregate, name string) {
	if name == "" {
		if existing != nil {
			removeDeclaration(owner, existing)
		}
		return
	}
	next := fresh.Services[0]
	if existing == nil {
		owner.Services = append(owner.Services, next)
		return
	}
	existing.Name = next.Name
	syncName(&existing.Node, next.Name, name)
	existing.Hint = scalarHint(existing.Hint, "role", factoryRole)
}
