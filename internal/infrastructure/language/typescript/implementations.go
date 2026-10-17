package typescript

import (
	"sort"

	"github.com/odvcencio/gotreesitter"
	"github.com/wixregiga/arclint/internal/domain/conformance"
)

const (
	tsClassHeritage     = "class_heritage"
	tsExtendsClause     = "extends_clause"
	tsImplementsClause  = "implements_clause"
	tsExtendsTypeClause = "extends_type_clause"
)

// abstraction is one interface or abstract class of the repository as
// indexed: its file, its declaration, and the reference naming it.
type abstraction struct {
	f   *resolutionFile
	n   *gotreesitter.Node
	ref conformance.Reference
	// methods are the operations it declares, read once for every class
	// matched against it.
	methods []string
}

// classImplementations lists what one class implements. Its implements
// and extends clauses, followed through the repository's classes and
// interfaces, name its abstractions exactly: an interface or abstract
// class a clause reaches, directly or through a base class or an
// interface it extends. A clause resolution cannot follow is
// undetermined, with the name it writes. Every other interface of the
// repository whose methods the class declares or inherits by name is a
// heuristic implementation, since TypeScript checks structure where a
// value is used, not where the class is declared.
func (r *resolutionIndex) classImplementations(f *resolutionFile, n *gotreesitter.Node, interfaces []abstraction) []conformance.Implementation {
	class := declared(f.path, "", f.name(n))
	if len(f.types[class.Name]) != 1 {
		// Which declaration of the name a clause or a member belongs to
		// is not resolved, so nothing about the class is established.
		reason := "declaration merging or duplicate type name is not resolved"
		return []conformance.Implementation{{
			Abstraction: uncertain(reason),
			Evidence:    conformance.ImplementationUndetermined,
			Reason:      reason,
		}}
	}
	var out []conformance.Implementation
	exact := map[string]bool{}
	var follow func(f *resolutionFile, n *gotreesitter.Node, depth int)
	follow = func(f *resolutionFile, n *gotreesitter.Node, depth int) {
		if depth > maxAliasDepth {
			return
		}
		for _, base := range f.heritage(n) {
			ref := r.receiverType(f, base, f.nodes[n])
			if ref.Resolution == conformance.ResolutionUndetermined {
				written := uncertain(ref.Reason)
				written.Name = normalizeType(f.text(base))
				out = append(out, conformance.Implementation{Abstraction: written, Evidence: conformance.ImplementationUndetermined, Reason: ref.Reason})
				continue
			}
			if ref.Resolution != conformance.ResolutionDeclared {
				// A library's interface or class is no abstraction of the
				// repository.
				continue
			}
			bf, bn := r.typeNode(ref)
			if bn == nil {
				continue
			}
			if bf.abstract(bn) {
				key := ref.File + "#" + ref.Name
				if !exact[key] {
					exact[key] = true
					out = append(out, r.implementation(class, abstraction{f: bf, n: bn, ref: ref}, conformance.ImplementationExact, ""))
				}
			}
			follow(bf, bn, depth+1)
		}
	}
	follow(f, n, 0)
	names := r.memberNames(f, n, map[string]bool{})
	for _, a := range interfaces {
		if exact[a.ref.File+"#"+a.ref.Name] {
			continue
		}
		if len(a.methods) == 0 || !subset(a.methods, names) {
			continue
		}
		out = append(out, r.implementation(class, a, conformance.ImplementationHeuristic,
			"the class declares or inherits every method of "+a.ref.Name+", but no implements or extends clause names it"))
	}
	return out
}

// implementation names, for each method the abstraction declares, the
// member the class's own lookup reaches: its own, or one inherited
// through its extends clauses.
func (r *resolutionIndex) implementation(class conformance.Reference, a abstraction, evidence conformance.ImplementationEvidence, reason string) conformance.Implementation {
	impl := conformance.Implementation{Abstraction: a.ref, Evidence: evidence, Reason: reason}
	for _, name := range r.methodNames(a.f, a.n, map[string]bool{}) {
		mf, mn, owner, why := r.member(class, name, false, map[string]bool{})
		switch {
		case why != "":
			ref := uncertain(why)
			ref.Name = name
			impl.Operations = append(impl.Operations, ref)
		case mn != nil:
			impl.Operations = append(impl.Operations, declared(mf.path, owner.Name, name))
		case owner.Resolution == conformance.ResolutionExternal:
			impl.Operations = append(impl.Operations, conformance.Reference{Resolution: conformance.ResolutionExternal, Package: owner.Package, Owner: owner.Name, Name: name})
		default:
			ref := uncertain(class.Name + " declares no member " + name)
			ref.Name = name
			impl.Operations = append(impl.Operations, ref)
		}
	}
	return impl
}

