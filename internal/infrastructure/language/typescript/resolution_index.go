package typescript

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// resolutionIndex is a request-local, lexical view of the same pinned AST
// used for declarations. It is never built for imports or syntactic facts.
type resolutionIndex struct {
	imports                 *resolver
	files                   map[string]*resolutionFile
	ambientModules          map[string]bool
	ambientTargets          map[string]bool
	globalQuestions         map[string]string
	externalMembersQuestion bool
	declarationGuardFailure string
}

type resolutionFile struct {
	path                 string
	src                  []byte
	lang                 *gotreesitter.Language
	tree                 *gotreesitter.Tree
	scope                *lexicalScope
	exports              map[string][]exportBinding
	stars                []string
	nodes                map[*gotreesitter.Node]*lexicalScope
	calls                []*gotreesitter.Node
	fields               map[string]*gotreesitter.Node
	types                map[string][]*gotreesitter.Node
	ambient              bool
	ambientModules       []string
	module               bool
	globalAugmentation   bool
	namespaceDeclaration bool
}

type lexicalScope struct {
	parent   *lexicalScope
	bindings map[string][]sourceBinding
	function bool
}

type sourceBinding struct {
	node           *gotreesitter.Node
	spec, imported string
	namespace      bool
	reason         string
	typeOnly       bool
}

type exportBinding struct{ local, spec, imported, reason string }

func newScope(parent *lexicalScope) *lexicalScope {
	return &lexicalScope{parent: parent, bindings: map[string][]sourceBinding{}}
}

func (s *lexicalScope) add(name string, b sourceBinding) {
	if name != "" {
		s.bindings[name] = append(s.bindings[name], b)
	}
}

func (s *lexicalScope) lookup(name string) []sourceBinding {
	for ; s != nil; s = s.parent {
		if bs, ok := s.bindings[name]; ok {
			return bs
		}
	}
	return nil
}

func (f *resolutionFile) kind(n *gotreesitter.Node) string {
	if n == nil {
		return ""
	}
	return n.Type(f.lang)
}

func (f *resolutionFile) text(n *gotreesitter.Node) string {
	if n == nil {
		return ""
	}
	return n.Text(f.src)
}

func (f *resolutionFile) child(n *gotreesitter.Node, field string) *gotreesitter.Node {
	if n == nil {
		return nil
	}
	return n.ChildByFieldName(field, f.lang)
}

func (f *resolutionFile) name(n *gotreesitter.Node) string {
	if name := f.text(f.child(n, tsNameField)); name != "" {
		return name
	}
	if defaultDeclaration(n, f.lang) {
		return tsDefaultKeyword
	}
	return ""
}

func (f *resolutionFile) annotation(n *gotreesitter.Node) *gotreesitter.Node {
	a := f.child(n, tsTypeField)
	if a != nil && a.NamedChildCount() > 0 {
		return a.NamedChild(0)
	}
	return nil
}

func parameterProperty(n *gotreesitter.Node, lang *gotreesitter.Language) bool {
	for i := 0; i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c.Type(lang) == "accessibility_modifier" || c.Type(lang) == "readonly" {
			return true
		}
	}
	return false
}

func hasToken(n *gotreesitter.Node, lang *gotreesitter.Language, token string) bool {
	for i := 0; i < n.ChildCount(); i++ {
		if n.Child(i).Type(lang) == token {
			return true
		}
	}
	return false
}

func fieldKey(owner, name string, line int) string {
	return owner + "\x00" + name + "\x00" + strconv.Itoa(line)
}

