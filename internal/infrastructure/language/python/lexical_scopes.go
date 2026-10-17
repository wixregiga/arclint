package python

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

type importBinding struct{ local, module, member, loaded string }

func importBindings(w *pyWalker, n *gotreesitter.Node) []importBinding {
	if n == nil {
		return nil
	}
	var out []importBinding
	switch n.Type(w.lang) {
	case nodeImport, nodeImportFrom:
		module := ""
		if m := n.ChildByFieldName("module_name", w.lang); m != nil {
			module = m.Text(w.src)
		}
		for i := 0; i < n.NamedChildCount(); i++ {
			ch := n.NamedChild(i)
			if m := n.ChildByFieldName("module_name", w.lang); m != nil && ch.StartByte() == m.StartByte() {
				continue
			}
			name, alias := ch.Text(w.src), ""
			if ch.Type(w.lang) == "aliased_import" {
				if x := ch.ChildByFieldName("name", w.lang); x != nil {
					name = x.Text(w.src)
				}
				if x := ch.ChildByFieldName("alias", w.lang); x != nil {
					alias = x.Text(w.src)
				}
			}
			if module != "" {
				if alias == "" {
					alias = name
				}
				out = append(out, importBinding{local: alias, module: module, member: name})
			} else {
				loaded := name
				if alias == "" {
					alias, _, _ = strings.Cut(name, ".")
					name = alias
				}
				out = append(out, importBinding{local: alias, module: name, loaded: loaded})
			}
		}
		return out
	case nodeClass, nodeFunction:
		return nil
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		out = append(out, importBindings(w, n.NamedChild(i))...)
	}
	return out
}

type pyBinding struct {
	kind, name       string
	node, annotation *gotreesitter.Node
	imp              importBinding
	scope            *pyScope
	class            *pyClass
	reason           string
}
type pyScope struct {
	parent    *pyScope
	module    *pyModule
	owner     *pyClass
	bindings  map[string]*pyBinding
	star      bool
	uncertain string
	// classBody marks the namespace of a class body, which the scopes
	// nested inside it do not see.
	classBody bool
}

// lexical is the scope a scope nested here looks names up in: this
// one, or, for a class body, the nearest enclosing scope that is not
// a class body, since Python skips class namespaces when it resolves a
// nested scope's names.
func (s *pyScope) lexical() *pyScope {
	for s != nil && s.classBody {
		s = s.parent
	}
	return s
}

// typeParameterScope is the scope a generic def's or class's PEP 695
// type parameters open, over parent; parent itself when it declares
// none. A type parameter stands for whatever type its user chooses, so
// a name it binds resolves to no declaration. around is the scope the
// definition statement runs in, whose owner the new scope keeps.
func (m *pyModule) typeParameterScope(n *gotreesitter.Node, parent, around *pyScope) *pyScope {
	w := m.facts.walker
	params := n.ChildByFieldName("type_parameters", w.lang)
	if params == nil {
		return parent
	}
	tp := &pyScope{parent: parent, module: m, owner: around.owner, bindings: map[string]*pyBinding{}}
	for i := 0; i < params.NamedChildCount(); i++ {
		if name := firstIdentifier(params.NamedChild(i), w); name != "" {
			tp.bind(&pyBinding{kind: bindingUnknown, name: name, scope: tp, reason: name + " is a type parameter, whose type its user chooses"})
		}
	}
	return tp
}

// firstIdentifier is the first identifier at or under a node: the name
// a type parameter declares, under a splat or a bound.
func firstIdentifier(n *gotreesitter.Node, w *pyWalker) string {
	if n.Type(w.lang) == nodeIdentifier {
		return n.Text(w.src)
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		if name := firstIdentifier(n.NamedChild(i), w); name != "" {
			return name
		}
	}
	return ""
}

type pyClass struct {
	name  string
	node  *gotreesitter.Node
	scope *pyScope
	// header is the scope the class's base list is evaluated in: its
	// type parameters' scope when it declares any, otherwise scope.
	header *pyScope
	// inner is the scope its body and methods look past their own
	// names into: its type parameters over the nearest scope that is not
	// a class body.
	inner     *pyScope
	members   map[string]*pyBinding
	bases     []*gotreesitter.Node
	unsafe    string
	dataclass bool
}
type pyCall struct {
	node  *gotreesitter.Node
	scope *pyScope
}
type pyField struct {
	declaration int
	annotation  *gotreesitter.Node
	scope       *pyScope
	class       *pyClass
}

