package typescript

import (
	"fmt"
	"strings"
	"sync"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// declarationFacts carries one file's extracted declarations, kept
// separate from import scanning so a strict-parse failure here yields
// honest fact absence without poisoning the import view.
type declarationFacts struct {
	Decls      []conformance.Declaration
	Calls      []conformance.Call
	ParseError string
}

// normalizeType collapses whitespace runs in source-text types so
// multi-line annotations compare stably.
func normalizeType(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// parsers is one worker goroutine's tree-sitter parsers, one per
// grammar, built on first use. A Parser is not safe for concurrent
// use and costs more to construct than a typical file costs to parse,
// so each worker keeps its own and reuses them file after file.
type parsers struct {
	byGrammar map[string]*gotreesitter.Parser
}

func newParsers() *parsers { return &parsers{byGrammar: map[string]*gotreesitter.Parser{}} }

// forFile returns the worker's parser for the grammar the file's
// extension selects.
func (ps *parsers) forFile(path string) (*gotreesitter.Parser, error) {
	name := grammarFor(path)
	if p, ok := ps.byGrammar[name]; ok {
		return p, nil
	}
	lang, err := languageNamed(name)
	if err != nil {
		return nil, err
	}
	p := gotreesitter.NewParser(lang)
	ps.byGrammar[name] = p
	return p, nil
}

// extractDeclarations extracts declaration facts from one TypeScript/TSX/JavaScript
// file via the pinned pure-Go tree-sitter runtime (pinned grammar, deterministic). The walker
// is arclint-owned: the shapes below are the fact schema, and the
// per-language tests are its contract.
func extractDeclarations(ps *parsers, path string, src []byte) *declarationFacts {
	out := &declarationFacts{}
	parser, err := ps.forFile(path)
	if err != nil {
		out.ParseError = err.Error()
		return out
	}
	tree, err := parser.ParseStrict(src)
	if err != nil {
		out.ParseError = fmt.Sprintf("parse: %v", err)
		return out
	}
	w := &tsWalker{lang: parser.Language(), src: src, out: out}
	w.walk(tree.RootNode(), "", false)
	w.walkCalls(tree.RootNode(), "", "", callScope{})
	return out
}

// grammarFor names the embedded grammar a file's extension selects.
func grammarFor(path string) string {
	switch {
	case strings.HasSuffix(path, ".tsx"):
		return "tsx"
	case strings.HasSuffix(path, ".ts"):
		return "typescript"
	}
	return "javascript"
}

var (
	tsLangMu    sync.Mutex
	tsLangCache = map[string]*gotreesitter.Language{}
)

// languageNamed loads one embedded grammar once per process; a
// Language is immutable once loaded and shared by every parser.
func languageNamed(name string) (*gotreesitter.Language, error) {
	tsLangMu.Lock()
	defer tsLangMu.Unlock()
	if l, ok := tsLangCache[name]; ok {
		return l, nil
	}
	entry := grammars.DetectLanguageByName(name)
	if entry == nil {
		return nil, fmt.Errorf("grammar %q is not embedded in this build", name)
	}
	l := entry.Language()
	if l == nil {
		return nil, fmt.Errorf("grammar %q failed to load", name)
	}
	tsLangCache[name] = l
	return l, nil
}

type tsWalker struct {
	lang *gotreesitter.Language
	src  []byte
	out  *declarationFacts
}

func (w *tsWalker) text(n *gotreesitter.Node) string { return n.Text(w.src) }

func (w *tsWalker) name(n *gotreesitter.Node) string {
	if id := n.ChildByFieldName(tsNameField, w.lang); id != nil {
		return w.text(id)
	}
	if defaultDeclaration(n, w.lang) {
		return tsDefaultKeyword
	}
	return ""
}

func (w *tsWalker) add(kind, name, owner string, exported bool, n *gotreesitter.Node) *conformance.Declaration {
	if name == "" {
		return nil
	}
	w.out.Decls = append(w.out.Decls, conformance.Declaration{
		Kind: kind, Name: name, Owner: owner, Exported: exported,
		Generic:   n.ChildByFieldName("type_parameters", w.lang) != nil,
		StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1,
	})
	return &w.out.Decls[len(w.out.Decls)-1]
}

// annotType unwraps a type_annotation (": T") to the normalized text of
// the type inside it.
func (w *tsWalker) annotType(annot *gotreesitter.Node) string {
	if annot == nil || annot.NamedChildCount() == 0 {
		return ""
	}
	return normalizeType(w.text(annot.NamedChild(0)))
}

// param maps one parameter node to the neutral shape. The TS grammar
// wraps every parameter in required_parameter / optional_parameter with
// fields pattern, type, value; the JS grammar puts the patterns
// (identifier, assignment_pattern, rest_pattern) directly under
// formal_parameters. Both shapes land here.
func (w *tsWalker) param(pn *gotreesitter.Node) conformance.DeclarationParam {
	p := conformance.DeclarationParam{}
	pattern := pn
	switch pn.Type(w.lang) {
	case tsRequiredParameter, tsOptionalParameter:
		p.Optional = pn.Type(w.lang) == tsOptionalParameter
		p.Type = w.annotType(pn.ChildByFieldName(tsTypeField, w.lang))
		if pn.ChildByFieldName(tsValueField, w.lang) != nil {
			p.Optional = true // a default value makes the parameter optional
		}
		if pat := pn.ChildByFieldName(tsPatternField, w.lang); pat != nil {
			pattern = pat
		}
	case "assignment_pattern": // JS default: x = 3
		p.Optional = true
		if left := pn.ChildByFieldName("left", w.lang); left != nil {
			pattern = left
		}
	}
	switch pattern.Type(w.lang) {
	case tsIdentifier, tsThis:
		p.Name = w.text(pattern)
	case "rest_pattern":
		p.Variadic = true
		if pattern.NamedChildCount() > 0 && pattern.NamedChild(0).Type(w.lang) == tsIdentifier {
			p.Name = w.text(pattern.NamedChild(0))
		}
	}
	// Destructuring patterns keep Name "": there is no single name at
	// the syntactic tier.
	return p
}

// signature extracts params and results from any function-shaped node:
// function_declaration, method_definition, method_signature,
// arrow_function, function_expression.
func (w *tsWalker) signature(fn *gotreesitter.Node) ([]conformance.DeclarationParam, []string) {
	var params []conformance.DeclarationParam
	if fp := fn.ChildByFieldName(tsParametersField, w.lang); fp != nil {
		for i := 0; i < fp.NamedChildCount(); i++ {
			params = append(params, w.param(fp.NamedChild(i)))
		}
	} else if single := fn.ChildByFieldName("parameter", w.lang); single != nil {
		// Arrow shorthand: x => x
		params = append(params, w.param(single))
	}
	var results []string
	if t := w.annotType(fn.ChildByFieldName("return_type", w.lang)); t != "" {
		results = []string{t}
	}
	return params, results
}

// memberPublic reports whether a class member lacks private/protected
// modifiers and a #-private name.
func (w *tsWalker) memberPublic(n *gotreesitter.Node) bool {
	for i := 0; i < n.ChildCount(); i++ {
		c := n.Child(i)
		switch c.Type(w.lang) {
		case "accessibility_modifier":
			if t := w.text(c); t == "private" || t == "protected" {
				return false
			}
		case "private_property_identifier":
			return false
		}
	}
	if id := n.ChildByFieldName(tsNameField, w.lang); id != nil && id.Type(w.lang) == "private_property_identifier" {
		return false
	}
	return true
}

// walk visits declaration-bearing nodes. exported tracks whether the
// current subtree sits under an export_statement.
func (w *tsWalker) walk(n *gotreesitter.Node, owner string, exported bool) {
	switch n.Type(w.lang) {
	case tsExportStatement:
		for i := 0; i < n.NamedChildCount(); i++ {
			w.walk(n.NamedChild(i), owner, true)
		}
		return
	case tsClassDeclaration, tsAbstractClassDeclaration, tsClassExpression:
		name := w.name(n)
		if d := w.add("class", name, owner, exported, n); d != nil {
			d.Abstract = n.Type(w.lang) == tsAbstractClassDeclaration
		}
		if body := n.ChildByFieldName(tsBodyField, w.lang); body != nil {
			for i := 0; i < body.NamedChildCount(); i++ {
				w.walk(body.NamedChild(i), name, exported)
			}
		}
		return
	case tsInterfaceDeclaration:
		name := w.name(n)
		w.add("interface", name, owner, exported, n)
		if body := n.ChildByFieldName(tsBodyField, w.lang); body != nil {
			for i := 0; i < body.NamedChildCount(); i++ {
				m := body.NamedChild(i)
				switch m.Type(w.lang) {
				case tsMethodSignature:
					if d := w.add("method", w.name(m), name, true, m); d != nil {
						d.Params, d.Results = w.signature(m)
					}
				case tsPropertySignature:
					w.add(tsFieldDeclaration, w.name(m), name, true, m)
				}
			}
		}
		return
	case tsTypeAliasDeclaration:
		w.add(tsTypeField, w.name(n), owner, exported, n)
		return
	case "enum_declaration":
		w.add("enum", w.name(n), owner, exported, n)
		return
	case tsFunctionDeclaration, tsGeneratorFunctionDeclaration, tsFunctionExpression, tsArrowFunction:
		if w.name(n) == "" {
			return
		}
		if d := w.add("func", w.name(n), owner, exported, n); d != nil {
			d.Params, d.Results = w.signature(n)
		}
		return
	case tsMethodDefinition, tsAbstractMethodSignature:
		// Member visibility is the member's own: private/protected/#name
		// make it unexported regardless of the class's export. An
		// abstract method is declared without a body.
		if d := w.add("method", w.name(n), owner, w.memberPublic(n), n); d != nil {
			d.Params, d.Results = w.signature(n)
		}
		if w.name(n) == tsConstructor {
			if params := n.ChildByFieldName(tsParametersField, w.lang); params != nil {
				for i := 0; i < params.NamedChildCount(); i++ {
					p := params.NamedChild(i)
					if parameterProperty(p, w.lang) {
						w.add(tsFieldDeclaration, w.param(p).Name, owner, w.memberPublic(p), p)
					}
				}
			}
		}
		return
	case tsAbstractMethodSignature:
		// An abstract member is declared without a body; its signature is
		// the operation every subclass completes.
		if d := w.add("method", w.name(n), owner, w.memberPublic(n), n); d != nil {
			d.Params, d.Results = w.signature(n)
		}
		return
	case tsPublicFieldDefinition, tsFieldDefinition:
		// A field is storage each instance holds; a static property
		// belongs to the class, the way Python's ClassVar does.
		if !hasToken(n, w.lang, "static") {
			w.add(tsFieldDeclaration, w.name(n), owner, w.memberPublic(n), n)
		}
		return
	case tsLexicalDeclaration, tsVariableDeclaration:
		for i := 0; i < n.NamedChildCount(); i++ {
			d := n.NamedChild(i)
			if d.Type(w.lang) != tsVariableDeclarator {
				continue
			}
			name := w.name(d)
			kind := "const"
			if strings.HasPrefix(w.text(n), tsVarKeyword) || strings.HasPrefix(w.text(n), "let") {
				kind = tsVarKeyword
			}
			var fnNode *gotreesitter.Node
			if v := d.ChildByFieldName(tsValueField, w.lang); v != nil {
				switch v.Type(w.lang) {
				case tsArrowFunction, tsFunctionExpression, tsFunction:
					kind = "func"
					fnNode = v
				}
			}
			decl := w.add(kind, name, owner, exported, d)
			if decl != nil && fnNode != nil {
				// The function, not the variable it is bound to, declares
				// the signature and the type parameters.
				decl.Params, decl.Results = w.signature(fnNode)
				decl.Generic = fnNode.ChildByFieldName("type_parameters", w.lang) != nil
			}
			if v := d.ChildByFieldName(tsValueField, w.lang); v != nil && v.Type(w.lang) == tsClassExpression && w.name(v) != "" {
				w.walk(v, owner, exported)
			}
		}
		return
	}
	// Descend only through containers that can hold declarations.
	switch n.Type(w.lang) {
	case tsProgram, tsStatementBlock, tsModule, tsInternalModule, "ambient_declaration":
		for i := 0; i < n.NamedChildCount(); i++ {
			w.walk(n.NamedChild(i), owner, exported)
		}
	}
}

// callScope is where calls run: the enclosing function and the class
// owning it.
type callScope struct {
	enclosing, owner string
}

// walkCalls records every call with the function it runs in. defined
// is the scope a class's decorators run in: the one around the class,
// since a decorator runs when the class is defined, not when its
// method does.
func (w *tsWalker) walkCalls(n *gotreesitter.Node, enclosing, owner string, defined callScope) {
	t := n.Type(w.lang)
	next := enclosing
	nextOwner := owner
	switch t {
	case "decorator":
		enclosing, owner = defined.enclosing, defined.owner
		next, nextOwner = enclosing, owner
	case tsClassDeclaration, tsAbstractClassDeclaration, tsClassExpression:
		defined = callScope{enclosing: enclosing, owner: owner}
		nextOwner = w.name(n)
		next = ""
	case tsMethodDefinition, tsMethodSignature:
		if n.Parent() != nil && n.Parent().Type(w.lang) != tsClassBody && n.Parent().Type(w.lang) != "interface_body" && n.Parent().Type(w.lang) != tsObjectType {
			// A method of an object literal belongs to no class: inside
			// a function it is part of that function, as an arrow is;
			// elsewhere it is its own ownerless function.
			if enclosing == "" {
				next, nextOwner = w.name(n), ""
			}
			break
		}
		if name := w.name(n); name != "" {
			next = name
		}
	case tsFunctionDeclaration, tsGeneratorFunctionDeclaration:
		nextOwner = ""
		if name := w.name(n); name != "" {
			next = name
		}
	case tsArrowFunction, tsFunctionExpression, tsFunction:
		if defaultDeclaration(n, w.lang) {
			next = tsDefaultKeyword
			nextOwner = ""
		}
		// A named function expression is its own function; an anonymous
		// one is part of the function around it, as an arrow is.
		if t == tsFunctionExpression || t == tsFunction {
			if name := w.name(n); name != "" {
				next, nextOwner = name, ""
			}
		}
	case tsVariableDeclarator:
		if v := n.ChildByFieldName(tsValueField, w.lang); v != nil {
			switch v.Type(w.lang) {
			case tsArrowFunction, tsFunctionExpression, tsFunction:
				if name := w.name(n); name != "" {
					next = name
				}
			}
		}
	case tsCallExpression:
		if callee := w.callCallee(n); callee != "" {
			w.out.Calls = append(w.out.Calls, conformance.Call{
				Callee:         callee,
				Line:           int(n.StartPoint().Row) + 1,
				Enclosing:      enclosing,
				EnclosingOwner: callOwner(enclosing, owner),
			})
		}
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if t == tsClassBody && staticMember(c, w.lang) {
			// A static field or static block runs when the class is
			// defined, in the scope around it.
			w.walkCalls(c, defined.enclosing, defined.owner, defined)
			continue
		}
		if c.Type(w.lang) == tsComputedName && definedOutside(n, c, w.lang) {
			// A member's computed name runs where its class or object
			// is defined: around the class, or in the enclosing function.
			if n.Parent() != nil && n.Parent().Type(w.lang) == tsClassBody {
				w.walkCalls(c, defined.enclosing, defined.owner, defined)
			} else {
				w.walkCalls(c, enclosing, owner, defined)
			}
			continue
		}
		w.walkCalls(c, next, nextOwner, defined)
	}
}

func (w *tsWalker) callCallee(n *gotreesitter.Node) string {
	fn := n.ChildByFieldName(tsFunction, w.lang)
	if fn == nil {
		return ""
	}
	switch fn.Type(w.lang) {
	case tsIdentifier:
		return w.text(fn)
	case tsMemberExpression:
		if prop := fn.ChildByFieldName(tsPropertyField, w.lang); prop != nil {
			return w.text(prop)
		}
	}
	return normalizeType(w.text(fn))
}

func callOwner(enclosing, owner string) string {
	if enclosing == "" {
		return ""
	}
	return owner
}

// staticMember reports a class member that runs when the class is
// defined: a static field or a static block.
func staticMember(n *gotreesitter.Node, lang *gotreesitter.Language) bool {
	switch n.Type(lang) {
	case "class_static_block":
		return true
	case tsPublicFieldDefinition, tsFieldDefinition:
		return hasToken(n, lang, "static")
	}
	return false
}