func resolveFacts(root string, imports *resolver, facts map[string]conformance.LanguageFacts, calls, fields bool) {
	initializeBuiltins()
	index := &resolutionIndex{imports: imports, files: map[string]*resolutionFile{}, ambientModules: map[string]bool{}, ambientTargets: map[string]bool{}, globalQuestions: map[string]string{}}
	ps := newParsers()
	for rel, fa := range facts {
		if !fa.CallsAvailable && !fa.DeclarationsAvailable {
			continue
		}
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		p, err := ps.forFile(rel)
		if err != nil {
			continue
		}
		tree, err := p.ParseStrict(src)
		if err != nil {
			continue
		}
		f := &resolutionFile{path: rel, src: src, lang: p.Language(), tree: tree, scope: newScope(nil), exports: map[string][]exportBinding{}, nodes: map[*gotreesitter.Node]*lexicalScope{}, fields: map[string]*gotreesitter.Node{}, types: map[string][]*gotreesitter.Node{}}
		f.scope.function = true
		f.index(tree.RootNode(), f.scope, "")
		index.files[rel] = f
		for _, spec := range f.ambientModules {
			index.ambientModules[spec] = true
		}
	}
	index.recordDeclarationGuards(root, ps)
	defer func() {
		for _, f := range index.files {
			f.tree.Release()
		}
	}()
	for rel, f := range index.files {
		fa := facts[rel]
		if calls && fa.CallsAvailable {
			fa.ResolvedCallsAvailable = true
			for i := range fa.Calls {
				if i >= len(f.calls) {
					fa.Calls[i].Target = uncertain("call syntax could not be indexed")
					continue
				}
				fa.Calls[i].Target, fa.Calls[i].Receiver = index.call(f, f.calls[i], f.thisClass(f.calls[i]))
			}
		}
		if fields && fa.DeclarationsAvailable {
			fa.FieldTypesAvailable = true
			for i := range fa.Declarations {
				d := &fa.Declarations[i]
				if d.Kind != tsFieldDeclaration {
					continue
				}
				d.Holds = index.fieldHolds(f, *d)
			}
		}
		facts[rel] = fa
	}
}

func (r *resolutionIndex) fieldHolds(f *resolutionFile, d conformance.Declaration) []conformance.Reference {
	n := f.fields[fieldKey(d.Owner, d.Name, d.StartLine)]
	if n == nil {
		return []conformance.Reference{uncertain("field syntax could not be indexed")}
	}
	if reason := f.unsafeType(d.Owner); reason != "" {
		return []conformance.Reference{uncertain(reason)}
	}
	refs := r.heldTypes(f, f.annotation(n), f.nodes[n], map[string]bool{})
	if !hasToken(n, f.lang, "?") {
		return refs
	}
	undefined := external("", "undefined")
	for _, ref := range refs {
		if ref == undefined {
			return refs
		}
	}
	return append(refs, undefined)
}

