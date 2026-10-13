package cmlvocab

// The edit-locality oracle shared by the edit tests. A recording that Record
// wrote (or that the syntax formatter printed) is canonical, so every
// declaration owns a contiguous run of lines that its AST positions and
// braces delimit. layoutOf paints each line with the element that owns it;
// the languages before and after an edit say which elements differ; and a
// line diff over (owner, text) pairs says which lines the edit touched. An
// edit is local when every touched line belongs to an element that differs.
//
// Keying the diff by owner removes the alignment ambiguity of a plain line
// diff (many lines are just a closing brace): a line can only be matched with
// a line of the same element, so a rewritten line of an unrelated element
// always shows as touched.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	"github.com/wixregiga/arclint/internal/infrastructure/vocab/cml/syntax"
)

const (
	keyProject = "project"
	keyMap     = "map"
	keyFrame   = "frame:"
)

// recordingLayout is one canonical recording with the element that owns each
// of its lines.
type recordingLayout struct {
	data      []byte
	file      *syntax.File
	lang      vocab.UbiquitousLanguage
	lines     []string
	owners    []string
	hasMap    bool
	hasDomain bool
}

func indentOf(line string) int { return len(line) - len(strings.TrimLeft(line, " \t")) }

// nodeExtent returns the first line of a declaration (its leading comments
// and doc string included), the line that names it, and its last line: the
// matching closing brace when the declaration opens a body, else the naming
// line. Canonical layout puts the closing brace at the declaration's indent.
func nodeExtent(lines []string, n *syntax.Node) (first, header, last int) {
	first, header = n.Pos.Line, n.NamePos.Line
	if header == 0 {
		header = first
	}
	for _, c := range n.LeadingComments {
		if c.Pos.Line > 0 && c.Pos.Line < first {
			first = c.Pos.Line
		}
	}
	last = header
	if strings.HasSuffix(strings.TrimRight(lines[header-1], " \t\r"), "{") {
		indent := indentOf(lines[header-1])
		for j := header; j < len(lines); j++ {
			if indentOf(lines[j]) == indent && strings.TrimSpace(lines[j]) == "}" {
				last = j + 1
				break
			}
		}
	}
	return first, header, last
}

func layoutOf(t testing.TB, data []byte) recordingLayout {
	t.Helper()
	rec, err := vocab.NewDomainRecording("domain.arclint.cml")
	if err != nil {
		t.Fatal(err)
	}
	file, err := syntax.ParseFile(rec.Path(), data)
	if err != nil {
		t.Fatalf("layout: %v", err)
	}
	lang, p, err := project(file, rec)
	if err != nil {
		t.Fatalf("layout: %v", err)
	}
	lines := strings.Split(string(data), "\n")
	layout := recordingLayout{data: data, file: file, lang: lang, lines: lines, owners: make([]string, len(lines)), hasMap: file.Map != nil, hasDomain: len(file.Domains) > 0}
	paint := func(first, last int, key string) {
		for i := first; i <= last && i <= len(lines); i++ {
			layout.owners[i-1] = key
		}
	}
	block := func(n *syntax.Node, key string, framed bool) {
		first, header, last := nodeExtent(lines, n)
		paint(first, last, key)
		if framed {
			layout.owners[header-1] = keyFrame + key
			layout.owners[last-1] = keyFrame + key
		}
	}
	for _, c := range file.LeadingComments {
		paint(c.Pos.Line, c.Pos.Line, keyProject)
	}
	if len(file.Domains) > 0 {
		block(&file.Domains[0].Node, keyProject, false)
	}
	handles := map[string]string{}
	for name, bc := range p.contexts {
		handles[bc.Name] = name
	}
	if file.Map != nil {
		block(&file.Map.Node, keyMap, false)
	}
	for name, bc := range p.contexts {
		block(&bc.Node, "context:"+name, false)
		for _, m := range bc.Modules {
			block(&m.Node, "module:"+name, true)
		}
	}
	for _, c := range lang.Contexts {
		for _, a := range c.Aggregates {
			key := "aggregate:" + c.Name + "/" + a.Name
			id := binding(c.Name, a.Name)
			block(&p.containers[id].Node, key, false)
			block(&p.roots[id].Node, key, false)
			if factory := p.factories[id]; factory != nil {
				block(&factory.Node, "factory:"+c.Name+"/"+a.Name, false)
			}
		}
	}
	for _, c := range lang.Contexts {
		for _, a := range c.Aggregates {
			for _, e := range a.Entities {
				block(&p.entities[binding(c.Name, e.Name)].Node, "entity:"+c.Name+"/"+e.Name, false)
			}
		}
		for _, v := range c.ValueObjects {
			block(&p.values[binding(c.Name, v.Name)].Node, "value:"+c.Name+"/"+v.Name, true)
		}
		for _, s := range c.Specifications {
			block(&p.values[binding(c.Name, s.Name)].Node, "spec:"+c.Name+"/"+s.Name, true)
		}
		for _, e := range c.Events {
			block(&p.events[binding(c.Name, e.Name)].Node, "event:"+c.Name+"/"+e.Name, false)
		}
		for _, s := range c.Services {
			block(&p.services[binding(c.Name, s.Name)].Node, "service:"+c.Name+"/"+s.Name, true)
		}
	}
	for _, c := range lang.Contexts {
		for _, a := range c.Aggregates {
			owner := c.Name + "/" + a.Name
			for _, op := range p.roots[binding(c.Name, a.Name)].Operations {
				layout.paintOperation(lines, &op.Node, owner, op.Name, op.Doc, op.Hint, true)
			}
		}
		for _, v := range append([]string(nil), valueNames(c)...) {
			owner := c.Name + "/" + v
			for _, op := range p.values[binding(c.Name, v)].Operations {
				layout.paintOperation(lines, &op.Node, owner, op.Name, op.Doc, op.Hint, false)
			}
		}
		for _, s := range c.Services {
			owner := c.Name + "/" + s.Name
			for _, op := range p.services[binding(c.Name, s.Name)].Operations {
				layout.paintOperation(lines, &op.Node, owner, op.Name, op.Doc, op.Hint, true)
			}
		}
		for _, q := range c.Questions {
			paint(q.Line, q.Line, "question:"+c.Name+"/"+q.Key)
		}
	}
	if file.Map != nil {
		for _, r := range file.Map.Relationships {
			from, to, _, n := relationshipValue(r)
			block(n, relationKey(handles[from], handles[to]), false)
		}
	}
	for _, r := range lang.Relations {
		if r.Kind == vocab.RelationSeparateWays {
			paint(r.Line, r.Line, relationKey(r.From, r.To))
		}
	}
	return layout
}

