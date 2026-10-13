package cmlvocab

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	"github.com/wixregiga/arclint/internal/infrastructure/vocab/cml/syntax"
)

type projection struct {
	file       *syntax.File
	path       string
	problems   []string
	contexts   map[string]*syntax.BoundedContext
	roots      map[string]*syntax.Entity
	containers map[string]*syntax.Aggregate
	entities   map[string]*syntax.Entity
	values     map[string]*syntax.ValueObject
	events     map[string]*syntax.DomainEvent
	services   map[string]*syntax.Service
	factories  map[string]*syntax.Service
	names      map[string]map[string]string
	directives []instruction
}

func binding(context, name string) string { return context + "\x00" + name }
func (p *projection) problem(pos syntax.Pos, message string) {
	line, col := pos.Line, pos.Column
	if line == 0 {
		line = 1
	}
	if col == 0 {
		col = 1
	}
	p.problems = append(p.problems, fmt.Sprintf("%s:%d:%d: %s", p.path, line, col, message))
}

// Parse projects CML into the same validated language held by the YAML adapter.
// No partial language is returned when syntax, mapping or domain validation fails.
func Parse(data []byte, recording vocab.DomainRecording) (vocab.UbiquitousLanguage, error) {
	if recording.Format() != vocab.DomainRecordingCML {
		return vocab.UbiquitousLanguage{}, fmt.Errorf("domain file %q: the CML adapter does not read a %q recording", recording.Path(), recording.Format())
	}
	file, err := syntax.ParseFile(recording.Path(), data)
	if err != nil {
		return vocab.UbiquitousLanguage{}, fmt.Errorf("parse CML: %w", err)
	}
	lang, _, err := project(file, recording)
	return lang, err
}

// Parser implements the application's VocabularySource port.
type Parser struct{}

// ParseUbiquitousLanguage reads a CML DomainRecording from supplied content.
func (Parser) ParseUbiquitousLanguage(content []byte, recording vocab.DomainRecording) (vocab.UbiquitousLanguage, error) {
	return Parse(content, recording)
}

func project(file *syntax.File, recording vocab.DomainRecording) (vocab.UbiquitousLanguage, *projection, error) {
	p := &projection{file: file, path: recording.Path(), contexts: map[string]*syntax.BoundedContext{}, roots: map[string]*syntax.Entity{}, containers: map[string]*syntax.Aggregate{}, entities: map[string]*syntax.Entity{}, values: map[string]*syntax.ValueObject{}, events: map[string]*syntax.DomainEvent{}, services: map[string]*syntax.Service{}, factories: map[string]*syntax.Service{}, names: map[string]map[string]string{}}
	visitNodes(file, func(n *syntax.Node) {
		for _, comments := range [][]syntax.Comment{n.LeadingComments, n.TrailingComments} {
			for _, c := range comments {
				ins, ok, err := readDirective(c)
				if err != nil {
					p.problem(c.Pos, err.Error())
				} else if ok {
					p.directives = append(p.directives, ins)
				}
			}
		}
	})
	sort.SliceStable(p.directives, func(i, j int) bool { return p.directives[i].comment.Pos.Offset < p.directives[j].comment.Pos.Offset })
	for _, imp := range file.Imports {
		p.problem(imp.Pos, "imports are not supported; record the project's domain in one file")
	}
	var raw vocab.UbiquitousLanguage
	if len(file.Domains) > 0 {
		d := file.Domains[0]
		raw.Project = declarationName(file, d.Name, &d.Node)
		raw.Description = d.DomainVisionStatement
	} else if file.Map != nil {
		raw.Project = declarationName(file, file.Map.Name, &file.Map.Node)
	}
	if strings.TrimSpace(raw.Project) == "" {
		p.problem(file.Pos, "add a Domain or a named ContextMap to name the project")
	}
	team := map[string]bool{}
	contextNames := map[string]string{}
	for _, bc := range file.BoundedContexts {
		if bc.Type == teamContext {
			team[bc.Name] = true
			continue
		}
		name := declarationName(file, bc.Name, &bc.Node)
		if !hasNameDirective(file, &bc.Node) && !contextPattern.MatchString(name) {
			name = snakeCase(name)
		}
		if !contextPattern.MatchString(name) {
			p.problem(bc.NamePos, fmt.Sprintf("context %q cannot become a lowercase context name", name))
		}
		if _, dup := p.contexts[name]; dup {
			p.problem(bc.NamePos, fmt.Sprintf("context %q is recorded twice", name))
		}
		p.contexts[name] = bc
		contextNames[bc.Name] = name
		raw.Contexts = append(raw.Contexts, p.context(name, bc))
	}
	if file.Map != nil {
		raw.Relations = p.readRelations(file.Map, team, contextNames)
	}

	// Validate all independent pieces, then the complete consistency unit. The
	// constructor remains the final authority, including invariants not mapped here.
	p.collectValidation(raw)
	lang, err := vocab.NewUbiquitousLanguage(raw.Project, raw.Description, raw.Contexts, raw.Relations)
	if err != nil {
		p.problem(file.Pos, err.Error())
	}
	if len(p.problems) > 0 {
		return vocab.UbiquitousLanguage{}, p, fmt.Errorf("CML domain mapping:\n%s", strings.Join(uniqueStrings(p.problems), "\n"))
	}
	lang.Recording = recording
	return lang, p, nil
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			out = append(out, s)
			seen[s] = true
		}
	}
	return out
}

