package golang_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wixregiga/arclint/internal/lang/golang"
	"github.com/wixregiga/arclint/internal/tree"
)

func writeTree(t *testing.T, files map[string]string) *tree.Tree {
	t.Helper()
	root := t.TempDir()
	for p, content := range files {
		abs := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tr, err := tree.Walk(root, tree.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

type wantImport struct {
	path      string
	class     golang.Class
	targetDir string
}

func assertImports(t *testing.T, a *golang.Analysis, file string, want []wantImport) {
	t.Helper()
	fa := a.Files[file]
	if fa == nil {
		t.Fatalf("%s: not analyzed; analyzed files: %v", file, keys(a.Files))
	}
	if fa.ParseError != "" {
		t.Fatalf("%s: parse error: %s", file, fa.ParseError)
	}
	if len(fa.Imports) != len(want) {
		t.Fatalf("%s: got %d imports %+v, want %d", file, len(fa.Imports), fa.Imports, len(want))
	}
	for i, w := range want {
		got := fa.Imports[i]
		if got.Path != w.path || got.Class != w.class || got.TargetDir != w.targetDir {
			t.Errorf("%s import[%d] = {%s %s %q}, want {%s %s %q}",
				file, i, got.Path, got.Class, got.TargetDir, w.path, w.class, w.targetDir)
		}
	}
}

func keys[V any](m map[string]*V) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestClassifySingleModule(t *testing.T) {
	tr := writeTree(t, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.24\n\nrequire github.com/pkg/errors v0.9.1\n",
		"a/a.go": `package a

import (
	"fmt"

	"example.com/app/b"
	"github.com/nobody/nothing"
	"github.com/pkg/errors"
)

var _ = fmt.Sprint(b.X, errors.New, nothing.Y)
`,
		"b/b.go": "package b\n\nvar X = 1\n",
	})
	a := golang.Analyze(tr)
	assertImports(t, a, "a/a.go", []wantImport{
		{"fmt", golang.ClassStdlib, ""},
		{"example.com/app/b", golang.ClassInternal, "b"},
		{"github.com/nobody/nothing", golang.ClassUnknown, ""},
		{"github.com/pkg/errors", golang.ClassExternal, ""},
	})
}

func TestCgoPseudoImport(t *testing.T) {
	tr := writeTree(t, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.24\n",
		"c/c.go": "package c\n\n// #include <stdio.h>\nimport \"C\"\n",
	})
	a := golang.Analyze(tr)
	assertImports(t, a, "c/c.go", []wantImport{{"C", golang.ClassCgo, ""}})
	if !a.Files["c/c.go"].HasCgo {
		t.Error("HasCgo not set")
	}
}

func TestNestedModuleOwnsItsFiles(t *testing.T) {
	tr := writeTree(t, map[string]string{
		"go.mod":       "module example.com/root\n\ngo 1.24\n\nrequire example.com/sub v1.0.0\n",
		"main.go":      "package main\n\nimport _ \"example.com/sub/x\"\n\nfunc main() {}\n",
		"sub/go.mod":   "module example.com/sub\n\ngo 1.24\n\nrequire github.com/pkg/errors v0.9.1\n",
		"sub/x/x.go":   "package x\n\nimport (\n\t_ \"example.com/sub/y\"\n\t_ \"example.com/root/z\"\n\t_ \"github.com/pkg/errors\"\n)\n",
		"sub/y/y.go":   "package y\n",
		"z/z.go":       "package z\n",
		"sub/go.sum":   "",
		".gitkeep":     "",
		"sub/.gitkeep": "",
	})
	a := golang.Analyze(tr)
	// The nested module owns its files: example.com/sub resolves against
	// sub/go.mod, and the parent module's path is NOT visible from inside
	// the nested module unless required.
	assertImports(t, a, "sub/x/x.go", []wantImport{
		{"example.com/sub/y", golang.ClassInternal, "sub/y"},
		{"example.com/root/z", golang.ClassUnknown, ""},
		{"github.com/pkg/errors", golang.ClassExternal, ""},
	})
	// From the root module, the sibling nested module is external when
	// required (resolvable via require) and never internal without
	// replace/workspace.
	assertImports(t, a, "main.go", []wantImport{
		{"example.com/sub/x", golang.ClassExternal, ""},
	})
}

func TestReplaceToLocal(t *testing.T) {
	tr := writeTree(t, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.24\n\n" +
			"require (\n\texample.com/lib v1.0.0\n\texample.com/outside v1.0.0\n)\n\n" +
			"replace example.com/lib => ./lib\n\nreplace example.com/outside => ../elsewhere\n",
		"main.go":       "package main\n\nimport (\n\t_ \"example.com/lib/util\"\n\t_ \"example.com/outside/pkg\"\n)\n\nfunc main() {}\n",
		"lib/go.mod":    "module example.com/lib\n\ngo 1.24\n",
		"lib/util/u.go": "package util\n",
	})
	a := golang.Analyze(tr)
	assertImports(t, a, "main.go", []wantImport{
		// Replace-to-local inside the repo: internal with a tree directory.
		{"example.com/lib/util", golang.ClassInternal, "lib/util"},
		// Replace-to-local outside the repo: internal for boundaries, no
		// tree directory.
		{"example.com/outside/pkg", golang.ClassInternal, ""},
	})
}

