package python

import (
	"sort"
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// implementationsOf lists, for every class of every module, the
// abstract classes of the repository it implements. Its bases, resolved
// the way Python resolves them and followed through the repository's
// classes, name them exactly: every ancestor its own declaration states
// to be abstract (abc.ABC, typing.Protocol, abc.ABCMeta). A base
// resolution cannot follow is undetermined, with the name it writes. A
// class the code registers as a virtual subclass through
// ABCMeta.register is undetermined, since registration checks none of
// its methods. Every other abstract class whose methods the class has
// by name is a heuristic implementation: duck typing is decided where a
// value is used.
func (idx *resolutionIndex) implementationsOf() map[*pyClass][]conformance.Implementation {
	idx.identities = true
	defer func() { idx.identities = false }()
	var classes []*pyClass
	files := make([]string, 0, len(idx.modules))
	for file := range idx.modules {
		files = append(files, file)
	}
	sort.Strings(files)
	for _, file := range files {
		classes = append(classes, idx.modules[file].declared...)
	}
	var abstractions []*pyClass
	for _, c := range classes {
		if declaredAbstract(c) && c.unnamed == "" {
			abstractions = append(abstractions, c)
		}
	}
	registered := idx.registrations(files)
	out := map[*pyClass][]conformance.Implementation{}
	for _, c := range classes {
		var impls []conformance.Implementation
		ancestors, open := idx.ancestors(c)
		exact := map[*pyClass]bool{}
		for _, a := range ancestors {
			if declaredAbstract(a) && a.unnamed == "" {
				exact[a] = true
				impls = append(impls, idx.implementation(c, a, conformance.ImplementationExact, ""))
			}
		}
		for _, o := range open {
			ref := unknown(o.reason)
			ref.Name = o.written
			impls = append(impls, conformance.Implementation{Abstraction: ref, Evidence: conformance.ImplementationUndetermined, Reason: o.reason})
		}
		for _, r := range registered[c] {
			if !exact[r] {
				exact[r] = true
				impls = append(impls, idx.implementation(c, r, conformance.ImplementationUndetermined,
					c.name+" is registered as a virtual subclass of "+r.name+" through ABCMeta.register, which checks none of its methods"))
			}
		}
		names := idx.memberNames(c)
		for _, a := range abstractions {
			if a == c || exact[a] {
				continue
			}
			methods := methodNames(a)
			if len(methods) == 0 || !hasAll(names, methods) {
				continue
			}
			impls = append(impls, idx.implementation(c, a, conformance.ImplementationHeuristic,
				c.name+" has every method of "+a.name+" by name, but no base names it"))
		}
		if len(impls) > 0 {
			out[c] = impls
		}
	}
	return out
}

// openBase is a base of a class that resolution cannot follow, with the
// name the class statement writes.
type openBase struct {
	written, reason string
}

// ancestors lists the repository classes a class derives from, directly
// or through other repository classes, in the order its bases name
// them, and the bases resolution cannot follow. A base outside the
// repository derives from no repository class.
func (idx *resolutionIndex) ancestors(c *pyClass) ([]*pyClass, []openBase) {
	var out []*pyClass
	var open []openBase
	seen := map[*pyClass]bool{c: true}
	var walk func(c *pyClass)
	walk = func(c *pyClass) {
		w := c.scope.module.facts.walker
		for _, node := range c.bases {
			text := node.Text(w.src)
			if head, _, ok := strings.Cut(text, "["); ok {
				text = head
			}
			v := idx.dotted(c.header, text, map[*pyBinding]bool{})
			switch {
			case v.class != nil:
				if !seen[v.class] {
					seen[v.class] = true
					out = append(out, v.class)
					walk(v.class)
				}
			case v.ref.Resolution == conformance.ResolutionUndetermined:
				written := strings.TrimSpace(text)
				if i := strings.LastIndex(written, "."); i >= 0 {
					written = written[i+1:]
				}
				open = append(open, openBase{written: written, reason: v.ref.Reason})
			}
		}
	}
	walk(c)
	return out, open
}

// registrations finds every class the repository registers as a
// virtual subclass of one of its abstract classes, through a call
// Abstraction.register(Class) or the decorator @Abstraction.register.
func (idx *resolutionIndex) registrations(files []string) map[*pyClass][]*pyClass {
	out := map[*pyClass][]*pyClass{}
	add := func(abstract, registered pyValue) {
		if abstract.class == nil || abstract.instance || !declaredAbstract(abstract.class) || registered.class == nil || registered.instance {
			return
		}
		out[registered.class] = append(out[registered.class], abstract.class)
	}
	for _, file := range files {
		m := idx.modules[file]
		w := m.facts.walker
		for _, c := range m.calls {
			fn := c.node.ChildByFieldName("function", w.lang)
			obj, ok := registerTarget(fn, w)
			if !ok {
				continue
			}
			args := c.node.ChildByFieldName("arguments", w.lang)
			if args == nil || args.NamedChildCount() == 0 {
				continue
			}
			add(idx.expression(c.scope, obj), idx.expression(c.scope, args.NamedChild(0)))
		}
		for _, c := range m.declared {
			decorated := c.node.Parent()
			if decorated == nil || decorated.Type(w.lang) != nodeDecorated {
				continue
			}
			for i := 0; i < decorated.NamedChildCount(); i++ {
				d := decorated.NamedChild(i)
				if d.Type(w.lang) != nodeDecorator || d.NamedChildCount() == 0 {
					continue
				}
				if obj, ok := registerTarget(d.NamedChild(0), w); ok {
					add(idx.expression(c.scope, obj), pyValue{class: c})
				}
			}
		}
	}
	return out
}

// registerTarget is the object of an attribute spelled register, the
// abstract class ABCMeta.register is called on.
func registerTarget(n *gotreesitter.Node, w *pyWalker) (*gotreesitter.Node, bool) {
	if n == nil || n.Type(w.lang) != nodeAttribute {
		return nil, false
	}
	attr := n.ChildByFieldName(nodeAttribute, w.lang)
	obj := n.ChildByFieldName("object", w.lang)
	if attr == nil || obj == nil || attr.Text(w.src) != "register" {
		return nil, false
	}
	return obj, true
}

// implementation names, for each method the abstraction declares, the
// member the class's own lookup reaches through its method resolution
// order.
func (idx *resolutionIndex) implementation(c, a *pyClass, evidence conformance.ImplementationEvidence, reason string) conformance.Implementation {
	impl := conformance.Implementation{Abstraction: declared(a.scope.module.file, "", a.name), Evidence: evidence, Reason: reason}
	idx.identities = false
	defer func() { idx.identities = true }()
	for _, name := range methodNames(a) {
		b, owner, why := idx.member(c, name, map[*pyClass]bool{})
		var ref conformance.Reference
		switch {
		case why != "":
			ref = unknown(why)
		case b == nil:
			ref = unknown(c.name + " has no member " + name)
		case b.kind != bindingFunction:
			ref = unknown(name + " of " + owner.name + " is not a method its class defines")
		case b.reason != "":
			// A decorator may replace the method with another function.
			ref = unknown(b.reason)
		case idx.patchedOnTheWay(c, owner, name) != "":
			ref = unknown("monkey patching changes " + idx.patchedOnTheWay(c, owner, name) + "." + name)
		default:
			ref = declared(owner.scope.module.file, owner.name, name)
		}
		if ref.Resolution == conformance.ResolutionUndetermined {
			ref.Name = name
		}
		impl.Operations = append(impl.Operations, ref)
	}
	return impl
}

// patchedOnTheWay names a class whose attribute of the name the
// repository patches, between the implementing class and the owner its
// lookup reaches, both included: a patch on any of them replaces what
// the lookup finds. "" when none is patched.
func (idx *resolutionIndex) patchedOnTheWay(c, owner *pyClass, name string) string {
	patched := func(k *pyClass) bool {
		file := k.scope.module.file
		return idx.patched[file+":"+k.name+":"+name] || idx.patched[file+":"+k.name+":*"]
	}
	order, reason := idx.mro(c, map[*pyClass]bool{})
	if reason != "" {
		order = []*pyClass{c, owner}
	}
	for _, k := range order {
		if patched(k) {
			return k.name
		}
		if k == owner {
			break
		}
	}
	return ""
}

// methodNames lists the methods a class body defines, in source order,
// leaving out the constructor, which no implementation shares.
func methodNames(c *pyClass) []string {
	var out []string
	for name, b := range c.members {
		if b.kind == bindingFunction && name != "__init__" && name != "__new__" {
			out = append(out, name)
		}
	}
	sort.Slice(out, func(i, j int) bool { return c.members[out[i]].node.StartByte() < c.members[out[j]].node.StartByte() })
	return out
}

// memberNames is every member name a class defines or inherits from the
// repository classes it derives from.
func (idx *resolutionIndex) memberNames(c *pyClass) map[string]bool {
	out := map[string]bool{}
	for name := range c.members {
		out[name] = true
	}
	ancestors, _ := idx.ancestors(c)
	for _, a := range ancestors {
		for name := range a.members {
			out[name] = true
		}
	}
	return out
}

func hasAll(names map[string]bool, want []string) bool {
	for _, n := range want {
		if !names[n] {
			return false
		}
	}
	return true
}

// declaredAbstract reports whether a class's own declaration states it
// is abstract, as markAbstractClasses decided for its declaration.
func declaredAbstract(c *pyClass) bool {
	line := int(c.node.StartPoint().Row) + 1
	for _, d := range c.scope.module.facts.Decls {
		if d.Kind == declClass && d.Name == c.name && d.StartLine == line {
			return d.Abstract
		}
	}
	return false
}
