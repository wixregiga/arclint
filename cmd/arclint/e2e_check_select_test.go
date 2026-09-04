package main

// check --only / --exclude narrow one run to the rules matching exact
// ids or path patterns, with selectors that match nothing failing
// loudly: the single-rule pathway for trying a rule on the real
// repository.

import (
	"os"
	"strings"
	"testing"
)

const selectRules = `runtime: [go]

modules:
  core: core/**

rules:
  core/no-extra:
    on: core
    structure:
      forbid: ["core/extra/**"]
  core/has-keep:
    on: core
    structure:
      require: ["core/keep.go"]
`

func TestCheckOnlyAndExclude(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "rules.yaml", selectRules)
	write(t, dir, "core/keep.go", "package core\n")
	write(t, dir, "core/extra/x.go", "package extra\n")

	// The full check fires the forbid violation.
	stdout, _, code := runBin(t, dir, os.Environ(), "check")
	if code != 1 || !strings.Contains(stdout, "core/no-extra") {
		t.Fatalf("full check: exit %d\n%s", code, stdout)
	}
	if !strings.Contains(stdout, "2 rule(s) applied") {
		t.Errorf("full check applied count:\n%s", stdout)
	}

	// --only narrows to the passing rule: clean exit, one rule applied.
	stdout, _, code = runBin(t, dir, os.Environ(), "check", "--only", "core/has-keep")
	if code != 0 || !strings.Contains(stdout, "1 rule(s) applied") {
		t.Errorf("--only exact: exit %d\n%s", code, stdout)
	}

	// --exclude drops the violating rule: same effect.
	stdout, _, code = runBin(t, dir, os.Environ(), "check", "--exclude", "core/no-extra")
	if code != 0 || !strings.Contains(stdout, "1 rule(s) applied") {
		t.Errorf("--exclude: exit %d\n%s", code, stdout)
	}

	// A pattern selects both; the violation still gates.
	stdout, _, code = runBin(t, dir, os.Environ(), "check", "--only", "core/*")
	if code != 1 || !strings.Contains(stdout, "2 rule(s) applied") {
		t.Errorf("--only pattern: exit %d\n%s", code, stdout)
	}

	// Comma-separated selectors work in one flag value.
	stdout, _, code = runBin(t, dir, os.Environ(), "check", "--only", "core/no-extra,core/has-keep")
	if code != 1 || !strings.Contains(stdout, "2 rule(s) applied") {
		t.Errorf("--only comma list: exit %d\n%s", code, stdout)
	}

	// A selector matching nothing is a loud configuration error.
	_, stderr, code := runBin(t, dir, os.Environ(), "check", "--only", "core/nope")
	if code != 2 || !strings.Contains(stderr, "matches no configured rule") {
		t.Errorf("unmatched selector: exit %d, stderr %s", code, stderr)
	}
}

func TestRulesSelectors(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "rules.yaml", selectRules)
	write(t, dir, "core/keep.go", "package core\n")

	// A pattern narrows the listing to its matches.
	stdout, _, code := runBin(t, dir, os.Environ(), "rules", "core/*")
	if code != 0 || !strings.Contains(stdout, "core/no-extra") ||
		!strings.Contains(stdout, "core/has-keep") || strings.Contains(stdout, "type:") {
		t.Errorf("rules pattern: exit %d\n%s", code, stdout)
	}

	// A trailing-slash prefix does the same.
	stdout, _, code = runBin(t, dir, os.Environ(), "rules", "core/")
	if code != 0 || !strings.Contains(stdout, "core/no-extra") || !strings.Contains(stdout, "core/has-keep") {
		t.Errorf("rules prefix: exit %d\n%s", code, stdout)
	}

	// A selector resolving to one rule shows the complete detail.
	stdout, _, code = runBin(t, dir, os.Environ(), "rules", "core/has-")
	if code != 0 || !strings.Contains(stdout, "type:") || !strings.Contains(stdout, "core/has-keep") {
		t.Errorf("rules single-match detail: exit %d\n%s", code, stdout)
	}

	// A selector matching nothing stays loud.
	_, stderr, code := runBin(t, dir, os.Environ(), "rules", "core/nope*")
	if code != 2 || !strings.Contains(stderr, "matches no configured rule") {
		t.Errorf("rules unmatched: exit %d, stderr %s", code, stderr)
	}
}

func TestCompletionRuleSelectors(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "rules.yaml", selectRules)

	// --only completes rule ids.
	stdout, _, code := runBin(t, dir, os.Environ(), "__complete", "check", "--only", "")
	if code != 0 || !strings.Contains(stdout, "core/no-extra") || !strings.Contains(stdout, "core/has-keep") {
		t.Errorf("--only completion: exit %d\n%s", code, stdout)
	}

	// After a comma the typed segments stay as the inserted prefix.
	stdout, _, code = runBin(t, dir, os.Environ(), "__complete", "check", "--only", "core/no-extra,")
	if code != 0 || !strings.Contains(stdout, "core/no-extra,core/has-keep") {
		t.Errorf("--only comma completion: exit %d\n%s", code, stdout)
	}

	// --exclude completes the same candidates.
	stdout, _, code = runBin(t, dir, os.Environ(), "__complete", "check", "--exclude", "")
	if code != 0 || !strings.Contains(stdout, "core/has-keep") {
		t.Errorf("--exclude completion: exit %d\n%s", code, stdout)
	}
}
