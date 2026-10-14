package golang

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// resolution answers, for one file, the declarations its calls reach
// and the declarations its fields hold.
type resolution interface {
	call(c *ast.CallExpr) (target, receiver conformance.Reference)
	holds(typeExpr ast.Expr) []conformance.Reference
	fieldType(typeExpr ast.Expr) conformance.Reference
}

// unresolvedFile answers for a file the toolchain did not type-check:
// every reference is undetermined, with the reason.
type unresolvedFile struct{ reason string }

func (u unresolvedFile) undetermined() conformance.Reference {
	return conformance.Reference{Resolution: conformance.ResolutionUndetermined, Reason: u.reason}
}

func (u unresolvedFile) call(*ast.CallExpr) (conformance.Reference, conformance.Reference) {
	return u.undetermined(), conformance.Reference{}
}

func (u unresolvedFile) holds(ast.Expr) []conformance.Reference {
	return []conformance.Reference{u.undetermined()}
}

func (u unresolvedFile) fieldType(ast.Expr) conformance.Reference { return u.undetermined() }

// typedResolution answers from the type information arclint computed
// for the file's package.
type typedResolution struct {
	tc   *typeCheck
	file typedFile
	info *types.Info
}

func newTypedResolution(tc *typeCheck, tf typedFile) typedResolution {
	return typedResolution{tc: tc, file: tf, info: tf.info}
}

func (t typedResolution) undetermined(reason string) conformance.Reference {
	return conformance.Reference{Resolution: conformance.ResolutionUndetermined, Reason: reason}
}

// invalid explains an expression the type checker could not type.
func (t typedResolution) invalid(what string) conformance.Reference {
	return t.undetermined(fmt.Sprintf("the type checker could not type %s", what))
}

func (t typedResolution) call(c *ast.CallExpr) (conformance.Reference, conformance.Reference) {
	fun := ast.Unparen(c.Fun)
	if tv, ok := t.info.Types[fun]; ok {
		switch {
		case tv.IsType():
			return t.typeRef(tv.Type, types.ExprString(fun)), conformance.Reference{}
		case tv.IsBuiltin():
			return external("", "", calleeIdent(fun)), conformance.Reference{}
		}
	}
	for {
		switch x := fun.(type) {
		case *ast.IndexExpr:
			fun = ast.Unparen(x.X)
			continue
		case *ast.IndexListExpr:
			fun = ast.Unparen(x.X)
			continue
		}
		break
	}
	switch x := fun.(type) {
	case *ast.Ident:
		return t.objectCall(t.info.Uses[x], x.Name), conformance.Reference{}
	case *ast.SelectorExpr:
		if id, ok := ast.Unparen(x.X).(*ast.Ident); ok {
			if pn, ok := t.info.Uses[id].(*types.PkgName); ok {
				if obj := t.info.Uses[x.Sel]; obj != nil || t.tc.sources[pn.Imported().Path()] {
					return t.objectCall(obj, x.Sel.Name), conformance.Reference{}
				}
				// A function of a package arclint knows by name only.
				return t.stubRef(pn.Imported().Path(), "", x.Sel.Name), conformance.Reference{}
			}
		}
		sel, ok := t.info.Selections[x]
		if !ok {
			return t.unselected(x)
		}
		switch sel.Kind() {
		case types.MethodVal, types.MethodExpr:
			fn, ok := sel.Obj().(*types.Func)
			if !ok {
				return t.invalid("the method " + x.Sel.Name), conformance.Reference{}
			}
			return t.methodRef(fn), t.typeRef(sel.Recv(), types.ExprString(x.X))
		case types.FieldVal:
			return t.undetermined(fmt.Sprintf("%s is a field of function type; the function it holds is not declared", x.Sel.Name)), conformance.Reference{}
		}
	}
	return t.undetermined("the called value is an expression, not a declared function"), conformance.Reference{}
}

// unselected resolves a method call the type checker could not select
// because the method belongs to a type arclint knows by name only: the
// type of the value, or one embedded in it, whose methods arclint does
// not read. When no repository type declares a method of the name, the
// call reaches no declaration of the repository: it is external, and
// which type outside the repository declares it is not known.
func (t typedResolution) unselected(x *ast.SelectorExpr) (conformance.Reference, conformance.Reference) {
	what := types.ExprString(x.X)
	var typ types.Type
	if tv, ok := t.info.Types[x.X]; ok {
		typ = tv.Type
	}
	receiver := t.undetermined(fmt.Sprintf("the type of %s comes from a package arclint reads by name only", what))
	if named := stubNamed(typ); named != nil {
		receiver = t.typeRef(typ, what)
	}
	if named := stubNamed(typ); named != nil && !t.tc.sources[named.Obj().Pkg().Path()] {
		// A type outside the repository neither declares nor embeds a
		// repository type's method.
		if reason, ok := t.tc.unknown[named.Obj().Pkg().Path()]; ok {
			return t.undetermined(reason), receiver
		}
		return external("", "", x.Sel.Name), receiver
	}
	if t.tc.methods[x.Sel.Name] {
		return t.undetermined(fmt.Sprintf("%s is not a method arclint can select on %s, and a type in this codebase also declares a method %s", x.Sel.Name, what, x.Sel.Name)), receiver
	}
	return external("", "", x.Sel.Name), receiver
}

