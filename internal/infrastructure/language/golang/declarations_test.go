package golang_test

import (
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	golang "github.com/wixregiga/arclint/internal/infrastructure/language/golang"
)

const declarationFixture = `package sample

import "io"

const Limit = 10

var counter int

type Store struct {
	Name string
	io.Reader
}

type Reader interface {
	ReadAll(dst []byte, opts ...string) (int, error)
}

func Public(a, b string) error { return nil }

func (s *Store) save(w io.Writer) {}
`

func TestDeclarationsExtractedExactly(t *testing.T) {
	root := t.TempDir()
	files := []conformance.ObservedFile{
		write(t, root, "go.mod", "module example.com/x\n\ngo 1.22\n"),
		write(t, root, "sample/sample.go", declarationFixture),
	}
	facts, err := golang.NewProducer().Facts(root, files, []rule.Fact{rule.FactDeclarations})
	if err != nil {
		t.Fatalf("Facts: %v", err)
	}
	got := facts["sample/sample.go"]
	if !got.DeclarationsAvailable || !got.Supports(rule.FactDeclarations) {
		t.Fatalf("declarations must be available when requested: %+v", got)
	}
	if got.Package != "sample" {
		t.Errorf("package = %q, want sample", got.Package)
	}

	byKey := map[string]conformance.Declaration{}
	for _, d := range got.Declarations {
		byKey[d.Kind+" "+d.Owner+"."+d.Name] = d
	}
	expect := func(key string, exported bool) conformance.Declaration {
		t.Helper()
		d, ok := byKey[key]
		if !ok {
			t.Fatalf("missing declaration %q; have %v", key, keysOf(byKey))
		}
		if d.Exported != exported {
			t.Errorf("%s exported = %v, want %v", key, d.Exported, exported)
		}
		return d
	}

	expect("const .Limit", true)
	expect("var .counter", false)
	expect("struct .Store", true)
	expect("field Store.Name", true)
	expect("field Store.Reader", true) // embedded field keeps its base type name
	expect("interface .Reader", true)

	readAll := expect("method Reader.ReadAll", true)
	if len(readAll.Params) != 2 || readAll.Params[0].Name != "dst" ||
		!readAll.Params[1].Variadic || readAll.Params[1].Type != "...string" {
		t.Errorf("ReadAll params = %+v", readAll.Params)
	}
	if len(readAll.Results) != 2 || readAll.Results[0] != "int" || readAll.Results[1] != "error" {
		t.Errorf("ReadAll results = %v", readAll.Results)
	}

	public := expect("func .Public", true)
	if len(public.Params) != 2 || public.Params[0].Type != "string" || public.Params[1].Name != "b" {
		t.Errorf("Public params = %+v", public.Params)
	}

	save := expect("method Store.save", false)
	if save.Owner != "Store" {
		t.Errorf("pointer receiver must unwrap to the base type, got %q", save.Owner)
	}
	if save.StartLine == 0 || save.EndLine < save.StartLine {
		t.Errorf("line range = %d..%d", save.StartLine, save.EndLine)
	}
}

func TestDeclarationsOnlyWhenRequested(t *testing.T) {
	root := t.TempDir()
	files := []conformance.ObservedFile{
		write(t, root, "go.mod", "module example.com/x\n\ngo 1.22\n"),
		write(t, root, "a/a.go", "package a\n\nfunc F() {}\n"),
	}
	facts, err := golang.NewProducer().Facts(root, files, nil)
	if err != nil {
		t.Fatalf("Facts: %v", err)
	}
	got := facts["a/a.go"]
	if got.DeclarationsAvailable || got.Supports(rule.FactDeclarations) || len(got.Declarations) != 0 {
		t.Errorf("unrequested declarations must be absent, not empty: %+v", got)
	}
	if !got.Supports(rule.FactImports) {
		t.Errorf("imports must stay available regardless")
	}
}

func keysOf(m map[string]conformance.Declaration) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
