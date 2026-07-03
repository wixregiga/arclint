package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeRules(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadValidExample(t *testing.T) {
	// The full `arclint init` default from docs/design/rules.md §6.
	f, err := Load(filepath.Join("testdata", "rules_valid.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if f.Version != 1 {
		t.Errorf("Version = %d, want 1", f.Version)
	}
	if got, want := len(f.Rules), 8; got != want {
		t.Errorf("len(Rules) = %d, want %d", got, want)
	}
	if got, want := len(f.Exclude), 2; got != want {
		t.Errorf("len(Exclude) = %d, want %d", got, want)
	}
	if len(f.Ignore) != 1 || f.Ignore[0].Path != "legacy/**" || len(f.Ignore[0].Rules) != 2 {
		t.Errorf("Ignore = %+v, want one entry for legacy/** with 2 rules", f.Ignore)
	}

	st := f.Rules["no-utils-dir"]
	if st.Type != CategoryStructure || st.Structure == nil || len(st.Structure.Forbid) != 2 {
		t.Errorf("no-utils-dir decoded wrong: %+v", st)
	}

	nm := f.Rules["package-dir-naming"]
	if nm.Naming == nil || nm.Naming.Target != "dir" || nm.Severity != SeverityWarn {
		t.Errorf("package-dir-naming decoded wrong: %+v", nm.Naming)
	}
	if def := f.Rules["go-file-naming"]; def.Naming == nil || def.Naming.Target != "file" {
		t.Errorf("go-file-naming target default = %+v, want file", def.Naming)
	}

	dep := f.Rules["layered-architecture"]
	if dep.Dependencies == nil || dep.Dependencies.Contract != "layers" || len(dep.Dependencies.Layers) != 3 {
		t.Errorf("layered-architecture decoded wrong: %+v", dep.Dependencies)
	}

	ct := f.Rules["no-println-outside-cmd"]
	if ct.Content == nil || len(ct.Content.MustNotContain) != 1 || ct.Files == nil || len(ct.Files.Exclude) != 1 {
		t.Errorf("no-println-outside-cmd decoded wrong: %+v", ct)
	}

	cu := f.Rules["openapi-lint"]
	if cu.Severity != SeverityOff {
		t.Errorf("openapi-lint severity = %q, want off", cu.Severity)
	}
	if cu.Custom == nil || cu.Custom.TimeoutSeconds != 60 || len(cu.Custom.Command) != 1 {
		t.Errorf("openapi-lint custom params decoded wrong: %+v", cu.Custom)
	}
}

func TestLoadRejectsBadCategory(t *testing.T) {
	path := writeRules(t, `
version: 1
rules:
  bad-rule:
    type: layout
    severity: error
    description: Not a real category.
    params: {}
`)
	if _, err := Load(path); err == nil {
		t.Fatal("Load accepted unknown category \"layout\", want schema error")
	}
}

func TestLoadCoercesBareOffSeverity(t *testing.T) {
	// A bare `off` may parse as YAML-1.1 boolean false; the loader must
	// coerce it to the string "off" (decision D10).
	path := writeRules(t, `
version: 1
rules:
  quiet-rule:
    type: structure
    severity: off
    description: Disabled rule with unquoted off.
    params:
      forbid: ["**/utils/**"]
`)
	f, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := f.Rules["quiet-rule"].Severity; got != SeverityOff {
		t.Errorf("severity = %q, want %q", got, SeverityOff)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "rules.yaml"))
	if err == nil {
		t.Fatal("Load succeeded on a missing file")
	}
	if !strings.Contains(err.Error(), "arclint init") {
		t.Errorf("missing-file error should point at `arclint init`, got: %v", err)
	}
}

func TestLoadRejectsUnknownIgnoreRuleID(t *testing.T) {
	path := writeRules(t, `
version: 1
ignore:
  - path: "legacy/**"
    rules: [ghost-rule]
rules:
  real-rule:
    type: structure
    severity: error
    description: A real rule.
    params:
      forbid: ["**/utils/**"]
`)
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "ghost-rule") {
		t.Fatalf("want semantic error naming ghost-rule, got: %v", err)
	}
}

func TestFindConfigRoot(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{".arclint", ".git", "sub/deep"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got, err := FindConfigRoot(filepath.Join(root, "sub", "deep"))
	if err != nil {
		t.Fatalf("FindConfigRoot: %v", err)
	}
	if want, _ := filepath.Abs(root); got != want {
		t.Errorf("FindConfigRoot = %q, want %q", got, want)
	}

	// A git root without .arclint is the discovery boundary.
	bare := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bare, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := FindConfigRoot(bare); err == nil {
		t.Error("FindConfigRoot found a config root above the git root")
	}
}

// TestLoadParseErrorIsOneLineSummary covers the F2 bug report: a corrupt
// rules.yaml produced a goccy/go-yaml parse error whose default
// multi-line, source-reflowing FormatError output got glued behind our
// "error: " prefix, reading as merged/mangled source lines. The fix must
// render a single summary line with line:col plus at most one clean quoted
// source line and a caret — never goccy's repeated-context window.
func TestLoadParseErrorIsOneLineSummary(t *testing.T) {
	// Bad indentation: "type:" is dedented relative to its parent mapping
	// item, and "severity:" is then indented deeper than "type:" — both
	// invalid, so goccy reports a syntax error with a 3-line context
	// window by default.
	path := writeRules(t, "version: 1\nrules:\n  bad-rule:\n  type: naming\n   severity: error\n")

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load accepted corrupt YAML, want parse error")
	}
	msg := err.Error()

	lines := strings.Split(msg, "\n")
	if len(lines) == 0 {
		t.Fatal("error message is empty")
	}
	summary := lines[0]

	if !strings.Contains(summary, "invalid at") {
		t.Errorf("summary line = %q, want it to contain line:col info (\"invalid at\")", summary)
	}
	if !strings.HasPrefix(summary, path) {
		t.Errorf("summary line = %q, want it to start with the rules.yaml path", summary)
	}

	// No merged-line artifact: goccy's own multi-line window repeats a
	// line of source both bare and prefixed with "> ", plus a blank
	// trailing line — none of that raw shape may leak through.
	if strings.Contains(msg, "> ") {
		t.Errorf("error contains goccy's raw \"> \" context marker, want it stripped: %q", msg)
	}
	if strings.Count(msg, "^") > 1 {
		t.Errorf("error contains more than one caret line, want exactly one: %q", msg)
	}
	// At most: one summary line + one source line + one caret line.
	if len(lines) > 3 {
		t.Errorf("error has %d lines, want at most 3 (summary, source, caret), got: %q", len(lines), msg)
	}
}
