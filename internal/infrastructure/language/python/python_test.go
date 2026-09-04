package python

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// writeFiles materializes a fixture tree and returns the root plus the
// sorted observed-file list the walk would produce.
func writeFiles(t *testing.T, files map[string]string) (string, []conformance.ObservedFile) {
	t.Helper()
	root := t.TempDir()
	var observed []conformance.ObservedFile
	for p, content := range files {
		abs := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		observed = append(observed, conformance.ObservedFile{Path: p, Size: int64(len(content))})
	}
	sort.Slice(observed, func(i, j int) bool { return observed[i].Path < observed[j].Path })
	return root, observed
}

func produce(t *testing.T, root string, files []conformance.ObservedFile) map[string]conformance.LanguageFacts {
	t.Helper()
	facts, err := NewProducer().Facts(root, files, nil)
	if err != nil {
		t.Fatalf("Facts: %v", err)
	}
	return facts
}

func TestFactsClassification(t *testing.T) {
	root, files := writeFiles(t, map[string]string{
		"pyproject.toml": `[project]
name = "app"
dependencies = ["requests>=2.0", "pydantic[email]==2.5; python_version > '3.8'", "typing-extensions"]

[project.optional-dependencies]
dev = ["pytest>=8"]
`,
		"app/__init__.py": "",
		"app/main.py": `import os
import json
import requests
import pytest
import typing_extensions
import app.helpers
from app import helpers
from . import helpers as h
from .helpers import fn
import yaml
import app.generated
`,
		"app/helpers.py": "def fn():\n    pass\n",
	})
	facts := produce(t, root, files)
	fa, ok := facts["app/main.py"]
	if !ok {
		t.Fatal("main.py not analyzed")
	}
	if fa.Language != rule.LanguagePython || !fa.ImportsAvailable || fa.ParseFailure != "" {
		t.Fatalf("facts header: %+v", fa)
	}
	type want struct {
		mod   string
		class conformance.ImportClass
		tf    string
	}
	wants := []want{
		{"os", conformance.ImportStdlib, ""},
		{"json", conformance.ImportStdlib, ""},
		{"requests", conformance.ImportExternal, ""},
		{"pytest", conformance.ImportExternal, ""},
		// typing_extensions matches typing-extensions via PEP 503
		// underscore/hyphen normalization.
		{"typing_extensions", conformance.ImportExternal, ""},
		{"app.helpers", conformance.ImportInternal, "app/helpers.py"},
		{"app", conformance.ImportInternal, ""},
		{".", conformance.ImportInternal, ""},
		{".helpers", conformance.ImportInternal, "app/helpers.py"},
		// PyYAML provides yaml: the dist/module name mismatch is the
		// documented limitation — unknown, never silently external.
		{"yaml", conformance.ImportUnknown, ""},
		// In-repo top-level package with an unresolvable submodule.
		{"app.generated", conformance.ImportInternal, ""},
	}
	if len(fa.Imports) != len(wants) {
		t.Fatalf("imports (%d): %+v", len(fa.Imports), fa.Imports)
	}
	for i, w := range wants {
		got := fa.Imports[i]
		if got.Path != w.mod || got.Class != w.class || got.TargetFile != w.tf {
			t.Errorf("[%d] = {%s %s tf=%q td=%q}, want {%s %s tf=%q}",
				i, got.Path, got.Class, got.TargetFile, got.TargetDir, w.mod, w.class, w.tf)
		}
		if got.Line != i+1 {
			t.Errorf("[%d] line = %d, want %d", i, got.Line, i+1)
		}
	}
}

