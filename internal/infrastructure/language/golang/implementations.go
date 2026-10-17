package golang

import (
	"go/types"
	"sort"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// implementations is, for every type-checked file, each named type it
// declares with the interfaces of the repository the type implements.
type implementations map[string]map[string][]conformance.Implementation

// implementationsOf computes, once per type check, which named types of
// the repository implement which interfaces of the repository, the way
// the Go compiler decides it: types.Implements on the type and on a
// pointer to it, so a method with a pointer receiver counts. A generic
// type or interface is left out, since only an instantiation of it has
// a method set to compare. For each implementation the facts name the
// method the type's own lookup reaches for every method of the
// interface: declared on the type, or promoted from an embedded field.
func (tc *typeCheck) implementationsOf() implementations {
	tc.implOnce.Do(func() {
		tc.impls = tc.computeImplementations()
	})
	return tc.impls
}

// namedDecl is one file-level type declaration of a type-checked file.
type namedDecl struct {
	obj  *types.TypeName
	file string
}

func (tc *typeCheck) computeImplementations() implementations {
	var ifaces, concrete []namedDecl
	seen := map[*types.Package]bool{}
	for _, tf := range tc.files {
		if tf.pkg == nil || seen[tf.pkg] {
			continue
		}
		seen[tf.pkg] = true
		scope := tf.pkg.Scope()
		for _, name := range scope.Names() {
			obj, ok := scope.Lookup(name).(*types.TypeName)
			if !ok || obj.IsAlias() {
				continue
			}
			named, ok := obj.Type().(*types.Named)
			if !ok || named.TypeParams().Len() > 0 {
				continue
			}
			rel := tc.relative(tf.fset.Position(obj.Pos()).Filename)
			if _, typed := tc.files[rel]; !typed || !tc.observed[rel] {
				continue
			}
			d := namedDecl{obj: obj, file: rel}
			if iface, isInterface := named.Underlying().(*types.Interface); isInterface {
				if iface.IsMethodSet() && iface.NumMethods() > 0 {
					ifaces = append(ifaces, d)
				}
				continue
			}
			concrete = append(concrete, d)
		}
	}
	byPosition := func(ds []namedDecl) {
		sort.Slice(ds, func(i, j int) bool {
			if ds[i].file != ds[j].file {
				return ds[i].file < ds[j].file
			}
			return ds[i].obj.Pos() < ds[j].obj.Pos()
		})
	}
	byPosition(ifaces)
	byPosition(concrete)
	out := implementations{}
	for _, t := range concrete {
		ptr := types.NewPointer(t.obj.Type())
		names := map[string]bool{}
		mset := types.NewMethodSet(ptr)
		for i := range mset.Len() {
			names[mset.At(i).Obj().Name()] = true
		}
		res := newTypedResolution(tc, tc.files[t.file])
		for _, i := range ifaces {
			iface := i.obj.Type().Underlying().(*types.Interface)
			if !hasMethodNames(iface, names) {
				continue
			}
			impl := conformance.Implementation{
				Abstraction: res.objectRef(i.obj, ""),
				Evidence:    conformance.ImplementationExact,
			}
			implements := types.Implements(t.obj.Type(), iface) || types.Implements(ptr, iface)
			invalid, byName := tc.signatureGaps(ptr, iface)
			switch {
			case invalid:
				// A comparison over a type the checker could not type
				// proves neither that the type implements the interface
				// nor that it does not.
				impl.Evidence = conformance.ImplementationUndetermined
				impl.Reason = "the type checker could not type every signature of " + i.obj.Name() + " and " + t.obj.Name() + ", so whether one implements the other is undetermined"
			case implements:
			case byName:
				// Types of a package arclint reads by name only may be one
				// type under two names, such as an alias in another
				// package, so a mismatch proves nothing.
				impl.Evidence = conformance.ImplementationUndetermined
				impl.Reason = "the signatures of " + i.obj.Name() + " and " + t.obj.Name() + " name types of a package arclint reads by name only, so whether one implements the other is undetermined"
			default:
				continue
			}
			for m := range iface.Methods() {
				impl.Operations = append(impl.Operations, res.reachedMethod(ptr, m))
			}
			if out[t.file] == nil {
				out[t.file] = map[string][]conformance.Implementation{}
			}
			out[t.file][t.obj.Name()] = append(out[t.file][t.obj.Name()], impl)
		}
	}
	return out
}

// signatureGaps reports whether the signatures compared to decide that
// a type implements an interface hold a type the checker could not
// type, and whether they hold one of a package arclint reads by name
// only: the interface's methods and the methods the type reaches by
// their names.
func (tc *typeCheck) signatureGaps(typ types.Type, iface *types.Interface) (invalid, byName bool) {
	seen := map[types.Type]bool{}
	for m := range iface.Methods() {
		i, n := tc.typeGaps(m.Type(), seen)
		invalid, byName = invalid || i, byName || n
		obj, _, _ := types.LookupFieldOrMethod(typ, false, m.Pkg(), m.Name())
		if fn, ok := obj.(*types.Func); ok {
			i, n = tc.typeGaps(fn.Type(), seen)
			invalid, byName = invalid || i, byName || n
		}
	}
	return invalid, byName
}

// typeGaps walks a type for one the checker could not type and for a
// declared type of a package arclint does not type-check from source,
// which it knows by name only.
func (tc *typeCheck) typeGaps(typ types.Type, seen map[types.Type]bool) (invalid, byName bool) {
	if typ == nil || seen[typ] {
		return false, false
	}
	seen[typ] = true
	merge := func(ts ...types.Type) {
		for _, t := range ts {
			i, n := tc.typeGaps(t, seen)
			invalid, byName = invalid || i, byName || n
		}
	}
	named := func(obj *types.TypeName, args *types.TypeList) {
		if pkg := obj.Pkg(); pkg != nil && !tc.sources[pkg.Path()] {
			byName = true
		}
		for t := range args.Types() {
			merge(t)
		}
	}
	switch x := typ.(type) {
	case *types.Basic:
		return x.Kind() == types.Invalid, false
	case *types.Named:
		named(x.Obj(), x.TypeArgs())
	case *types.Alias:
		named(x.Obj(), x.TypeArgs())
		merge(x.Rhs())
	case *types.Pointer:
		merge(x.Elem())
	case *types.Slice:
		merge(x.Elem())
	case *types.Array:
		merge(x.Elem())
	case *types.Chan:
		merge(x.Elem())
	case *types.Map:
		merge(x.Key(), x.Elem())
	case *types.Signature:
		merge(x.Params(), x.Results())
	case *types.Tuple:
		for v := range x.Variables() {
			merge(v.Type())
		}
	case *types.Struct:
		for f := range x.Fields() {
			merge(f.Type())
		}
	case *types.Interface:
		for m := range x.Methods() {
			merge(m.Type())
		}
	case *types.TypeParam, *types.Union:
	}
	return invalid, byName
}

// hasMethodNames reports whether a method set holds a method of every
// name the interface declares: a cheap test before types.Implements.
func hasMethodNames(iface *types.Interface, names map[string]bool) bool {
	for m := range iface.Methods() {
		if !names[m.Name()] {
			return false
		}
	}
	return true
}

// reachedMethod names the method a selector of the interface method's
// name reaches on a value of the type: the type's own, or one promoted
// from an embedded field, which may be an embedded interface's.
func (t typedResolution) reachedMethod(typ types.Type, m *types.Func) conformance.Reference {
	obj, _, _ := types.LookupFieldOrMethod(typ, false, m.Pkg(), m.Name())
	fn, ok := obj.(*types.Func)
	if !ok {
		return t.undetermined("the method " + m.Name() + " does not reach a declared method")
	}
	return t.methodRef(fn)
}
