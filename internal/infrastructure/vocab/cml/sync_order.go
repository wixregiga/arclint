package cmlvocab

import (
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	"github.com/wixregiga/arclint/internal/infrastructure/vocab/cml/syntax"
)

// reorderDeclarations fills the existing modeled slots in document order.
// Foreign declarations retain their slots; a moved declaration carries its Node,
// including leading comments, and takes only the destination's ordering position.
func reorderDeclarations(owner any, names []string, nameOf func(any) string, fresh map[*syntax.Node]bool) {
	type slot struct {
		value    reflect.Value
		node     *syntax.Node
		position syntax.Pos
		name     string
		added    bool
		placed   bool
	}
	// A moved declaration may have joined a different grammar slice. Restore
	// that slice's document order before assigning modeled slots.
	walkSyntax(owner, func(v reflect.Value, _ string) {
		if v.Kind() != reflect.Slice || v.Len() < 2 || syntaxNode(v.Index(0).Interface()) == nil {
			return
		}
		kind := reflect.TypeOf(v.Index(0).Interface())
		if kind == reflect.TypeOf((*syntax.DomainObjectOperation)(nil)) || kind == reflect.TypeOf((*syntax.ServiceOperation)(nil)) {
			return
		}
		sort.SliceStable(v.Interface(), func(i, j int) bool {
			a, b := syntaxNode(v.Index(i).Interface()), syntaxNode(v.Index(j).Interface())
			if (a.Pos.Line == 0) != (b.Pos.Line == 0) {
				return b.Pos.Line == 0
			}
			return a.Pos.Offset < b.Pos.Offset
		})
	})
	wanted := map[string]bool{}
	for _, name := range names {
		wanted[name] = true
	}
	placeFreshBesideNeighbors(owner, names, wanted, nameOf, fresh)
	var slots []slot
	declarations := map[string]reflect.Value{}
	walkSyntax(owner, func(v reflect.Value, _ string) {
		if v.Kind() != reflect.Slice {
			return
		}
		for i := 0; i < v.Len(); i++ {
			item := v.Index(i)
			if !item.CanSet() || !item.CanInterface() {
				continue
			}
			name := nameOf(item.Interface())
			if !wanted[name] {
				continue
			}
			node := syntaxNode(item.Interface())
			if node == nil {
				continue
			}
			slots = append(slots, slot{item, node, node.Pos, name, fresh[node], false})
			copyValue := reflect.New(item.Type()).Elem()
			copyValue.Set(item)
			declarations[name] = copyValue
		}
	})
	// A declaration the file does not hold yet takes the slot beside the
	// declaration that precedes it in the requested order (or, when it leads,
	// the one that follows), so inserting one shifts no other declaration out of
	// its slot. Without that, every later declaration of the kind would move to
	// the next slot, and past any other kind written between them.
	byName := map[string]*slot{}
	for i := range slots {
		byName[slots[i].name] = &slots[i]
	}
	var anchor *slot
	distance := 0
	for _, name := range names {
		s := byName[name]
		switch {
		case s == nil:
		case !s.added:
			anchor, distance = s, 0
		case anchor != nil:
			distance++
			s.position = syntax.Pos{Offset: anchor.position.Offset + distance, Line: max(anchor.position.Line, 1), Column: anchor.position.Column}
			s.placed = true
		}
	}
	var following *slot
	distance = 0
	for i := len(names) - 1; i >= 0; i-- {
		s := byName[names[i]]
		switch {
		case s == nil:
		case !s.added || s.placed:
			following, distance = s, 0
		case following != nil:
			distance++
			s.position = syntax.Pos{Offset: following.position.Offset - distance, Line: max(following.position.Line, 1), Column: following.position.Column}
		}
	}
	sort.SliceStable(slots, func(i, j int) bool {
		if (slots[i].position.Line == 0) != (slots[j].position.Line == 0) {
			return slots[j].position.Line == 0
		}
		return slots[i].position.Offset < slots[j].position.Offset
	})
	for i, name := range names {
		if i >= len(slots) || !declarations[name].IsValid() {
			continue
		}
		slots[i].value.Set(declarations[name])
		// New slots come after authored slots. Their zero provenance is kept so
		// the printer follows its grammar order for new declaration groups.
		syntaxNode(declarations[name].Interface()).Pos = slots[i].position
	}
}