func (f *resolutionFile) index(n *gotreesitter.Node, scope *lexicalScope, owner string) {
	kind := f.kind(n)
	switch kind {
	case tsImportStatement:
		f.module = n.Parent() != nil && n.Parent().Type(f.lang) == tsProgram || f.module
		spec := strings.Trim(f.text(f.child(n, "source")), "\"'")
		for i := 0; i < n.NamedChildCount(); i++ {
			c := n.NamedChild(i)
			if f.kind(c) == "import_clause" {
				f.importClause(c, scope, spec, hasToken(n, f.lang, tsTypeField))
			}
		}
	case tsExportStatement:
		f.module = n.Parent() != nil && n.Parent().Type(f.lang) == tsProgram || f.module
		f.exportStatement(n)
	case tsClassExpression:
		name := f.name(n)
		if defaultDeclaration(n, f.lang) {
			scope.add(name, sourceBinding{node: n})
		}
		if name != "" {
			f.types[name] = append(f.types[name], n)
		}
		scope = newScope(scope)
		scope.add(name, sourceBinding{node: n})
		owner = name
	case tsClassDeclaration, tsAbstractClassDeclaration, tsInterfaceDeclaration, tsTypeAliasDeclaration, "enum_declaration":
		scope.add(f.name(n), sourceBinding{node: n})
		f.types[f.name(n)] = append(f.types[f.name(n)], n)
		owner = f.name(n)
		scope = newScope(scope)
	case tsFunctionDeclaration, tsGeneratorFunctionDeclaration:
		scope.add(f.name(n), sourceBinding{node: n})
		scope = newScope(scope)
		scope.function = true
		owner = ""
	case tsFunctionExpression, tsFunction, tsArrowFunction, tsMethodDefinition, tsMethodSignature:
		if defaultDeclaration(n, f.lang) {
			scope.add(tsDefaultKeyword, sourceBinding{node: n})
		}
		scope = newScope(scope)
		scope.function = true
		if kind == tsFunctionExpression || kind == tsFunction {
			scope.add(f.name(n), sourceBinding{node: n})
			owner = ""
		}
	case tsStatementBlock, "for_statement", "for_in_statement", "catch_clause":
		scope = newScope(scope)
		if kind == "catch_clause" {
			f.bindPattern(f.child(n, "parameter"), scope, "catch parameter has no established type")
		}
	case tsVariableDeclarator:
		bindingScope := scope
		if parent := n.Parent(); f.kind(parent) == tsVariableDeclaration && hasToken(parent, f.lang, tsVarKeyword) {
			for bindingScope.parent != nil && !bindingScope.function {
				bindingScope = bindingScope.parent
			}
		}
		name := f.name(n)
		if f.kind(f.child(n, tsNameField)) != tsIdentifier {
			f.bindPattern(f.child(n, tsNameField), bindingScope, "destructured binding has no established type")
		} else {
			bindingScope.add(name, sourceBinding{node: n})
		}
	case tsRequiredParameter, tsOptionalParameter:
		pattern := f.child(n, tsPatternField)
		if f.kind(pattern) == tsIdentifier {
			scope.add(f.text(pattern), sourceBinding{node: n})
		} else {
			f.bindPattern(pattern, scope, "destructured parameter has no established type")
		}
		if parameterProperty(n, f.lang) && owner != "" {
			f.fields[fieldKey(owner, f.text(pattern), int(n.StartPoint().Row)+1)] = n
		}
	case tsTypeParameter:
		scope.add(f.name(n), sourceBinding{node: n, reason: "generic type parameter has no concrete declaration"})
	case tsPublicFieldDefinition, tsFieldDefinition, tsPropertySignature:
		f.fields[fieldKey(owner, f.name(n), int(n.StartPoint().Row)+1)] = n
	case tsCallExpression:
		f.calls = append(f.calls, n)
	case tsInternalModule, tsModule:
		// Namespace and ambient module members need declaration merging, which
		// this adapter deliberately does not pretend to implement.
		f.ambient = true
		if name := f.text(f.child(n, tsNameField)); name == tsGlobalNamespace {
			f.globalAugmentation = true
		} else if name != "" && !strings.HasPrefix(name, "\"") && !strings.HasPrefix(name, "'") {
			f.namespaceDeclaration = true
			scope.add(name, sourceBinding{node: n, reason: "namespace declaration requires declaration merging"})
		}
		if name := f.text(f.child(n, tsNameField)); strings.HasPrefix(name, "\"") || strings.HasPrefix(name, "'") {
			f.ambientModules = append(f.ambientModules, strings.Trim(name, "\"'"))
		}
	}
	f.nodes[n] = scope
	for i := 0; i < n.NamedChildCount(); i++ {
		f.index(n.NamedChild(i), scope, owner)
	}
}

func (f *resolutionFile) bindPattern(n *gotreesitter.Node, s *lexicalScope, reason string) {
	if n == nil {
		return
	}
	if f.kind(n) == tsIdentifier || f.kind(n) == "shorthand_property_identifier_pattern" {
		s.add(f.text(n), sourceBinding{node: n, reason: reason})
		return
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		f.bindPattern(n.NamedChild(i), s, reason)
	}
}

func (f *resolutionFile) importClause(n *gotreesitter.Node, s *lexicalScope, spec string, typeOnly bool) {
	for i := 0; i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		switch f.kind(c) {
		case tsIdentifier:
			s.add(f.text(c), sourceBinding{node: c, spec: spec, imported: tsDefaultKeyword, typeOnly: typeOnly})
		case tsNamespaceImport:
			if c.NamedChildCount() > 0 {
				s.add(f.text(c.NamedChild(0)), sourceBinding{node: c, spec: spec, namespace: true, typeOnly: typeOnly})
			}
		case "named_imports":
			for j := 0; j < c.NamedChildCount(); j++ {
				b := c.NamedChild(j)
				name := f.text(f.child(b, tsNameField))
				alias := f.text(f.child(b, "alias"))
				if alias == "" {
					alias = name
				}
				s.add(alias, sourceBinding{node: b, spec: spec, imported: name, typeOnly: typeOnly || hasToken(b, f.lang, tsTypeField)})
			}
		}
	}
}