// Directives within braces belong to the block's trailing comments or its
// child nodes. A leading directive on the block itself lies outside its body.
func scopedDirectives(owner any) []instruction {
	root := reflect.ValueOf(owner).Elem().FieldByName(nodeField).Addr().Interface().(*syntax.Node)
	var out []instruction
	visitNodes(owner, func(n *syntax.Node) {
		comments := append([]syntax.Comment(nil), n.TrailingComments...)
		if n != root {
			comments = append(comments, n.LeadingComments...)
		}
		for _, c := range comments {
			ins, ok, err := readDirective(c)
			if ok && err == nil {
				out = append(out, ins)
			}
		}
	})
	sort.SliceStable(out, func(i, j int) bool { return out[i].comment.Pos.Offset < out[j].comment.Pos.Offset })
	return out
}

func (p *projection) context(name string, bc *syntax.BoundedContext) vocab.BoundedContext {
	c := vocab.BoundedContext{Name: name, Definition: definition(bc.DomainVisionStatement, bc.Comment), Line: bc.Pos.Line}
	p.requireDefinition(bc.Pos, "context", name, c.Definition)
	p.names[name] = map[string]string{}
	var aggregates []*syntax.Aggregate
	aggregates = append(aggregates, bc.Aggregates...)
	for _, m := range bc.Modules {
		aggregates = append(aggregates, m.Aggregates...)
	}
	ordered(aggregates, func(a *syntax.Aggregate) int { return a.Pos.Offset })
	rootNames := map[string]string{}
	for _, a := range aggregates {
		for _, obj := range a.DomainObjects {
			if e, ok := obj.(*syntax.Entity); ok && e.AggregateRoot {
				rootNames[e.Name] = exactName(e.Name, &e.Node)
			}
		}
	}
	// Resolve explicit identity types through declarations in this context.
	var objects []syntax.SimpleDomainObject
	for _, a := range aggregates {
		objects = append(objects, a.DomainObjects...)
	}
	for _, m := range bc.Modules {
		objects = append(objects, m.DomainObjects...)
	}
	if bc.Application != nil {
		for _, e := range bc.Application.Events {
			objects = append(objects, e)
		}
	}
	ordered(objects, func(o syntax.SimpleDomainObject) int { return objectNode(o).Pos.Offset })
	for _, obj := range objects {
		v := reflect.ValueOf(obj).Elem()
		n := objectNode(obj)
		field := v.FieldByName("Name")
		if field.IsValid() {
			p.names[name][field.String()] = exactName(field.String(), n)
		}
	}
	for _, a := range aggregates {
		var roots []*syntax.Entity
		for _, obj := range a.DomainObjects {
			if e, ok := obj.(*syntax.Entity); ok && e.AggregateRoot {
				roots = append(roots, e)
			}
		}
		if len(roots) > 1 {
			p.problem(a.Pos, "Aggregate contains several aggregateRoot entities")
			continue
		}
		if len(roots) == 0 {
			continue
		}
		root := roots[0]
		rootName := exactName(root.Name, &root.Node)
		id := binding(name, rootName)
		p.roots[id] = root
		p.containers[id] = a
		aggregate := vocab.Aggregate{Name: rootName, Definition: definition(root.Doc, root.Comment), Identity: p.identity(name, root.Attributes, root.References), Aliases: hints(root.Hint, "alias"), Line: root.Pos.Line}
		p.requireDefinition(root.Pos, "aggregate", rootName, aggregate.Definition)
		if aggregate.Identity == "" {
			p.problem(root.Pos, fmt.Sprintf("aggregate %q has no key property with an identity type", rootName))
		}
		aggregate.Invariants, aggregate.Assertions = p.operations(root.Operations)
		if root.Repository != nil {
			aggregate.Repository = exactName(root.Repository.Name, &root.Repository.Node)
		} else {
			aggregate.Repository = hintOne(root.Hint, "repository")
		}
		for _, obj := range a.DomainObjects {
			e, ok := obj.(*syntax.Entity)
			if !ok || e == root {
				continue
			}
			entityName := exactName(e.Name, &e.Node)
			p.entities[binding(name, entityName)] = e
			member := vocab.Entity{Name: entityName, Definition: definition(e.Doc, e.Comment), Identity: p.identity(name, e.Attributes, e.References), Aliases: hints(e.Hint, "alias"), Line: e.Pos.Line}
			p.requireDefinition(e.Pos, "entity", entityName, member.Definition)
			aggregate.Entities = append(aggregate.Entities, member)
		}
		for _, s := range a.Services {
			if hintOne(s.Hint, "role") == factoryRole {
				if aggregate.Factory != "" {
					p.problem(s.Pos, "aggregate has several factory services")
				}
				aggregate.Factory = exactName(s.Name, &s.Node)
				p.factories[id] = s
			}
		}
		c.Aggregates = append(c.Aggregates, aggregate)
	}
	for _, obj := range objects {
		switch o := obj.(type) {
		case *syntax.ValueObject:
			n := exactName(o.Name, &o.Node)
			id := binding(name, n)
			p.values[id] = o
			def := definition(o.Doc, o.Comment)
			p.requireDefinition(o.Pos, "value object", n, def)
			spec := false
			for _, op := range o.Operations {
				if op.Name == satisfactionOperation || op.Name == "SatisfiedBy" || op.Name == "satisfied_by" || op.Name == "isSatisfiedBy" {
					spec = true
				}
			}
			if spec {
				c.Specifications = append(c.Specifications, vocab.Specification{Name: n, Definition: def, Line: o.Pos.Line})
			} else {
				inv, _ := p.operations(o.Operations)
				c.ValueObjects = append(c.ValueObjects, vocab.ValueObject{Name: n, Definition: def, Aliases: hints(o.Hint, "alias"), Invariants: inv, Line: o.Pos.Line})
			}
		case *syntax.DomainEvent:
			n := exactName(o.Name, &o.Node)
			p.events[binding(name, n)] = o
			def := definition(o.Doc, o.Comment)
			p.requireDefinition(o.Pos, "event", n, def)
			raised := rootNames[o.BelongsTo.Name]
			if o.BelongsTo.Name != "" && raised == "" {
				p.problem(o.BelongsTo.Pos, fmt.Sprintf("belongsTo %q names no root in context %q", o.BelongsTo.Name, name))
			}
			if o.BelongsTo.Name == "" {
				for _, a := range aggregates {
					for _, child := range a.DomainObjects {
						if child == o {
							for _, root := range a.DomainObjects {
								if e, ok := root.(*syntax.Entity); ok && e.AggregateRoot {
									raised = rootNames[e.Name]
								}
							}
						}
					}
				}
			}
			c.Events = append(c.Events, vocab.DomainEvent{Name: n, Definition: def, RaisedBy: raised, Line: o.Pos.Line})
		}
	}
	var services []*syntax.Service
	services = append(services, bc.DomainServices...)
	for _, m := range bc.Modules {
		services = append(services, m.Services...)
	}
	for _, a := range aggregates {
		for _, s := range a.Services {
			if hintOne(s.Hint, "role") != factoryRole {
				services = append(services, s)
			}
		}
	}
	ordered(services, func(s *syntax.Service) int { return s.Pos.Offset })
	for _, s := range services {
		n := exactName(s.Name, &s.Node)
		p.services[binding(name, n)] = s
		def := definition(s.Doc, s.Comment)
		p.requireDefinition(s.Pos, "service", n, def)
		c.Services = append(c.Services, vocab.DomainService{Name: n, Definition: def, Assertions: p.serviceAssertions(s.Operations), Line: s.Pos.Line})
	}
	for _, ins := range scopedDirectives(bc) {
		if ins.kind == questionDirective {
			c.Questions = append(c.Questions, vocab.Question{Key: ins.args[0], Text: ins.args[1], Line: ins.comment.Pos.Line})
		}
	}
	return c
}