// pyFunction is one def with the scope its annotations are evaluated
// in, the scope around it.
type pyFunction struct {
	node  *gotreesitter.Node
	scope *pyScope
}
type pyModule struct {
	file    string
	facts   *declarationFacts
	scope   *pyScope
	calls   []pyCall
	fields  []pyField
	classes map[string]*pyClass
	// declared lists every class in source order, nested ones included,
	// where classes keeps one per name.
	declared      []*pyClass
	functions     []pyFunction
	mutations     []pyCall
	staticMethods map[uint32]bool
}
type resolutionIndex struct {
	resolver *resolver
	modules  map[string]*pyModule
	patched  map[string]bool
}

func (s *pyScope) bind(b *pyBinding) {
	if old := s.bindings[b.name]; old != nil {
		// Assigning a typed variable keeps its declared type. Competing
		// declarations/imports are ambiguous; do not pick the last spelling.
		if b.kind == bindingUnknown && old.kind == bindingAnnotation {
			return
		}
		b = &pyBinding{kind: bindingUnknown, name: b.name, reason: b.name + " has competing bindings", scope: s}
	}
	s.bindings[b.name] = b
}

func (s *pyScope) lookup(name string) *pyBinding {
	if s.star {
		return &pyBinding{kind: bindingUnknown, name: name, reason: "star import may bind " + name, scope: s}
	}
	if b := s.bindings[name]; b != nil {
		return b
	}
	if s.uncertain != "" {
		return &pyBinding{kind: bindingUnknown, name: name, reason: s.uncertain + " " + name, scope: s}
	}
	if s.parent != nil {
		return s.parent.lookup(name)
	}
	return nil
}

// newModule collects the lexical scopes of one file. Extraction already
// collected them to decide which classes are abstract; that module is
// reused, named for its file.
func newModule(file string, df *declarationFacts) *pyModule {
	if df.module != nil {
		df.module.file = file
		return df.module
	}
	m := &pyModule{file: file, facts: df, classes: map[string]*pyClass{}, staticMethods: map[uint32]bool{}}
	m.scope = &pyScope{module: m, bindings: map[string]*pyBinding{}}
	m.collect(df.root, m.scope, nil, false)
	return m
}

