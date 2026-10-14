package typescript

import (
	"path"
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/wixregiga/arclint/internal/domain/conformance"
)

func uncertain(reason string) conformance.Reference {
	return conformance.Reference{Resolution: conformance.ResolutionUndetermined, Reason: reason}
}

func declared(file, owner, name string) conformance.Reference {
	return conformance.Reference{Resolution: conformance.ResolutionDeclared, File: file, Owner: owner, Name: name}
}

func external(pkg, name string) conformance.Reference {
	return conformance.Reference{Resolution: conformance.ResolutionExternal, Package: pkg, Name: name}
}

func resolved(ref conformance.Reference) bool {
	return ref.Resolution == conformance.ResolutionDeclared || ref.Resolution == conformance.ResolutionExternal
}

// Builtin identities and member ownership come from the pinned compiler
// declaration tables in builtin_declarations_gen.go. Lexical bindings win.

func (r *resolutionIndex) imported(f *resolutionFile, spec, name string, seen map[string]bool) conformance.Reference {
	if r.declarationGuardFailure != "" {
		return uncertain(r.declarationGuardFailure)
	}
	if r.ambientModules[spec] {
		return uncertain("ambient modules or namespaces require declaration merging")
	}
	class, _, target := r.imports.classify(r.imports.owners[f.path], path.Dir(f.path), spec)
	switch class {
	case conformance.ImportExternal, conformance.ImportStdlib:
		return external(spec, name)
	case conformance.ImportInternal:
		if r.ambientTargets[target] {
			return uncertain("ambient module augmentation requires declaration merging")
		}
		if strings.HasSuffix(target, ".d.ts") {
			return uncertain("import reaches a .d.ts-only declaration the adapter skips")
		}
		next := r.files[target]
		if next == nil {
			return uncertain("import target is not an observed TypeScript declaration")
		}
		return r.exported(next, name, seen)
	case conformance.ImportUnknown, conformance.ImportCgo:
	}
	return uncertain("import " + spec + " is not resolved by the configured import graph")
}

func (r *resolutionIndex) exported(f *resolutionFile, name string, seen map[string]bool) conformance.Reference {
	key := f.path + "#export#" + name
	if seen[key] {
		return uncertain("cyclic re-export cannot be resolved")
	}
	seen[key] = true
	defer delete(seen, key)
	if bs := f.exports[name]; len(bs) > 0 {
		if len(bs) != 1 {
			return uncertain("conflicting explicit exports cannot be resolved")
		}
		b := bs[0]
		if b.reason != "" {
			return uncertain(b.reason)
		}
		if b.spec != "" {
			return r.imported(f, b.spec, b.imported, seen)
		}
		return r.named(f, b.local, f.scope, seen)
	}
	if name == tsDefaultKeyword {
		return uncertain("module has no declared default export")
	}
	var found conformance.Reference
	for _, spec := range f.stars {
		ref := r.imported(f, spec, name, seen)
		if ref.Resolution == conformance.ResolutionUndetermined {
			if strings.HasPrefix(ref.Reason, "module does not export ") {
				continue
			}
			return ref
		}
		if !found.IsZero() && found != ref {
			return uncertain("conflicting star exports cannot be resolved")
		}
		found = ref
	}
	if !found.IsZero() {
		return found
	}
	return uncertain("module does not export " + name)
}

func (r *resolutionIndex) named(f *resolutionFile, name string, s *lexicalScope, seen map[string]bool) conformance.Reference {
	if r.declarationGuardFailure != "" {
		return uncertain(r.declarationGuardFailure)
	}
	if name == "any" || name == "unknown" {
		return uncertain(name + " has no established declaration")
	}
	if f.ambient {
		return uncertain("ambient modules or namespaces require declaration merging")
	}
	// Qualified names are supported only through an explicit namespace import.
	if left, right, ok := strings.Cut(name, "."); ok {
		bs := s.lookup(left)
		if len(bs) == 1 && bs[0].namespace {
			return r.imported(f, bs[0].spec, right, seen)
		}
		return uncertain("qualified type is not an imported namespace")
	}
	bs := s.lookup(name)
	if len(bs) == 0 || !f.module {
		if r.declarationGuardFailure != "" {
			return uncertain(r.declarationGuardFailure)
		}
		if reason := r.globalQuestions[name]; reason != "" {
			return uncertain(reason)
		}
	}
	if len(bs) > 1 {
		return uncertain("declaration merging or duplicate binding is not resolved")
	}
	if len(bs) == 1 {
		return r.namedBinding(f, name, bs[0], seen)
	}

	if builtinTypes[name] || builtinValues[name] {
		return external("", name)
	}
	return uncertain(name + " is neither declared in the file nor imported")
}

