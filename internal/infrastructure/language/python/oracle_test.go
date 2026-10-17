//go:build pyoracle

package python

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/odvcencio/gotreesitter"
	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/infrastructure/language/fanout"
)

type oracleQuery struct {
	File            string
	Line, Character int
	Method          string
	Expected        conformance.Reference
	Kind            string
}

// TestExportPythonOracle exports every produced reference, including open
// questions. The Python driver queries Pyright without editing target files.
func TestExportPythonOracle(t *testing.T) {
	root := os.Getenv("ARCLINT_PY_ORACLE_ROOT")
	output := os.Getenv("ARCLINT_PY_ORACLE_OUTPUT")
	if root == "" || output == "" {
		t.Skip("set oracle root and output")
	}
	files := oracleFiles(t, root)
	res := newResolver(root, files)
	facts := resolvedFacts(root, files, res, true, true, true, true)
	if _, err := conformance.NewObservations(files, facts); err != nil {
		t.Fatal(err)
	}
	references := 0
	for _, f := range facts {
		for _, c := range f.Calls {
			references++
			if !c.Receiver.IsZero() {
				references++
			}
		}
		for _, d := range f.Declarations {
			references += len(d.Holds)
		}
	}
	ps := newParsers()
	idx := &resolutionIndex{resolver: res, modules: map[string]*pyModule{}, patched: map[string]bool{}}
	for _, file := range files {
		if !facts[file.Path].DeclarationsAvailable {
			continue
		}
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			t.Fatal(err)
		}
		idx.modules[file.Path] = newModule(file.Path, extractDeclarations(ps, src, nil))
	}
	var queries []oracleQuery
	add := func(file string, src []byte, offset uint32, method, kind string, ref conformance.Reference) {
		prefix := string(src[:offset])
		line := strings.Count(prefix, "\n")
		col := prefix[strings.LastIndex(prefix, "\n")+1:]
		queries = append(queries, oracleQuery{file, line, len(utf16.Encode([]rune(col))), method, ref, kind})
	}
	for _, file := range files {
		f, ok := facts[file.Path]
		if !ok || !f.DeclarationsAvailable {
			continue
		}
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			t.Fatal(err)
		}
		m := idx.modules[file.Path]
		df := m.facts
		w := df.walker
		for i, c := range f.Calls {
			fn := df.CallNodes[i].ChildByFieldName("function", w.lang)
			site := fn
			if fn.Type(w.lang) == "attribute" {
				site = fn.ChildByFieldName("attribute", w.lang)
			}
			if site == nil {
				t.Fatal("call site absent")
			}
			add(file.Path, src, site.StartByte(), "textDocument/definition", "call", c.Target)
			if !c.Receiver.IsZero() {
				obj := fn.ChildByFieldName("object", w.lang)
				if obj == nil {
					t.Fatal("receiver absent")
				}
				// Query the final attribute of self.field, not the self token.
				if obj.Type(w.lang) == "attribute" {
					obj = obj.ChildByFieldName("attribute", w.lang)
				}
				if obj.Type(w.lang) == "call" && c.Receiver.Resolution == conformance.ResolutionDeclared {
					// super() dispatch uses the enclosing class's implicit
					// self type; Pyright exposes super itself as a builtin.
					if owner := m.classes[c.Receiver.Name]; owner != nil {
						name := owner.node.ChildByFieldName("name", w.lang)
						add(file.Path, src, name.StartByte(), "textDocument/definition", "receiver", c.Receiver)
						continue
					}
				}
				add(file.Path, src, obj.StartByte(), "textDocument/typeDefinition", "receiver", c.Receiver)
			}
		}
		for _, field := range m.fields {
			d := f.Declarations[field.declaration]
			if field.annotation == nil {
				site := uint32(0)
				if b := field.class.members[d.Name]; b != nil && b.node != nil {
					site = b.node.StartByte()
				}
				for _, ref := range d.Holds {
					add(file.Path, src, site, "", "field", ref)
				}
				continue
			}
			sites := annotationOracleSites(t, idx, field.scope, field.annotation)
			for i, ref := range d.Holds {
				if ref.Resolution == conformance.ResolutionUndetermined {
					add(file.Path, src, field.annotation.StartByte(), "", "field", ref)
					continue
				}
				if i >= len(sites) {
					t.Fatalf("%s %s missing query site for %+v; sites %v", file.Path, d.Name, ref, sites)
				}
				add(file.Path, src, sites[i], "textDocument/definition", "field", ref)
			}
		}
	}
	if len(queries) != references {
		t.Fatalf("exported %d queries for %d produced references", len(queries), references)
	}
	data, err := json.MarshalIndent(queries, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// Sites follow the annotation grammar, including nested callable types and
// typing aliases. Missing source locations fail export rather than dropping
// a reference from the oracle's denominator.
func annotationOracleSites(t testing.TB, idx *resolutionIndex, s *pyScope, n *gotreesitter.Node) []uint32 {
	t.Helper()
	w := s.module.facts.walker
	switch n.Type(w.lang) {
	case "type":
		if n.NamedChildCount() == 1 {
			return annotationOracleSites(t, idx, s, n.NamedChild(0))
		}
	case "none":
		return nil
	case "identifier", "attribute", "dotted_name":
		if n.Text(w.src) == "None" {
			return nil
		}
		text := n.Text(w.src)
		return []uint32{n.StartByte() + uint32(strings.LastIndex(text, ".")+1)}
	case "string":
		scope, ann, reason := decodeAnnotation(s, n)
		if reason != "" {
			return []uint32{n.StartByte()}
		}
		var content *gotreesitter.Node
		for i := 0; i < n.NamedChildCount(); i++ {
			if ch := n.NamedChild(i); ch.Type(w.lang) == "string_content" {
				content = ch
				break
			}
		}
		if content == nil {
			return []uint32{n.StartByte()}
		}
		if strings.Contains(content.Text(w.src), "\\") {
			t.Fatalf("escaped annotation needs source offset mapping: %s", n.Text(w.src))
		}
		sites := annotationOracleSites(t, idx, scope, ann)
		for i := range sites {
			sites[i] = content.StartByte() + sites[i] - 3
		}
		return sites
	case "generic_type", "subscript":
		if n.NamedChildCount() < 2 {
			break
		}
		head := n.NamedChild(0)
		_, q := idx.annotationName(s, head.Text(w.src), map[*pyBinding]bool{})
		sites := annotationOracleSites(t, idx, s, head)
		switch q {
		case "typing.Callable", "typing_extensions.Callable", "collections.abc.Callable", "typing.Literal", "typing_extensions.Literal":
			return sites
		case "typing.Annotated", "typing_extensions.Annotated":
			args := n.NamedChild(1)
			if args.NamedChildCount() > 0 {
				sites = append(sites, annotationOracleSites(t, idx, s, args.NamedChild(0))...)
			}
			return sites
		}
		for i := 1; i < n.NamedChildCount(); i++ {
			sites = append(sites, annotationOracleSites(t, idx, s, n.NamedChild(i))...)
		}
		return sites
	case "type_parameter", "union_type", "binary_operator", "tuple":
		var sites []uint32
		for i := 0; i < n.NamedChildCount(); i++ {
			sites = append(sites, annotationOracleSites(t, idx, s, n.NamedChild(i))...)
		}
		return sites
	}
	return []uint32{n.StartByte()}
}

func oracleFiles(t testing.TB, root string) []conformance.ObservedFile {
	t.Helper()
	var files []conformance.ObservedFile
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && p != root || d.Name() == "node_modules" || d.Name() == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if strings.HasSuffix(rel, ".py") || filepath.Base(rel) == "pyproject.toml" {
			files = append(files, conformance.ObservedFile{Path: filepath.ToSlash(rel)})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func BenchmarkPythonRepository(b *testing.B) {
	root := os.Getenv("ARCLINT_PY_ORACLE_ROOT")
	if root == "" {
		b.Skip("set ARCLINT_PY_ORACLE_ROOT")
	}
	files := oracleFiles(b, root)
	b.Run("legacy_imports", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			res := newResolver(root, files)
			fanout.Analyze(files, analyzable, func() fanout.Analyzer {
				ps := newParsers()
				return func(rel string) conformance.LanguageFacts { return analyzeFile(ps, res, root, rel, false, false) }
			})
		}
	})
	for _, tc := range []struct {
		name  string
		facts []rule.Fact
	}{{"imports", nil}, {"syntax", []rule.Fact{rule.FactDeclarations, rule.FactCalls}}, {"resolution", []rule.Fact{rule.FactResolvedCalls, rule.FactFieldTypes}}} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := NewProducer().Facts(root, files, tc.facts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