func objectNode(obj syntax.SimpleDomainObject) *syntax.Node {
	return reflect.ValueOf(obj).Elem().FieldByName(nodeField).Addr().Interface().(*syntax.Node)
}

func (p *projection) requireDefinition(pos syntax.Pos, kind, name, def string) {
	if strings.TrimSpace(def) == "" {
		p.problem(pos, fmt.Sprintf("%s %q is missing a definition", kind, name))
	}
}

var builtinTypes = map[string]bool{"String": true, "int": true, "Integer": true, "long": true, "Long": true, "boolean": true, "Boolean": true, "Date": true, "DateTime": true, "Timestamp": true, "BigDecimal": true, "BigInteger": true, "double": true, "Double": true, "float": true, "Float": true, "Key": true, "PagingParameter": true, "PagedResult": true, "Blob": true, "Clob": true, "Object[]": true}

func (p *projection) identity(context string, attrs []*syntax.Attribute, refs []*syntax.Reference) string {
	type key struct {
		name      string
		n         *syntax.Node
		offset    int
		reference bool
	}
	var keys []key
	for _, a := range attrs {
		if a.Key {
			keys = append(keys, key{a.Type, &a.Node, a.Pos.Offset, strings.HasPrefix(a.Spellings["type"], "^")})
		}
	}
	for _, r := range refs {
		if r.Key {
			keys = append(keys, key{r.DomainObjectType.Name, &r.Node, r.Pos.Offset, true})
		}
	}
	sort.SliceStable(keys, func(i, j int) bool { return keys[i].offset < keys[j].offset })
	for _, k := range keys {
		if !identifier.MatchString(k.name) || (!k.reference && builtinTypes[k.name]) {
			continue
		}
		name := exactName(k.name, k.n)
		if name == k.name && p.names[context][k.name] != "" {
			name = p.names[context][k.name]
		}
		return name
	}
	return ""
}