func (m *pyModule) collect(n *gotreesitter.Node, s *pyScope, owner *pyClass, inFunction bool) {
	w := m.facts.walker
	switch n.Type(w.lang) {
	case nodeDecorated:
		m.collectDecorated(n, s, owner, inFunction)
		return
	case nodeImport, nodeImportFrom:
		for _, imp := range importBindings(w, n) {
			if imp.member == "*" {
				s.star = true
				continue
			}
			s.bind(&pyBinding{kind: bindingImport, name: imp.local, imp: imp, scope: s, node: n})
		}
		return
	case nodeClass:
		c := &pyClass{name: w.name(n), node: n, scope: s, header: m.typeParameterScope(n, s, s), inner: m.typeParameterScope(n, s.lexical(), s), members: map[string]*pyBinding{}}
		if bases := n.ChildByFieldName("superclasses", w.lang); bases != nil {
			for i := 0; i < bases.NamedChildCount(); i++ {
				b := bases.NamedChild(i)
				if b.Type(w.lang) == "keyword_argument" {
					c.unsafe = "class " + c.name + " uses a metaclass or class keyword"
				} else {
					c.bases = append(c.bases, b)
				}
			}
		}
		s.bind(&pyBinding{kind: declClass, name: c.name, node: n, scope: s, class: c})
		// A nested same-spelled class cannot be named uniquely by Reference.
		if old := m.classes[c.name]; old != nil {
			old.unsafe = "class name " + c.name + " is ambiguous"
			c.unsafe = old.unsafe
		}
		m.classes[c.name] = c
		m.declared = append(m.declared, c)
		cs := &pyScope{parent: c.inner, module: m, owner: c, bindings: c.members, classBody: true}
		if body := n.ChildByFieldName("body", w.lang); body != nil {
			m.collect(body, cs, c, false)
		}
		return
	case nodeFunction:
		name := w.name(n)
		b := &pyBinding{kind: bindingFunction, name: name, node: n, scope: s}
		s.bind(b)
		// Annotations see the def's type parameters, which shadow names
		// around it; so does its body.
		annotations := m.typeParameterScope(n, s, s)
		m.functions = append(m.functions, pyFunction{node: n, scope: annotations})
		if owner != nil && !inFunction && (name == "__getattribute__" || name == "__getattr__") {
			owner.unsafe = "class " + owner.name + " customizes attribute access"
		}
		parent := s
		if owner != nil && !inFunction {
			parent = owner.inner
		}
		fs := &pyScope{parent: m.typeParameterScope(n, parent, s), module: m, bindings: map[string]*pyBinding{}}
		if owner != nil && !inFunction && !m.staticMethods[n.StartByte()] {
			fs.owner = owner
		}
		m.collectParameters(n, annotations, fs)
		// Defaults, annotations, and decorators are evaluated in the
		// enclosing scope, the return annotation included.
		if ps := n.ChildByFieldName("parameters", w.lang); ps != nil {
			m.collectCalls(ps, s)
		}
		if rt := n.ChildByFieldName("return_type", w.lang); rt != nil {
			// Walked as code, so a lambda inside keeps its own scope.
			m.collect(rt, s, owner, inFunction)
		}
		if body := n.ChildByFieldName("body", w.lang); body != nil {
			m.collect(body, fs, owner, true)
		}
		return
	case nodeAssignment, nodeAugmented, "named_expression":
		m.collectAssignment(n, s, owner, inFunction)
	case nodeAsPattern:
		for i := 0; i < n.NamedChildCount(); i++ {
			ch := n.NamedChild(i)
			if ch.Type(w.lang) == "as_pattern_target" {
				m.bindPattern(ch, s)
			}
		}
	case "for_statement", nodeForInClause:
		if left := n.ChildByFieldName("left", w.lang); left != nil {
			m.bindPattern(left, s)
		}
	case "list_comprehension", "dictionary_comprehension", "set_comprehension", "generator_expression":
		cs := &pyScope{parent: s, module: m, bindings: map[string]*pyBinding{}, owner: s.owner}
		// Comprehension targets bind throughout the comprehension scope.
		for i := 0; i < n.NamedChildCount(); i++ {
			ch := n.NamedChild(i)
			if ch.Type(w.lang) == nodeForInClause {
				if left := ch.ChildByFieldName("left", w.lang); left != nil {
					m.bindPattern(left, cs)
				}
			}
		}
		for i := 0; i < n.NamedChildCount(); i++ {
			m.collect(n.NamedChild(i), cs, owner, inFunction)
		}
		return
	case "case_pattern":
		// A pattern holds no code to run; it binds its captures.
		m.bindCaptures(n, s)
		return
	case "global_statement", "nonlocal_statement":
		for i := 0; i < n.NamedChildCount(); i++ {
			ch := n.NamedChild(i)
			if ch.Type(w.lang) == nodeIdentifier {
				name := ch.Text(w.src)
				s.bindings[name] = &pyBinding{kind: bindingUnknown, name: name, scope: s, reason: "global or nonlocal rebinding of " + name}
			}
		}
		return
	case "lambda":
		// Lambda parameters and comprehensions require their own scope; calls
		// inside them remain observed but must not borrow outer bindings.
		ls := &pyScope{parent: s, module: m, bindings: map[string]*pyBinding{}, uncertain: "lambda bindings are not resolved for"}
		for i := 0; i < n.NamedChildCount(); i++ {
			m.collect(n.NamedChild(i), ls, nil, true)
		}
		return
	case nodeCall:
		m.calls = append(m.calls, pyCall{n, s})
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		m.collect(n.NamedChild(i), s, owner, inFunction)
	}
}

func (m *pyModule) collectCalls(n *gotreesitter.Node, s *pyScope) {
	w := m.facts.walker
	if n.Type(w.lang) == nodeCall {
		m.calls = append(m.calls, pyCall{n, s})
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		m.collectCalls(n.NamedChild(i), s)
	}
}

func (m *pyModule) bindPattern(n *gotreesitter.Node, s *pyScope) {
	w := m.facts.walker
	if n.Type(w.lang) == nodeIdentifier {
		s.bind(&pyBinding{kind: bindingUnknown, name: n.Text(w.src), scope: s, reason: n.Text(w.src) + " has no declared type"})
		return
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		m.bindPattern(n.NamedChild(i), s)
	}
}