func TestSrcLayoutAndNamespacePackages(t *testing.T) {
	root, files := writeFiles(t, map[string]string{
		"pyproject.toml":         "[project]\nname = \"lib\"\ndependencies = []\n",
		"src/mylib/__init__.py":  "",
		"src/mylib/core.py":      "import mylib.util\n",
		"src/mylib/util.py":      "",
		"src/namespace_pkg/x.py": "",
		"consumer.py":            "import mylib\nimport namespace_pkg.x\n",
	})
	facts := produce(t, root, files)
	fa := facts["consumer.py"]
	if fa.Imports[0].Class != conformance.ImportInternal || fa.Imports[0].TargetDir != "src/mylib" {
		t.Errorf("src layout: %+v", fa.Imports[0])
	}
	// PEP 420: no __init__.py, still a package directory.
	if fa.Imports[1].Class != conformance.ImportInternal || fa.Imports[1].TargetFile != "src/namespace_pkg/x.py" {
		t.Errorf("namespace package: %+v", fa.Imports[1])
	}
}

func TestPoetryDependencies(t *testing.T) {
	root, files := writeFiles(t, map[string]string{
		"pyproject.toml": `[tool.poetry]
name = "legacy"

[tool.poetry.dependencies]
python = "^3.11"
Django = "^5.0"
`,
		"site.py": "import django\n",
	})
	facts := produce(t, root, files)
	// Django (dist) vs django (module): PEP 503 normalization is
	// case-insensitive, so this resolves external.
	if got := facts["site.py"].Imports[0]; got.Class != conformance.ImportExternal {
		t.Errorf("poetry dep: %+v", got)
	}
}

// TestOnlyOwnedFilesClaimed pins the target vocabulary: the producer
// claims exactly the .py files.
func TestOnlyOwnedFilesClaimed(t *testing.T) {
	root, files := writeFiles(t, map[string]string{
		"mod.py":    "import os\n",
		"notes.txt": "import os\n",
		"tool.pyi":  "import os\n",
	})
	facts := produce(t, root, files)
	if _, ok := facts["mod.py"]; !ok {
		t.Error("mod.py not claimed")
	}
	for _, unclaimed := range []string{"notes.txt", "tool.pyi"} {
		if _, ok := facts[unclaimed]; ok {
			t.Errorf("%s claimed; Python owns only .py", unclaimed)
		}
	}
}

func TestStdlibTableSanity(t *testing.T) {
	for _, m := range []string{"os", "sys", "json", "asyncio", "importlib"} {
		if !isStdlib(m) {
			t.Errorf("%s missing from stdlib table", m)
		}
	}
	for _, m := range []string{"requests", "django", "numpy"} {
		if isStdlib(m) {
			t.Errorf("%s wrongly stdlib", m)
		}
	}
}

// TestFactsDeclarationsAcrossManyFiles proves parallel analysis with
// per-worker parser reuse keeps every file's declarations its own:
// more files than workers, each declaring one uniquely named class and
// function, and each file's facts name exactly those two.
func TestFactsDeclarationsAcrossManyFiles(t *testing.T) {
	sources := map[string]string{}
	for i := range 4 * runtime.GOMAXPROCS(0) {
		sources[fmt.Sprintf("pkg/m%03d.py", i)] = fmt.Sprintf(
			"class Thing%d:\n    pass\n\n\ndef make%d():\n    return Thing%d()\n", i, i, i)
	}
	root, files := writeFiles(t, sources)
	facts, err := NewProducer().Facts(root, files, []rule.Fact{rule.FactDeclarations, rule.FactCalls})
	if err != nil {
		t.Fatalf("Facts: %v", err)
	}
	for i := range len(sources) {
		rel := fmt.Sprintf("pkg/m%03d.py", i)
		got, ok := facts[rel]
		if !ok || !got.DeclarationsAvailable || !got.CallsAvailable {
			t.Fatalf("%s: declarations or calls unavailable: %+v", rel, got)
		}
		var names []string
		for _, d := range got.Declarations {
			names = append(names, d.Kind+" "+d.Name)
		}
		want := []string{fmt.Sprintf("class Thing%d", i), fmt.Sprintf("func make%d", i)}
		if fmt.Sprint(names) != fmt.Sprint(want) {
			t.Errorf("%s declarations = %v, want %v", rel, names, want)
		}
	}
}