func ownedOperation(name, doc, hint string, node *syntax.Node) (string, string, bool) {
	if doc == "" && !node.Present["doc"] {
		return "", "", false
	}
	for _, prefix := range []string{invariantPrefix, assertionPrefix} {
		if strings.HasPrefix(name, prefix) {
			key, ok := decodeKey(strings.TrimPrefix(name, prefix))
			if ok && (prefix == invariantPrefix || len(hints(hint, "on")) > 0) {
				return prefix, key, true
			}
		}
	}
	return "", "", false
}

func (p *projection) operations(ops []*syntax.DomainObjectOperation) ([]vocab.Invariant, []vocab.Assertion) {
	var inv []vocab.Invariant
	var assertions []vocab.Assertion
	for _, op := range ops {
		kind, key, ok := ownedOperation(op.Name, op.Doc, op.Hint, &op.Node)
		if !ok {
			continue
		}
		if kind == invariantPrefix {
			inv = append(inv, vocab.Invariant{Key: key, Statement: op.Doc, Line: op.Pos.Line})
		} else {
			assertions = append(assertions, vocab.Assertion{Key: key, On: hintOne(op.Hint, "on"), Statement: op.Doc, Line: op.Pos.Line})
		}
	}
	return inv, assertions
}

func (p *projection) serviceAssertions(ops []*syntax.ServiceOperation) []vocab.Assertion {
	var out []vocab.Assertion
	for _, op := range ops {
		kind, key, ok := ownedOperation(op.Name, op.Doc, op.Hint, &op.Node)
		if ok && kind == assertionPrefix {
			out = append(out, vocab.Assertion{Key: key, On: hintOne(op.Hint, "on"), Statement: op.Doc, Line: op.Pos.Line})
		}
	}
	return out
}

func relationshipValue(r syntax.Relationship) (string, string, vocab.RelationKind, *syntax.Node) {
	switch v := r.(type) {
	case *syntax.Partnership:
		return v.Participant1.Name, v.Participant2.Name, vocab.RelationPartnership, &v.Node
	case *syntax.SharedKernel:
		return v.Participant1.Name, v.Participant2.Name, vocab.RelationSharedKernel, &v.Node
	case *syntax.CustomerSupplierRelationship:
		return v.Upstream.Name, v.Downstream.Name, vocab.RelationCustomerSupplier, &v.Node
	case *syntax.UpstreamDownstreamRelationship:
		kind := vocab.RelationConformist
		has := func(role string) bool {
			for _, d := range v.DownstreamRoles {
				if string(d) == role {
					return true
				}
			}
			for _, u := range v.UpstreamRoles {
				if string(u) == role {
					return true
				}
			}
			return false
		}
		switch {
		case has("ACL"):
			kind = vocab.RelationAnticorruptionLayer
		case has("CF"):
			kind = vocab.RelationConformist
		case has("OHS"):
			kind = vocab.RelationOpenHostService
		case has("PL"):
			kind = vocab.RelationPublishedLanguage
		}
		return v.Upstream.Name, v.Downstream.Name, kind, &v.Node
	default:
		return "", "", "", &syntax.Node{}
	}
}

