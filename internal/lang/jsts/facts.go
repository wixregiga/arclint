package jsts

import (
	"fmt"
	"strings"
	"sync"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"

	"github.com/wixregiga/arclint/internal/lang"
)

// Facts extracts declaration facts from one TypeScript/TSX/JavaScript
// file via the pinned pure-Go tree-sitter runtime (M8 ADR). The walker
// is arclint-owned: the shapes below are the fact schema, and the
// per-language tests are its contract.
func Facts(path string, src []byte) *lang.FileFacts {
	out := &lang.FileFacts{Path: path}
	tsLang, err := languageFor(path)
	if err != nil {
		out.ParseError = err.Error()
		return out
	}
	parser := gotreesitter.NewParser(tsLang)
	tree, err := parser.ParseStrict(src)
	if err != nil {
		out.ParseError = fmt.Sprintf("parse: %v", err)
		return out
	}
	w := &tsWalker{lang: tsLang, src: src, out: out}
	w.walk(tree.RootNode(), "", false)
	return out
}

var (
	tsLangMu    sync.Mutex
	tsLangCache = map[string]*gotreesitter.Language{}
)

// languageFor loads the grammar for a file's extension once per process.
func languageFor(path string) (*gotreesitter.Language, error) {
	name := "javascript"
	switch {
	case strings.HasSuffix(path, ".tsx"):
		name = "tsx"
	case strings.HasSuffix(path, ".ts"):
		name = "typescript"
	}
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
	out  *lang.FileFacts
}

func (w *tsWalker) text(n *gotreesitter.Node) string { return n.Text(w.src) }

func (w *tsWalker) name(n *gotreesitter.Node) string {
	if id := n.ChildByFieldName("name", w.lang); id != nil {
		return w.text(id)
	}
	return ""
}

func (w *tsWalker) add(kind, name, owner string, exported bool, n *gotreesitter.Node) *lang.Decl {
	if name == "" {
		return nil
	}
	w.out.Decls = append(w.out.Decls, lang.Decl{
		Kind: kind, Name: name, Owner: owner, Exported: exported,
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
	return lang.NormalizeType(w.text(annot.NamedChild(0)))
}

// param maps one parameter node to the neutral shape. The TS grammar
// wraps every parameter in required_parameter / optional_parameter with
// fields pattern, type, value; the JS grammar puts the patterns
// (identifier, assignment_pattern, rest_pattern) directly under
// formal_parameters. Both shapes land here.
func (w *tsWalker) param(pn *gotreesitter.Node) lang.Param {
	p := lang.Param{}
	pattern := pn
	switch pn.Type(w.lang) {
	case "required_parameter", "optional_parameter":
		p.Optional = pn.Type(w.lang) == "optional_parameter"
		p.Type = w.annotType(pn.ChildByFieldName("type", w.lang))
		if pn.ChildByFieldName("value", w.lang) != nil {
			p.Optional = true // a default value makes the parameter optional
		}
		if pat := pn.ChildByFieldName("pattern", w.lang); pat != nil {
			pattern = pat
		}
	case "assignment_pattern": // JS default: x = 3
		p.Optional = true
		if left := pn.ChildByFieldName("left", w.lang); left != nil {
			pattern = left
		}
	}
	switch pattern.Type(w.lang) {
	case "identifier", "this":
		p.Name = w.text(pattern)
	case "rest_pattern":
		p.Variadic = true
		if pattern.NamedChildCount() > 0 && pattern.NamedChild(0).Type(w.lang) == "identifier" {
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
func (w *tsWalker) signature(fn *gotreesitter.Node) ([]lang.Param, []string) {
	var params []lang.Param
	if fp := fn.ChildByFieldName("parameters", w.lang); fp != nil {
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
	if id := n.ChildByFieldName("name", w.lang); id != nil && id.Type(w.lang) == "private_property_identifier" {
		return false
	}
	return true
}

// walk visits declaration-bearing nodes. exported tracks whether the
// current subtree sits under an export_statement.
func (w *tsWalker) walk(n *gotreesitter.Node, owner string, exported bool) {
	switch n.Type(w.lang) {
	case "export_statement":
		for i := 0; i < n.NamedChildCount(); i++ {
			w.walk(n.NamedChild(i), owner, true)
		}
		return
	case "class_declaration", "abstract_class_declaration":
		name := w.name(n)
		w.add("class", name, owner, exported, n)
		if body := n.ChildByFieldName("body", w.lang); body != nil {
			for i := 0; i < body.NamedChildCount(); i++ {
				w.walk(body.NamedChild(i), name, exported)
			}
		}
		return
	case "interface_declaration":
		name := w.name(n)
		w.add("interface", name, owner, exported, n)
		if body := n.ChildByFieldName("body", w.lang); body != nil {
			for i := 0; i < body.NamedChildCount(); i++ {
				m := body.NamedChild(i)
				switch m.Type(w.lang) {
				case "method_signature":
					if d := w.add("method", w.name(m), name, true, m); d != nil {
						d.Params, d.Results = w.signature(m)
					}
				case "property_signature":
					w.add("field", w.name(m), name, true, m)
				}
			}
		}
		return
	case "type_alias_declaration":
		w.add("type", w.name(n), owner, exported, n)
		return
	case "enum_declaration":
		w.add("enum", w.name(n), owner, exported, n)
		return
	case "function_declaration", "generator_function_declaration":
		if d := w.add("func", w.name(n), owner, exported, n); d != nil {
			d.Params, d.Results = w.signature(n)
		}
		return
	case "method_definition":
		// Member visibility is the member's own: private/protected/#name
		// make it unexported regardless of the class's export.
		if d := w.add("method", w.name(n), owner, w.memberPublic(n), n); d != nil {
			d.Params, d.Results = w.signature(n)
		}
		return
	case "public_field_definition", "field_definition":
		w.add("field", w.name(n), owner, w.memberPublic(n), n)
		return
	case "lexical_declaration", "variable_declaration":
		for i := 0; i < n.NamedChildCount(); i++ {
			d := n.NamedChild(i)
			if d.Type(w.lang) != "variable_declarator" {
				continue
			}
			name := w.name(d)
			kind := "const"
			if strings.HasPrefix(w.text(n), "var") || strings.HasPrefix(w.text(n), "let") {
				kind = "var"
			}
			var fnNode *gotreesitter.Node
			if v := d.ChildByFieldName("value", w.lang); v != nil {
				switch v.Type(w.lang) {
				case "arrow_function", "function_expression", "function":
					kind = "func"
					fnNode = v
				}
			}
			decl := w.add(kind, name, owner, exported, d)
			if decl != nil && fnNode != nil {
				decl.Params, decl.Results = w.signature(fnNode)
			}
		}
		return
	}
	// Descend only through containers that can hold declarations.
	switch n.Type(w.lang) {
	case "program", "statement_block", "module", "internal_module", "ambient_declaration":
		for i := 0; i < n.NamedChildCount(); i++ {
			w.walk(n.NamedChild(i), owner, exported)
		}
	}
}
