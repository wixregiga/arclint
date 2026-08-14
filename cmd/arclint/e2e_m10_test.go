package main

// M10 affordance tests: arclint facts, check --only, inert-rule
// visibility, absolute extension-resolution errors, rules scaffold, and
// the explain nudge — the author/debug loop, proven through the binary.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// writeTree materializes a map of repo-relative files into a temp root.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestFactsCommand(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"rules.yaml": "runtime: [go]\n\nmodules:\n  app: [\"app/**\"]\n",
		"app/svc.go": `package app

type Repo interface {
	Find(id string) (string, error)
}

func New(size int, opts ...string) *int { return &size }
`,
	})

	stdout, stderr, code := runBin(t, dir, os.Environ(), "facts", "app/svc.go")
	if code != 0 {
		t.Fatalf("facts: exit %d, stderr %s", code, stderr)
	}
	for _, want := range []string{
		"package: app",
		"(id string) -> (string, error)",
		"(size int, ...opts string) -> *int",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("facts human output missing %q:\n%s", want, stdout)
		}
	}

	stdout, stderr, code = runBin(t, dir, os.Environ(), "facts", "app/svc.go", "--format", "json")
	if code != 0 {
		t.Fatalf("facts json: exit %d, stderr %s", code, stderr)
	}
	var facts struct {
		Decls []struct {
			Kind   string `json:"kind"`
			Name   string `json:"name"`
			Params []struct {
				Name     string `json:"name"`
				Type     string `json:"type"`
				Variadic bool   `json:"variadic"`
			} `json:"params"`
			Results []string `json:"results"`
		} `json:"decls"`
	}
	if err := json.Unmarshal([]byte(stdout), &facts); err != nil {
		t.Fatalf("facts json: %v\n%s", err, stdout)
	}
	var found bool
	for _, d := range facts.Decls {
		if d.Kind == "func" && d.Name == "New" {
			found = true
			if len(d.Params) != 2 || !d.Params[1].Variadic || d.Results[0] != "*int" {
				t.Errorf("New signature: %+v", d)
			}
		}
	}
	if !found {
		t.Errorf("func New missing from json decls:\n%s", stdout)
	}

	// A file outside every language target is a usage error that says why.
	_, stderr, code = runBin(t, dir, os.Environ(), "facts", "rules.yaml")
	if code != 2 || !strings.Contains(stderr, "no language target") {
		t.Errorf("facts on rules.yaml: exit %d, stderr %s", code, stderr)
	}
}

func TestCheckOnly(t *testing.T) {
	dir := copyFixture(t, "extension-handler-naming")

	stdout, _, code := runBin(t, dir, os.Environ(), "check", ".", "--only", "rules.handler-naming[0]", "--format", "json")
	if code != 1 {
		t.Fatalf("--only exit = %d, want 1\n%s", code, stdout)
	}
	var violations []map[string]any
	if err := json.Unmarshal([]byte(stdout), &violations); err != nil {
		t.Fatal(err)
	}
	if len(violations) != 1 || violations[0]["ruleId"] != "rules.handler-naming[0]" {
		t.Errorf("filtered violations: %v", violations)
	}

	// A typo'd --only must fail loudly, never report a false clean.
	_, stderr, code := runBin(t, dir, os.Environ(), "check", ".", "--only", "rules.handler-naming[9]")
	if code != 2 || !strings.Contains(stderr, "matches no rule id or namespace") {
		t.Errorf("unknown --only: exit %d, stderr %s", code, stderr)
	}
}

