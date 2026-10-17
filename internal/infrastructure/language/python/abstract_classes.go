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
// metaclass abc.ABCMeta, each reached through the module's imports and
// shadowed by no binding of an enclosing class or function. A class
// that inherits abstractness from a repository base without stating it
// is not marked.
func (w *pyWalker) markAbstractClasses() {
	for _, c := range w.classes {
		bases := c.node.ChildByFieldName("superclasses", w.lang)
		if bases == nil {
			continue
		}
		for i := 0; i < bases.NamedChildCount(); i++ {
			if w.declaresAbstract(c.node, bases.NamedChild(i)) {
				w.out.Decls[c.decl].Abstract = true
				break
			}
		}
	}
}

// declaresAbstract reports whether one argument of a class's base list
// names an abstract base or the abstract metaclass: ABC, Protocol[T],
// abc.ABC, or metaclass=ABCMeta.
func (w *pyWalker) declaresAbstract(class, arg *gotreesitter.Node) bool {
	if arg.Type(w.lang) == "keyword_argument" {
		name := arg.ChildByFieldName("name", w.lang)
		value := arg.ChildByFieldName("value", w.lang)
		return name != nil && value != nil && name.Text(w.src) == "metaclass" &&
			w.moduleName(class, value.Text(w.src)) == abstractMetaclass
	}
	if arg.Type(w.lang) == nodeSubscript {
		if value := arg.ChildByFieldName("value", w.lang); value != nil {
			arg = value
		}
	}
	switch arg.Type(w.lang) {
	case nodeIdentifier, nodeAttribute:
		return abstractBases[w.moduleName(class, arg.Text(w.src))]
	}
	return false
}

// moduleName qualifies a name a class's base list spells by the
// module's imports, unless a class or function enclosing the class
// binds the name itself, so the base is that binding and not the import.
func (w *pyWalker) moduleName(class *gotreesitter.Node, text string) string {
	head, _, _ := strings.Cut(text, ".")
	for n := class.Parent(); n != nil; n = n.Parent() {
		switch n.Type(w.lang) {
		case nodeClass, nodeFunction:
			if w.scopeBinds(n, head) {
				return ""
			}
		}
	}
	return w.importedName(text, 0)
}

// scopeBinds reports whether a class or function binds a name in its
// own scope: as a parameter, or in its body by a def, a class, an
// import, or an assignment, for, or with target, never looking into a
// nested definition's body.
func (w *pyWalker) scopeBinds(def *gotreesitter.Node, name string) bool {
	if params := def.ChildByFieldName("parameters", w.lang); params != nil {
		for i := 0; i < params.NamedChildCount(); i++ {
			if p, ok := w.param(params.NamedChild(i)); ok && strings.TrimLeft(p.Name, "*") == name {
				return true
			}
		}
	}
	body := def.ChildByFieldName("body", w.lang)
	return body != nil && w.bodyBinds(body, name)
}

func (w *pyWalker) bodyBinds(n *gotreesitter.Node, name string) bool {
	switch n.Type(w.lang) {
	case nodeClass, nodeFunction:
		return w.name(n) == name
	case nodeImport, nodeImportFrom:
		for _, imp := range importBindings(w, n) {
			if imp.local == name || imp.member == "*" {
				return true
			}
		}
		return false
	case nodeAssignment, nodeAugmented, "for_statement", nodeForInClause, "with_item", nodeAsPattern:
		for _, field := range []string{"left", "alias"} {
			if target := n.ChildByFieldName(field, w.lang); target != nil && w.patternBinds(target, name) {
				return true
			}
		}
		if n.Type(w.lang) == nodeAsPattern {
			for i := 0; i < n.NamedChildCount(); i++ {
				if ch := n.NamedChild(i); ch.Type(w.lang) == "as_pattern_target" && w.patternBinds(ch, name) {
					return true
				}
			}
		}
	case "global_statement", "nonlocal_statement":
		return w.patternBinds(n, name)
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		if w.bodyBinds(n.NamedChild(i), name) {
			return true
		}
	}
	return false
}

// patternBinds reports whether an assignment target binds the name: the
// identifier itself, or one inside a tuple or list pattern.
func (w *pyWalker) patternBinds(n *gotreesitter.Node, name string) bool {
	switch n.Type(w.lang) {
	case nodeIdentifier:
		return n.Text(w.src) == name
	case nodeAttribute, nodeSubscript:
		return false
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		if w.patternBinds(n.NamedChild(i), name) {
			return true
		}
	}
	return false
}

// importedName qualifies a module-level name by the import that binds
// it, following plain aliases such as Base = ABC: ABC imported from abc
// is abc.ABC, and abc.ABC through import abc is abc.ABC. A name no
// import binds, or one the module binds more than once, qualifies to
// "".
func (w *pyWalker) importedName(text string, depth int) string {
	if depth > maxAliasHops {
		return ""
	}
	head, tail, dotted := strings.Cut(text, ".")
	if w.rebound[head] {
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
		return q
	}
	if alias, ok := w.aliases[head]; ok && !dotted {
		return w.importedName(alias, depth+1)
	}
	return ""
}