func (m *pyModule) collectDecorated(n *gotreesitter.Node, s *pyScope, owner *pyClass, inFunction bool) {
	w := m.facts.walker
	def := n.ChildByFieldName("definition", w.lang)
	if def == nil {
		return
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		ch := n.NamedChild(i)
		if ch.Type(w.lang) == nodeDecorator {
			m.collectCalls(ch, s)
		}
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		ch := n.NamedChild(i)
		if ch.Type(w.lang) == nodeDecorator && decoratorName(s, ch, w) == builtinStaticmethod {
			m.staticMethods[def.StartByte()] = true
		}
	}
	m.collect(def, s, owner, inFunction)
	b := s.bindings[w.name(def)]
	if b == nil {
		return
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		ch := n.NamedChild(i)
		if ch.Type(w.lang) != nodeDecorator {
			continue
		}
		d := decoratorName(s, ch, w)
		if d == "dataclasses.dataclass" && b.class != nil {
			b.class.dataclass = true
		}
		if d == builtinStaticmethod || d == builtinClassmethod || d == "dataclasses.dataclass" || d == "abc.abstractmethod" {
			continue
		}
		b.reason = "decorated declaration " + b.name + " may be a descriptor or replacement"
		if b.class != nil {
			b.class.unsafe = b.reason
		}
	}
}

func (m *pyModule) collectParameters(n *gotreesitter.Node, outer, fs *pyScope) {
	w := m.facts.walker
	params := n.ChildByFieldName("parameters", w.lang)
	if params == nil {
		return
	}
	for i := 0; i < params.NamedChildCount(); i++ {
		pn := params.NamedChild(i)
		p, ok := w.param(pn)
		if !ok {
			continue
		}
		kind := bindingUnknown
		reason := strings.TrimLeft(p.Name, "*") + " has no declared type"
		ann := pn.ChildByFieldName(nodeType, w.lang)
		if ann != nil {
			kind = bindingAnnotation
			reason = ""
		}
		switch {
		case strings.HasPrefix(p.Name, "**"):
			kind = bindingUnknown
			reason = "**kwargs plumbing has no declared target"
		case strings.HasPrefix(p.Name, "*"):
			// *repos: Repo holds a tuple of Repo; the annotation names its
			// elements, not the parameter.
			kind = bindingVariadic
			reason = ""
		}
		// Parameter annotations are resolved where the function is defined,
		// not against parameters or locals sharing a type's name.
		pb := &pyBinding{kind: kind, name: strings.TrimLeft(p.Name, "*"), node: pn, annotation: ann, scope: outer, reason: reason}
		// The first parameter of a method is its instance (or, for a
		// classmethod, its class), whatever it is spelled; a staticmethod
		// has no owner here and so no such parameter.
		if i == 0 && fs.owner != nil && !strings.HasPrefix(p.Name, "*") {
			pb.kind = bindingSelf
			pb.class = fs.owner
			pb.reason = ""
		}
		fs.bind(pb)
	}
}

func (m *pyModule) collectAssignment(n *gotreesitter.Node, s *pyScope, owner *pyClass, inFunction bool) {
	w := m.facts.walker
	left := n.ChildByFieldName("left", w.lang)
	if left == nil {
		left = n.ChildByFieldName("name", w.lang)
	}
	if left == nil {
		return
	}
	ann := n.ChildByFieldName(nodeType, w.lang)
	name := left.Text(w.src)
	switch left.Type(w.lang) {
	case nodeIdentifier:
		if owner != nil && !inFunction && (name == "__getattr__" || name == "__getattribute__") {
			owner.unsafe = "class " + owner.name + " customizes attribute access"
		}
		kind := bindingUnknown
		if ann != nil {
			kind = bindingAnnotation
		}
		s.bind(&pyBinding{kind: kind, name: name, node: n, annotation: ann, scope: s, reason: name + " has no declared type"})
	case nodeAttribute:
		m.mutations = append(m.mutations, pyCall{node: left, scope: s})
	default:
		m.bindPattern(left, s)
	}
	if owner == nil || inFunction && s.owner == nil {
		return
	}
	// self.x, or x on whatever the method names its instance.
	if obj, attr, ok := strings.Cut(name, "."); ok {
		if b := s.lookup(obj); b != nil && b.kind == bindingSelf {
			name = attr
		}
	}
	for j, d := range m.facts.Decls {
		if d.Kind != declField || d.Owner != owner.name || d.Name != name || d.StartLine != int(n.StartPoint().Row)+1 {
			continue
		}
		for _, field := range m.fields {
			if field.declaration == j {
				return
			}
		}
		m.fields = append(m.fields, pyField{j, ann, s, owner})
		if old := owner.members[name]; old == nil {
			owner.members[name] = &pyBinding{kind: bindingAnnotation, name: name, node: n, annotation: ann, scope: s}
		} else if old.kind == bindingFunction {
			old.reason = "instance assignment may replace " + owner.name + "." + name
		}
	}
}

