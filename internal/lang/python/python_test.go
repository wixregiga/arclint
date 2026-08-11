package python

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/wixregiga/arclint/internal/lang"
	"github.com/wixregiga/arclint/internal/tree"
)

// TestExtractForms covers exactly the statement forms the research report
// (multi-language-rule-engines.md §4) commits to: import and from-import,
// legal at any indentation, with parenthesized multi-line and backslash
// continuation.
func TestExtractForms(t *testing.T) {
	src := `import os
import os.path as p
import first, second.sub as s
from x.y import z
from . import sibling
from ..parent import thing
from pkg import (
    m1,
    m2,
)
def f():
    import inside_function
    if True:
        from cond import q
import third, \
    fourth
`
	got := extract(src)
	var mods []string
	for _, ri := range got {
		mods = append(mods, ri.module)
	}
	want := []string{
		"os", "os.path", "first", "second.sub", "x.y", ".", "..parent",
		"pkg", "inside_function", "cond", "third", "fourth",
	}
	if !reflect.DeepEqual(mods, want) {
		t.Errorf("extracted %v\nwant %v", mods, want)
	}
	if got[0].line != 1 || got[7].line != 7 {
		t.Errorf("line anchors: first=%d pkg=%d", got[0].line, got[7].line)
	}
}

// TestDocumentedFalseNegatives: computed imports stay invisible at this
// tier — the documented contract.
func TestDocumentedFalseNegatives(t *testing.T) {
	src := `import importlib
mod = importlib.import_module("boto3")
dyn = __import__("requests")
s = "import fake_in_string"
# import commented
def g():
    """
    import fake_in_docstring
    from fake import thing
    """
    return s
t = '''
import fake_in_single_triple
'''
`
	got := extract(src)
	if len(got) != 1 || got[0].module != "importlib" {
		t.Errorf("extracted %+v, want only the literal `import importlib`", got)
	}
}

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

func TestClassification(t *testing.T) {
	tr := writeTree(t, map[string]string{
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
	a := Analyze(tr)
	fa := a.Files["app/main.py"]
	if fa == nil {
		t.Fatal("main.py not analyzed")
	}
	type want struct {
		mod   string
		class lang.Class
		tf    string
	}
	wants := []want{
		{"os", lang.ClassStdlib, ""},
		{"json", lang.ClassStdlib, ""},
		{"requests", lang.ClassExternal, ""},
		{"pytest", lang.ClassExternal, ""},
		// typing_extensions matches typing-extensions via PEP 503
		// underscore/hyphen normalization.
		{"typing_extensions", lang.ClassExternal, ""},
		{"app.helpers", lang.ClassInternal, "app/helpers.py"},
		{"app", lang.ClassInternal, ""},
		{".", lang.ClassInternal, ""},
		{".helpers", lang.ClassInternal, "app/helpers.py"},
		// PyYAML provides yaml: the dist/module name mismatch is the
		// documented limitation — unknown, never silently external.
		{"yaml", lang.ClassUnknown, ""},
		// In-repo top-level package with an unresolvable submodule.
		{"app.generated", lang.ClassInternal, ""},
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
	}
}

func TestSrcLayoutAndNamespacePackages(t *testing.T) {
	tr := writeTree(t, map[string]string{
		"pyproject.toml":         "[project]\nname = \"lib\"\ndependencies = []\n",
		"src/mylib/__init__.py":  "",
		"src/mylib/core.py":      "import mylib.util\n",
		"src/mylib/util.py":      "",
		"src/namespace_pkg/x.py": "",
		"consumer.py":            "import mylib\nimport namespace_pkg.x\n",
	})
	a := Analyze(tr)
	fa := a.Files["consumer.py"]
	if fa.Imports[0].Class != lang.ClassInternal || fa.Imports[0].TargetDir != "src/mylib" {
		t.Errorf("src layout: %+v", fa.Imports[0])
	}
	// PEP 420: no __init__.py, still a package directory.
	if fa.Imports[1].Class != lang.ClassInternal || fa.Imports[1].TargetFile != "src/namespace_pkg/x.py" {
		t.Errorf("namespace package: %+v", fa.Imports[1])
	}
}

func TestPoetryDependencies(t *testing.T) {
	tr := writeTree(t, map[string]string{
		"pyproject.toml": `[tool.poetry]
name = "legacy"

[tool.poetry.dependencies]
python = "^3.11"
Django = "^5.0"
`,
		"site.py": "import django\n",
	})
	a := Analyze(tr)
	// Django (dist) vs django (module): PEP 503 normalization is
	// case-insensitive, so this resolves external.
	if got := a.Files["site.py"].Imports[0]; got.Class != lang.ClassExternal {
		t.Errorf("poetry dep: %+v", got)
	}
}

func TestStdlibTableSanity(t *testing.T) {
	for _, m := range []string{"os", "sys", "json", "asyncio", "importlib"} {
		if !IsStdlib(m) {
			t.Errorf("%s missing from stdlib table", m)
		}
	}
	for _, m := range []string{"requests", "django", "numpy"} {
		if IsStdlib(m) {
			t.Errorf("%s wrongly stdlib", m)
		}
	}
}
