package main

// Dynamic completion runs through the hidden __complete command the
// shells call on TAB. These tests invoke it exactly as a shell would:
// values must appear for real rulesets and degrade to nothing — never
// an error — without one.

import (
	"os"
	"strings"
	"testing"
)

func TestCompletionValues(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"rules.yaml": "runtime: [go]\n\nmodules:\n  app:\n    paths: [\"app/**\"]\n    description: \"application services\"\n",
		"app/svc.go": "package app\n",
	})

	stdout, stderr, code := runBin(t, dir, os.Environ(), "__complete", "module", "info", "")
	if code != 0 {
		t.Fatalf("__complete module info: exit %d, stderr %s", code, stderr)
	}
	if !strings.Contains(stdout, "app\tapplication services") || !strings.Contains(stdout, ":4") {
		t.Errorf("module completion missing candidates or NoFileComp directive:\n%s", stdout)
	}

	// Rule ids for --only, including derived extension instance ids.
	fixture := copyFixture(t, "extension-handler-naming")
	stdout, _, code = runBin(t, fixture, os.Environ(), "__complete", "check", "--only", "")
	if code != 0 {
		t.Fatalf("__complete check --only: exit %d", code)
	}
	for _, want := range []string{"rules.handler-naming[0]", "rules.wiring-audit[1]"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("--only completion missing %q:\n%s", want, stdout)
		}
	}

	// Patterns complete without any rules.yaml.
	empty := t.TempDir()
	stdout, _, code = runBin(t, empty, os.Environ(), "__complete", "init", "--pattern", "")
	if code != 0 {
		t.Fatalf("__complete init --pattern: exit %d", code)
	}
	for _, want := range []string{"ddd-flat", "starter"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("pattern completion missing %q:\n%s", want, stdout)
		}
	}

	// Explain kinds come from the builtin doc table.
	stdout, _, _ = runBin(t, empty, os.Environ(), "__complete", "explain", "")
	if !strings.Contains(stdout, "naming\t") {
		t.Errorf("explain completion missing builtin kinds:\n%s", stdout)
	}
}

// TestCompletionWithoutRules proves the no-rules.yaml degradation: no
// candidates, no error, NoFileComp directive so the shell shows nothing
// instead of falling back to filenames.
func TestCompletionWithoutRules(t *testing.T) {
	dir := t.TempDir()
	stdout, _, code := runBin(t, dir, os.Environ(), "__complete", "module", "info", "")
	if code != 0 {
		t.Fatalf("__complete without rules.yaml: exit %d", code)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 1 || lines[0] != ":4" {
		t.Errorf("expected only the :4 directive, got:\n%s", stdout)
	}
}
