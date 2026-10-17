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
// the library. A file that assigns any name on that way, abc.ABC =
// object or facade.ABC = object, has replaced it. A class that inherits abstractness from a
// repository base without stating it is not marked.
func markAbstractClasses(df *declarationFacts, from importOrigin, dir string) {
	if df.ParseError != "" || df.module == nil {
		// Facts that did not parse hold no classes to mark.
		return
	}
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
		chain := libraryChain(qualifiedTarget(c.header, n.Text(w.src), w, 0), from, dir)
		if len(chain) == 0 {
			return ""
		}
		for _, q := range chain {
			if replaced(df, q) {
				return ""
			}
		}
		return chain[len(chain)-1]
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

// libraryChain follows a qualified name through the repository's
// modules to the library name it reaches, and lists every name on the
// way, the library name last: facade.abstract.ABC, where facade imports
// abc as abstract, is facade.abstract.ABC, then abc.ABC. The longest
// prefix that is a module of the repository is the module the rest is
// looked up in. Nil when the name ends in the repository or cannot be
// followed.
func libraryChain(q string, from importOrigin, dir string) []string {
	var chain []string
	for hop := 0; q != "" && hop < maxAliasHops; hop++ {
		chain = append(chain, q)
		relative := strings.HasPrefix(q, ".")
		top, _, dotted := strings.Cut(strings.TrimLeft(q, "."), ".")
		if !dotted && !relative || !relative && !from.repository(dir, top) {
			return chain
		}
		next := ""
		// Each dot from the right splits the name into a module and the
		// names looked up in it: facade.abstract.ABC is abstract in
		// facade, then its ABC, and .ABC is ABC in the package . itself.
		lead := len(q) - len(strings.TrimLeft(q, "."))
		for _, at := range splitPoints(q, lead) {
			module, rest := q[:at], q[at+1:]
			if at < lead {
				module, rest = q[:lead], q[lead:]
			}
			name, attributes, _ := strings.Cut(rest, ".")
			followed, moduleDir := from.follow(dir, module, name)
			if followed == "" {
				continue
			}
			next, dir = followed, moduleDir
			if attributes != "" {
				next += "." + attributes
			}
			break
		}
		q = next
	}
	return nil
}

// replaced reports whether the file assigns a qualified name or a
// prefix it is reached through: abc.ABC = object replaces abc.ABC, and
// facade.abstract = x replaces facade.abstract.ABC.
func replaced(df *declarationFacts, q string) bool {
	return mutates(df.module, q)
}

// mutates reports whether a module assigns a qualified name or a prefix
// of it.
func mutates(m *pyModule, q string) bool {
	w := m.facts.walker
	for _, mutation := range m.mutations {
		target := qualifiedTarget(mutation.scope, mutation.node.Text(w.src), w, 0)
		if target != "" && (q == target || strings.HasPrefix(q, target+".")) {
			return true
		}
	}
	return false
}

// splitPoints lists, right to left, the dots that may separate a module
// from the names in it, and the end of the leading dots of a relative
// name, where the package itself is the module.
func splitPoints(q string, lead int) []int {
	var out []int
	for at := strings.LastIndex(q, "."); at >= lead; at = strings.LastIndex(q[:at], ".") {
		out = append(out, at)
	}
	if lead > 0 {
		out = append(out, lead-1)
	}
	return out
}

// unparenthesized strips the parentheses around an expression: (ABC) is
// ABC.
func unparenthesized(n *gotreesitter.Node, w *pyWalker) *gotreesitter.Node {
	for n.Type(w.lang) == "parenthesized_expression" && n.NamedChildCount() == 1 {
		n = n.NamedChild(0)
	}
	return n
}