func (r *resolutionIndex) namedBinding(f *resolutionFile, name string, b sourceBinding, seen map[string]bool) conformance.Reference {
	if b.reason != "" {
		return uncertain(b.reason)
	}
	if b.namespace {
		return uncertain("namespace binding needs a named member")
	}
	if b.spec != "" {
		return r.imported(f, b.spec, b.imported, seen)
	}
	if reason := f.unsafeType(name); reason != "" {
		return uncertain(reason)
	}
	return declared(f.path, "", name)
}

func (r *resolutionIndex) namedType(f *resolutionFile, name string, s *lexicalScope) conformance.Reference {
	ref := r.named(f, name, s, map[string]bool{})
	if ref.Resolution == conformance.ResolutionDeclared {
		_, n := r.typeNode(ref)
		if n == nil {
			return uncertain("type name " + name + " does not reach an indexed type declaration")
		}
	}
	return ref
}

func (r *resolutionIndex) typeNode(ref conformance.Reference) (*resolutionFile, *gotreesitter.Node) {
	f := r.files[ref.File]
	if f == nil {
		return nil, nil
	}
	ns := f.types[ref.Name]
	if len(ns) != 1 {
		return f, nil
	}
	return f, ns[0]
}

// heldTypes walks type syntax, including alias contents and generic arguments,
// but stops at callable signatures. Unsupported operators contribute an open
// question instead of a guessed operand declaration.
func (r *resolutionIndex) heldTypes(f *resolutionFile, n *gotreesitter.Node, s *lexicalScope, seen map[string]bool) []conformance.Reference {
	if n == nil {
		return []conformance.Reference{uncertain("field has no written type annotation")}
	}
	var refs []conformance.Reference
	add := func(more []conformance.Reference) {
		for _, ref := range more {
			duplicate := false
			for _, old := range refs {
				if old == ref {
					duplicate = true
					break
				}
			}
			if !duplicate {
				refs = append(refs, ref)
			}
		}
	}
	switch f.kind(n) {
	case tsFunctionType, tsConstructorType:
		return nil
	case tsTypeIdentifier, tsIdentifier, "predefined_type", tsNestedTypeIdentifier:
		ref := r.namedType(f, f.text(n), s)
		add([]conformance.Reference{ref})
		if ref.Resolution == conformance.ResolutionExternal && ref.Package == "" {
			if reason := builtinComputations[ref.Name]; reason != "" {
				add([]conformance.Reference{uncertain(reason)})
			}
		}
		if ref.Resolution == conformance.ResolutionDeclared {
			next, alias := r.typeNode(ref)
			if next != nil && next.kind(alias) == tsTypeAliasDeclaration {
				key := ref.File + "#type#" + ref.Name
				if seen[key] {
					add([]conformance.Reference{uncertain("cyclic type alias cannot be resolved")})
				} else {
					seen[key] = true
					add(r.heldTypes(next, next.child(alias, tsValueField), next.nodes[alias], seen))
					delete(seen, key)
				}
			}
		}
	case tsGenericType:
		add(r.heldTypes(f, f.child(n, tsNameField), s, seen))
		if r.callableAlias(f, f.child(n, tsNameField), s) {
			// Factory<Pricing> = () => Pricing holds a function, not
			// its type arguments.
			break
		}
		if args := f.child(n, "type_arguments"); args != nil {
			for i := 0; i < args.NamedChildCount(); i++ {
				add(r.heldTypes(f, args.NamedChild(i), s, seen))
			}
		}
	case "array_type":
		add([]conformance.Reference{external("", f.arrayTypeName(n))})
		if n.NamedChildCount() > 0 {
			add(r.heldTypes(f, n.NamedChild(0), s, seen))
		}
	case tsTupleType:
		add([]conformance.Reference{external("", f.arrayTypeName(n))})
		for i := 0; i < n.NamedChildCount(); i++ {
			add(r.heldTypes(f, n.NamedChild(i), s, seen))
		}
	case "union_type", "intersection_type", tsParenthesizedType, "optional_type", "rest_type", tsReadonlyType, "type_arguments":
		for i := 0; i < n.NamedChildCount(); i++ {
			add(r.heldTypes(f, n.NamedChild(i), s, seen))
		}
	case "literal_type":
		if n.NamedChildCount() > 0 {
			switch f.kind(n.NamedChild(0)) {
			case tsNullType, tsUndefinedType:
				add([]conformance.Reference{external("", f.text(n.NamedChild(0)))})
			case tsStringType:
				add([]conformance.Reference{external("", tsStringType)})
			case tsNumber:
				name := tsNumber
				if strings.HasSuffix(f.text(n.NamedChild(0)), "n") {
					name = tsBigintType
				}
				add([]conformance.Reference{external("", name)})
			case "true", "false":
				add([]conformance.Reference{external("", tsBooleanType)})
			default:
				add([]conformance.Reference{uncertain("literal type is not resolved")})
			}
		}
	case tsObjectType:
		if containsKind(n, f.lang, tsMappedTypeClause) {
			return []conformance.Reference{uncertain("mapped type is not resolved")}
		}
		for i := 0; i < n.NamedChildCount(); i++ {
			c := n.NamedChild(i)
			if f.kind(c) == tsPropertySignature {
				add(r.heldTypes(f, f.annotation(c), s, seen))
			}
		}
	case tsConditionalType:
		add([]conformance.Reference{uncertain("conditional type is not resolved")})
	case tsLookupType, tsIndexedAccessType:
		add([]conformance.Reference{uncertain("indexed-access type is not resolved")})
	case tsMappedTypeClause:
		add([]conformance.Reference{uncertain("mapped type is not resolved")})
	default:
		add([]conformance.Reference{uncertain("unsupported type expression: " + normalizeType(f.text(n)))})
	}
	return refs
}

