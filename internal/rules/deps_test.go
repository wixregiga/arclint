package rules

import (
	"path/filepath"
	"testing"

	"github.com/wixregiga/arclint/internal/config"
)

// The dependencies tests run against the committed fixture tree at
// testdata/depsrepo: a Go module (example.com/depsrepo) with a layer
// inversion, a TS feature pair with a cross-import, and a Python feature
// pair with a forbidden edge.

func depsFixture(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("testdata", "depsrepo"))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func depsRule(p *config.DependenciesParams) config.Rule {
	return config.Rule{
		Type: config.CategoryDependencies, Severity: config.SeverityError,
		Description: "t", Dependencies: p, FixHint: "invert the dependency",
	}
}

func TestDepsLayersGo(t *testing.T) {
	root := depsFixture(t)
	rule := depsRule(&config.DependenciesParams{
		Modules: map[string][]string{
			"infra":  {"internal/infra/**"},
			"app":    {"internal/app/**"},
			"domain": {"internal/domain/**"},
		},
		Contract: "layers",
		Layers:   []string{"infra", "app", "domain"},
	})
	vs := evalOne(t, root, "layered", rule)
	if len(vs) != 1 {
		t.Fatalf("want exactly 1 layer violation, got %+v", vs)
	}
	v := vs[0]
	if v.Path != "internal/domain/user.go" || v.Line == nil || *v.Line != 6 {
		t.Fatalf("want internal/domain/user.go:6, got %+v", v)
	}
}

func TestDepsIndependenceTS(t *testing.T) {
	root := depsFixture(t)
	rule := depsRule(&config.DependenciesParams{
		Modules: map[string][]string{
			"billing": {"src/billing/**"},
			"search":  {"src/search/**"},
		},
		Contract: "independence",
		Among:    []string{"billing", "search"},
	})
	vs := evalOne(t, root, "features-independent", rule)
	if len(vs) != 1 {
		t.Fatalf("want 1 independence violation, got %+v", vs)
	}
	v := vs[0]
	if v.Path != "src/billing/index.ts" || v.Line == nil || *v.Line != 1 {
		t.Fatalf("want src/billing/index.ts:1, got %+v", v)
	}
}

func TestDepsForbiddenPy(t *testing.T) {
	root := depsFixture(t)
	rule := depsRule(&config.DependenciesParams{
		Modules: map[string][]string{
			"fa": {"pyapp/feature_a/**"},
			"fb": {"pyapp/feature_b/**"},
		},
		Contract:  "forbidden",
		Forbidden: []config.ForbiddenEdge{{From: []string{"fa"}, To: []string{"fb"}}},
	})
	vs := evalOne(t, root, "no-a-to-b", rule)
	if len(vs) != 1 {
		t.Fatalf("want 1 forbidden-edge violation, got %+v", vs)
	}
	v := vs[0]
	if v.Path != "pyapp/feature_a/mod.py" || v.Line == nil || *v.Line != 1 {
		t.Fatalf("want pyapp/feature_a/mod.py:1, got %+v", v)
	}
}

func TestDepsMayDependOn(t *testing.T) {
	root := depsFixture(t)
	rule := depsRule(&config.DependenciesParams{
		Modules: map[string][]string{
			"domain": {"internal/domain/**"},
			"infra":  {"internal/infra/**"},
			"app":    {"internal/app/**"},
		},
		Contract: "mayDependOn",
		// domain may depend on nothing; app and infra are unconstrained
		// (no whitelist entry).
		MayDependOn: map[string][]string{"domain": {}},
	})
	vs := evalOne(t, root, "domain-pure", rule)
	if len(vs) != 1 {
		t.Fatalf("want 1 mayDependOn violation, got %+v", vs)
	}
	if vs[0].Path != "internal/domain/user.go" {
		t.Fatalf("want internal/domain/user.go, got %+v", vs[0])
	}
}

func TestDepsThirdPartyIgnored(t *testing.T) {
	// "fmt" and other non-module imports resolve to no module and never
	// violate; app→domain is a legal downward import.
	root := depsFixture(t)
	rule := depsRule(&config.DependenciesParams{
		Modules: map[string][]string{
			"app":    {"internal/app/**"},
			"domain": {"internal/domain/**"},
		},
		Contract: "layers",
		Layers:   []string{"app", "domain"},
	})
	if vs := evalOne(t, root, "app-over-domain", rule); len(vs) != 0 {
		t.Fatalf("app→domain is a legal downward import, got %+v", vs)
	}
}

// TestExtractPyTolerantForms covers MEDIUM 5: the Python import regex
// previously anchored "import ..." to end-of-line, so it missed
// "import os; import sys" (semicolon-joined statements), trailing
// comments, and lost the module identity of "import x as y" (a
// non-issue in isolation, but paired with the "$" anchor it silently
// dropped the whole line whenever any of these trailing forms appeared).
func TestExtractPyTolerantForms(t *testing.T) {
	src := "import os; import sys\n" +
		"import json  # noqa: keep for compat\n" +
		"import numpy as np\n" +
		"from . import sibling\n" +
		"import pkg.mod as m, other\n"

	refs := extractPy(src)
	got := make(map[string]int, len(refs))
	for _, r := range refs {
		got[r.raw] = r.line
	}

	// "from . import sibling" extracts raw "." (the dotted module path
	// captured by pyFrom) — extractPy has never captured the imported
	// name for "from" statements, only the module path being imported
	// from, so that behavior is unchanged and intentionally out of
	// scope here.
	wantLine := map[string]int{
		"os":      1,
		"sys":     1,
		"json":    2,
		"numpy":   3,
		".":       4,
		"pkg.mod": 5,
		"other":   5,
	}
	for raw, line := range wantLine {
		if got[raw] != line {
			t.Errorf("want import %q at line %d, got line %d (all: %+v)", raw, line, got[raw], refs)
		}
	}
	if len(refs) != len(wantLine) {
		t.Errorf("want exactly %d import refs, got %d: %+v", len(wantLine), len(refs), refs)
	}
}
