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
}

// fields observes instance storage, including assignments outside __init__.
// The first declaration wins; a later annotation cannot retroactively give
// an earlier unannotated declaration a type.
func (w *pyWalker) fields(n *gotreesitter.Node, sc fieldScope) {
	switch n.Type(w.lang) {
	case nodeClass:
		sc = fieldScope{owner: w.name(n), rebound: map[string]bool{}}
	case nodeFunction:
		if sc.inMethod {
			return
		}
		if sc.owner != "" {
			sc = fieldScope{owner: sc.owner, inMethod: true, instance: w.instanceParameter(n), rebound: sc.rebound}
		}
	case nodeAssignment:
		left := n.ChildByFieldName("left", w.lang)
		typ := n.ChildByFieldName(nodeType, w.lang)
		name := w.fieldName(left, typ, sc)
		if name != "" && !w.nonInstanceAnnotation(typ, sc.rebound) {
			seen := false
			for _, d := range w.out.Decls {
				if d.Kind == "field" && d.Owner == sc.owner && d.Name == name {
					seen = true
					break
				}
			}
			if !seen {
				w.add("field", name, sc.owner, n)
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
func (w *pyWalker) fieldName(left, typ *gotreesitter.Node, sc fieldScope) string {
	if left == nil || sc.owner == "" {
		return ""
	}
	switch {
	case !sc.inMethod && left.Type(w.lang) == nodeIdentifier:
		if typ == nil {
			sc.rebound[left.Text(w.src)] = true
			return ""
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
// classmethod decorator says the method takes no instance.
func (w *pyWalker) instanceParameter(fn *gotreesitter.Node) string {
	if parent := fn.Parent(); parent != nil && parent.Type(w.lang) == nodeDecorated {
		for i := 0; i < parent.NamedChildCount(); i++ {
			d := parent.NamedChild(i)
			if d.Type(w.lang) != "decorator" {
				continue
			}
			switch strings.TrimSpace(strings.TrimPrefix(d.Text(w.src), "@")) {
			case "staticmethod", "classmethod":
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
			continue
		case "expression_statement":
			assignments = assignments[:0]
			for j := 0; j < n.NamedChildCount(); j++ {
				assignments = append(assignments, n.NamedChild(j))
			}
		}
		for _, a := range assignments {
			if a.Type(w.lang) != nodeAssignment && a.Type(w.lang) != "augmented_assignment" {
				continue
			}
			if left := a.ChildByFieldName("left", w.lang); left != nil && left.Type(w.lang) == nodeIdentifier {
				count[left.Text(w.src)]++
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