func valueNames(c vocab.BoundedContext) []string {
	var names []string
	for _, v := range c.ValueObjects {
		names = append(names, v.Name)
	}
	for _, s := range c.Specifications {
		names = append(names, s.Name)
	}
	return names
}

func relationKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return "relation:" + a + "/" + b
}

// paintOperation gives an invariant or assertion carrier to its element. Any
// other operation of an aggregate root or a service stays its own element so
// that only an assertion naming it can excuse a change; the operations of a
// value object stay with the value object.
func (l *recordingLayout) paintOperation(lines []string, n *syntax.Node, owner, name, doc, hint string, own bool) {
	kind, key, ok := ownedOperation(name, doc, hint, n)
	var element string
	switch {
	case ok && kind == invariantPrefix:
		element = "invariant:" + owner + "/" + key
	case ok:
		element = "assertion:" + owner + "/" + key
	case own:
		element = "operation:" + owner + "/" + name
	default:
		return
	}
	first, _, last := nodeExtent(lines, n)
	for i := first; i <= last; i++ {
		l.owners[i-1] = element
	}
}

// keyedLine is one non-blank line with the element that owns it. Blank lines
// only separate declarations, and the formatter alone decides where they go.
type keyedLine struct {
	number int
	key    string
	text   string
}

func (l recordingLayout) keyed() []keyedLine {
	var out []keyedLine
	for i, line := range l.lines {
		if strings.TrimSpace(line) != "" {
			out = append(out, keyedLine{number: i + 1, key: l.owners[i], text: line})
		}
	}
	return out
}

// lcsDiff returns the lines of a that no longest common subsequence keeps and
// the lines of b that it does not contain.
func lcsDiff(a, b []keyedLine) (deleted, inserted []keyedLine) {
	same := func(x, y keyedLine) bool { return x.key == y.key && x.text == y.text }
	n, m := len(a), len(b)
	table := make([][]int32, n+1)
	for i := range table {
		table[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			switch {
			case same(a[i], b[j]):
				table[i][j] = table[i+1][j+1] + 1
			case table[i+1][j] >= table[i][j+1]:
				table[i][j] = table[i+1][j]
			default:
				table[i][j] = table[i][j+1]
			}
		}
	}
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case same(a[i], b[j]):
			i++
			j++
		case table[i+1][j] >= table[i][j+1]:
			deleted = append(deleted, a[i])
			i++
		default:
			inserted = append(inserted, b[j])
			j++
		}
	}
	deleted = append(deleted, a[i:]...)
	inserted = append(inserted, b[j:]...)
	return deleted, inserted
}

// elementModel is a language reduced to its elements: each element's key, its
// own value (nested elements excluded), and the order of every section.
type elementModel struct {
	values  map[string]string
	orders  map[string][]string
	on      map[string]string
	parents map[string]string
}

