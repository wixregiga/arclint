package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/walk"
)

// writeTree materializes a fixture tree under root.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, content := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// listFiles walks root the same way the check command does and returns
// sorted root-relative slash paths.
func listFiles(t *testing.T, root string) []string {
	t.Helper()
	abs, err := walk.WalkFiles([]string{root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(abs))
	for _, a := range abs {
		rel, err := filepath.Rel(root, a)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, filepath.ToSlash(rel))
	}
	return out
}

// evalCfg runs Evaluate over the tree at root.
func evalCfg(t *testing.T, root string, cfg *config.File) []Violation {
	t.Helper()
	vs, err := Evaluate(cfg, root, listFiles(t, root))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	return vs
}

// evalOne runs a single rule.
func evalOne(t *testing.T, root, id string, r config.Rule) []Violation {
	t.Helper()
	return evalCfg(t, root, &config.File{Version: 1, Rules: map[string]config.Rule{id: r}})
}

func structureRule(sev config.Severity, p *config.StructureParams, files *config.FileFilter) config.Rule {
	return config.Rule{Type: config.CategoryStructure, Severity: sev, Description: "test", Files: files, Structure: p, FixHint: "fix it"}
}

func TestStructureRequire(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"main.go": "package main\n"})

	vs := evalOne(t, root, "req", structureRule(config.SeverityError,
		&config.StructureParams{Require: []string{"README.md", "docs/**"}}, nil))
	if len(vs) != 2 {
		t.Fatalf("want 2 violations, got %d: %+v", len(vs), vs)
	}
	if vs[0].Path != "README.md" || vs[1].Path != "docs/**" {
		t.Errorf("unexpected paths: %q, %q", vs[0].Path, vs[1].Path)
	}
	if vs[0].Line != nil {
		t.Error("structure violations must not carry a line")
	}

	writeTree(t, root, map[string]string{"README.md": "# x\n", "docs/a.md": "a\n"})
	if vs := evalOne(t, root, "req", structureRule(config.SeverityError,
		&config.StructureParams{Require: []string{"README.md", "docs/**"}}, nil)); len(vs) != 0 {
		t.Fatalf("want clean, got %+v", vs)
	}
}

func TestStructureForbid(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"pkg/utils/x.go":    "package utils\n",
		"pkg/core/y.go":     "package core\n",
		"legacy/utils/z.go": "package utils\n",
	})

	rule := structureRule(config.SeverityError,
		&config.StructureParams{Forbid: []string{"**/utils/**"}},
		&config.FileFilter{Exclude: []string{"legacy/**"}})
	vs := evalOne(t, root, "no-utils", rule)
	if len(vs) != 1 {
		t.Fatalf("want 1 violation, got %d: %+v", len(vs), vs)
	}
	v := vs[0]
	if v.Path != "pkg/utils/x.go" || v.RuleID != "no-utils" || v.Severity != config.SeverityError {
		t.Errorf("unexpected violation: %+v", v)
	}
	if !strings.Contains(v.Message, "**/utils/**") || v.FixHint != "fix it" {
		t.Errorf("unexpected message/hint: %+v", v)
	}
}

func TestCompileStyleTable(t *testing.T) {
	cases := []struct {
		style, name string
		ok          bool
	}{
		{"snake_case", "walk_test", true},
		{"snake_case", "BadName", false},
		{"camelCase", "fooBarBaz", true},
		{"camelCase", "FooBar", false},
		{"PascalCase", "FooBar", true},
		{"PascalCase", "fooBar", false},
		{"kebab-case", "foo-bar2", true},
		{"kebab-case", "foo_bar", false},
		{"SCREAMING_SNAKE_CASE", "FOO_BAR", true},
		{"SCREAMING_SNAKE_CASE", "foo_bar", false},
		{"lowercase", "foo", true},
		{"lowercase", "Foo", false},
		{"lowercase | kebab-case | regex:v[0-9]+", "v42", true},
		{"lowercase | kebab-case | regex:v[0-9]+", "V42", false},
	}
	for _, c := range cases {
		alts, err := compileStyle(c.style)
		if err != nil {
			t.Fatalf("compileStyle(%q): %v", c.style, err)
		}
		if got := matchesStyle(alts, c.name); got != c.ok {
			t.Errorf("style %q name %q: got %v want %v", c.style, c.name, got, c.ok)
		}
	}
	if _, err := compileStyle("bogusStyle"); err == nil {
		t.Error("unknown token must error")
	}
	if _, err := compileStyle("regex:["); err == nil {
		t.Error("bad regex must error")
	}
}

