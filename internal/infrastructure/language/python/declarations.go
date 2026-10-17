package python

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
	CallNodes  []*gotreesitter.Node
	ParseError string
	walker     *pyWalker
	root       *gotreesitter.Node
	// module is the file's lexical scopes, collected once the
	// declarations and fields are known.
	module *pyModule
}

// normalizeType collapses whitespace runs in source-text types so
// multi-line annotations compare stably.
func normalizeType(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// parsers is one worker goroutine's tree-sitter parser, built on
// first use. A Parser is not safe for concurrent use and costs more
// to construct than a typical file costs to parse, so each worker
// keeps its own and reuses it file after file.
type parsers struct {
	python *gotreesitter.Parser
}

func newParsers() *parsers { return &parsers{} }

func (ps *parsers) forPython() (*gotreesitter.Parser, error) {
	if ps.python != nil {
		return ps.python, nil
	}
	lang, err := pythonLanguage()
	if err != nil {
		return nil, err
	}
	ps.python = gotreesitter.NewParser(lang)
	return ps.python, nil
}

// extractDeclarations extracts declaration facts from one Python file via the pinned
// pure-Go tree-sitter runtime (pinned grammar, deterministic). Classes, functions, and methods
// carry line spans; visibility follows the leading-underscore
// convention.
func extractDeclarations(ps *parsers, src []byte) *declarationFacts {
	out := &declarationFacts{}
	parser, err := ps.forPython()
	if err != nil {
		out.ParseError = err.Error()
		return out
	}
	tree, err := parser.ParseStrict(src)
	if err != nil {
		out.ParseError = fmt.Sprintf("parse: %v", err)
		return out
	}
	w := &pyWalker{lang: parser.Language(), src: src, out: out}
	w.walk(tree.RootNode(), "", false)
	out.walker, out.root = w, tree.RootNode()
	w.imports = importBindings(w, tree.RootNode())
	w.bound, w.aliases = map[string]bool{}, map[string]string{}
	w.rebound = moduleRebindings(w, tree.RootNode())
	w.fields(tree.RootNode(), fieldScope{})
	w.walkCalls(tree.RootNode(), "", "", "")
	out.module = newModule("", out)
	return out
}

var (
	pyLangOnce sync.Once
	pyLang     *gotreesitter.Language
	pyLangErr  error
)

func pythonLanguage() (*gotreesitter.Language, error) {
	pyLangOnce.Do(func() {
		entry := grammars.DetectLanguageByName("python")
		if entry == nil {
			pyLangErr = fmt.Errorf("grammar \"python\" is not embedded in this build")
			return
		}
		pyLang = entry.Language()
		if pyLang == nil {
			pyLangErr = fmt.Errorf("grammar \"python\" failed to load")
		}
	})
	return pyLang, pyLangErr
}

type pyWalker struct {
	lang    *gotreesitter.Language
	src     []byte
	out     *declarationFacts
	imports []importBinding
	// rebound names a module-level name bound more than once, by a
	// second import or by an assignment, def, or class, so an import of
	// it does not decide what it means.
	rebound map[string]bool
	// bound names every name a module-level assignment, def, or class
	// binds, and aliases the plain ones, cm = classmethod, by what they
	// were assigned.
	bound   map[string]bool
	aliases map[string]string
}

func (w *pyWalker) name(n *gotreesitter.Node) string {
	if id := n.ChildByFieldName("name", w.lang); id != nil {
		return id.Text(w.src)
	}
	return ""
}

func (w *pyWalker) add(kind, name, owner string, n *gotreesitter.Node) *conformance.Declaration {
	if name == "" {
		return nil
	}
	w.out.Decls = append(w.out.Decls, conformance.Declaration{
		Kind: kind, Name: name, Owner: owner,
		Exported:  !strings.HasPrefix(name, "_"),
		Generic:   n.ChildByFieldName("type_parameters", w.lang) != nil,
		StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1,
	})
	return &w.out.Decls[len(w.out.Decls)-1]
}

// nodeIdentifier is the tree-sitter node type of a bare identifier.
const (
	nodeType            = "type"
	pythonNone          = "None"
	bindingImport       = "import"
	nodeDecorator       = "decorator"
	builtinStaticmethod = "staticmethod"
	builtinClassmethod  = "classmethod"
	builtinSetattr      = "setattr"

	builtinSuper      = "super"
	nodeString        = "string"
	builtinsModule    = "builtins"
	nodeIdentifier    = "identifier"
	nodeAttribute     = "attribute"
	nodeAssignment    = "assignment"
	nodeClass         = "class_definition"
	nodeFunction      = "function_definition"
	nodeDecorated     = "decorated_definition"
	declField         = "field"
	nodeCall          = "call"
	nodeSubscript     = "subscript"
	nodeImport        = "import_statement"
	nodeImportFrom    = "import_from_statement"
	nodeAugmented     = "augmented_assignment"
	nodeAsPattern     = "as_pattern"
	nodeForInClause   = "for_in_clause"
	bindingFunction   = "func"
	declClass         = "class"
	declMethod        = "method"
	bindingUnknown    = "unknown"
	bindingAnnotation = "annotation"
	// bindingVariadic is a *args parameter: always a tuple.
	bindingVariadic = "variadic"
	bindingSelf     = "self"
	typingModule    = "typing"
)

// param maps one node under parameters to the neutral shape. Splat
// parameters keep their prefix in Name ("*args", "**kwargs") so the two
// flavors stay distinguishable; `self` and `cls` stay in the list;
// dropping them would be interpretation, not syntax.
func (w *pyWalker) param(pn *gotreesitter.Node) (conformance.DeclarationParam, bool) {
	p := conformance.DeclarationParam{}
	switch pn.Type(w.lang) {
	case nodeIdentifier:
		p.Name = pn.Text(w.src)
	case "typed_parameter":
		// First named child is the pattern: identifier or a splat.
		if pn.NamedChildCount() > 0 {
			first := pn.NamedChild(0)
			switch first.Type(w.lang) {
			case nodeIdentifier:
				p.Name = first.Text(w.src)
			case "list_splat_pattern", "dictionary_splat_pattern":
				p.Name = first.Text(w.src)
				p.Variadic = true
			}
		}
		if t := pn.ChildByFieldName(nodeType, w.lang); t != nil {
			p.Type = normalizeType(t.Text(w.src))
		}
	case "default_parameter", "typed_default_parameter":
		p.Optional = true
		if nm := pn.ChildByFieldName("name", w.lang); nm != nil && nm.Type(w.lang) == nodeIdentifier {
			p.Name = nm.Text(w.src)
		} else if pn.NamedChildCount() > 0 && pn.NamedChild(0).Type(w.lang) == nodeIdentifier {
			p.Name = pn.NamedChild(0).Text(w.src)
		}
		if t := pn.ChildByFieldName(nodeType, w.lang); t != nil {
			p.Type = normalizeType(t.Text(w.src))
		}
	case "list_splat_pattern", "dictionary_splat_pattern":
		p.Name = pn.Text(w.src)
		p.Variadic = true
	default:
		// positional_separator "/", keyword_separator "*", and any
		// pattern this tier does not model.
		return p, false
	}
	return p, true
}

// signature extracts params and the annotated return type of one
// function_definition.
func (w *pyWalker) signature(n *gotreesitter.Node) ([]conformance.DeclarationParam, []string) {
	var params []conformance.DeclarationParam
	if ps := n.ChildByFieldName("parameters", w.lang); ps != nil {
		for i := 0; i < ps.NamedChildCount(); i++ {
			if p, ok := w.param(ps.NamedChild(i)); ok {
				params = append(params, p)
			}
		}
	}
	var results []string
	if rt := n.ChildByFieldName("return_type", w.lang); rt != nil {
		results = []string{normalizeType(rt.Text(w.src))}
	}
	return params, results
}

// walk visits definitions. owner is the enclosing class or function
// name; inFunction distinguishes nested defs from methods.
func (w *pyWalker) walk(n *gotreesitter.Node, owner string, inFunction bool) {
	switch n.Type(w.lang) {
	case nodeDecorated:
		if def := n.ChildByFieldName("definition", w.lang); def != nil {
			w.walk(def, owner, inFunction)
		}
		return
	case nodeClass:
		name := w.name(n)
		w.add(declClass, name, owner, n)
		if body := n.ChildByFieldName("body", w.lang); body != nil {
			for i := 0; i < body.NamedChildCount(); i++ {
				w.walk(body.NamedChild(i), name, false)
			}
		}
		return
	case nodeFunction:
		name := w.name(n)
		kind := declMethod
		if owner == "" || inFunction {
			kind = bindingFunction
		}
		if d := w.add(kind, name, owner, n); d != nil {
			d.Params, d.Results = w.signature(n)
		}
		if body := n.ChildByFieldName("body", w.lang); body != nil {
			for i := 0; i < body.NamedChildCount(); i++ {
				w.walk(body.NamedChild(i), name, true)
			}
		}
		return
	case "module", "block", "if_statement", "elif_clause", "else_clause",
		"try_statement", "except_clause", "except_group_clause", "finally_clause",
		"with_statement", "for_statement", "while_statement", "match_statement", "case_clause":
		for i := 0; i < n.NamedChildCount(); i++ {
			w.walk(n.NamedChild(i), owner, inFunction)
		}
	}
}

// dottedName spells a name or a chain of attributes as dots join it,
// whatever spaces or parentheses the source writes around its parts:
// (abc) . ABC is abc.ABC. Any other expression spells "".
func (w *pyWalker) dottedName(n *gotreesitter.Node) string {
	n = w.unparenthesized(n)
	if n == nil {
		return ""
	}
	switch n.Type(w.lang) {
	case nodeIdentifier:
		return n.Text(w.src)
	case nodeAttribute:
		object, attribute := w.dottedName(n.ChildByFieldName("object", w.lang)), n.ChildByFieldName("attribute", w.lang)
		if object == "" || attribute == nil {
			return ""
		}
		return object + "." + attribute.Text(w.src)
	}
	return ""
}

// unparenthesized is the expression inside any parentheses written
// around it, whatever comments sit beside it: (ABC) names ABC.
func (w *pyWalker) unparenthesized(n *gotreesitter.Node) *gotreesitter.Node {
	for n != nil && n.Type(w.lang) == "parenthesized_expression" {
		var inner *gotreesitter.Node
		for i := 0; i < n.NamedChildCount(); i++ {
			c := n.NamedChild(i)
			if c.Type(w.lang) == "comment" {
				continue
			}
			if inner != nil {
				return n
			}
			inner = c
		}
		if inner == nil {
			return n
		}
		n = inner
	}
	return n
}

// walkCalls records every call with the function it runs in and the
// class that function belongs to. class names the class whose body the
// walk stands in directly, whose functions are its methods; a class
// body itself runs in the scope around it, so a class defined inside a
// method runs its body's calls in that method.
func (w *pyWalker) walkCalls(n *gotreesitter.Node, enclosing, owner, class string) {
	t := n.Type(w.lang)
	switch t {
	case nodeClass:
		class = w.name(n)
	case nodeFunction:
		// Defaults and annotations run where the function is defined;
		// only the body runs inside it.
		next, bodyOwner := enclosing, class
		if name := w.name(n); name != "" {
			next = name
		}
		body := n.ChildByFieldName("body", w.lang)
		for i := 0; i < n.NamedChildCount(); i++ {
			c := n.NamedChild(i)
			if body != nil && c.StartByte() == body.StartByte() && c.EndByte() == body.EndByte() {
				w.walkCalls(c, next, bodyOwner, "")
			} else {
				w.walkCalls(c, enclosing, owner, class)
			}
		}
		return
	case nodeCall:
		if callee := w.callCallee(n); callee != "" {
			callOwner := owner
			if enclosing == "" {
				callOwner = ""
			}
			w.out.CallNodes = append(w.out.CallNodes, n)
			w.out.Calls = append(w.out.Calls, conformance.Call{
				Callee:         callee,
				Line:           int(n.StartPoint().Row) + 1,
				Enclosing:      enclosing,
				EnclosingOwner: callOwner,
			})
		}
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		w.walkCalls(n.NamedChild(i), enclosing, owner, class)
	}
}

func (w *pyWalker) callCallee(n *gotreesitter.Node) string {
	fn := n.ChildByFieldName("function", w.lang)
	if fn == nil {
		return ""
	}
	switch fn.Type(w.lang) {
	case nodeIdentifier:
		return fn.Text(w.src)
	case nodeAttribute:
		if attr := fn.ChildByFieldName(nodeAttribute, w.lang); attr != nil {
			return attr.Text(w.src)
		}
	}
	return fn.Text(w.src)
}
