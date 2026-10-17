package python

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// fieldScope is where the field walk stands: the class whose fields it
// collects, the name the current method gives its instance ("" outside
// an instance method), and the names the class body has rebound so far.
type fieldScope struct {
	owner    string
	instance string
	inMethod bool
	rebound  map[string]bool
	// nested marks a class defined inside a function, where a parameter
	// or local may shadow any module-level name.
	nested bool
}

// fields observes instance storage, including assignments outside __init__.
// The first declaration wins; a later annotation cannot retroactively give
// an earlier unannotated declaration a type.
func (w *pyWalker) fields(n *gotreesitter.Node, sc fieldScope) {
	switch n.Type(w.lang) {
	case nodeClass:
		sc = fieldScope{owner: w.name(n), rebound: map[string]bool{}, nested: sc.nested || sc.inMethod}
	case nodeFunction:
		if sc.inMethod {
			return
		}
		if sc.owner == "" {
			sc.nested = true
			break
		}
		// Decorators are read before the def binds its own name, which
		// then shadows a module-level decorator for later methods.
		instance := w.instanceParameter(n, sc.rebound, sc.nested)
		sc.rebound[w.name(n)] = true
		sc = fieldScope{owner: sc.owner, inMethod: true, instance: instance, rebound: sc.rebound, nested: sc.nested}
	case nodeAssignment:
		left := n.ChildByFieldName("left", w.lang)
		typ := n.ChildByFieldName(nodeType, w.lang)
		name := w.fieldName(left, typ, n.ChildByFieldName("right", w.lang), sc)
		if name != "" && !w.nonInstanceAnnotation(typ, sc.rebound) {
			seen := false
			for _, d := range w.out.Decls {
				if d.Kind == declField && d.Owner == sc.owner && d.Name == name {
					seen = true
					break
				}
			}
			if !seen {
				w.add(declField, name, sc.owner, n)
			}
		}
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		w.fields(n.NamedChild(i), sc)
	}
}

// fieldName is the instance field an assignment declares, "" for none:
// an annotated name in the class body, or an attribute of the method's
// instance. A plain class-level assignment rebinds its name for the
// annotations after it.
func (w *pyWalker) fieldName(left, typ, right *gotreesitter.Node, sc fieldScope) string {
	if left == nil || sc.owner == "" {
		return ""
	}
	switch {
	case !sc.inMethod && left.Type(w.lang) == nodeIdentifier:
		if typ == nil {
			sc.rebound[left.Text(w.src)] = true
			return ""
		}
		// An annotated assignment with a value binds the name in the
		// class body as well as declaring the field.
		if right != nil {
			sc.rebound[left.Text(w.src)] = true
		}
		return left.Text(w.src)
	case sc.inMethod && sc.instance != "" && left.Type(w.lang) == nodeAttribute:
		obj := left.ChildByFieldName("object", w.lang)
		attr := left.ChildByFieldName(nodeAttribute, w.lang)
		if obj != nil && attr != nil && obj.Text(w.src) == sc.instance {
			return attr.Text(w.src)
		}
	}
	return ""
}

// instanceParameter is the name a method gives its instance: its first
// parameter, whatever it is spelled, unless a staticmethod or
// classmethod decorator says the method takes no instance. classRebound
// holds the names the class body has bound before the method, which
// shadow module-level ones. In a class defined inside a function only
// the builtin names themselves count: a parameter or local there may
// shadow any module-level alias.
func (w *pyWalker) instanceParameter(fn *gotreesitter.Node, classRebound map[string]bool, nested bool) string {
	if parent := fn.Parent(); parent != nil && parent.Type(w.lang) == nodeDecorated {
		for i := 0; i < parent.NamedChildCount(); i++ {
			d := parent.NamedChild(i)
			if d.Type(w.lang) != nodeDecorator || d.NamedChildCount() == 0 {
				continue
			}
			text := d.NamedChild(0).Text(w.src)
			if head, _, _ := strings.Cut(text, "."); classRebound[head] {
				continue
			}
			name := w.builtinName(text, 0)
			if nested && name != text {
				continue
			}
			switch name {
			case builtinStaticmethod, builtinClassmethod:
				return ""
			}
		}
	}
	params := fn.ChildByFieldName("parameters", w.lang)
	if params == nil || params.NamedChildCount() == 0 {
		return ""
	}
	first := params.NamedChild(0)
	switch first.Type(w.lang) {
	case nodeIdentifier:
		return first.Text(w.src)
	case "typed_parameter", "default_parameter", "typed_default_parameter":
		if name := first.ChildByFieldName("name", w.lang); name != nil {
			return name.Text(w.src)
		}
		if first.NamedChildCount() > 0 && first.NamedChild(0).Type(w.lang) == nodeIdentifier {
			return first.NamedChild(0).Text(w.src)
		}
	}
	return ""
}

