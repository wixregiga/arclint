package python

import (
	"path"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/conformance"

	"github.com/odvcencio/gotreesitter"
)

type pyValue struct {
	ref                    conformance.Reference
	class                  *pyClass
	module, externalModule string
	instance               bool
	loaded                 string
}

func unknown(reason string) conformance.Reference {
	return conformance.Reference{Resolution: conformance.ResolutionUndetermined, Reason: reason}
}

func external(pkg, name string) conformance.Reference {
	return conformance.Reference{Resolution: conformance.ResolutionExternal, Package: pkg, Name: name}
}

func declared(file, owner, name string) conformance.Reference {
	return conformance.Reference{Resolution: conformance.ResolutionDeclared, File: file, Owner: owner, Name: name}
}

func (idx *resolutionIndex) moduleValue(from, module string) pyValue {
	r := idx.resolver
	class, _, _ := r.classify(path.Dir(from), module)
	if class == conformance.ImportStdlib || class == conformance.ImportExternal {
		return pyValue{externalModule: module}
	}
	candidates := map[string]bool{}
	namespace := false
	add := func(base, dotted string) {
		file, dir, ok := r.resolveUnder(base, dotted)
		if !ok {
			return
		}
		if file == "" {
			file = path.Join(dir, "__init__.py")
			if !r.files[file] {
				namespace = true
				return
			}
		}
		// Each directory between the source root and module must be a regular
		// package. The import scanner still supports namespace packages;
		// semantic resolution deliberately does not claim their bindings.
		for d := path.Dir(file); d != "." && d != base && d != ""; d = path.Dir(d) {
			if !r.files[path.Join(d, "__init__.py")] {
				namespace = true
				return
			}
		}
		candidates[file] = true
	}
	if strings.HasPrefix(module, ".") {
		level := 0
		for level < len(module) && module[level] == '.' {
			level++
		}
		base := path.Dir(from)
		if !r.files[path.Join(base, "__init__.py")] {
			namespace = true
		} else {
			for i := 1; i < level; i++ {
				base = path.Dir(base)
			}
			add(base, module[level:])
		}
	} else {
		for _, root := range r.sourceRoots {
			add(root, module)
		}
	}
	if len(candidates) > 1 {
		return pyValue{ref: unknown(module + " has competing source roots")}
	}
	if namespace {
		return pyValue{ref: unknown(module + " uses a namespace package")}
	}
	for f := range candidates {
		if idx.modules[f] == nil {
			return pyValue{ref: unknown(module + " has no parsed module")}
		}
		return pyValue{module: f}
	}
	return pyValue{ref: unknown(module + " does not resolve to a declared module or dependency")}
}

func (idx *resolutionIndex) symbol(s *pyScope, name string, seen map[*pyBinding]bool) pyValue {
	b := s.lookup(name)
	if b == nil {
		if _, builtin := pyBuiltins[name]; builtin {
			return pyValue{ref: external("", name)}
		}
		return pyValue{ref: unknown(name + " is neither declared in the file nor imported")}
	}
	if seen[b] {
		return pyValue{ref: unknown("cyclic binding for " + name)}
	}
	seen[b] = true
	defer delete(seen, b)
	if b.reason != "" && b.kind != bindingAnnotation {
		return pyValue{ref: unknown(b.reason)}
	}
	switch b.kind {
	case bindingSelf:
		return pyValue{ref: declared(b.class.scope.module.file, "", b.class.name), class: b.class, instance: true}
	case declClass:
		if b.class.scope.owner != nil {
			return pyValue{ref: unknown("nested class " + b.name + " has no unambiguous top-level declaration")}
		}
		if b.class.unsafe != "" {
			return pyValue{ref: unknown(b.class.unsafe)}
		}
		return pyValue{ref: declared(b.scope.module.file, "", b.name), class: b.class}
	case bindingFunction:
		if idx.patched[b.scope.module.file+"::"+b.name] {
			return pyValue{ref: unknown("monkey patching changes " + b.name)}
		}
		return pyValue{ref: declared(b.scope.module.file, "", b.name)}
	case bindingVariadic:
		return pyValue{ref: external("", "tuple"), instance: true}
	case bindingAnnotation:
		if b.annotation == nil {
			return pyValue{ref: unknown(b.name + " has no declared type")}
		}
		w := b.scope.module.facts.walker
		if _, q := idx.annotationName(b.scope, b.annotation.Text(w.src), seen); q == "typing.TypeAlias" || q == "typing_extensions.TypeAlias" {
			// An explicit alias stands for the type on its right.
			right := b.node.ChildByFieldName("right", w.lang)
			if right == nil {
				return pyValue{ref: unknown("type alias " + b.name + " has no value")}
			}
			switch right.Type(w.lang) {
			case nodeIdentifier, nodeAttribute:
				return idx.dotted(b.scope, right.Text(w.src), seen)
			}
			return pyValue{ref: unknown("type alias " + b.name + " names a type expression arclint does not follow")}
		}
		return idx.annotationReceiver(b.scope, b.annotation, seen)
	case bindingImport:
		v := idx.moduleValue(b.scope.module.file, b.imp.module)
		if b.imp.member != "" {
			return idx.attribute(v, b.imp.member, seen)
		}
		v.loaded = b.imp.loaded
		return v
	}
	return pyValue{ref: unknown(b.name + " has no declared type")}
}