func (f *resolutionFile) exportStatement(n *gotreesitter.Node) {
	spec := strings.Trim(f.text(f.child(n, "source")), "\"'")
	for i := 0; i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if f.kind(c) == "namespace_export" {
			name := f.name(c)
			if name == "" && c.NamedChildCount() > 0 {
				name = f.text(c.NamedChild(0))
			}
			f.exports[name] = append(f.exports[name], exportBinding{reason: "namespace re-export is not resolved"})
			return
		}
		if f.kind(c) == "export_clause" {
			for j := 0; j < c.NamedChildCount(); j++ {
				b := c.NamedChild(j)
				local := f.text(f.child(b, tsNameField))
				name := f.text(f.child(b, "alias"))
				if name == "" {
					name = local
				}
				f.exports[name] = append(f.exports[name], exportBinding{local: local, spec: spec, imported: local})
			}
			return
		}
		name := f.name(c)
		if name != "" {
			if hasToken(n, f.lang, tsDefaultKeyword) {
				f.exports[tsDefaultKeyword] = append(f.exports[tsDefaultKeyword], exportBinding{local: name})
			} else {
				f.exports[name] = append(f.exports[name], exportBinding{local: name})
			}
			return
		}
		if f.kind(c) == tsLexicalDeclaration || f.kind(c) == tsVariableDeclaration {
			for j := 0; j < c.NamedChildCount(); j++ {
				name = f.name(c.NamedChild(j))
				if name != "" {
					f.exports[name] = append(f.exports[name], exportBinding{local: name})
				}
			}
			return
		}
		if f.kind(c) == tsIdentifier && hasToken(n, f.lang, tsDefaultKeyword) {
			f.exports[tsDefaultKeyword] = append(f.exports[tsDefaultKeyword], exportBinding{local: f.text(c)})
			return
		}
	}
	if spec != "" && hasToken(n, f.lang, "*") {
		f.stars = append(f.stars, spec)
	}
}

func (f *resolutionFile) unsafeType(name string) string {
	ns := f.types[name]
	if len(ns) > 1 {
		return "declaration merging or duplicate type name is not resolved"
	}
	for _, n := range ns {
		if containsKind(n, f.lang, "decorator") {
			return "decorated type may change members"
		}
	}
	return ""
}

func containsKind(n *gotreesitter.Node, lang *gotreesitter.Language, kind string) bool {
	if n.Type(lang) == kind {
		return true
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		if containsKind(n.NamedChild(i), lang, kind) {
			return true
		}
	}
	return false
}

// TypeScript's anonymous default export has the declared symbol "default".
func defaultDeclaration(n *gotreesitter.Node, lang *gotreesitter.Language) bool {
	parent := n.Parent()
	if parent == nil || parent.Type(lang) != tsExportStatement || !hasToken(parent, lang, tsDefaultKeyword) {
		return false
	}
	switch n.Type(lang) {
	case tsClassExpression, tsClassDeclaration, tsFunctionExpression, tsArrowFunction:
		return true
	}
	return false
}

// A field initializer can have a lexical this without belonging to an
// explicitly declared method. Ordinary nested functions break that binding.
func (f *resolutionFile) thisClass(n *gotreesitter.Node) string {
	for parent := n.Parent(); parent != nil; parent = parent.Parent() {
		switch f.kind(parent) {
		case tsFunctionDeclaration, tsGeneratorFunctionDeclaration, tsFunctionExpression, tsFunction:
			return ""
		case tsClassDeclaration, tsAbstractClassDeclaration, tsClassExpression:
			return f.name(parent)
		}
	}
	return ""
}

func (f *resolutionFile) arrayTypeName(n *gotreesitter.Node) string {
	if parent := n.Parent(); parent != nil && parent.Type(f.lang) == tsReadonlyType {
		return tsReadonlyArray
	}
	return tsBuiltinArray
}
