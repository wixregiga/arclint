package python

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
// it. A class that inherits abstractness from a repository base without
// stating it is not marked.
func markAbstractClasses(df *declarationFacts) {
	w := df.walker
	for _, c := range df.module.declared {
		if !declaresAbstract(w, c) {
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
// abstract base or the abstract metaclass: ABC, Protocol[T], abc.ABC,
// or metaclass=ABCMeta.
func declaresAbstract(w *pyWalker, c *pyClass) bool {
	bases := c.node.ChildByFieldName("superclasses", w.lang)
	if bases == nil {
		return false
	}
	for i := 0; i < bases.NamedChildCount(); i++ {
		arg := bases.NamedChild(i)
		if arg.Type(w.lang) == "keyword_argument" {
			name := arg.ChildByFieldName("name", w.lang)
			value := arg.ChildByFieldName("value", w.lang)
			if name != nil && value != nil && name.Text(w.src) == "metaclass" &&
				qualifiedTarget(c.header, value.Text(w.src), w, 0) == abstractMetaclass {
				return true
			}
			continue
		}
		if arg.Type(w.lang) == nodeSubscript {
			if value := arg.ChildByFieldName("value", w.lang); value != nil {
				arg = value
			}
		}
		switch arg.Type(w.lang) {
		case nodeIdentifier, nodeAttribute:
			if abstractBases[qualifiedTarget(c.header, arg.Text(w.src), w, 0)] {
				return true
			}
		}
	}
	return false
}