// placeFreshBesideNeighbors writes each declaration the file does not hold yet
// next to the declaration that precedes it in the requested order (or, when it
// leads, the one that follows), in the same list of the grammar. A new
// declaration is appended to the end of its container, and filling the
// existing slots in order from there would carry every later declaration of
// the kind one slot on, past any other kind written between them.
func placeFreshBesideNeighbors(owner any, names []string, wanted map[string]bool, nameOf func(any) string, fresh map[*syntax.Node]bool) {
	type entry struct {
		list  reflect.Value
		index int
		added bool
	}
	found := map[string]entry{}
	lists := map[uintptr]reflect.Value{}
	walkSyntax(owner, func(v reflect.Value, _ string) {
		if v.Kind() != reflect.Slice || !v.CanAddr() {
			return
		}
		for i := 0; i < v.Len(); i++ {
			item := v.Index(i)
			if !item.CanInterface() {
				continue
			}
			name := nameOf(item.Interface())
			node := syntaxNode(item.Interface())
			if wanted[name] && node != nil {
				found[name] = entry{v, i, fresh[node]}
				lists[v.Addr().Pointer()] = v
			}
		}
	})
	listOf := func(e entry) uintptr { return e.list.Addr().Pointer() }
	after := map[uintptr]map[int][]int{}
	before := map[uintptr]map[int][]int{}
	moving := map[uintptr]map[int]bool{}
	note := func(into map[uintptr]map[int][]int, e entry, at int) {
		key := listOf(e)
		if into[key] == nil {
			into[key] = map[int][]int{}
		}
		into[key][at] = append(into[key][at], e.index)
		if moving[key] == nil {
			moving[key] = map[int]bool{}
		}
		moving[key][e.index] = true
	}
	var anchor *entry
	anchored := map[string]bool{}
	for _, name := range names {
		e, ok := found[name]
		switch {
		case !ok:
		case !e.added:
			anchor = &e
		case anchor != nil:
			anchored[name] = true
			if listOf(*anchor) == listOf(e) {
				note(after, e, anchor.index)
			}
		}
	}
	var following *entry
	for i := len(names) - 1; i >= 0; i-- {
		e, ok := found[names[i]]
		switch {
		case !ok:
		case !e.added:
			following = &e
		case !anchored[names[i]] && following != nil && listOf(*following) == listOf(e):
			note(before, e, following.index)
		}
	}
	for key, list := range lists {
		if len(moving[key]) == 0 {
			continue
		}
		var order []int
		for i := 0; i < list.Len(); i++ {
			if moving[key][i] {
				continue
			}
			// The backward pass met the leading declarations last-first.
			leading := before[key][i]
			slices.Reverse(leading)
			order = append(order, leading...)
			order = append(order, i)
			order = append(order, after[key][i]...)
		}
		rebuilt := reflect.MakeSlice(list.Type(), list.Len(), list.Len())
		for to, from := range order {
			rebuilt.Index(to).Set(list.Index(from))
		}
		list.Set(rebuilt)
	}
}

// freshNodes finds the declarations this sync created: those without a source
// position. They gain one when the context is reprinted to refresh offsets, so
// they must be told apart before the first reordering.
func freshNodes(owner any) map[*syntax.Node]bool {
	out := map[*syntax.Node]bool{}
	walkSyntax(owner, func(v reflect.Value, _ string) {
		if v.Kind() != reflect.Slice {
			return
		}
		for i := 0; i < v.Len(); i++ {
			if item := v.Index(i); item.CanInterface() {
				if n := syntaxNode(item.Interface()); n != nil && n.Pos.Line == 0 {
					out[n] = true
				}
			}
		}
	})
	return out
}

func syntaxNode(value any) *syntax.Node {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return nil
	}
	field := v.Elem().FieldByName(nodeField)
	if !field.IsValid() {
		return nil
	}
	return field.Addr().Interface().(*syntax.Node)
}

