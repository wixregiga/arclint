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
	ParseError string
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
	w.walkCalls(tree.RootNode(), "")
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
	lang *gotreesitter.Language
	src  []byte
	out  *declarationFacts
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
		StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1,
	})
	return &w.out.Decls[len(w.out.Decls)-1]
}

// nodeIdentifier is the tree-sitter node type of a bare identifier.
const nodeIdentifier = "identifier"

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
		if t := pn.ChildByFieldName("type", w.lang); t != nil {
			p.Type = normalizeType(t.Text(w.src))
		}
	case "default_parameter", "typed_default_parameter":
		p.Optional = true
		if nm := pn.ChildByFieldName("name", w.lang); nm != nil && nm.Type(w.lang) == nodeIdentifier {
			p.Name = nm.Text(w.src)
		} else if pn.NamedChildCount() > 0 && pn.NamedChild(0).Type(w.lang) == nodeIdentifier {
			p.Name = pn.NamedChild(0).Text(w.src)
		}
		if t := pn.ChildByFieldName("type", w.lang); t != nil {
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
	case "decorated_definition":
		if def := n.ChildByFieldName("definition", w.lang); def != nil {
			w.walk(def, owner, inFunction)
		}
		return
	case "class_definition":
		name := w.name(n)
		w.add("class", name, owner, n)
		if body := n.ChildByFieldName("body", w.lang); body != nil {
			for i := 0; i < body.NamedChildCount(); i++ {
				w.walk(body.NamedChild(i), name, false)
			}
		}
		return
	case "function_definition":
		name := w.name(n)
		kind := "method"
		if owner == "" || inFunction {
			kind = "func"
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
	case "module", "block", "if_statement", "try_statement", "with_statement":
		for i := 0; i < n.NamedChildCount(); i++ {
			w.walk(n.NamedChild(i), owner, inFunction)
		}
	}
}

func (w *pyWalker) walkCalls(n *gotreesitter.Node, enclosing string) {
	t := n.Type(w.lang)
	next := enclosing
	switch t {
	case "function_definition":
		if name := w.name(n); name != "" {
			next = name
		}
	case "call":
		if callee := w.callCallee(n); callee != "" && enclosing != "" {
			w.out.Calls = append(w.out.Calls, conformance.Call{
				Callee:    callee,
				Line:      int(n.StartPoint().Row) + 1,
				Enclosing: enclosing,
			})
		}
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		w.walkCalls(n.NamedChild(i), next)
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
	case "attribute":
		if attr := fn.ChildByFieldName("attribute", w.lang); attr != nil {
			return attr.Text(w.src)
		}
	}
	return ""
}