func (p *projection) collectValidation(lang vocab.UbiquitousLanguage) {
	// Each context is independently checked, so one missing definition does not
	// conceal problems in later contexts. Individual terms are checked in their
	// full context with other independent entries removed.
	for _, c := range lang.Contexts {
		validate := func(one vocab.BoundedContext, line int) {
			_, err := vocab.NewUbiquitousLanguage("validation", "", []vocab.BoundedContext{one}, nil)
			if err != nil {
				p.problem(syntax.Pos{Line: line, Column: 1}, err.Error())
			}
		}
		shell := vocab.BoundedContext{Name: c.Name, Definition: c.Definition, Line: c.Line}
		validate(shell, c.Line)
		for _, a := range c.Aggregates {
			one := shell
			one.Aggregates = []vocab.Aggregate{a}
			one.ValueObjects = c.ValueObjects
			one.Services = c.Services
			validate(one, a.Line)
		}
		for _, v := range c.ValueObjects {
			one := shell
			one.ValueObjects = []vocab.ValueObject{v}
			validate(one, v.Line)
		}
		for _, s := range c.Services {
			one := shell
			one.Services = []vocab.DomainService{s}
			validate(one, s.Line)
		}
		for _, s := range c.Specifications {
			one := shell
			one.Specifications = []vocab.Specification{s}
			validate(one, s.Line)
		}
		for _, q := range c.Questions {
			one := shell
			one.Questions = []vocab.Question{q}
			validate(one, q.Line)
		}
		for _, e := range c.Events {
			one := shell
			one.Events = []vocab.DomainEvent{e}
			one.Aggregates = c.Aggregates
			one.ValueObjects = c.ValueObjects
			validate(one, e.Line)
		}
		validate(c, c.Line)
	}
}

func (p *projection) readRelations(m *syntax.ContextMap, team map[string]bool, contextNames map[string]string) []vocab.ContextRelation {
	out := make([]vocab.ContextRelation, 0, len(m.Relationships))
	type positionedRelation struct {
		relation vocab.ContextRelation
		offset   int
		pos      syntax.Pos
	}
	var relations []positionedRelation
	for _, r := range m.Relationships {
		from, to, kind, n := relationshipValue(r)
		if team[from] || team[to] {
			continue
		}
		description := ""
		for _, c := range n.LeadingComments {
			ins, ok, err := readDirective(c)
			if ok && err == nil && ins.kind == "description" {
				description = ins.args[0]
			}
		}
		relation := vocab.ContextRelation{From: contextNames[from], To: contextNames[to], Kind: kind, Description: description, Line: n.Pos.Line}
		if relation.From == "" {
			relation.From = from
		}
		if relation.To == "" {
			relation.To = to
		}
		relations = append(relations, positionedRelation{relation, n.Pos.Offset, n.Pos})
	}
	for _, ins := range scopedDirectives(m) {
		if ins.kind != separateWaysDirective {
			continue
		}
		from, to := ins.args[0], ins.args[1]
		if team[from] || team[to] {
			continue
		}
		description := ""
		if len(ins.args) == 3 {
			description = ins.args[2]
		}
		if resolved := contextNames[from]; resolved != "" {
			from = resolved
		}
		if resolved := contextNames[to]; resolved != "" {
			to = resolved
		}
		relations = append(relations, positionedRelation{vocab.ContextRelation{From: from, To: to, Kind: vocab.RelationSeparateWays, Description: description, Line: ins.comment.Pos.Line}, ins.comment.Pos.Offset, ins.comment.Pos})
	}
	sort.SliceStable(relations, func(i, j int) bool { return relations[i].offset < relations[j].offset })
	pairs := map[[2]string]bool{}
	for _, item := range relations {
		r := item.relation
		if p.contexts[r.From] == nil || p.contexts[r.To] == nil {
			p.problem(item.pos, fmt.Sprintf("relationship names an unknown context: %q, %q", r.From, r.To))
		}
		if r.From == r.To {
			p.problem(item.pos, "a context cannot relate to itself")
		}
		pair := [2]string{r.From, r.To}
		if pair[0] > pair[1] {
			pair[0], pair[1] = pair[1], pair[0]
		}
		if pairs[pair] {
			p.problem(item.pos, "two relationships record the same context pair")
		}
		pairs[pair] = true
		out = append(out, r)
	}
	return out
}