func (idx *resolutionIndex) attribute(v pyValue, name string, seen map[*pyBinding]bool) pyValue {
	if v.module != "" {
		return idx.moduleAttribute(v.module, name, seen)
	}
	if v.externalModule != "" {
		candidate := v.externalModule + "." + name
		if v.loaded == candidate || strings.HasPrefix(v.loaded, candidate+".") {
			return pyValue{externalModule: candidate, loaded: v.loaded}
		}
		return pyValue{ref: external(v.externalModule, name)}
	}
	if v.ref.Resolution == conformance.ResolutionUndetermined {
		return v
	}
	if v.class != nil {
		return idx.classAttribute(v, name, seen)
	}
	if v.ref.Resolution == conformance.ResolutionExternal && v.instance {
		return pyValue{ref: unknown("external receiver " + v.ref.Name + " has no observed member declaration for " + name)}
	}
	return pyValue{ref: unknown("attribute " + name + " has no declared receiver type")}
}

func (idx *resolutionIndex) member(c *pyClass, name string, seen map[*pyClass]bool) (*pyBinding, *pyClass, string) {
	if c.unsafe != "" {
		return nil, nil, c.unsafe
	}
	if b := c.members[name]; b != nil {
		return b, c, ""
	}
	mro, reason := idx.mro(c, seen)
	if reason != "" {
		return nil, nil, reason
	}
	for _, owner := range mro {
		if b := owner.members[name]; b != nil {
			return b, owner, ""
		}
		if owner.dataclass && dataclassGeneratedMember(name) {
			return nil, nil, "dataclass generated member " + name + " has no source declaration"
		}
	}
	return nil, nil, ""
}

// mro implements Python's C3 linearization rather than guessing the first
// base that spells a member. Unknown bases keep the lookup undetermined.
func (idx *resolutionIndex) mro(c *pyClass, seen map[*pyClass]bool) ([]*pyClass, string) {
	if seen[c] {
		return nil, "cyclic base classes for " + c.name
	}
	if c.unsafe != "" {
		return nil, c.unsafe
	}
	seen[c] = true
	defer delete(seen, c)
	var bases []*pyClass
	var seqs [][]*pyClass
	w := c.scope.module.facts.walker
	for _, node := range c.bases {
		text := node.Text(w.src)
		if head, _, ok := strings.Cut(text, "["); ok {
			text = head
		}
		v := idx.dotted(c.scope, text, map[*pyBinding]bool{})
		if v.ref.Resolution == conformance.ResolutionExternal && ((v.ref.Package == "" && v.ref.Name == "object") || (v.ref.Package == typingModule && (v.ref.Name == "Protocol" || v.ref.Name == "Generic"))) {
			continue
		}
		if v.class == nil {
			return nil, "base " + text + " of " + c.name + " does not resolve to a repository class"
		}
		chain, reason := idx.mro(v.class, seen)
		if reason != "" {
			return nil, reason
		}
		seqs = append(seqs, chain)
		bases = append(bases, v.class)
	}
	seqs = append(seqs, bases)
	result := []*pyClass{c}
	for {
		var candidate *pyClass
		remaining := false
		for _, seq := range seqs {
			if len(seq) == 0 {
				continue
			}
			remaining = true
			head := seq[0]
			inTail := false
			for _, other := range seqs {
				for j := 1; j < len(other); j++ {
					if other[j] == head {
						inTail = true
					}
				}
			}
			if !inTail {
				candidate = head
				break
			}
		}
		if !remaining {
			return result, ""
		}
		if candidate == nil {
			return nil, "inconsistent method resolution order for " + c.name
		}
		result = append(result, candidate)
		for i, seq := range seqs {
			if len(seq) > 0 && seq[0] == candidate {
				seqs[i] = seq[1:]
			}
		}
	}
}