// classLevel names the annotations that mark a class-level value, not
// storage each instance holds.
var classLevel = map[string]bool{
	"typing.ClassVar":            true,
	"typing_extensions.ClassVar": true,
	"dataclasses.InitVar":        true,
}

func (w *pyWalker) nonInstanceAnnotation(n *gotreesitter.Node, classRebound map[string]bool) bool {
	if n == nil {
		return false
	}
	text := strings.Trim(n.Text(w.src), "\"'")
	head, _, _ := strings.Cut(text, "[")
	head = strings.TrimSpace(head)
	if root, _, _ := strings.Cut(head, "."); w.rebound[root] || classRebound[root] {
		return false
	}
	if classLevel[head] {
		return true
	}
	// Import aliases are syntax, not inferred names. A user class named
	// ClassVar is still an ordinary field type.
	for _, imp := range w.imports {
		if head == imp.local && imp.member != "" && classLevel[imp.module+"."+imp.member] {
			return true
		}
		if strings.HasPrefix(head, imp.local+".") && imp.member == "" && classLevel[imp.module+strings.TrimPrefix(head, imp.local)] {
			return true
		}
	}
	return false
}

// builtinName names the builtin a module-level name refers to, through
// imports from builtins and plain aliases such as cm = classmethod; ""
// when it refers to anything else.
func (w *pyWalker) builtinName(text string, depth int) string {
	if depth > maxAliasHops {
		return ""
	}
	head, tail, dotted := strings.Cut(text, ".")
	if w.rebound[head] {
		// Bound more than once at module level: no one binding decides
		// what the name refers to.
		return ""
	}
	for _, imp := range w.imports {
		if imp.local != head {
			continue
		}
		q := imp.module
		if imp.member != "" {
			q += "." + imp.member
		}
		if dotted {
			q += "." + tail
		}
		if name, ok := strings.CutPrefix(q, "builtins."); ok && !strings.Contains(name, ".") {
			return name
		}
		return ""
	}
	if dotted {
		return ""
	}
	if alias, ok := w.aliases[head]; ok {
		return w.builtinName(alias, depth+1)
	}
	if w.bound[head] {
		return ""
	}
	return head
}

// moduleRebindings lists the module-level names bound more than once:
// imported twice, or imported and also assigned, defined, or declared
// as a class at the top level.
func moduleRebindings(w *pyWalker, root *gotreesitter.Node) map[string]bool {
	count := map[string]int{}
	for _, imp := range w.imports {
		count[imp.local]++
	}
	for i := 0; i < root.NamedChildCount(); i++ {
		n := root.NamedChild(i)
		if n.Type(w.lang) == nodeDecorated {
			if def := n.ChildByFieldName("definition", w.lang); def != nil {
				n = def
			}
		}
		assignments := []*gotreesitter.Node{n}
		switch n.Type(w.lang) {
		case nodeClass, nodeFunction:
			count[w.name(n)]++
			w.bound[w.name(n)] = true
			continue
		case "expression_statement":
			assignments = assignments[:0]
			for j := 0; j < n.NamedChildCount(); j++ {
				assignments = append(assignments, n.NamedChild(j))
			}
		}
		for _, a := range assignments {
			if a.Type(w.lang) != nodeAssignment && a.Type(w.lang) != nodeAugmented {
				continue
			}
			left := a.ChildByFieldName("left", w.lang)
			if left == nil || left.Type(w.lang) != nodeIdentifier {
				continue
			}
			count[left.Text(w.src)]++
			w.bound[left.Text(w.src)] = true
			right := a.ChildByFieldName("right", w.lang)
			if a.Type(w.lang) == nodeAssignment && right != nil && (right.Type(w.lang) == nodeIdentifier || right.Type(w.lang) == nodeAttribute) {
				w.aliases[left.Text(w.src)] = right.Text(w.src)
			}
		}
	}
	out := map[string]bool{}
	for name, n := range count {
		if n > 1 {
			out[name] = true
		}
	}
	return out
}