// stubRef names a declaration of a package arclint does not
// type-check: external, or undetermined when the import reaches no
// package arclint can read.
func (t typedResolution) stubRef(pkgPath, owner, name string) conformance.Reference {
	if reason, ok := t.tc.unknown[pkgPath]; ok {
		return t.undetermined(reason)
	}
	return external(pkgPath, owner, name)
}

// stubNamed is the defined type of a value, through one pointer.
func stubNamed(typ types.Type) *types.Named {
	if typ == nil {
		return nil
	}
	return namedOf(typ)
}

// objectCall resolves a call through the object an identifier names.
func (t typedResolution) objectCall(obj types.Object, name string) conformance.Reference {
	switch o := obj.(type) {
	case *types.Func:
		if recv := o.Signature().Recv(); recv != nil {
			return t.methodRef(o)
		}
		return t.objectRef(o, "")
	case *types.Builtin:
		return external("", "", o.Name())
	case *types.TypeName:
		return t.objectRef(o, "")
	case *types.Var:
		return t.undetermined(fmt.Sprintf("%s is a function value; the function it holds is not declared", name))
	case nil:
		return t.invalid(name)
	}
	return t.undetermined(fmt.Sprintf("%s does not name a function", name))
}

// methodRef names a method by the type that declares it; a method of
// an interface or struct literal, or of a type local to a function, is
// local.
func (t typedResolution) methodRef(fn *types.Func) conformance.Reference {
	fn = fn.Origin()
	recv := fn.Signature().Recv()
	if recv == nil {
		return t.objectRef(fn, "")
	}
	named := namedOf(recv.Type())
	if named == nil || !fileLevel(named.Obj()) {
		return local(fn.Name())
	}
	return t.objectRef(fn, named.Obj().Name())
}

// typeRef names the declaration of a type a value has: through
// pointers to its alias or defined type, the builtin a basic type is,
// and undetermined for a type parameter, whose type its instantiation
// chooses.
func (t typedResolution) typeRef(typ types.Type, what string) conformance.Reference {
	for {
		p, ok := typ.(*types.Pointer)
		if !ok {
			break
		}
		typ = p.Elem()
	}
	switch x := typ.(type) {
	case *types.Alias:
		return t.objectRef(x.Obj(), "")
	case *types.Named:
		return t.objectRef(x.Origin().Obj(), "")
	case *types.Basic:
		if x.Kind() == types.Invalid {
			return t.invalid(what)
		}
		return external("", "", x.Name())
	case *types.TypeParam:
		return t.undetermined(fmt.Sprintf("%s has the type parameter %s, whose type the instantiation chooses", what, x.Obj().Name()))
	}
	return local(types.TypeString(typ, types.RelativeTo(t.file.pkg)))
}

// objectRef names one object: declared when it is in an observed file
// of a package the toolchain loaded from repository source, external
// when its package is the standard library or a dependency, and
// undetermined when repository source holds it outside the observed
// files.
func (t typedResolution) objectRef(obj types.Object, owner string) conformance.Reference {
	if obj.Pkg() == nil {
		return external("", owner, obj.Name())
	}
	if _, isType := obj.(*types.TypeName); isType && !fileLevel(obj) {
		return local(obj.Name())
	}
	if !t.tc.sources[obj.Pkg().Path()] {
		return t.stubRef(obj.Pkg().Path(), owner, obj.Name())
	}
	rel := t.tc.relative(t.file.fset.Position(obj.Pos()).Filename)
	if !t.tc.observed[rel] {
		return t.undetermined(fmt.Sprintf("%s is declared in a file of this codebase that arclint does not observe", obj.Name()))
	}
	return conformance.Reference{Resolution: conformance.ResolutionDeclared, File: rel, Owner: owner, Name: obj.Name()}
}