// callableAlias reports a type name that names an alias of a function
// type, through aliases of aliases.
func (r *resolutionIndex) callableAlias(f *resolutionFile, name *gotreesitter.Node, s *lexicalScope) bool {
	for range maxAliasDepth {
		if name == nil {
			return false
		}
		ref := r.namedType(f, f.text(name), s)
		if ref.Resolution != conformance.ResolutionDeclared {
			return false
		}
		next, alias := r.typeNode(ref)
		if next == nil || next.kind(alias) != tsTypeAliasDeclaration {
			return false
		}
		value := next.child(alias, tsValueField)
		for value != nil && next.kind(value) == tsParenthesizedType && value.NamedChildCount() > 0 {
			value = value.NamedChild(0)
		}
		if value == nil {
			return false
		}
		switch next.kind(value) {
		case tsFunctionType, tsConstructorType:
			return true
		case tsGenericType:
			f, name, s = next, next.child(value, tsNameField), next.nodes[alias]
		case tsTypeIdentifier, tsNestedTypeIdentifier:
			f, name, s = next, value, next.nodes[alias]
		default:
			return false
		}
	}
	return false
}

func (r *resolutionIndex) receiverType(f *resolutionFile, n *gotreesitter.Node, s *lexicalScope) conformance.Reference {
	if n == nil {
		return uncertain("receiver has no declared type")
	}
	switch f.kind(n) {
	case "type_annotation", tsParenthesizedType, tsReadonlyType:
		if n.NamedChildCount() > 0 {
			return r.receiverType(f, n.NamedChild(0), s)
		}
	case tsTypeIdentifier, tsIdentifier, tsNestedTypeIdentifier, "predefined_type", tsMemberExpression:
		return r.namedType(f, f.text(n), s)
	case tsGenericType:
		return r.receiverType(f, f.child(n, tsNameField), s)
	case "array_type":
		return external("", f.arrayTypeName(n))
	case tsTupleType:
		return uncertain("tuple receiver has no single named declaring type")
	case tsFunctionType:
		return uncertain("function type has no declared receiver")
	case tsConditionalType:
		return uncertain("conditional type is not resolved")
	case tsLookupType, tsIndexedAccessType:
		return uncertain("indexed-access type is not resolved")
	case tsObjectType:
		if containsKind(n, f.lang, tsMappedTypeClause) {
			return uncertain("mapped type is not resolved")
		}
	}
	return uncertain("receiver annotation has no single established type: " + normalizeType(f.text(n)))
}

