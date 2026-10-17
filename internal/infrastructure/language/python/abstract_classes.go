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
// metaclass abc.ABCMeta, each reached through the module's imports. A
// class that inherits abstractness from a repository base without
// stating it is not marked.
func (w *pyWalker) markAbstractClasses() {
	for _, c := range w.classes {
		bases := c.node.ChildByFieldName("superclasses", w.lang)
		if bases == nil {
			continue
		}
		for i := 0; i < bases.NamedChildCount(); i++ {
			if w.declaresAbstract(bases.NamedChild(i)) {
				w.out.Decls[c.decl].Abstract = true
				break
			}
		}
	}
}

// declaresAbstract reports whether one argument of a class's base list
// names an abstract base or the abstract metaclass: ABC, Protocol[T],
// abc.ABC, or metaclass=ABCMeta.
func (w *pyWalker) declaresAbstract(arg *gotreesitter.Node) bool {
	if arg.Type(w.lang) == "keyword_argument" {
		name := arg.ChildByFieldName("name", w.lang)
		value := arg.ChildByFieldName("value", w.lang)
		return name != nil && value != nil && name.Text(w.src) == "metaclass" &&
			w.importedName(value.Text(w.src), 0) == abstractMetaclass
	}
	if arg.Type(w.lang) == nodeSubscript {
		if value := arg.ChildByFieldName("value", w.lang); value != nil {
			arg = value
		}
	}
	switch arg.Type(w.lang) {
	case nodeIdentifier, nodeAttribute:
		return abstractBases[w.importedName(arg.Text(w.src), 0)]
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
