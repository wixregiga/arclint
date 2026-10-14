package python

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// fields observes instance storage, including assignments outside __init__.
// The first declaration wins; a later annotation cannot retroactively give
// an earlier unannotated declaration a type.
func (w *pyWalker) fields(n *gotreesitter.Node, owner string, inMethod bool) {
	switch n.Type(w.lang) {
	case nodeClass:
		owner, inMethod = w.name(n), false
	case nodeFunction:
		if inMethod {
			return
		}
		inMethod = owner != ""
	case nodeAssignment:
		left := n.ChildByFieldName("left", w.lang)
		typ := n.ChildByFieldName(nodeType, w.lang)
		name := ""
		if left != nil && owner != "" {
			if !inMethod && typ != nil && left.Type(w.lang) == nodeIdentifier {
				name = left.Text(w.src)
			}
			if inMethod && left.Type(w.lang) == nodeAttribute {
				obj := left.ChildByFieldName("object", w.lang)
				attr := left.ChildByFieldName(nodeAttribute, w.lang)
				if obj != nil && attr != nil && obj.Text(w.src) == bindingSelf {
					name = attr.Text(w.src)
				}
			}
		}
		if name != "" && !w.nonInstanceAnnotation(typ) {
			seen := false
			for _, d := range w.out.Decls {
				if d.Kind == "field" && d.Owner == owner && d.Name == name {
					seen = true
					break
				}
			}
			if !seen {
				w.add("field", name, owner, n)
			}
		}
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		w.fields(n.NamedChild(i), owner, inMethod)
	}
}

func (w *pyWalker) nonInstanceAnnotation(n *gotreesitter.Node) bool {
	if n == nil {
		return false
	}
	text := strings.Trim(n.Text(w.src), "\"'")
	head, _, _ := strings.Cut(text, "[")
	head = strings.TrimSpace(head)
	if head == "typing.ClassVar" || head == "dataclasses.InitVar" {
		return true
	}
	// Import aliases are syntax, not inferred names. A user class named
	// ClassVar is still an ordinary field type.
	for _, imp := range w.imports {
		if head == imp.local && ((imp.module == typingModule && imp.member == "ClassVar") || (imp.module == "dataclasses" && imp.member == "InitVar")) {
			return true
		}
		if strings.HasPrefix(head, imp.local+".") && imp.member == "" {
			if imp.module+strings.TrimPrefix(head, imp.local) == "typing.ClassVar" || imp.module+strings.TrimPrefix(head, imp.local) == "dataclasses.InitVar" {
				return true
			}
		}
	}
	return false
}