// holds lists the declarations a field of the written type holds.
func (t typedResolution) holds(typeExpr ast.Expr) []conformance.Reference {
	tv, ok := t.info.Types[typeExpr]
	if !ok || tv.Type == nil {
		return []conformance.Reference{t.invalid("the field's type " + types.ExprString(typeExpr))}
	}
	var out []conformance.Reference
	seen := map[types.Type]bool{}
	t.collectHolds(tv.Type, types.ExprString(typeExpr), seen, &out)
	return out
}

// fieldType names the declaration a field's own type is, through
// pointers and the repository's own aliases: the type of a value the
// field stores directly. An alias outside the repository is named as
// written; arclint knows such a package by name only.
func (t typedResolution) fieldType(typeExpr ast.Expr) conformance.Reference {
	tv, ok := t.info.Types[typeExpr]
	if !ok || tv.Type == nil {
		return t.invalid("the field's type " + types.ExprString(typeExpr))
	}
	typ := tv.Type
	for {
		switch x := typ.(type) {
		case *types.Pointer:
			typ = x.Elem()
			continue
		case *types.Alias:
			if pkg := x.Obj().Pkg(); pkg != nil && t.tc.sources[pkg.Path()] {
				typ = x.Rhs()
				continue
			}
		}
		break
	}
	return t.typeRef(typ, types.ExprString(typeExpr))
}

// collectHolds walks a type into the declarations its values hold: an
// alias and the type it stands for, a defined type with its type
// arguments and, for a defined collection, its elements, and the
// elements of pointers, slices, arrays, maps,
// channels, and the fields of an unnamed struct. A function or an
// interface literal holds no value of a declared type; a type
// parameter holds what its instantiation names, so the field that
// instantiates it holds those types.
func (t typedResolution) collectHolds(typ types.Type, what string, seen map[types.Type]bool, out *[]conformance.Reference) {
	if seen[typ] {
		return
	}
	seen[typ] = true
	switch x := typ.(type) {
	case *types.Alias:
		*out = append(*out, t.objectRef(x.Obj(), ""))
		for i := range x.TypeArgs().Len() {
			t.collectHolds(x.TypeArgs().At(i), what, seen, out)
		}
		t.collectHolds(x.Rhs(), what, seen, out)
	case *types.Named:
		*out = append(*out, t.objectRef(x.Origin().Obj(), ""))
		// A defined function type, Factory[Pricing] = func() Pricing,
		// holds a function, not its type arguments.
		if _, isFunc := x.Underlying().(*types.Signature); isFunc {
			return
		}
		for i := range x.TypeArgs().Len() {
			t.collectHolds(x.TypeArgs().At(i), what, seen, out)
		}
		// A defined collection, type Lines []OrderLine, holds its
		// elements; a defined struct's fields are its own.
		switch u := x.Underlying().(type) {
		case *types.Pointer, *types.Slice, *types.Array, *types.Map, *types.Chan:
			t.collectHolds(u, what, seen, out)
		}
	case *types.Basic:
		if x.Kind() == types.Invalid {
			*out = append(*out, t.invalid("the field's type "+what))
			return
		}
		*out = append(*out, external("", "", x.Name()))
	case *types.Pointer:
		t.collectHolds(x.Elem(), what, seen, out)
	case *types.Slice:
		t.collectHolds(x.Elem(), what, seen, out)
	case *types.Array:
		t.collectHolds(x.Elem(), what, seen, out)
	case *types.Map:
		t.collectHolds(x.Key(), what, seen, out)
		t.collectHolds(x.Elem(), what, seen, out)
	case *types.Chan:
		t.collectHolds(x.Elem(), what, seen, out)
	case *types.Struct:
		for i := range x.NumFields() {
			t.collectHolds(x.Field(i).Type(), what, seen, out)
		}
	case *types.Signature, *types.Interface, *types.TypeParam, *types.Tuple:
	}
}

// namedOf returns the defined type a receiver has, through a pointer.
func namedOf(typ types.Type) *types.Named {
	if p, ok := typ.(*types.Pointer); ok {
		typ = p.Elem()
	}
	named, _ := types.Unalias(typ).(*types.Named)
	if named == nil {
		return nil
	}
	return named.Origin()
}

func local(name string) conformance.Reference {
	return conformance.Reference{Resolution: conformance.ResolutionLocal, Name: name}
}

// fileLevel reports an object declared in its package's scope, the
// declarations the facts list; a type declared inside a function is
// not one.
func fileLevel(obj types.Object) bool {
	return obj.Pkg() == nil || obj.Parent() == nil || obj.Parent() == obj.Pkg().Scope()
}

func external(pkg, owner, name string) conformance.Reference {
	return conformance.Reference{Resolution: conformance.ResolutionExternal, Package: pkg, Owner: owner, Name: name}
}