// walkSyntax visits exported grammar fields, excluding parser provenance.
func walkSyntax(owner any, fn func(reflect.Value, string)) {
	var walk func(reflect.Value, string)
	walk = func(v reflect.Value, kind string) {
		if !v.IsValid() {
			return
		}
		if v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
			if !v.IsNil() {
				walk(v.Elem(), kind)
			}
			return
		}
		fn(v, kind)
		switch v.Kind() {
		default:
			return
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() && v.Type().Field(i).Name != nodeField {
					walk(v.Field(i), v.Type().Name())
				}
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i), kind)
			}
		}
	}
	walk(reflect.ValueOf(owner), "")
}

func syncContextOrder(bc *syntax.BoundedContext, want vocab.BoundedContext) error {
	placeContextServices(bc)
	fresh := freshNodes(bc)
	names := make([]string, 0, len(want.Aggregates))
	for _, a := range want.Aggregates {
		names = append(names, a.Name)
	}
	reorderDeclarations(bc, names, func(value any) string {
		if a, ok := value.(*syntax.Aggregate); ok {
			for _, obj := range a.DomainObjects {
				if e, ok := obj.(*syntax.Entity); ok && e.AggregateRoot {
					return exactName(e.Name, &e.Node)
				}
			}
		}
		return ""
	}, fresh)
	if err := refreshContextOffsets(bc); err != nil {
		return err
	}
	for _, a := range allAggregates(bc) {
		var rootName string
		for _, obj := range a.DomainObjects {
			if e, ok := obj.(*syntax.Entity); ok && e.AggregateRoot {
				rootName = exactName(e.Name, &e.Node)
			}
		}
		aggregate, ok := want.Aggregate(rootName)
		if !ok {
			continue
		}
		names = make([]string, 0, len(aggregate.Entities))
		for _, e := range aggregate.Entities {
			names = append(names, e.Name)
		}
		reorderDeclarations(a, names, func(value any) string {
			if e, ok := value.(*syntax.Entity); ok && !e.AggregateRoot {
				return exactName(e.Name, &e.Node)
			}
			return ""
		}, fresh)
	}
	if err := refreshContextOffsets(bc); err != nil {
		return err
	}
	names = make([]string, 0, len(want.ValueObjects))
	for _, v := range want.ValueObjects {
		names = append(names, v.Name)
	}
	reorderDeclarations(bc, names, valueDeclarationName, fresh)
	if err := refreshContextOffsets(bc); err != nil {
		return err
	}
	names = make([]string, 0, len(want.Specifications))
	for _, s := range want.Specifications {
		names = append(names, s.Name)
	}
	reorderDeclarations(bc, names, valueDeclarationName, fresh)
	if err := refreshContextOffsets(bc); err != nil {
		return err
	}
	names = make([]string, 0, len(want.Events))
	for _, e := range want.Events {
		names = append(names, e.Name)
	}
	rootHandles := map[string]string{}
	for _, a := range allAggregates(bc) {
		for _, obj := range a.DomainObjects {
			if e, ok := obj.(*syntax.Entity); ok && e.AggregateRoot {
				rootHandles[exactName(e.Name, &e.Node)] = e.Name
			}
		}
	}
	var current []*syntax.DomainEvent
	for _, d := range syntaxDeclarations(&syntax.File{BoundedContexts: []*syntax.BoundedContext{bc}}) {
		if d.value.Type() == reflect.TypeOf(syntax.DomainEvent{}) {
			current = append(current, d.value.Addr().Interface().(*syntax.DomainEvent))
		}
	}
	sort.SliceStable(current, func(i, j int) bool { return current[i].Pos.Offset < current[j].Pos.Offset })
	// Only a change in the order of the events the file already holds needs the
	// ownership repair below; an event added or removed leaves the others where
	// they are.
	var kept []string
	for _, event := range current {
		if !fresh[&event.Node] {
			kept = append(kept, exactName(event.Name, &event.Node))
		}
	}
	var requested []string
	for _, name := range names {
		if slices.Contains(kept, name) {
			requested = append(requested, name)
		}
	}
	changeOrder := !slices.Equal(kept, requested)
	if changeOrder {
		// Keep ownership explicit when permutations cross an implicit aggregate
		// boundary. An event without RaisedBy must stay outside every Aggregate.
		for _, event := range current {
			if _, modeled := want.Event(exactName(event.Name, &event.Node)); !modeled {
				continue
			}
			wanted, _ := want.Event(exactName(event.Name, &event.Node))
			event.BelongsTo.Name = rootHandles[wanted.RaisedBy]
			removeDeclaration(bc, event)
			m := contextModule(bc)
			m.DomainObjects = append(m.DomainObjects, event)
		}
		if err := refreshContextOffsets(bc); err != nil {
			return err
		}
	}
	// Explicit ownership makes event order independent of its enclosing container.
	// Events with no RaisedBy cannot be swapped into an Aggregate: move those to
	// the fresh-write Module before ordering them.
	reorderDeclarations(bc, names, func(value any) string {
		if e, ok := value.(*syntax.DomainEvent); ok {
			return exactName(e.Name, &e.Node)
		}
		return ""
	}, fresh)
	names = make([]string, 0, len(want.Services))
	for _, s := range want.Services {
		names = append(names, s.Name)
	}
	reorderDeclarations(bc, names, func(value any) string {
		if s, ok := value.(*syntax.Service); ok && hintOne(s.Hint, "role") != factoryRole {
			return exactName(s.Name, &s.Node)
		}
		return ""
	}, fresh)
	return nil
}

