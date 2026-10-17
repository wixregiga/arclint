package python

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// abstractBases are the bases whose naming declares a class abstract:
// an abstract base class through abc.ABC, or a protocol, which PEP 544
// declares only by naming typing.Protocol among the class's own bases.
var abstractBases = map[string]bool{
	"abc.ABC":                    true,
	"typing.Protocol":            true,
	"typing_extensions.Protocol": true,
}

// abstractMetaclass is the metaclass whose naming declares a class an
// abstract base class.
const abstractMetaclass = "abc.ABCMeta"

// markAbstractClasses marks every class whose own declaration states
// that it is abstract: a base naming abc.ABC or a Protocol, or the
// metaclass abc.ABCMeta. Each name is looked up in the scope the class
// statement runs in, the way Python looks it up, so an import counts
// only when no nearer binding, rebinding, or star import competes with
// it, and only when the import reaches the library, not a module of the
// repository that shares its name. A class that inherits abstractness
// from a repository base without stating it is not marked.
func markAbstractClasses(df *declarationFacts, repository func(module string) bool) {
	w := df.walker
	for _, c := range df.module.declared {
		if !declaresAbstract(w, c, repository) {
			continue
		}
		line := int(c.node.StartPoint().Row) + 1
		for i := range df.Decls {
			if d := &df.Decls[i]; d.Kind == declClass && d.Name == c.name && d.StartLine == line {
				d.Abstract = true
			}
		}
	}
}

// declaresAbstract reports whether a class's base list names an
// abstract base or the abstract metaclass: ABC, (ABC), Protocol[T],
// abc.ABC, or metaclass=ABCMeta.
func declaresAbstract(w *pyWalker, c *pyClass, repository func(module string) bool) bool {
	bases := c.node.ChildByFieldName("superclasses", w.lang)
	if bases == nil {
		return false
	}
	names := func(n *gotreesitter.Node) string {
		n = unparenthesized(n, w)
		if n.Type(w.lang) == nodeSubscript {
			if value := n.ChildByFieldName("value", w.lang); value != nil {
				n = unparenthesized(value, w)
			}
		}
		switch n.Type(w.lang) {
		case nodeIdentifier, nodeAttribute:
		default:
			return ""
		}
		q := qualifiedTarget(c.header, n.Text(w.src), w, 0)
		if module, _, ok := cutLast(q, "."); !ok || repository != nil && repository(module) {
			return ""
		}
		return q
	}
	for i := 0; i < bases.NamedChildCount(); i++ {
		arg := bases.NamedChild(i)
		if arg.Type(w.lang) == "keyword_argument" {
			name := arg.ChildByFieldName("name", w.lang)
			value := arg.ChildByFieldName("value", w.lang)
			if name != nil && value != nil && name.Text(w.src) == "metaclass" && names(value) == abstractMetaclass {
				return true
			}
			continue
		}
		if abstractBases[names(arg)] {
			return true
		}
	}
	return false
}

// unparenthesized strips the parentheses around an expression: (ABC) is
// ABC.
func unparenthesized(n *gotreesitter.Node, w *pyWalker) *gotreesitter.Node {
	for n.Type(w.lang) == "parenthesized_expression" && n.NamedChildCount() == 1 {
		n = n.NamedChild(0)
	}
	return n
}

// cutLast splits s around the last sep: abc.ABC is abc and ABC.
func cutLast(s, sep string) (string, string, bool) {
	i := strings.LastIndex(s, sep)
	if i < 0 {
		return s, "", false
	}
	return s[:i], s[i+len(sep):], true
}
