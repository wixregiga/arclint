package python

import (
	"strconv"
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// annotationName follows aliases before interpreting a typing special form.
// A project class called Optional or Callable is not a typing special form.
func (idx *resolutionIndex) annotationName(s *pyScope, text string, seen map[*pyBinding]bool) (pyValue, string) {
	v := idx.dotted(s, text, seen)
	qualified := v.ref.Name
	if v.ref.Package != "" {
		qualified = v.ref.Package + "." + qualified
	}
	return v, qualified
}

func (idx *resolutionIndex) annotationRefs(s *pyScope, n *gotreesitter.Node, seen map[*pyBinding]bool) []conformance.Reference {
	if n == nil {
		return nil
	}
	w := s.module.facts.walker
	switch n.Type(w.lang) {
	case nodeType:
		if n.NamedChildCount() == 1 {
			return idx.annotationRefs(s, n.NamedChild(0), seen)
		}
	case "none":
		return nil
	case "identifier", nodeAttribute, "dotted_name":
		if n.Text(w.src) == pythonNone {
			return nil
		}
		v, q := idx.annotationName(s, n.Text(w.src), seen)
		if q == "typing.Any" || q == "typing_extensions.Any" {
			return []conformance.Reference{unknown("Any does not declare a concrete type")}
		}
		if v.class != nil && (v.class.members["__get__"] != nil || v.class.members["__set__"] != nil) {
			return []conformance.Reference{unknown("descriptor type " + v.class.name + " needs descriptor resolution")}
		}
		if v.ref.Resolution == conformance.ResolutionExternal && v.ref.Package == "" && !pyBuiltins[v.ref.Name] {
			return []conformance.Reference{unknown(v.ref.Name + " is a builtin value, not a declared type")}
		}
		if v.ref.IsZero() {
			return []conformance.Reference{unknown(n.Text(w.src) + " names a module, not a declared type")}
		}
		if v.ref.Resolution == conformance.ResolutionDeclared && v.class == nil {
			return []conformance.Reference{unknown(n.Text(w.src) + " does not resolve to a declared class")}
		}
		return []conformance.Reference{v.ref}
	case nodeString:
		scope, ann, reason := decodeAnnotation(s, n)
		if reason != "" {
			return []conformance.Reference{unknown(reason)}
		}
		return idx.annotationRefs(scope, ann, seen)
	case "generic_type", nodeSubscript:
		if n.NamedChildCount() < 2 {
			break
		}
		head := n.NamedChild(0)
		_, q := idx.annotationName(s, head.Text(w.src), seen)
		refs := idx.annotationRefs(s, head, seen)
		if q == "typing.Callable" || q == "collections.abc.Callable" || q == "typing_extensions.Callable" || q == "typing.Literal" || q == "typing_extensions.Literal" {
			return refs
		}
		args := n.NamedChild(1)
		if q == "typing.Annotated" || q == "typing_extensions.Annotated" {
			if args.NamedChildCount() > 0 {
				refs = append(refs, idx.annotationRefs(s, args.NamedChild(0), seen)...)
			}
			return refs
		}
		for i := 1; i < n.NamedChildCount(); i++ {
			refs = append(refs, idx.annotationRefs(s, n.NamedChild(i), seen)...)
		}
		return refs
	case "type_parameter", "union_type", "binary_operator", "tuple":
		var out []conformance.Reference
		for i := 0; i < n.NamedChildCount(); i++ {
			out = append(out, idx.annotationRefs(s, n.NamedChild(i), seen)...)
		}
		return out
	}
	return []conformance.Reference{unknown("unsupported type expression " + normalizeType(n.Text(w.src)))}
}

func pythonString(text string) (string, bool) {
	// Python's ordinary single and double quoted forward references. Escape
	// sequences follow Python spelling; uncommon escapes stay undetermined.
	text = strings.TrimSpace(text)
	for len(text) > 0 && strings.ContainsRune("rRuU", rune(text[0])) {
		text = text[1:]
	}
	if len(text) < 2 || (text[0] != '\'' && text[0] != '"') {
		return "", false
	}
	quote := text[0]
	count := 1
	if len(text) >= 6 && text[1] == quote && text[2] == quote {
		count = 3
	}
	if len(text) < 2*count || text[len(text)-1] != quote {
		return "", false
	}
	body := text[count : len(text)-count]
	if !strings.Contains(body, "\\") {
		return body, true
	}
	// strconv handles the shared escape repertoire after quote conversion.
	body = strings.ReplaceAll(body, "\\'", "'")
	body = strings.ReplaceAll(body, "\"", "\\\"")
	decoded, err := strconv.Unquote("\"" + body + "\"")
	return decoded, err == nil
}

// annotationReceiver follows the outermost annotation type. Type arguments
// belong to Holds, so list[A | B] still has receiver list.
func (idx *resolutionIndex) annotationReceiver(s *pyScope, n *gotreesitter.Node, seen map[*pyBinding]bool) pyValue {
	if n == nil {
		return pyValue{ref: unknown("annotation has no concrete receiver type")}
	}
	w := s.module.facts.walker
	switch n.Type(w.lang) {
	case nodeType:
		if n.NamedChildCount() == 1 {
			return idx.annotationReceiver(s, n.NamedChild(0), seen)
		}
	case nodeString:
		scope, ann, reason := decodeAnnotation(s, n)
		if reason != "" {
			return pyValue{ref: unknown(reason)}
		}
		return idx.annotationReceiver(scope, ann, seen)
	case "generic_type", nodeSubscript:
		if n.NamedChildCount() < 2 {
			break
		}
		head := n.NamedChild(0)
		_, q := idx.annotationName(s, head.Text(w.src), seen)
		args := n.NamedChild(1)
		switch q {
		case "typing.Optional", "typing_extensions.Optional", "typing.Annotated", "typing_extensions.Annotated", "typing.Final", "typing_extensions.Final":
			if args.NamedChildCount() > 0 {
				return idx.annotationReceiver(s, args.NamedChild(0), seen)
			}
		case "typing.Union", "typing_extensions.Union":
			return idx.unionReceiver(s, args, seen)
		default:
			return idx.annotationReceiver(s, head, seen)
		}
	case "union_type", "binary_operator":
		return idx.unionReceiver(s, n, seen)
	}
	refs := idx.annotationRefs(s, n, seen)
	if len(refs) != 1 {
		return pyValue{ref: unknown("annotation has no concrete receiver type")}
	}
	ref := refs[0]
	v := pyValue{ref: ref, instance: true}
	if ref.Resolution == conformance.ResolutionDeclared {
		v.class = idx.modules[ref.File].classes[ref.Name]
	}
	return v
}

func (idx *resolutionIndex) unionReceiver(s *pyScope, n *gotreesitter.Node, seen map[*pyBinding]bool) pyValue {
	var result pyValue
	for i := 0; i < n.NamedChildCount(); i++ {
		ch := n.NamedChild(i)
		if ch.Text(s.module.facts.walker.src) == pythonNone {
			continue
		}
		v := idx.annotationReceiver(s, ch, seen)
		if v.ref.Resolution == conformance.ResolutionUndetermined {
			return v
		}
		if !result.ref.IsZero() && result.ref != v.ref {
			return pyValue{ref: unknown("union annotation has no single receiver type")}
		}
		result = v
	}
	if result.ref.IsZero() {
		result.ref = unknown("annotation has no concrete receiver type")
	}
	return result
}

func decodeAnnotation(s *pyScope, n *gotreesitter.Node) (*pyScope, *gotreesitter.Node, string) {
	text, ok := pythonString(n.Text(s.module.facts.walker.src))
	if !ok {
		return nil, nil, "annotation string cannot be decoded"
	}
	p, err := newParsers().forPython()
	if err != nil {
		return nil, nil, err.Error()
	}
	src := []byte("x: " + text + "\n")
	tree, err := p.ParseStrict(src)
	if err != nil {
		return nil, nil, "annotation string is not a supported type expression"
	}
	var ann *gotreesitter.Node
	var find func(*gotreesitter.Node)
	find = func(node *gotreesitter.Node) {
		if node.Type(p.Language()) == nodeAssignment {
			ann = node.ChildByFieldName(nodeType, p.Language())
			return
		}
		for i := 0; i < node.NamedChildCount(); i++ {
			find(node.NamedChild(i))
		}
	}
	find(tree.RootNode())
	if ann == nil {
		return nil, nil, "annotation string has no type expression"
	}
	copyModule := *s.module
	copyFacts := *copyModule.facts
	copyWalker := *copyFacts.walker
	copyWalker.src = src
	copyFacts.walker = &copyWalker
	copyModule.facts = &copyFacts
	copyScope := *s
	copyScope.module = &copyModule
	return &copyScope, ann, ""
}