// heritage lists the type expressions a class or interface names as
// its bases: a class's extends and implements clauses, an interface's
// extends clause.
func (f *resolutionFile) heritage(n *gotreesitter.Node) []*gotreesitter.Node {
	var out []*gotreesitter.Node
	for i := 0; i < n.NamedChildCount(); i++ {
		h := n.NamedChild(i)
		switch f.kind(h) {
		case tsExtendsTypeClause:
			for j := 0; j < h.NamedChildCount(); j++ {
				out = append(out, h.NamedChild(j))
			}
		case tsClassHeritage:
			for j := 0; j < h.NamedChildCount(); j++ {
				c := h.NamedChild(j)
				switch f.kind(c) {
				case tsExtendsClause:
					if v := f.child(c, tsValueField); v != nil {
						out = append(out, v)
					} else if c.NamedChildCount() > 0 {
						out = append(out, c.NamedChild(0))
					}
				case tsImplementsClause:
					for k := 0; k < c.NamedChildCount(); k++ {
						out = append(out, c.NamedChild(k))
					}
				}
			}
		}
	}
	return out
}

// abstract reports a declaration a class implements rather than
// inherits an implementation from: an interface or an abstract class.
func (f *resolutionFile) abstract(n *gotreesitter.Node) bool {
	switch f.kind(n) {
	case tsInterfaceDeclaration, tsAbstractClassDeclaration:
		return true
	}
	return false
}

// methodNames lists the operations an interface or abstract class
// declares, its own and those of the interfaces and classes it extends,
// in source order; a base resolution cannot follow contributes none.
func (r *resolutionIndex) methodNames(f *resolutionFile, n *gotreesitter.Node, seen map[string]bool) []string {
	key := f.path + "#" + f.name(n)
	if seen[key] {
		return nil
	}
	seen[key] = true
	var out []string
	have := map[string]bool{}
	add := func(name string) {
		if name != "" && name != tsConstructor && !have[name] {
			have[name] = true
			out = append(out, name)
		}
	}
	if body := f.child(n, tsBodyField); body != nil {
		for i := 0; i < body.NamedChildCount(); i++ {
			m := body.NamedChild(i)
			switch f.kind(m) {
			case tsMethodSignature, tsAbstractMethodSignature, tsMethodDefinition:
				if !hasToken(m, f.lang, "static") {
					add(f.name(m))
				}
			}
		}
	}
	for _, base := range f.heritage(n) {
		ref := r.receiverType(f, base, f.nodes[n])
		if ref.Resolution != conformance.ResolutionDeclared {
			continue
		}
		if bf, bn := r.typeNode(ref); bn != nil && bf.abstract(bn) {
			for _, name := range r.methodNames(bf, bn, seen) {
				add(name)
			}
		}
	}
	return out
}

// memberNames is every instance member name a class declares or
// inherits through the repository's classes.
func (r *resolutionIndex) memberNames(f *resolutionFile, n *gotreesitter.Node, seen map[string]bool) map[string]bool {
	out := map[string]bool{}
	key := f.path + "#" + f.name(n)
	if seen[key] {
		return out
	}
	seen[key] = true
	if body := f.child(n, tsBodyField); body != nil {
		for i := 0; i < body.NamedChildCount(); i++ {
			m := body.NamedChild(i)
			if !hasToken(m, f.lang, "static") {
				if name := f.name(m); name != "" {
					out[name] = true
				}
			}
		}
	}
	for _, base := range f.heritage(n) {
		ref := r.receiverType(f, base, f.nodes[n])
		if ref.Resolution != conformance.ResolutionDeclared {
			continue
		}
		// A base class's members are inherited; an interface's are only
		// promised.
		if bf, bn := r.typeNode(ref); bn != nil && bf.isClass(bn) {
			for name := range r.memberNames(bf, bn, seen) {
				out[name] = true
			}
		}
	}
	return out
}

func subset(names []string, of map[string]bool) bool {
	for _, n := range names {
		if !of[n] {
			return false
		}
	}
	return true
}

// repositoryInterfaces lists every interface the index holds under one
// declaration, in a stable order: the abstractions a class may match by
// member names.
func (r *resolutionIndex) repositoryInterfaces() []abstraction {
	var out []abstraction
	for _, f := range r.files {
		if f.ambient {
			continue
		}
		for name, ns := range f.types {
			if len(ns) == 1 && f.kind(ns[0]) == tsInterfaceDeclaration {
				out = append(out, abstraction{f: f, n: ns[0], ref: declared(f.path, "", name), methods: r.methodNames(f, ns[0], map[string]bool{})})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ref.File != out[j].ref.File {
			return out[i].ref.File < out[j].ref.File
		}
		return out[i].ref.Name < out[j].ref.Name
	})
	return out
}

// isClass reports a class declaration or a named class expression.
func (f *resolutionFile) isClass(n *gotreesitter.Node) bool {
	switch f.kind(n) {
	case tsClassDeclaration, tsAbstractClassDeclaration, tsClassExpression:
		return true
	}
	return false
}