func (idx *resolutionIndex) dotted(s *pyScope, text string, seen map[*pyBinding]bool) pyValue {
	parts := strings.Split(strings.TrimSpace(text), ".")
	v := idx.symbol(s, parts[0], seen)
	for _, name := range parts[1:] {
		v = idx.attribute(v, name, seen)
	}
	return v
}

func (idx *resolutionIndex) expression(s *pyScope, n *gotreesitter.Node) pyValue {
	w := s.module.facts.walker
	if n == nil {
		return pyValue{ref: unknown("computed member call")}
	}
	switch n.Type(w.lang) {
	case nodeIdentifier:
		return idx.symbol(s, n.Text(w.src), map[*pyBinding]bool{})
	case nodeAttribute:
		obj := idx.expression(s, n.ChildByFieldName("object", w.lang))
		attr := n.ChildByFieldName(nodeAttribute, w.lang)
		if attr != nil {
			return idx.attribute(obj, attr.Text(w.src), map[*pyBinding]bool{})
		}
	case nodeCall:
		fn := n.ChildByFieldName("function", w.lang)
		if fn != nil && fn.Text(w.src) == builtinSuper && isBuiltin(idx.expression(s, fn).ref, builtinSuper) {
			if args := n.ChildByFieldName("arguments", w.lang); args != nil && args.NamedChildCount() > 0 {
				return pyValue{ref: unknown("explicit super() arguments are not resolved")}
			}
			if s.owner == nil || len(s.owner.bases) == 0 {
				return pyValue{ref: unknown("super() has no resolvable base")}
			}
			// super dispatch needs the MRO of the runtime receiver. Only a single
			// fully resolvable base establishes a static declaration here.
			if len(s.owner.bases) != 1 {
				return pyValue{ref: unknown("super() has no unambiguous resolvable base")}
			}
			v := idx.expression(s.owner.scope, s.owner.bases[0])
			if v.class != nil {
				v.instance = true
				return v
			}
			return pyValue{ref: unknown("super() has no resolvable base")}
		}
	}
	return pyValue{ref: unknown("computed member call")}
}

func (idx *resolutionIndex) call(c pyCall) (conformance.Reference, conformance.Reference) {
	w := c.scope.module.facts.walker
	fn := c.node.ChildByFieldName("function", w.lang)
	if fn == nil {
		return unknown("computed member call"), conformance.Reference{}
	}
	if args := c.node.ChildByFieldName("arguments", w.lang); args != nil {
		for i := 0; i < args.NamedChildCount(); i++ {
			if args.NamedChild(i).Type(w.lang) == "dictionary_splat" {
				receiver := conformance.Reference{}
				if fn.Type(w.lang) == nodeAttribute {
					_, receiver = idx.methodCall(c, fn)
				}
				return unknown("**kwargs plumbing has no declared target"), receiver
			}
		}
	}

	if fn.Type(w.lang) == nodeAttribute {
		return idx.methodCall(c, fn)
	}
	if fn.Type(w.lang) == nodeIdentifier {
		if b := c.scope.lookup(fn.Text(w.src)); b != nil && (b.kind == bindingAnnotation || b.kind == bindingSelf) {
			return unknown("callable value " + b.name + " has no declared call target"), conformance.Reference{}
		}
	}
	v := idx.expression(c.scope, fn)
	if v.ref.Resolution == conformance.ResolutionExternal && v.ref.Package == "" && v.ref.Name == builtinSuper {
		base := idx.expression(c.scope, c.node)
		if base.ref.Resolution == conformance.ResolutionUndetermined {
			return base.ref, conformance.Reference{}
		}
	}
	if v.ref.Resolution == conformance.ResolutionExternal && (v.ref.Package == "" || v.ref.Package == builtinsModule) && (v.ref.Name == "getattr" || v.ref.Name == builtinSetattr) {
		return unknown(v.ref.Name + " uses dynamic attribute access"), conformance.Reference{}
	}
	if v.ref.IsZero() {
		return unknown("module is not callable"), conformance.Reference{}
	}
	return v.ref, conformance.Reference{}
}