func TestNamingFiles(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/good_name.go": "package a\n",
		"internal/BadName.go":   "package a\n",
		"internal/readme.txt":   "not go\n",
	})
	rule := config.Rule{
		Type: config.CategoryNaming, Severity: config.SeverityError, Description: "t",
		Files:  &config.FileFilter{Include: []string{"**/*.go"}},
		Naming: &config.NamingParams{Style: "snake_case", Target: "file"},
	}
	vs := evalOne(t, root, "go-naming", rule)
	if len(vs) != 1 || vs[0].Path != "internal/BadName.go" {
		t.Fatalf("want 1 violation for BadName.go, got %+v", vs)
	}
}

func TestNamingDirs(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/BadDir/x.go":   "package a\n",
		"internal/good-dir/y.go": "package a\n",
	})
	rule := config.Rule{
		Type: config.CategoryNaming, Severity: config.SeverityWarn, Description: "t",
		Files:  &config.FileFilter{Include: []string{"internal/*/**"}},
		Naming: &config.NamingParams{Style: "lowercase | kebab-case", Target: "dir"},
	}
	vs := evalOne(t, root, "dir-naming", rule)
	if len(vs) != 1 || vs[0].Path != "internal/BadDir" || vs[0].Severity != config.SeverityWarn {
		t.Fatalf("want 1 violation for internal/BadDir, got %+v", vs)
	}
}

func TestContentMustNotContain(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/a.go": "package a\n\nvar x = fmt.Println(1)\n",
		"cmd/b.go":      "package b\n\nvar y = fmt.Println(2)\n",
	})
	rule := config.Rule{
		Type: config.CategoryContent, Severity: config.SeverityWarn, Description: "t",
		Files: &config.FileFilter{Include: []string{"internal/**/*.go"}},
		Content: &config.ContentParams{
			MustNotContain: []config.ContentMatcher{{Pattern: `fmt\.Println\(`, Message: "use slog"}},
		},
	}
	vs := evalOne(t, root, "no-println", rule)
	if len(vs) != 1 {
		t.Fatalf("want 1 violation, got %+v", vs)
	}
	v := vs[0]
	if v.Path != "internal/a.go" || v.Line == nil || *v.Line != 3 || v.Message != "use slog" {
		t.Fatalf("unexpected violation: %+v", v)
	}
}

func TestContentMustContain(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"a.go": "// license\npackage a\n",
		"b.go": "package b\n",
	})
	rule := config.Rule{
		Type: config.CategoryContent, Severity: config.SeverityError, Description: "t",
		Files: &config.FileFilter{Include: []string{"**/*.go"}},
		Content: &config.ContentParams{
			MustContain: []config.ContentMatcher{{Pattern: `// license`}},
		},
	}
	vs := evalOne(t, root, "license", rule)
	if len(vs) != 1 || vs[0].Path != "b.go" || vs[0].Line != nil {
		t.Fatalf("want 1 line-less violation for b.go, got %+v", vs)
	}
}

func TestCustomCommand(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"a.go": "package a\n"})
	rule := config.Rule{
		Type: config.CategoryCustom, Severity: config.SeverityWarn, Description: "t",
		FixHint: "rule hint",
		Custom: &config.CustomParams{
			Command:        []string{"/bin/sh", "-c", `cat >/dev/null; printf '[{"path":"a.go","line":2,"message":"custom bad"}]'`},
			TimeoutSeconds: 10,
		},
	}
	vs := evalOne(t, root, "ext", rule)
	if len(vs) != 1 {
		t.Fatalf("want 1 violation, got %+v", vs)
	}
	v := vs[0]
	if v.Path != "a.go" || v.Line == nil || *v.Line != 2 || v.Message != "custom bad" || v.FixHint != "rule hint" {
		t.Fatalf("unexpected violation: %+v", v)
	}
	if v.Severity != config.SeverityWarn || v.Category != config.CategoryCustom {
		t.Fatalf("severity/category not injected: %+v", v)
	}
}