// Xtext's selected Module and Aggregate loops consume a following doc STRING
// before checking its declaration keyword. Keep the entire direct services
// block before those loops whenever it contains a documented service. Moving
// ordering offsets carries each service's authored body and comments unchanged.
func placeContextServices(bc *syntax.BoundedContext) {
	documented := false
	for _, service := range bc.DomainServices {
		documented = documented || service.Doc != "" || service.Present["doc"]
	}
	if !documented {
		return
	}
	first := bc.Pos.Offset + 1
	found := false
	for _, module := range bc.Modules {
		if module.Pos.Line > 0 && (!found || module.Pos.Offset < first) {
			first, found = module.Pos.Offset, true
		}
	}
	for _, aggregate := range bc.Aggregates {
		if aggregate.Pos.Line > 0 && (!found || aggregate.Pos.Offset < first) {
			first, found = aggregate.Pos.Offset, true
		}
	}
	move := !found
	for _, service := range bc.DomainServices {
		move = move || service.Pos.Line == 0 || service.Pos.Offset >= first
	}
	if !move {
		return
	}
	for i, service := range bc.DomainServices {
		service.Pos.Offset = first - len(bc.DomainServices) + i
		if service.Pos.Line == 0 {
			service.Pos.Line = 1
		}
	}
}

func valueDeclarationName(value any) string {
	if v, ok := value.(*syntax.ValueObject); ok {
		return exactName(v.Name, &v.Node)
	}
	return ""
}

func orderOperations[T any](existing []T, desired []T, nameOf func(T) string, owned func(T) bool) []T {
	var names []string
	for _, op := range desired {
		if owned(op) {
			names = append(names, nameOf(op))
		}
	}
	byName := map[string]T{}
	for _, op := range existing {
		if owned(op) {
			byName[nameOf(op)] = op
		}
	}
	i := 0
	for j, op := range existing {
		if owned(op) && i < len(names) {
			existing[j] = byName[names[i]]
			i++
		}
	}
	return existing
}

func contextModule(bc *syntax.BoundedContext) *syntax.SculptorModule {
	for _, m := range bc.Modules {
		if m.Name == bc.Name {
			return m
		}
	}
	m := &syntax.SculptorModule{Name: bc.Name}
	bc.Modules = append(bc.Modules, m)
	return m
}

// Offset order is local to an owner. Moving an Aggregate changes its children's
// document positions too. Use the existing grammar printer as the ordering
// oracle, while keeping the bound nodes and their authored source lines intact.
func refreshContextOffsets(bc *syntax.BoundedContext) error {
	data, err := syntax.FormatChecked(&syntax.File{BoundedContexts: []*syntax.BoundedContext{bc}})
	if err != nil {
		return fmt.Errorf("order context %q: %w", bc.Name, err)
	}
	file, err := syntax.Parse(data)
	if err != nil {
		return fmt.Errorf("read reordered context %q: %w", bc.Name, err)
	}
	original := syntaxDeclarations(&syntax.File{BoundedContexts: []*syntax.BoundedContext{bc}})
	formatted := syntaxDeclarations(file)
	if len(original) != len(formatted) {
		return fmt.Errorf("order context %q: formatting changed the number of bound declarations", bc.Name)
	}
	base := bc.Pos.Offset
	for i, declaration := range original {
		declaration.node.Pos.Offset = base + formatted[i].node.Pos.Offset
		if declaration.node.Pos.Line == 0 {
			declaration.node.Pos.Line = 1
		}
	}
	return nil
}