func modelOf(l vocab.UbiquitousLanguage) elementModel {
	m := elementModel{values: map[string]string{}, orders: map[string][]string{}, on: map[string]string{}, parents: map[string]string{}}
	add := func(section, key, value, parent string) {
		m.values[key] = value
		m.orders[section] = append(m.orders[section], key)
		m.parents[key] = parent
	}
	m.values[keyProject] = fmt.Sprintf("%q %q", l.Project, l.Description)
	for _, c := range l.Contexts {
		context := "context:" + c.Name
		add("contexts", context, fmt.Sprintf("%q", c.Definition), "")
		section := func(kind string) string { return c.Name + "|" + kind }
		for _, a := range c.Aggregates {
			owner := c.Name + "/" + a.Name
			aggregate := "aggregate:" + owner
			add(section("aggregates"), aggregate, fmt.Sprintf("%q %q %q %q", a.Definition, a.Identity, a.Aliases, a.Repository), context)
			if a.Factory != "" {
				add(section(a.Name+"|factory"), "factory:"+owner, fmt.Sprintf("%q", a.Factory), aggregate)
			}
			for _, e := range a.Entities {
				add(section(a.Name+"|entities"), "entity:"+c.Name+"/"+e.Name, fmt.Sprintf("%q %q %q %q", a.Name, e.Definition, e.Identity, e.Aliases), aggregate)
			}
			for _, inv := range a.Invariants {
				add(section(a.Name+"|invariants"), "invariant:"+owner+"/"+inv.Key, fmt.Sprintf("%q", inv.Statement), aggregate)
			}
			for _, as := range a.Assertions {
				key := "assertion:" + owner + "/" + as.Key
				add(section(a.Name+"|assertions"), key, fmt.Sprintf("%q %q", as.On, as.Statement), aggregate)
				m.on[key] = as.On
			}
		}
		for _, v := range c.ValueObjects {
			owner := c.Name + "/" + v.Name
			value := "value:" + owner
			add(section("values"), value, fmt.Sprintf("%q %q", v.Definition, v.Aliases), context)
			for _, inv := range v.Invariants {
				add(section(v.Name+"|invariants"), "invariant:"+owner+"/"+inv.Key, fmt.Sprintf("%q", inv.Statement), value)
			}
		}
		for _, s := range c.Specifications {
			add(section("specifications"), "spec:"+c.Name+"/"+s.Name, fmt.Sprintf("%q", s.Definition), context)
		}
		for _, e := range c.Events {
			add(section("events"), "event:"+c.Name+"/"+e.Name, fmt.Sprintf("%q %q", e.Definition, e.RaisedBy), context)
		}
		for _, s := range c.Services {
			owner := c.Name + "/" + s.Name
			service := "service:" + owner
			add(section("services"), service, fmt.Sprintf("%q", s.Definition), context)
			for _, as := range s.Assertions {
				key := "assertion:" + owner + "/" + as.Key
				add(section(s.Name+"|assertions"), key, fmt.Sprintf("%q %q", as.On, as.Statement), service)
				m.on[key] = as.On
			}
		}
		for _, q := range c.Questions {
			add(section("questions"), "question:"+c.Name+"/"+q.Key, fmt.Sprintf("%q", q.Text), context)
		}
	}
	for _, r := range l.Relations {
		add("relations", relationKey(r.From, r.To), fmt.Sprintf("%q %q %q %q", r.From, r.To, r.Kind, r.Description), "")
	}
	return m
}

// editScope says which elements an edit between two languages may touch.
type editScope struct {
	differs    map[string]bool
	operations map[string]bool
	mapEdited  bool
	models     [2]elementModel
}

// scopeOf derives the touched elements from the two languages alone: an
// element is touched when it exists in one language only, when its own value
// differs, or when its section holds the same elements in another order.
func scopeOf(before, after recordingLayout) editScope {
	a, b := modelOf(before.lang), modelOf(after.lang)
	scope := editScope{differs: map[string]bool{}, operations: map[string]bool{}, models: [2]elementModel{a, b}}
	for key, value := range a.values {
		if other, ok := b.values[key]; !ok || other != value {
			scope.differs[key] = true
		}
	}
	for key := range b.values {
		if _, ok := a.values[key]; !ok {
			scope.differs[key] = true
		}
	}
	moved := map[string]bool{}
	sections := map[string]bool{}
	for section := range a.orders {
		sections[section] = true
	}
	for section := range b.orders {
		sections[section] = true
	}
	for section := range sections {
		common := func(order, other []string) []string {
			present := map[string]bool{}
			for _, key := range other {
				present[key] = true
			}
			var out []string
			for _, key := range order {
				if present[key] {
					out = append(out, key)
				}
			}
			return out
		}
		x, y := common(a.orders[section], b.orders[section]), common(b.orders[section], a.orders[section])
		if strings.Join(x, "\x00") != strings.Join(y, "\x00") {
			for _, key := range x {
				scope.differs[key] = true
				moved[key] = true
			}
		}
	}
	// An element that moves takes everything nested in it along.
	for _, m := range []elementModel{a, b} {
		for key := range m.values {
			for parent := m.parents[key]; parent != ""; parent = m.parents[parent] {
				if moved[parent] {
					scope.differs[key] = true
					break
				}
			}
		}
	}
	for _, m := range []elementModel{a, b} {
		for key, on := range m.on {
			if scope.differs[key] {
				owner := strings.TrimPrefix(key, "assertion:")
				scope.operations[owner[:strings.LastIndex(owner, "/")]+"/"+on] = true
			}
		}
	}
	scope.mapEdited = strings.Join(a.orders["contexts"], "\x00") != strings.Join(b.orders["contexts"], "\x00") ||
		before.hasMap != after.hasMap || (!before.hasDomain && scope.differs[keyProject])
	return scope
}

