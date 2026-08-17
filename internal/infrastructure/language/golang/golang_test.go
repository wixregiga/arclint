package golang_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	golang "github.com/wixregiga/arclint/internal/infrastructure/language/golang"
)

func write(t *testing.T, root, rel, content string) conformance.ObservedFile {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return conformance.ObservedFile{Path: rel}
}

func TestFactsClassifyExactly(t *testing.T) {
	root := t.TempDir()
	files := []conformance.ObservedFile{
		write(t, root, "go.mod", "module example.com/x\n\ngo 1.22\n\nrequire github.com/pkg/errors v0.9.1\n"),
		write(t, root, "a/main.go", `package a

import (
	"fmt"

	"example.com/x/util"
	"github.com/pkg/errors"
	"mystery.example/zzz"
)

var _ = fmt.Sprint(util.V, errors.New(""), zzz.X)
`),
		write(t, root, "util/util.go", "package util\n\nvar V = 1\n"),
		write(t, root, "broken.go", "package broken\n\nimport (\n"),
		write(t, root, "_ignored/skip.go", "package skip\n"),
		write(t, root, "cgo/cgo.go", "package cgo\n\nimport \"C\"\n"),
	}
	facts, err := golang.NewProducer().Facts(root, files, nil)
	if err != nil {
		t.Fatalf("Facts: %v", err)
	}

	if _, ok := facts["_ignored/skip.go"]; ok {
		t.Errorf("underscore paths are not analyzable by the go tool")
	}
	if got := facts["broken.go"]; got.ParseFailure == "" {
		t.Errorf("parse failure not recorded for broken.go")
	}

	main := facts["a/main.go"]
	if main.ParseFailure != "" || !main.ImportsAvailable {
		t.Fatalf("main.go facts unusable: %+v", main)
	}
	wantClasses := map[string]conformance.ImportClass{
		"fmt":                   conformance.ImportStdlib,
		"example.com/x/util":    conformance.ImportInternal,
		"github.com/pkg/errors": conformance.ImportExternal,
		"mystery.example/zzz":   conformance.ImportUnknown,
	}
	seen := map[string]bool{}
	for _, imp := range main.Imports {
		want, ok := wantClasses[imp.Path]
		if !ok {
			t.Errorf("unexpected import %q", imp.Path)
			continue
		}
		seen[imp.Path] = true
		if imp.Class != want {
			t.Errorf("import %q class = %q, want %q", imp.Path, imp.Class, want)
		}
		if imp.Line == 0 {
			t.Errorf("import %q has no line", imp.Path)
		}
	}
	for path := range wantClasses {
		if !seen[path] {
			t.Errorf("import %q not extracted", path)
		}
	}
	for _, imp := range main.Imports {
		if imp.Path == "example.com/x/util" && imp.TargetDir != "util" {
			t.Errorf("internal import target = %q, want util", imp.TargetDir)
		}
	}

	cgo := facts["cgo/cgo.go"]
	if len(cgo.Imports) != 1 || cgo.Imports[0].Class != conformance.ImportCgo {
		t.Errorf("cgo facts = %+v, want the C pseudo-import", cgo.Imports)
	}
}
