package patterns_test

// The full pattern gate (each builtin, materialized per runtime, loads
// and checks clean through the real binary) lives in cmd/arclint's e2e
// tests: pulling config/engine into this package's tests would break the
// patterns module's leaf contract in rules.yaml. Here: the pure surface.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wixregiga/arclint/internal/patterns"
)

func TestBuiltinsPresent(t *testing.T) {
	builtins, err := patterns.Builtins()
	if err != nil {
		t.Fatal(err)
	}
	if len(builtins) == 0 {
		t.Fatal("no builtin patterns embedded")
	}
	for _, p := range builtins {
		if p.Description == "" || len(p.Runtimes) == 0 || len(p.RulesYAML) == 0 {
			t.Errorf("pattern %q: incomplete bundle: %+v", p.Name, p)
		}
	}
}

func TestMaterializeRefusesOverwriteWithoutForce(t *testing.T) {
	p, err := patterns.Find("", "starter")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "rules.yaml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Materialize(root, []string{"go"}, false); err == nil {
		t.Error("existing rules.yaml overwritten without force")
	}
	if _, err := p.Materialize(root, []string{"go"}, true); err != nil {
		t.Errorf("force overwrite failed: %v", err)
	}
}

func TestRenderRulesRejectsUnsupportedRuntime(t *testing.T) {
	p, err := patterns.Find("", "feature-slice")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.RenderRules([]string{"ts"}); err == nil {
		t.Error("feature-slice rendered for ts, which it does not support")
	}
	out, err := p.RenderRules([]string{"go"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "runtime: [go]"; !containsLine(out, want) {
		t.Errorf("rendered template missing %q", want)
	}
}

func TestLocalPatternsDiscoveredAndShadowing(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".arclint", "patterns", "fsd", "go")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "pattern.yaml"),
		"description: \"Team FSD variant.\"\nruntimes: [go]\n")
	writeFile(t, filepath.Join(dir, "rules.yaml"),
		"runtime: [go]\nmodules:\n  all: [\"**\"]\n")

	local, err := patterns.Local(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(local) != 1 || local[0].Name != "fsd/go" || local[0].Source != ".arclint/patterns/fsd/go" {
		t.Fatalf("local = %+v", local)
	}

	// A local pattern named like a builtin shadows it in All().
	shadow := filepath.Join(root, ".arclint", "patterns", "starter")
	if err := os.MkdirAll(shadow, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(shadow, "pattern.yaml"),
		"description: \"Local starter.\"\nruntimes: [go]\n")
	writeFile(t, filepath.Join(shadow, "rules.yaml"),
		"runtime: [go]\nmodules:\n  all: [\"**\"]\n")
	all, err := patterns.All(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range all {
		if p.Name == "starter" && p.Source == "builtin" {
			t.Error("local starter did not shadow the builtin")
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func containsLine(data []byte, line string) bool {
	for _, l := range splitLines(string(data)) {
		if l == line {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