// withServiceBlock widens the scope the way the adapter documents: it keeps a
// context's documented direct services in one block ahead of its modules and
// aggregates, so adding or changing a service of a hand-written context may
// carry the context's other modeled services into that block.
func (s editScope) withServiceBlock() editScope {
	contexts := map[string]bool{}
	for key := range s.differs {
		if rest, ok := strings.CutPrefix(key, "service:"); ok {
			contexts[rest[:strings.Index(rest, "/")]] = true
		}
	}
	for _, m := range s.models {
		for key := range m.values {
			if rest, ok := strings.CutPrefix(key, "service:"); ok && contexts[rest[:strings.Index(rest, "/")]] {
				s.differs[key] = true
			}
		}
	}
	return s
}

// touchesChildren reports whether an element of the given kind nested in the
// parent differs.
func (s editScope) touchesChildren(parent, kindPrefix string) bool {
	for key := range s.differs {
		if !strings.HasPrefix(key, kindPrefix) {
			continue
		}
		for _, m := range s.models {
			if m.parents[key] == parent {
				return true
			}
		}
	}
	return s.differs[parent]
}

func (s editScope) touchesPrefix(prefixes ...string) bool {
	for key := range s.differs {
		for _, prefix := range prefixes {
			if strings.HasPrefix(key, prefix) {
				return true
			}
		}
	}
	return false
}

// allows reports whether an edit between the two languages may change a line
// owned by key. A framed line is the opening or closing line of a body the
// element gains or loses with its nested elements: a value object or service
// without invariants or assertions has none.
func (s editScope) allows(key string) bool {
	switch {
	case key == "":
		return false
	case s.differs[key]:
		return true
	case key == keyMap:
		return s.mapEdited
	case strings.HasPrefix(key, "factory:"):
		// The printer places an aggregate's factory service by the grouping of
		// the declarations around it, so changing the aggregate's entities may
		// move it.
		return s.touchesChildren("aggregate:"+strings.TrimPrefix(key, "factory:"), "entity:")
	case strings.HasPrefix(key, "operation:"):
		operation := strings.TrimPrefix(key, "operation:")
		owner := operation[:strings.LastIndex(operation, "/")]
		return s.operations[operation] || s.differs["aggregate:"+owner] || s.differs["service:"+owner]
	case strings.HasPrefix(key, keyFrame):
		inner := strings.TrimPrefix(key, keyFrame)
		kind, rest, _ := strings.Cut(inner, ":")
		switch kind {
		case "module":
			return s.differs["context:"+rest] || s.touchesPrefix("value:"+rest+"/", "spec:"+rest+"/", "event:"+rest+"/", "service:"+rest+"/")
		case "value":
			return s.differs[inner] || s.touchesPrefix("invariant:"+rest+"/")
		case "service":
			return s.differs[inner] || s.touchesPrefix("assertion:"+rest+"/")
		}
		return s.differs[inner]
	}
	return false
}

// editViolations returns every line the edit touched that no differing
// element owns, as a readable report; nil when the edit is local.
func editViolations(before, after recordingLayout) []string {
	return scopeViolations(scopeOf(before, after), before, after)
}

func scopeViolations(scope editScope, before, after recordingLayout) []string {
	deleted, inserted := lcsDiff(before.keyed(), after.keyed())
	var out []string
	for _, line := range deleted {
		if !scope.allows(line.key) {
			out = append(out, fmt.Sprintf("-%d [%s] %s", line.number, line.key, strings.TrimSpace(line.text)))
		}
	}
	for _, line := range inserted {
		if !scope.allows(line.key) {
			out = append(out, fmt.Sprintf("+%d [%s] %s", line.number, line.key, strings.TrimSpace(line.text)))
		}
	}
	return out
}
