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

func (w *pyWalker) add(kind, name, owner string, n *gotreesitter.Node) {
	if name == "" {
		return
	}
	w.out.Decls = append(w.out.Decls, lang.Decl{
		Kind: kind, Name: name, Owner: owner,
		Exported:  !strings.HasPrefix(name, "_"),
		StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1,
	})
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
		w.add(kind, name, owner, n)
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