func (r *resolutionIndex) bindingType(f *resolutionFile, b sourceBinding) conformance.Reference {
	if b.reason != "" {
		return uncertain(b.reason)
	}
	if a := f.annotation(b.node); a != nil {
		return r.receiverType(f, a, f.nodes[b.node])
	}
	v := f.child(b.node, tsValueField)
	if f.kind(v) != tsNewExpression {
		return uncertain("binding has no written annotation or resolvable new initializer")
	}
	ref := r.named(f, f.text(f.child(v, tsConstructor)), f.nodes[b.node], map[string]bool{})
	if !resolved(ref) {
		return ref
	}
	if ref.Resolution == conformance.ResolutionExternal {
		if ref.Package != "" {
			return ref
		}
		instance, ok := builtinConstructors[ref.Name]
		if !ok {
			return uncertain("new target is not a resolvable class")
		}
		if instance == "" || instance == "__type" {
			return uncertain("builtin constructor " + ref.Name + " has no single named instance type")
		}
		return external("", instance)
	}
	next, n := r.typeNode(ref)
	if next == nil || (next.kind(n) != tsClassDeclaration && next.kind(n) != tsAbstractClassDeclaration && next.kind(n) != tsClassExpression) {
		return uncertain("new target is not a resolvable class")
	}
	bindingName := f.name(b.node)
	if bindingName == "" {
		bindingName = f.text(f.child(b.node, tsPatternField))
	}
	if f.reassigned(bindingName) {
		return uncertain("inferred new binding is reassigned")
	}
	return ref
}

func (f *resolutionFile) reassigned(name string) bool {
	var visit func(*gotreesitter.Node) bool
	visit = func(n *gotreesitter.Node) bool {
		if f.kind(n) == "assignment_expression" || f.kind(n) == "augmented_assignment_expression" {
			if f.text(f.child(n, "left")) == name {
				return true
			}
		}
		if f.kind(n) == "update_expression" && strings.Contains(f.text(n), name) {
			return true
		}
		for i := 0; i < n.NamedChildCount(); i++ {
			if visit(n.NamedChild(i)) {
				return true
			}
		}
		return false
	}
	return visit(f.tree.RootNode())
}

func (r *resolutionIndex) importedValueType(ref conformance.Reference) conformance.Reference {
	if ref.Resolution != conformance.ResolutionDeclared {
		return ref
	}
	f := r.files[ref.File]
	if f == nil {
		return uncertain("imported value declaration is not indexed")
	}
	bs := f.scope.lookup(ref.Name)
	if len(bs) != 1 {
		return uncertain("imported value binding is ambiguous")
	}
	switch f.kind(bs[0].node) {
	case tsClassDeclaration, tsAbstractClassDeclaration, tsClassExpression:
		return ref
	}
	return r.bindingType(f, bs[0])
}