func TestReplaceToModule(t *testing.T) {
	tr := writeTree(t, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.24\n\n" +
			"require example.com/lib v1.0.0\n\nreplace example.com/lib => example.com/fork v1.2.0\n",
		"main.go": "package main\n\nimport _ \"example.com/lib/util\"\n\nfunc main() {}\n",
	})
	a := golang.Analyze(tr)
	assertImports(t, a, "main.go", []wantImport{
		{"example.com/lib/util", golang.ClassExternal, ""},
	})
}

func TestGoWorkWorkspace(t *testing.T) {
	files := map[string]string{
		"go.work":       "go 1.24\n\nuse (\n\t./moda\n\t./modb\n)\n",
		"moda/go.mod":   "module example.com/moda\n\ngo 1.24\n",
		"moda/a.go":     "package moda\n\nimport _ \"example.com/modb/pkg\"\n",
		"modb/go.mod":   "module example.com/modb\n\ngo 1.24\n",
		"modb/pkg/p.go": "package pkg\n",
	}
	tr := writeTree(t, files)
	a := golang.Analyze(tr)
	// Workspace member imports resolve internal, matching go tool
	// workspace mode.
	assertImports(t, a, "moda/a.go", []wantImport{
		{"example.com/modb/pkg", golang.ClassInternal, "modb/pkg"},
	})

	// Without go.work the same import is unresolvable (not required).
	delete(files, "go.work")
	tr2 := writeTree(t, files)
	a2 := golang.Analyze(tr2)
	assertImports(t, a2, "moda/a.go", []wantImport{
		{"example.com/modb/pkg", golang.ClassUnknown, ""},
	})
}

func TestVendorExcludedAndExternal(t *testing.T) {
	tr := writeTree(t, map[string]string{
		"go.mod":                                 "module example.com/app\n\ngo 1.24\n\nrequire github.com/pkg/errors v0.9.1\n",
		"main.go":                                "package main\n\nimport _ \"github.com/pkg/errors\"\n\nfunc main() {}\n",
		"vendor/github.com/pkg/errors/errors.go": "package errors\n",
		"vendor/modules.txt":                     "# github.com/pkg/errors v0.9.1\n",
	})
	a := golang.Analyze(tr)
	if _, ok := a.Files["vendor/github.com/pkg/errors/errors.go"]; ok {
		t.Error("vendored file was analyzed; vendor/ must be excluded from the scan")
	}
	assertImports(t, a, "main.go", []wantImport{
		{"github.com/pkg/errors", golang.ClassExternal, ""},
	})
}

func TestUnparseableWarnsAndSkips(t *testing.T) {
	tr := writeTree(t, map[string]string{
		"go.mod":  "module example.com/app\n\ngo 1.24\n",
		"bad.go":  "pkg broken {{{ not go at all\n",
		"good.go": "package app\n\nimport _ \"fmt\"\n",
	})
	a := golang.Analyze(tr)
	if a.Files["bad.go"] == nil || a.Files["bad.go"].ParseError == "" {
		t.Fatal("expected recorded parse error for bad.go")
	}
	if len(a.Warnings) == 0 {
		t.Error("expected a warning for the unparseable file")
	}
	assertImports(t, a, "good.go", []wantImport{{"fmt", golang.ClassStdlib, ""}})
}

func TestBuildConstrainedFilesStillScanned(t *testing.T) {
	tr := writeTree(t, map[string]string{
		"go.mod":           "module example.com/app\n\ngo 1.24\n",
		"f_linux.go":       "package app\n\nimport _ \"os\"\n",
		"f_windows.go":     "package app\n\nimport _ \"syscall\"\n",
		"f_constrained.go": "//go:build never\n\npackage app\n\nimport _ \"net\"\n",
	})
	a := golang.Analyze(tr)
	for _, f := range []string{"f_linux.go", "f_windows.go", "f_constrained.go"} {
		if a.Files[f] == nil {
			t.Errorf("%s: not analyzed; build-constrained files must still be scanned", f)
		}
	}
}

func TestDotAndUnderscorePrefixIgnored(t *testing.T) {
	tr := writeTree(t, map[string]string{
		"go.mod":         "module example.com/app\n\ngo 1.24\n",
		"_gen.go":        "package app\n",
		".hidden.go":     "package app\n",
		"_dir/inside.go": "package inside\n",
		"ok.go":          "package app\n",
	})
	a := golang.Analyze(tr)
	for _, f := range []string{"_gen.go", ".hidden.go", "_dir/inside.go"} {
		if a.Files[f] != nil {
			t.Errorf("%s: analyzed, but the go tool ignores dot/underscore-prefixed paths", f)
		}
	}
	if a.Files["ok.go"] == nil {
		t.Error("ok.go: not analyzed")
	}
}

func TestNoGoModAllNonStdlibUnknown(t *testing.T) {
	tr := writeTree(t, map[string]string{
		"a.go": "package a\n\nimport (\n\t_ \"fmt\"\n\t_ \"github.com/x/y\"\n)\n",
	})
	a := golang.Analyze(tr)
	assertImports(t, a, "a.go", []wantImport{
		{"fmt", golang.ClassStdlib, ""},
		{"github.com/x/y", golang.ClassUnknown, ""},
	})
}

func TestStdlibTableSanity(t *testing.T) {
	for _, p := range []string{"fmt", "net/http", "encoding/json", "go/parser", "internal/abi"} {
		if !golang.IsStdlib(p) {
			t.Errorf("%s: missing from embedded stdlib table", p)
		}
	}
	for _, p := range []string{"github.com/spf13/cobra", "C", "example.com/x"} {
		if golang.IsStdlib(p) {
			t.Errorf("%s: wrongly in stdlib table", p)
		}
	}
}
