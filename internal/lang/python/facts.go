package python

import (
	"fmt"
	"strings"
	"sync"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"

	"github.com/wixregiga/arclint/internal/lang"
)

// Facts extracts declaration facts from one Python file via the pinned
// pure-Go tree-sitter runtime (M8 ADR). Classes, functions, and methods
// carry line spans; visibility follows the leading-underscore
// convention.
func Facts(path string, src []byte) *lang.FileFacts {
	out := &lang.FileFacts{Path: path}
	pyLang, err := pythonLanguage()
	if err != nil {
		out.ParseError = err.Error()
		return out
	}
	parser := gotreesitter.NewParser(pyLang)
	tree, err := parser.ParseStrict(src)
	if err != nil {
		out.ParseError = fmt.Sprintf("parse: %v", err)
		return out
	}
	w := &pyWalker{lang: pyLang, src: src, out: out}
	w.walk(tree.RootNode(), "", false)
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
	out  *lang.FileFacts
}

func (w *pyWalker) name(n *gotreesitter.Node) string {
	if id := n.ChildByFieldName("name", w.lang); id != nil {
		return id.Text(w.src)
	}
	return ""
}

func (w *pyWalker) add(kind, name, owner string, n *gotreesitter.Node) *lang.Decl {
	if name == "" {
		return nil
	}
	w.out.Decls = append(w.out.Decls, lang.Decl{
		Kind: kind, Name: name, Owner: owner,
		Exported:  !strings.HasPrefix(name, "_"),
		StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1,
	})
	return &w.out.Decls[len(w.out.Decls)-1]
}

// param maps one node under parameters to the neutral shape. Splat
// parameters keep their prefix in Name ("*args", "**kwargs") so the two
// flavors stay distinguishable; `self` and `cls` stay in the list —
// dropping them would be interpretation, not syntax.
func (w *pyWalker) param(pn *gotreesitter.Node) (lang.Param, bool) {
	p := lang.Param{}
	switch pn.Type(w.lang) {
	case "identifier":
		p.Name = pn.Text(w.src)
	case "typed_parameter":
		// First named child is the pattern: identifier or a splat.
		if pn.NamedChildCount() > 0 {
			first := pn.NamedChild(0)
			switch first.Type(w.lang) {
			case "identifier":
				p.Name = first.Text(w.src)
			case "list_splat_pattern", "dictionary_splat_pattern":
				p.Name = first.Text(w.src)
				p.Variadic = true
			}
		}
		if t := pn.ChildByFieldName("type", w.lang); t != nil {
			p.Type = lang.NormalizeType(t.Text(w.src))
		}
	case "default_parameter", "typed_default_parameter":
		p.Optional = true
		if nm := pn.ChildByFieldName("name", w.lang); nm != nil && nm.Type(w.lang) == "identifier" {
			p.Name = nm.Text(w.src)
		} else if pn.NamedChildCount() > 0 && pn.NamedChild(0).Type(w.lang) == "identifier" {
			p.Name = pn.NamedChild(0).Text(w.src)
		}
		if t := pn.ChildByFieldName("type", w.lang); t != nil {
			p.Type = lang.NormalizeType(t.Text(w.src))
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
func (w *pyWalker) signature(n *gotreesitter.Node) ([]lang.Param, []string) {
	var params []lang.Param
	if ps := n.ChildByFieldName("parameters", w.lang); ps != nil {
		for i := 0; i < ps.NamedChildCount(); i++ {
			if p, ok := w.param(ps.NamedChild(i)); ok {
				params = append(params, p)
			}
		}
	}
	var results []string
	if rt := n.ChildByFieldName("return_type", w.lang); rt != nil {
		results = []string{lang.NormalizeType(rt.Text(w.src))}
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