func (r *resolutionIndex) valueType(f *resolutionFile, n *gotreesitter.Node, s *lexicalScope, owner string) conformance.Reference {
	switch f.kind(n) {
	case tsThis:
		if owner == "" {
			return uncertain("this has no enclosing declared class")
		}
		if f.ambient {
			return uncertain("ambient modules or namespaces require declaration merging")
		}
		if r.declarationGuardFailure != "" {
			return uncertain(r.declarationGuardFailure)
		}
		if reason := f.unsafeType(owner); reason != "" {
			return uncertain(reason)
		}
		if !f.module {
			if reason := r.globalQuestions[owner]; reason != "" {
				return uncertain(reason)
			}
		}
		if len(f.types[owner]) != 1 {
			return uncertain("this has no indexed lexical class")
		}
		return declared(f.path, "", owner)
	case tsIdentifier:
		bs := s.lookup(f.text(n))
		if len(bs) > 1 {
			return uncertain("declaration merging or duplicate binding is not resolved")
		}
		if len(bs) == 1 {
			b := bs[0]
			if b.spec != "" {
				if b.typeOnly {
					return uncertain("type-only import cannot be used as a value")
				}
				ref := r.named(f, f.text(n), s, map[string]bool{})
				return r.importedValueType(ref)
			}
			switch f.kind(b.node) {
			case tsClassDeclaration, tsAbstractClassDeclaration, tsClassExpression:
				return r.named(f, f.text(n), s, map[string]bool{})
			}
			return r.bindingType(f, b)
		}
		ref := r.named(f, f.text(n), s, map[string]bool{})
		if ref.Resolution == conformance.ResolutionExternal && ref.Package == "" {
			if name := builtinValueTypes[ref.Name]; name != "" {
				ref.Name = name
			}
		}
		return ref
	case tsMemberExpression:
		base := f.child(n, tsObjectKeyword)
		member := f.text(f.child(n, tsPropertyField))
		if strings.HasPrefix(member, "#") && f.kind(base) != tsThis {
			return uncertain("private field access requires lexical private-name resolution")
		}
		if f.kind(base) == tsIdentifier {
			bs := s.lookup(f.text(base))
			if len(bs) == 1 && bs[0].namespace {
				return r.importedValueType(r.imported(f, bs[0].spec, member, map[string]bool{}))
			}
		}
		receiver := r.valueType(f, base, s, owner)
		if !resolved(receiver) {
			return receiver
		}
		next, field, _, reason := r.member(receiver, member, r.staticReceiver(f, base, s), map[string]bool{})
		if reason != "" {
			return uncertain(reason)
		}
		if field == nil || next == nil {
			return uncertain("receiver member has no written type annotation")
		}
		if next.annotation(field) == nil {
			return uncertain("field has no written type annotation")
		}
		return r.bindingType(next, sourceBinding{node: field})
	case "parenthesized_expression":
		if n.NamedChildCount() == 1 {
			return r.valueType(f, n.NamedChild(0), s, owner)
		}
	}
	return uncertain("receiver expression has no established declared type")
}

// member returns the actual declaring type while the call keeps its static
// Receiver. It follows known extends clauses, never implements clauses.
func (r *resolutionIndex) externalMember(ref conformance.Reference, name string) (conformance.Reference, string) {
	if ref.Package != "" {
		if r.externalMembersQuestion {
			return ref, "ambient namespace can merge external declaration members"
		}
		return ref, ""
	}
	owner, ok := builtinMemberOwners[ref.Name+"."+name]
	if !ok {
		return ref, "builtin member " + ref.Name + "." + name + " is not declared"
	}
	if reason := r.globalQuestions[ref.Name]; reason != "" {
		return ref, reason
	}
	if reason := r.globalQuestions[owner]; reason != "" {
		return ref, reason
	}
	return external("", owner), ""
}