// TestCheckLineAndSarifFormats proves the two editor/CI encodings
// through the binary: line for problemMatcher/errorformat, SARIF for
// code scanning.
func TestCheckLineAndSarifFormats(t *testing.T) {
	dir := copyFixture(t, "extension-handler-naming")

	stdout, _, code := runBin(t, dir, os.Environ(), "check", ".", "--format", "line")
	if code != 1 {
		t.Fatalf("--format line: exit %d\n%s", code, stdout)
	}
	lineRe := regexp.MustCompile(`(?m)^internal/api/handlers/broken\.go:\d+: error: .+ \[rules\.handler-naming\[0\]\]$`)
	if !lineRe.MatchString(stdout) {
		t.Errorf("line format mismatch:\n%s", stdout)
	}

	stdout, _, code = runBin(t, dir, os.Environ(), "check", ".", "--format", "sarif")
	if code != 1 {
		t.Fatalf("--format sarif: exit %d", code)
	}
	var log struct {
		Version string `json:"version"`
		Runs    []struct {
			Results []struct {
				RuleID string `json:"ruleId"`
				Level  string `json:"level"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal([]byte(stdout), &log); err != nil {
		t.Fatalf("sarif is not valid JSON: %v", err)
	}
	if log.Version != "2.1.0" || len(log.Runs) != 1 || len(log.Runs[0].Results) != 2 {
		t.Errorf("sarif shape: %s", stdout)
	}
	if log.Runs[0].Results[0].Level != "error" {
		t.Errorf("sarif level: %+v", log.Runs[0].Results[0])
	}
}

const inertExtension = `import { defineRule, s } from "arclint";

export default defineRule({
  type: "aggregate-purity",
  description: "Reports each configured aggregate (test rule).",
  params: s.object({ aggregates: s.array(s.string()).default([]) }),
  check(ctx, params) {
    for (const name of params.aggregates as string[]) {
      ctx.report({ path: "rules.yaml", message: "aggregate " + name });
    }
  },
});
`

const dormantExtension = `import { defineRule, s } from "arclint";

export default defineRule({
  type: "probe-unused",
  description: "Registered by no rules.yaml instance (test rule).",
  params: s.object({}),
  check(ctx, params) {},
});
`

func TestInertRuleVisibility(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"rules.yaml": "runtime: [go]\n\nmodules:\n  app: [\"app/**\"]\n\n" +
			"rules:\n  - type: aggregate-purity\n    params: { aggregates: [] }\n",
		"app/svc.go":                             "package app\n",
		".arclint/extensions/aggregate-purity.ts": inertExtension,
		".arclint/extensions/probe-unused.ts":     dormantExtension,
	})

	stdout, stderr, code := runBin(t, dir, os.Environ(), "load", "rules.yaml")
	if code != 0 {
		t.Fatalf("load: exit %d, stderr %s", code, stderr)
	}
	if !strings.Contains(stdout, "registered but not instantiated: probe-unused") {
		t.Errorf("load does not list the dormant type:\n%s", stdout)
	}
	if !strings.Contains(stderr, `param "aggregates" is an empty list (rule may be inert)`) {
		t.Errorf("load does not warn on the inert instance:\n%s", stderr)
	}

	// check warns too: the silent-in-the-field case is silent no more.
	stdout, stderr, code = runBin(t, dir, os.Environ(), "check", ".")
	if code != 0 {
		t.Fatalf("check: exit %d\nstdout %s\nstderr %s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, `param "aggregates" is an empty list`) {
		t.Errorf("check does not warn on the inert instance:\n%s", stderr)
	}
}

func TestUnknownRuleTypeErrorPrintsAbsoluteDir(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"rules.yaml": "runtime: [go]\n\nmodules:\n  app: [\"app/**\"]\n\n" +
			"rules:\n  - type: ghost-rule\n",
		"app/svc.go": "package app\n",
	})
	_, stderr, code := runBin(t, dir, os.Environ(), "check", ".")
	wantDir := filepath.Join(dir, ".arclint", "extensions")
	if code != 2 || !strings.Contains(stderr, wantDir) {
		t.Errorf("unknown rule type: exit %d, stderr must name %s:\n%s", code, wantDir, stderr)
	}
}

func TestScaffoldRedFirst(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"rules.yaml": "runtime: [go]\n\nmodules:\n  app: [\"app/**\"]\n",
		"app/svc.go": "package app\n",
	})

	stdout, stderr, code := runBin(t, dir, os.Environ(), "rules", "scaffold", "naming-probe")
	if code != 0 {
		t.Fatalf("scaffold: exit %d, stderr %s", code, stderr)
	}
	for _, want := range []string{"wrote", "type: naming-probe", "id: naming-probe", "rules test"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("scaffold output missing %q:\n%s", want, stdout)
		}
	}
	for _, rel := range []string{".arclint/extensions/naming-probe.ts", ".arclint/tests/naming-probe.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("scaffold did not write %s: %v", rel, err)
		}
	}

	// Re-running refuses to overwrite the author's work.
	_, stderr, code = runBin(t, dir, os.Environ(), "rules", "scaffold", "naming-probe")
	if code != 2 || !strings.Contains(stderr, "refusing to overwrite") {
		t.Errorf("scaffold overwrite: exit %d, stderr %s", code, stderr)
	}

	// Red first: the scaffolded case fails until the rule reports.
	stdout, _, code = runBin(t, dir, os.Environ(), "rules", "test")
	if code != 1 {
		t.Fatalf("rules test on scaffold: exit %d, want 1\n%s", code, stdout)
	}
	if !strings.Contains(stdout, "missing:") || !strings.Contains(stdout, "naming-probe") {
		t.Errorf("scaffolded case failure not visible:\n%s", stdout)
	}

	// Bad names are rejected before anything is written.
	_, stderr, code = runBin(t, dir, os.Environ(), "rules", "scaffold", "Bad_Name")
	if code != 2 || !strings.Contains(stderr, "kebab-case") {
		t.Errorf("scaffold bad name: exit %d, stderr %s", code, stderr)
	}
}

func TestExplainNudgesRulesTest(t *testing.T) {
	dir := t.TempDir()
	stdout, stderr, code := runBin(t, dir, os.Environ(), "explain", "naming")
	if code != 0 {
		t.Fatalf("explain: exit %d, stderr %s", code, stderr)
	}
	if !strings.Contains(stdout, "arclint rules test") || !strings.Contains(stdout, "rules scaffold") {
		t.Errorf("explain misses the rules-test nudge:\n%s", stdout)
	}
}
