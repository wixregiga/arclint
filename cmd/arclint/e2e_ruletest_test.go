package main

// Rule Tests and the published Rule Schema run through the real binary:
// the authoring loop's contract is that failures print ready-to-paste
// expect entries and any failing test exits with the findings code.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const ruleTestRules = `runtime: [go]

zones:
  core: core/**

rules:
  core/no-extra:
    on: core
    structure:
      forbid: ["core/extra/**"]
`

func TestRuleTestsRunThroughBinary(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "rules.arclint.yaml", ruleTestRules)
	write(t, dir, "core/keep.go", "package core\n")
	write(t, dir, ".arclint/tests/clean.yaml",
		"rule: \"core/no-extra\"\nfiles:\n  core/keep.go: |\n    package core\nexpect: []\n")

	stdout, stderr, code := runBin(t, dir, os.Environ(), "rules", "test")
	if code != 0 {
		t.Fatalf("rules test: exit %d, stderr %s", code, stderr)
	}
	if !strings.Contains(stdout, "ok   clean (core/no-extra)") || !strings.Contains(stdout, "1 passed · 0 failed") {
		t.Errorf("unexpected output:\n%s", stdout)
	}

	// A failing test prints a ready-to-paste expect entry and exits 1.
	write(t, dir, ".arclint/tests/violating.yaml",
		"rule: \"core/no-extra\"\nfiles:\n  core/extra/x.go: |\n    package extra\nexpect: []\n")
	stdout, _, code = runBin(t, dir, os.Environ(), "rules", "test")
	if code != 1 {
		t.Fatalf("failing rules test: exit %d, want 1\n%s", code, stdout)
	}
	for _, want := range []string{
		"FAIL violating (core/no-extra)",
		"- kind: violation",
		"path: core/extra/x.go",
		"1 passed · 1 failed",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("missing %q in:\n%s", want, stdout)
		}
	}

	// Selecting one test by name runs only it; an unknown name is a
	// configuration error.
	stdout, _, code = runBin(t, dir, os.Environ(), "rules", "test", "clean")
	if code != 0 || strings.Contains(stdout, "violating") {
		t.Errorf("name filter: exit %d\n%s", code, stdout)
	}
	_, stderr, code = runBin(t, dir, os.Environ(), "rules", "test", "absent")
	if code != 2 || !strings.Contains(stderr, "no rule test named") {
		t.Errorf("unknown test name: exit %d, stderr %s", code, stderr)
	}
}

// TestRuleTestEmptyExpectPrintsExactConsumesMessage locks the CLI
// authoring contract: with expect empty, unexpected findings print as
// ready-to-paste YAML including the evaluator's exact message. A Rule
// Author can copy Zone(s) ["adapters"] without reading check_imports.
func TestRuleTestEmptyExpectPrintsExactConsumesMessage(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "rules.arclint.yaml", `runtime: [go]
scan:
  unknown_imports: error
zones:
  core: core/**
  adapters: adapters/**
rules:
  core/consumes:
    on: core
    imports:
      internal: []
      external: forbid
`)
	write(t, dir, ".arclint/tests/disallowed_adapters_import.yaml",
		`rule: "core/consumes"
files:
  go.mod: "module example.com/app\n\ngo 1.26\n"
  core/a.go: "package core\n\nimport _ \"example.com/app/adapters\"\n"
  adapters/a.go: "package adapters\n"
expect: []
`)

	stdout, stderr, code := runBin(t, dir, os.Environ(), "rules", "test")
	if code != 1 {
		t.Fatalf("rules test: exit %d, want findings exit 1\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	// Exact Diagnostic text the consumes evaluator emits for a
	// disallowed internal import into Zone adapters, exposed
	// verbatim so authors paste it into expect.message.
	wantMessage := `import "example.com/app/adapters" resolves to Zone(s) ["adapters"], not in the allow-list of Zone "core"`
	for _, want := range []string{
		"FAIL disallowed_adapters_import (core/consumes)",
		"- kind: violation",
		"path: core/a.go",
		"line: 3",
		fmt.Sprintf("message: %q", wantMessage),
		"0 passed · 1 failed",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("missing %q in CLI output:\n%s", want, stdout)
		}
	}
}

func TestRuleSchemaCommand(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "rules.arclint.yaml", ruleTestRules)
	stdout, stderr, code := runBin(t, dir, os.Environ(), "rules", "schema")
	if code != 0 {
		t.Fatalf("rules schema: exit %d, stderr %s", code, stderr)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("schema output is not JSON: %v", err)
	}
	if _, ok := doc["$defs"]; !ok {
		t.Errorf("schema output lacks $defs:\n%.200s", stdout)
	}
	// The committed editor schema and the binary's output are the same
	// bytes; the byte-level drift test lives beside the yaml loader.
	committed, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(ruleSchemaLitmus)))
	if err != nil {
		t.Fatal(err)
	}
	if stdout != string(committed) {
		t.Errorf("rules schema output differs from %s", ruleSchemaLitmus)
	}

	// --write lands the schema under the project's schema directory by
	// default, byte-identical to the printed form.
	if _, stderr, code := runBin(t, dir, os.Environ(), "rules", "schema", "--write"); code != 0 {
		t.Fatalf("rules schema --write: exit %d, stderr %s", code, stderr)
	}
	written, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(ruleSchemaProjectPath)))
	if err != nil {
		t.Fatalf("rules schema --write did not create %s: %v", ruleSchemaProjectPath, err)
	}
	if string(written) != string(committed) {
		t.Errorf("rules schema --write bytes differ from %s", ruleSchemaLitmus)
	}
}

const (
	// ruleSchemaProjectPath is where rules schema --write lands the
	// schema relative to the project root; the litmus is the release
	// copy under docs/schemas.
	ruleSchemaProjectPath = ".arclint/schemas/rules.arclint.schema.json"
	ruleSchemaLitmus      = "docs/schemas/rules.arclint.schema.json"
)

const contentRuleRules = `runtime: [go]

zones:
  core: core/**

rules:
  core/no-panic:
    rationale: "Core code never panics; a value that cannot be built is an error."
    on: core
    content:
      forbid: '\bpanic\('
`

// TestContentRuleProvesItsPatternNotItsRationale runs a content Rule
// through every surface that shows its Assurance, and through the Rule
// Tests, which ask for a failing case drawn from its Rationale.
func TestContentRuleProvesItsPatternNotItsRationale(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "rules.arclint.yaml", contentRuleRules)
	write(t, dir, "core/keep.go", "package core\n")
	write(t, dir, ".arclint/tests/clean.yaml",
		"rule: \"core/no-panic\"\nfiles:\n  core/keep.go: |\n    package core\nexpect: []\n")

	const label = "exact for its pattern; rationale not checked"
	for _, args := range [][]string{
		{"rules", "--no-color"},
		{"rules", "core/no-panic", "--no-color"},
		{"context", "core/keep.go", "--no-color"},
		{"agents", "md"},
	} {
		stdout, stderr, code := runBin(t, dir, os.Environ(), args...)
		if code != 0 || !strings.Contains(stdout, label) {
			t.Errorf("%v: exit %d, no %q in:\n%s\n%s", args, code, label, stdout, stderr)
		}
	}
	stdout, stderr, code := runBin(t, dir, os.Environ(), "check", ".", "--no-color")
	if code != 0 || !strings.Contains(stdout, "coverage: these rules are exact for their pattern; their rationale is not checked: core/no-panic") {
		t.Errorf("check: exit %d\n%s\n%s", code, stdout, stderr)
	}
	stdout, _, _ = runBin(t, dir, os.Environ(), "check", ".", "--format", "json")
	if !strings.Contains(stdout, `"message": "rule core/no-panic is exact for its pattern; its rationale is not checked"`) {
		t.Errorf("check json carries no pattern-bound coverage:\n%s", stdout)
	}

	// Only a clean case: the Rule is named, and the run fails.
	stdout, _, code = runBin(t, dir, os.Environ(), "rules", "test", "--no-color")
	if code != 1 {
		t.Fatalf("rules test without a failing case: exit %d, want 1\n%s", code, stdout)
	}
	for _, want := range []string{
		"ok   clean (core/no-panic)",
		"content rules without a failing case",
		"  core/no-panic\n",
		"1 passed · 0 failed · 1 content rule(s) without a failing case",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("missing %q in:\n%s", want, stdout)
		}
	}
	stdout, _, code = runBin(t, dir, os.Environ(), "rules", "test", "--format", "json")
	var run struct {
		Results            []struct{ Passed bool } `json:"results"`
		WithoutFailingCase []string                `json:"withoutFailingCase"`
	}
	if err := json.Unmarshal([]byte(stdout), &run); err != nil || code != 1 ||
		len(run.Results) != 1 || !run.Results[0].Passed ||
		len(run.WithoutFailingCase) != 1 || run.WithoutFailingCase[0] != "core/no-panic" {
		t.Errorf("rules test json: exit %d, decode %v\n%s", code, err, stdout)
	}

	// One selected test says nothing of the suite.
	stdout, _, code = runBin(t, dir, os.Environ(), "rules", "test", "clean", "--no-color")
	if code != 0 || strings.Contains(stdout, "without a failing case") {
		t.Errorf("name filter: exit %d\n%s", code, stdout)
	}

	// A failing case drawn from the Rationale clears the report.
	write(t, dir, ".arclint/tests/panics.yaml", `rule: "core/no-panic"
files:
  core/value.go: |
    package core

    func Must(ok bool) {
        if !ok {
            panic("cannot build")
        }
    }
expect:
  - kind: violation
    path: core/value.go
    line: 5
    message: "forbidden content matching /\\bpanic\\(/"
`)
	stdout, _, code = runBin(t, dir, os.Environ(), "rules", "test", "--no-color")
	if code != 0 || strings.Contains(stdout, "without a failing case") || !strings.Contains(stdout, "2 passed · 0 failed\n") {
		t.Errorf("with a failing case: exit %d\n%s", code, stdout)
	}
}