func (r *resolutionIndex) member(ref conformance.Reference, name string, static bool, seen map[string]bool) (*resolutionFile, *gotreesitter.Node, conformance.Reference, string) {
	if ref.Resolution == conformance.ResolutionExternal {
		target, reason := r.externalMember(ref, name)
		return nil, nil, target, reason
	}
	key := ref.File + "#member#" + ref.Name
	if seen[key] {
		return nil, nil, ref, "cyclic inheritance or alias cannot be resolved"
	}
	seen[key] = true
	defer delete(seen, key)
	f, n := r.typeNode(ref)
	if f == nil || n == nil {
		return nil, nil, ref, "receiver is not an indexed type declaration"
	}
	if reason := f.unsafeType(ref.Name); reason != "" {
		return nil, nil, ref, reason
	}
	if f.kind(n) == tsTypeAliasDeclaration {
		target := r.receiverType(f, f.child(n, tsValueField), f.nodes[n])
		if !resolved(target) {
			return nil, nil, ref, target.Reason
		}
		return r.member(target, name, static, seen)
	}
	if f.kind(n) != tsClassDeclaration && f.kind(n) != tsAbstractClassDeclaration && f.kind(n) != tsClassExpression && f.kind(n) != tsInterfaceDeclaration {
		return nil, nil, ref, "receiver declaration has no class or interface member model"
	}
	if m := f.ownMember(n, name, static); m != nil {
		return f, m, ref, ""
	}

	var bases []*gotreesitter.Node
	for i := 0; i < n.NamedChildCount(); i++ {
		h := n.NamedChild(i)
		if f.kind(h) == "extends_type_clause" {
			for j := 0; j < h.NamedChildCount(); j++ {
				bases = append(bases, h.NamedChild(j))
			}
		}
		if f.kind(h) == "class_heritage" {
			for j := 0; j < h.NamedChildCount(); j++ {
				c := h.NamedChild(j)
				if f.kind(c) == "extends_clause" {
					v := f.child(c, tsValueField)
					if v == nil && c.NamedChildCount() > 0 {
						v = c.NamedChild(0)
					}
					bases = append(bases, v)
				}
			}
		}
	}
	for _, base := range bases {
		b := r.receiverType(f, base, f.nodes[n])
		if !resolved(b) {
			return nil, nil, ref, b.Reason
		}
		bf, bn, br, reason := r.member(b, name, static, seen)
		if reason != "" {
			return nil, nil, ref, reason
		}
		if bn != nil || br.Resolution == conformance.ResolutionExternal {
			return bf, bn, br, ""
		}
	}
	return f, nil, ref, ""
}

func (f *resolutionFile) ownMember(n *gotreesitter.Node, name string, static bool) *gotreesitter.Node {
	body := f.child(n, tsBodyField)
	if body == nil {
		return nil
	}
	for i := 0; i < body.NamedChildCount(); i++ {
		m := body.NamedChild(i)
		if f.name(m) == name && hasToken(m, f.lang, "static") == static {
			return m
		}
		if static || f.kind(m) != tsMethodDefinition || f.name(m) != tsConstructor {
			continue
		}
		params := f.child(m, tsParametersField)
		if params == nil {
			continue
		}
		for j := 0; j < params.NamedChildCount(); j++ {
			p := params.NamedChild(j)
			if parameterProperty(p, f.lang) && f.text(f.child(p, tsPatternField)) == name {
				return p
			}
		}
	}
	return nil
}

func (r *resolutionIndex) staticReceiver(f *resolutionFile, base *gotreesitter.Node, s *lexicalScope) bool {
	if f.kind(base) == tsThis {
		for n := base.Parent(); n != nil; n = n.Parent() {
			if f.kind(n) == tsMethodDefinition {
				return hasToken(n, f.lang, "static")
			}
		}
	}
	if f.kind(base) == tsMemberExpression {
		object := f.child(base, tsObjectKeyword)
		bs := s.lookup(f.text(object))
		if f.kind(object) == tsIdentifier && len(bs) == 1 && bs[0].namespace {
			original := r.imported(f, bs[0].spec, f.text(f.child(base, tsPropertyField)), map[string]bool{})
			next, n := r.typeNode(original)
			return next != nil && (next.kind(n) == tsClassDeclaration || next.kind(n) == tsAbstractClassDeclaration || next.kind(n) == tsClassExpression)
		}
		return false
	}
	if f.kind(base) != tsIdentifier {
		return false
	}
	bs := s.lookup(f.text(base))
	if len(bs) != 1 {
		return builtinTypes[f.text(base)]
	}
	b := bs[0]
	if b.spec != "" {
		original := r.named(f, f.text(base), s, map[string]bool{})
		_, n := r.typeNode(original)
		return n != nil && (f.kind(n) == tsClassDeclaration || f.kind(n) == tsAbstractClassDeclaration || f.kind(n) == tsClassExpression)
	}
	return f.kind(b.node) == tsClassDeclaration || f.kind(b.node) == tsAbstractClassDeclaration || f.kind(b.node) == tsClassExpression
}