func fillModeledSlots[T any](existing, modeled []T, foreign func(T) bool) []T {
	out := make([]T, 0, len(existing)+len(modeled))
	next := 0
	for _, item := range existing {
		if foreign(item) {
			out = append(out, item)
			continue
		}
		if next < len(modeled) {
			out = append(out, modeled[next])
			next++
		}
	}
	return append(out, modeled[next:]...)
}

func syncQuestions(bc *syntax.BoundedContext, want vocab.BoundedContext) {
	questions := want.Questions
	terms := map[string]bool{}
	for _, term := range want.Terms() {
		terms[term.Name] = true
	}
	declarations := map[*syntax.Node]syntaxDeclaration{}
	for _, declaration := range syntaxDeclarations(&syntax.File{BoundedContexts: []*syntax.BoundedContext{bc}}) {
		declarations[declaration.node] = declaration
	}
	leading := map[string][]syntax.Comment{}
	type authoredQuestion struct {
		key    string
		offset int
	}
	var authored []authoredQuestion
	strip := func(comments []syntax.Comment) []syntax.Comment {
		var kept, pending []syntax.Comment
		for _, comment := range comments {
			instruction, owned, err := readDirective(comment)
			if err == nil && owned && instruction.kind == questionDirective {
				leading[instruction.args[0]] = append([]syntax.Comment(nil), pending...)
				authored = append(authored, authoredQuestion{instruction.args[0], comment.Pos.Offset})
				pending = nil
				continue
			}
			if owned {
				kept = append(kept, pending...)
				pending = nil
				kept = append(kept, comment)
				continue
			}
			pending = append(pending, comment)
		}
		return append(kept, pending...)
	}
	visitNodes(bc, func(n *syntax.Node) {
		if n != &bc.Node {
			before := append([]syntax.Comment(nil), n.LeadingComments...)
			n.LeadingComments = strip(n.LeadingComments)
			if !reflect.DeepEqual(before, n.LeadingComments) {
				syncQuestionComment(declarations[n], terms)
			}
		}
		n.TrailingComments = strip(n.TrailingComments)
	})
	var ordered []syntax.Comment
	retained := map[string]bool{}
	for _, question := range questions {
		retained[question.Key] = true
		ordered = append(ordered, leading[question.Key]...)
		ordered = append(ordered, directive(questionDirective, question.Key, question.Text))
	}
	sort.SliceStable(authored, func(i, j int) bool { return authored[i].offset < authored[j].offset })
	for _, question := range authored {
		if !retained[question.key] {
			ordered = append(ordered, leading[question.key]...)
		}
	}
	bc.TrailingComments = append(ordered, bc.TrailingComments...)
}

// Comment is a parser-derived view of the leading comment run. Keep it aligned
// when directives move, so an empty run cannot resurrect them through the
// printer's comment fallback. A modeled fallback definition becomes an explicit
// doc when its old comment run must move; the requested definition stays exact.
func syncQuestionComment(declaration syntaxDeclaration, terms map[string]bool) {
	comment := declaration.value.FieldByName("Comment")
	if !comment.IsValid() {
		return
	}
	doc := declaration.value.FieldByName("Doc")
	var oldDefinition string
	name := declaration.value.FieldByName("Name")
	modeled := name.IsValid() && terms[exactName(name.String(), declaration.node)] && doc.IsValid()
	if modeled {
		oldDefinition = definition(doc.String(), comment.String())
	}
	texts := make([]string, 0, len(declaration.node.LeadingComments))
	for _, c := range declaration.node.LeadingComments {
		texts = append(texts, c.Text)
	}
	comment.SetString(strings.Join(texts, "\n"))
	if modeled && doc.String() == "" && oldDefinition != definition("", comment.String()) {
		doc.SetString(oldDefinition)
		delete(declaration.node.Present, "doc")
	}
}
