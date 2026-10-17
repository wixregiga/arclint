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
// it. An import through a module of the repository is followed to what
// that module imports, so a re-export of abc.ABC is abc.ABC, while a
// module of the repository that only shares the library's name is not
// the library. A file that assigns the library name itself, abc.ABC =
// object, has replaced it. A class that inherits abstractness from a
// repository base without stating it is not marked.
func markAbstractClasses(df *declarationFacts, from importOrigin, dir string) {
	for _, c := range df.module.declared {
		if !declaresAbstract(df, c, from, dir) {
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

// importOrigin follows an imported name to where it comes from.
type importOrigin interface {
	// repository reports whether a module imported from a file of the
	// directory is a module of the repository.
	repository(dir, module string) bool
	// follow names what a module of the repository binds under a name,
	// qualified by that module's own imports, with the module's
	// directory; "" when the module defines the name itself or does
	// not resolve to one file.
	follow(dir, module, name string) (string, string)
}

// declaresAbstract reports whether a class's base list names an
// abstract base or the abstract metaclass: ABC, (ABC), Protocol[T],
// abc.ABC, or metaclass=ABCMeta.
func declaresAbstract(df *declarationFacts, c *pyClass, from importOrigin, dir string) bool {
	w := df.walker
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
		q := libraryName(qualifiedTarget(c.header, n.Text(w.src), w, 0), from, dir)
		if q == "" || replaced(df, q) {
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

// libraryName follows a qualified name through the repository's modules
// to the library name it reaches; "" when it ends in the repository.
func libraryName(q string, from importOrigin, dir string) string {
	for hop := 0; hop < maxAliasHops; hop++ {
		module, name, ok := cutLast(q, ".")
		if !ok {
			return ""
		}
		if !from.repository(dir, module) {
			return q
		}
		q, dir = from.follow(dir, module, name)
	}
	return ""
}

// replaced reports whether the file assigns the library name itself, as
// abc.ABC = object does.
func replaced(df *declarationFacts, q string) bool {
	w := df.walker
	for _, m := range df.module.mutations {
		if qualifiedTarget(m.scope, m.node.Text(w.src), w, 0) == q {
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