func (r *resolutionIndex) importedFunction(ref conformance.Reference) conformance.Reference {
	if ref.Resolution != conformance.ResolutionDeclared {
		return ref
	}
	f := r.files[ref.File]
	if f == nil {
		return uncertain("callee declaration is not indexed")
	}
	bs := f.scope.lookup(ref.Name)
	if len(bs) != 1 {
		return uncertain("callee declaration is ambiguous")
	}
	n := bs[0].node
	v := f.child(n, tsValueField)
	if f.kind(n) == tsFunctionDeclaration || f.kind(n) == tsGeneratorFunctionDeclaration || f.kind(v) == tsArrowFunction || f.kind(v) == tsFunctionExpression || f.kind(v) == tsFunction || f.kind(n) == tsFunctionExpression || f.kind(n) == tsArrowFunction {
		return ref
	}
	return uncertain("callee has no declared function implementation")
}

func (r *resolutionIndex) directCall(f *resolutionFile, fn *gotreesitter.Node, s *lexicalScope) conformance.Reference {
	name := f.text(fn)
	bs := s.lookup(name)
	if len(bs) == 1 && bs[0].typeOnly {
		return uncertain("type-only import cannot be used as a value")
	}
	if len(bs) == 1 && bs[0].spec == "" {
		b := bs[0]
		v := f.child(b.node, tsValueField)
		if f.kind(b.node) != tsFunctionDeclaration && f.kind(b.node) != tsGeneratorFunctionDeclaration && f.kind(v) != tsArrowFunction && f.kind(v) != tsFunctionExpression && f.kind(v) != tsFunction {
			return uncertain("callee has no declared function implementation")
		}
	}
	ref := r.named(f, name, s, map[string]bool{})
	if len(bs) == 1 && bs[0].spec != "" {
		ref = r.importedFunction(ref)
	}
	return ref
}

func (r *resolutionIndex) call(f *resolutionFile, n *gotreesitter.Node, owner string) (conformance.Reference, conformance.Reference) {
	fn := f.child(n, tsFunction)
	s := f.nodes[n]
	if f.kind(fn) == "subscript_expression" {
		ref := uncertain("computed member call")
		return ref, ref
	}
	if f.kind(fn) == tsIdentifier {
		return r.directCall(f, fn, s), conformance.Reference{}
	}

	if f.kind(fn) != tsMemberExpression {
		return uncertain("call expression is not an established function or member"), conformance.Reference{}
	}
	base := f.child(fn, tsObjectKeyword)
	name := f.text(f.child(fn, tsPropertyField))
	if strings.HasPrefix(name, "#") {
		return uncertain("private member call requires lexical private-name resolution"), r.valueType(f, base, s, owner)
	}
	if f.kind(base) == tsIdentifier {
		bs := s.lookup(f.text(base))
		if len(bs) == 1 && bs[0].namespace {
			if bs[0].typeOnly {
				return uncertain("type-only import cannot be used as a value"), conformance.Reference{}
			}
			return r.importedFunction(r.imported(f, bs[0].spec, name, map[string]bool{})), conformance.Reference{}
		}
	}
	receiver := r.valueType(f, base, s, owner)
	if !resolved(receiver) {
		return receiver, receiver
	}
	static := r.staticReceiver(f, base, s)
	_, member, target, reason := r.member(receiver, name, static, map[string]bool{})
	if reason != "" {
		return uncertain(reason), receiver
	}
	if member == nil && target.Resolution == conformance.ResolutionDeclared {
		prototype := tsBuiltinObject
		if static {
			prototype = tsBuiltinFunction
		}
		if owner, ok := builtinMemberOwners[prototype+"."+name]; ok {
			target = external("", owner)
		}
	}
	target.Owner, target.Name = target.Name, name

	return target, receiver
}