func (idx *resolutionIndex) moduleAttribute(file, name string, seen map[*pyBinding]bool) pyValue {
	if idx.patched[file+"::"+name] || idx.patched[file+"::*"] {
		return pyValue{ref: unknown("monkey patching changes " + name)}
	}
	m := idx.modules[file]
	if m.scope.lookup(name) != nil {
		return idx.symbol(m.scope, name, seen)
	}
	if path.Base(file) != "__init__.py" {
		return pyValue{ref: unknown(name + " is not declared or re-exported by " + file)}
	}
	sub := idx.moduleValue(file, "."+name)
	if sub.ref.Resolution != conformance.ResolutionUndetermined {
		return sub
	}
	return pyValue{ref: unknown(name + " is not declared or re-exported by " + file)}
}

func (idx *resolutionIndex) classAttribute(v pyValue, name string, seen map[*pyBinding]bool) pyValue {
	member, owner, reason := idx.member(v.class, name, map[*pyClass]bool{})
	if reason != "" {
		return pyValue{ref: unknown(reason)}
	}
	if member == nil {
		return pyValue{ref: declared(v.ref.File, v.class.name, name)}
	}
	if member.reason != "" && member.kind != bindingAnnotation {
		return pyValue{ref: unknown(member.reason)}
	}
	if idx.patched[owner.scope.module.file+":"+owner.name+":"+name] || idx.patched[owner.scope.module.file+":"+owner.name+":*"] {
		return pyValue{ref: unknown("monkey patching changes " + owner.name + "." + name)}
	}
	if member.kind == bindingUnknown {
		return pyValue{ref: unknown(member.name + " has no declared type")}
	}
	if member.kind == bindingAnnotation {
		return idx.symbol(&pyScope{bindings: map[string]*pyBinding{name: member}}, name, seen)
	}
	return pyValue{ref: declared(owner.scope.module.file, owner.name, name)}
}

func (idx *resolutionIndex) methodCall(c pyCall, fn *gotreesitter.Node) (conformance.Reference, conformance.Reference) {
	w := c.scope.module.facts.walker
	objectNode := fn.ChildByFieldName("object", w.lang)
	obj := idx.expression(c.scope, objectNode)
	member := fn.ChildByFieldName(nodeAttribute, w.lang)
	receiver := obj.ref
	if obj.ref.Resolution != conformance.ResolutionUndetermined && obj.class == nil && !obj.instance && obj.module == "" && obj.externalModule == "" {
		receiver = unknown(obj.ref.Name + " has no declared receiver type")
	}
	if objectNode != nil && objectNode.Type(w.lang) == nodeCall && c.scope.owner != nil && obj.class != nil {
		superFn := objectNode.ChildByFieldName("function", w.lang)
		if superFn != nil && superFn.Text(w.src) == builtinSuper {
			receiver = declared(c.scope.owner.scope.module.file, "", c.scope.owner.name)
		}
	}
	if obj.module != "" || obj.externalModule != "" {
		receiver = conformance.Reference{}
	}
	if member == nil {
		return unknown("computed member call"), receiver
	}
	if obj.class != nil {
		binding, _, _ := idx.member(obj.class, member.Text(w.src), map[*pyClass]bool{})
		if binding != nil && binding.kind == bindingAnnotation {
			return unknown("callable field " + binding.name + " has no declared call target"), receiver
		}
	}
	target := idx.attribute(obj, member.Text(w.src), map[*pyBinding]bool{}).ref
	if isBuiltin(target, "getattr") || isBuiltin(target, builtinSetattr) {
		target = unknown(target.Name + " uses dynamic attribute access")
	}
	if target.IsZero() {
		target = unknown("module is not callable")
	}
	return target, receiver
}

func isBuiltin(ref conformance.Reference, name string) bool {
	return ref.Resolution == conformance.ResolutionExternal && (ref.Package == "" || ref.Package == builtinsModule) && ref.Name == name
}

func dataclassGeneratedMember(name string) bool {
	switch name {
	case "__init__", "__repr__", "__eq__", "__hash__", "__lt__", "__le__", "__gt__", "__ge__":
		return true
	}
	return false
}