// decoratorName recognizes only imported standard decorators and unshadowed
// builtins. A local function named dataclass may replace the declaration.
func decoratorName(s *pyScope, n *gotreesitter.Node, w *pyWalker) string {
	if n.NamedChildCount() == 0 {
		return ""
	}
	expr := n.NamedChild(0)
	if expr.Type(w.lang) == nodeCall {
		expr = expr.ChildByFieldName("function", w.lang)
	}
	if expr == nil {
		return ""
	}
	return qualifiedTarget(s, expr.Text(w.src), w, 0)
}

// qualifiedTarget names what a dotted name refers to in a scope, a
// decorator's or a class base's: a builtin, an imported name, or,
// through an alias such as cm = classmethod, base = abc for base.ABC,
// or Base: TypeAlias = ABC, what the alias stands for; "" for anything
// else.
func qualifiedTarget(s *pyScope, text string, w *pyWalker, depth int) string {
	head, tail, dotted := strings.Cut(text, ".")
	b := s.lookup(head)
	if b == nil {
		if !dotted && (head == builtinStaticmethod || head == builtinClassmethod) {
			return head
		}
		return ""
	}
	if b.kind != bindingImport {
		right := aliasValue(b, w, depth)
		if right == "" || depth >= maxAliasHops {
			return ""
		}
		if dotted {
			right += "." + tail
		}
		return qualifiedTarget(b.scope, right, w, depth+1)
	}
	qualified := b.imp.module
	if b.imp.member != "" {
		qualified += "." + b.imp.member
	}
	if dotted {
		qualified += "." + tail
	}
	if qualified == "builtins.staticmethod" {
		return builtinStaticmethod
	}
	if qualified == "builtins.classmethod" {
		return builtinClassmethod
	}
	return qualified
}

// maxAliasHops bounds following one plain alias to another.
const maxAliasHops = 8

// typeAliasAnnotations are the annotations that declare an assignment
// an alias of what it assigns: Base: TypeAlias = ABC.
var typeAliasAnnotations = map[string]bool{
	"typing.TypeAlias":            true,
	"typing_extensions.TypeAlias": true,
}

// aliasValue is the name a binding was assigned from, cm for cm = x.y
// is "x.y", when the binding is one assignment of a name, plain or
// annotated as a TypeAlias.
func aliasValue(b *pyBinding, w *pyWalker, depth int) string {
	if b.node == nil || b.node.Type(w.lang) != nodeAssignment {
		return ""
	}
	if b.annotation != nil && (depth >= maxAliasHops || !typeAliasAnnotations[qualifiedTarget(b.scope, b.annotation.Text(w.src), w, depth+1)]) {
		return ""
	}
	right := b.node.ChildByFieldName("right", w.lang)
	if right == nil {
		return ""
	}
	switch right.Type(w.lang) {
	case nodeIdentifier, nodeAttribute:
		return right.Text(w.src)
	}
	return ""
}

// bindCaptures binds every name a match pattern captures: a bare name
// in the pattern or as a keyword pattern's value, a *rest or **rest
// splat, and the name after as. A dotted name is a value the pattern
// compares, a class pattern's head names the class it matches, and _
// binds nothing.
func (m *pyModule) bindCaptures(n *gotreesitter.Node, s *pyScope) {
	w := m.facts.walker
	capture := func(id *gotreesitter.Node) {
		if name := id.Text(w.src); name != "_" {
			s.bind(&pyBinding{kind: bindingUnknown, name: name, scope: s, reason: name + " is captured by a match pattern"})
		}
	}
	switch n.Type(w.lang) {
	case "dotted_name":
		if parent := n.Parent(); n.NamedChildCount() == 1 && parent != nil && parent.Type(w.lang) != "class_pattern" {
			capture(n.NamedChild(0))
		}
		return
	case "splat_pattern":
		if n.NamedChildCount() == 1 && n.NamedChild(0).Type(w.lang) == nodeIdentifier {
			capture(n.NamedChild(0))
		}
		return
	case nodeAsPattern:
		if last := n.NamedChild(n.NamedChildCount() - 1); last != nil && last.Type(w.lang) == nodeIdentifier {
			capture(last)
		}
	case "keyword_pattern":
		// The keyword names an attribute; only its value pattern binds.
		for i := 1; i < n.NamedChildCount(); i++ {
			m.bindCaptures(n.NamedChild(i), s)
		}
		return
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		m.bindCaptures(n.NamedChild(i), s)
	}
}
