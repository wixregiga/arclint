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

modules:
  core:
    paths: ["core/**"]

contracts:
  core:
    invariants:
      - id: "t:core/no-extra"
        kind: structure
        forbid:
          - "core/extra/**"
`

func TestRuleTestsRunThroughBinary(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "rules.yaml", ruleTestRules)
	write(t, dir, "core/keep.go", "package core\n")
	write(t, dir, ".arclint/tests/clean.yaml",
		"rule: \"t:core/no-extra\"\nfiles:\n  core/keep.go: |\n    package core\nexpect: []\n")

	stdout, stderr, code := runBin(t, dir, os.Environ(), "rules", "test")
	if code != 0 {
		t.Fatalf("rules test: exit %d, stderr %s", code, stderr)
	}
	if !strings.Contains(stdout, "ok   clean (t:core/no-extra)") || !strings.Contains(stdout, "1 passed · 0 failed") {
		t.Errorf("unexpected output:\n%s", stdout)
	}

	// A failing test prints a ready-to-paste expect entry and exits 1.
	write(t, dir, ".arclint/tests/violating.yaml",
		"rule: \"t:core/no-extra\"\nfiles:\n  core/extra/x.go: |\n    package extra\nexpect: []\n")
	stdout, _, code = runBin(t, dir, os.Environ(), "rules", "test")
	if code != 1 {
		t.Fatalf("failing rules test: exit %d, want 1\n%s", code, stdout)
	}
	for _, want := range []string{
		"FAIL violating (t:core/no-extra)",
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
// Author can copy Module(s) ["adapters"] without reading check_imports.
func TestRuleTestEmptyExpectPrintsExactConsumesMessage(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "rules.yaml", `runtime: [go]
scan:
  unknown_imports: error
modules:
  core:
    paths: ["core/**"]
  adapters:
    paths: ["adapters/**"]
contracts:
  core:
    consumes:
      id: "t:core/consumes"
      internal: []
      external: forbid
      stdlib: allow
`)
	write(t, dir, ".arclint/tests/disallowed_adapters_import.yaml",
		`rule: "t:core/consumes"
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
	// disallowed internal import into Module adapters — exposed
	// verbatim so authors paste it into expect.message.
	wantMessage := `import "example.com/app/adapters" resolves to Module(s) ["adapters"], not in the allow-list of Module "core"`
	for _, want := range []string{
		"FAIL disallowed_adapters_import (t:core/consumes)",
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
	write(t, dir, "rules.yaml", ruleTestRules)
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
	committed, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "rules.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if stdout != string(committed) {
		t.Errorf("rules schema output differs from docs/rules.schema.json")
	}
}