func TestCustomCommandFailure(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"a.go": "package a\n"})
	cfg := &config.File{Version: 1, Rules: map[string]config.Rule{
		"ext": {
			Type: config.CategoryCustom, Severity: config.SeverityError, Description: "t",
			Custom: &config.CustomParams{Command: []string{"/bin/sh", "-c", "exit 3"}, TimeoutSeconds: 10},
		},
	}}
	if _, err := Evaluate(cfg, root, listFiles(t, root)); err == nil || !strings.Contains(err.Error(), "failed") {
		t.Fatalf("want execution error, got %v", err)
	}
}

func TestCustomCommandTimeout(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"a.go": "package a\n"})
	cfg := &config.File{Version: 1, Rules: map[string]config.Rule{
		"slow": {
			Type: config.CategoryCustom, Severity: config.SeverityError, Description: "t",
			Custom: &config.CustomParams{Command: []string{"/bin/sh", "-c", "sleep 5"}, TimeoutSeconds: 1},
		},
	}}
	if _, err := Evaluate(cfg, root, listFiles(t, root)); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("want timeout error, got %v", err)
	}
}

func TestBaselineSuppression(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"pkg/utils/x.go": "package utils\n"})
	rule := structureRule(config.SeverityError, &config.StructureParams{Forbid: []string{"**/utils/**"}}, nil)

	vs := evalOne(t, root, "no-utils", rule)
	if len(vs) != 1 {
		t.Fatalf("want 1 violation before baselining, got %+v", vs)
	}

	baseline := "- ruleId: no-utils\n  path: " + vs[0].Path + "\n  messageHash: " + MessageHash(vs[0].Message) + "\n"
	writeTree(t, root, map[string]string{".arclint/baseline.yaml": baseline})

	cfg := &config.File{
		Version:  1,
		Baseline: ".arclint/baseline.yaml",
		Rules:    map[string]config.Rule{"no-utils": rule},
	}
	if vs := evalCfg(t, root, cfg); len(vs) != 0 {
		t.Fatalf("baseline must suppress, got %+v", vs)
	}
}

func TestIgnoreSuppression(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"legacy/utils/x.go": "package utils\n"})
	rule := structureRule(config.SeverityError, &config.StructureParams{Forbid: []string{"**/utils/**"}}, nil)

	cfg := &config.File{Version: 1,
		Ignore: []config.Ignore{{Path: "legacy/**"}},
		Rules:  map[string]config.Rule{"no-utils": rule},
	}
	if vs := evalCfg(t, root, cfg); len(vs) != 0 {
		t.Fatalf("path ignore must suppress everything, got %+v", vs)
	}

	cfg.Ignore = []config.Ignore{{Path: "legacy/**", Rules: []string{"some-other-rule"}}}
	if vs := evalCfg(t, root, cfg); len(vs) != 1 {
		t.Fatalf("rule-scoped ignore for another rule must not suppress, got %+v", vs)
	}
}

func TestSeverityOffSkips(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"pkg/utils/x.go": "package utils\n"})
	rule := structureRule(config.SeverityOff, &config.StructureParams{Forbid: []string{"**/utils/**"}}, nil)
	if vs := evalOne(t, root, "no-utils", rule); len(vs) != 0 {
		t.Fatalf("severity off must skip, got %+v", vs)
	}
}

func TestExtendsRecommended(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"pkg/utils/x.go": "package utils\n"})

	cfg := &config.File{Version: 1, Extends: []string{"arclint:recommended"}}
	vs := evalCfg(t, root, cfg)
	got := map[string]config.Severity{}
	for _, v := range vs {
		got[v.RuleID] = v.Severity
	}
	if got["no-utils-dir"] != config.SeverityError || got["readme-required"] != config.SeverityWarn {
		t.Fatalf("want preset violations, got %+v", vs)
	}

	// Local same-id rule fully replaces the preset rule; severity off is
	// the idiomatic disable.
	cfg.Rules = map[string]config.Rule{
		"no-utils-dir": structureRule(config.SeverityOff, &config.StructureParams{Forbid: []string{"**/utils/**"}}, nil),
	}
	vs = evalCfg(t, root, cfg)
	for _, v := range vs {
		if v.RuleID == "no-utils-dir" {
			t.Fatalf("local off must disable the preset rule, got %+v", vs)
		}
	}
}

func TestExtendsUnknownPreset(t *testing.T) {
	cfg := &config.File{Version: 1, Extends: []string{"arclint:bogus"}}
	if _, err := Evaluate(cfg, t.TempDir(), nil); err == nil || !strings.Contains(err.Error(), "unknown preset") {
		t.Fatalf("want unknown-preset error, got %v", err)
	}
}
