package main

// End-to-end shell completion tests: the compiled binary is driven
// through cobra's hidden __complete command exactly as a shell would
// on TAB. Candidates end with the ":4" directive line (the numeric
// form of ShellCompDirectiveNoFileComp), which the shell parses and
// never displays.

import (
	"os"
	"strings"
	"testing"
)

// completionRules is a minimal but real ruleset: one naming Rule
// whose id the completion callbacks must surface.
const completionRules = `runtime: [go]
modules:
  src: src/**
rules:
  src/snake:
    on: src
    files: "src/**/*.go"
    naming: snake_case
`

// containsLine reports whether output has want as one complete line;
// completion candidates and the directive are line-oriented.
func containsLine(output, want string) bool {
	for _, line := range strings.Split(output, "\n") {
		if line == want || strings.HasPrefix(line, want+"\t") {
			return true
		}
	}
	return false
}

// TestCompletionListsRuleIDs proves rules completes its
// positional argument from the configured ruleset.
func TestCompletionListsRuleIDs(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", completionRules)
	write(t, root, "src/ok.go", "package src\n")

	for _, sub := range []string{"rules"} {
		stdout, stderr, code := runBin(t, root, os.Environ(), "__complete", sub, "")
		if code != 0 {
			t.Fatalf("__complete %s: exit %d\nstdout: %s\nstderr: %s", sub, code, stdout, stderr)
		}
		if !containsLine(stdout, "src/snake") {
			t.Errorf("__complete %s misses the rule id\n%s", sub, stdout)
		}
		if !containsLine(stdout, ":4") {
			t.Errorf("__complete %s misses the NoFileComp directive\n%s", sub, stdout)
		}
	}
}

// TestCompletionListsFormatValues proves the check --format flag
// completes its closed value set.
func TestCompletionListsFormatValues(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", completionRules)

	stdout, stderr, code := runBin(t, root, os.Environ(), "__complete", "check", "--format", "")
	if code != 0 {
		t.Fatalf("__complete check --format: exit %d\nstderr: %s", code, stderr)
	}
	for _, want := range []string{"human", "json"} {
		if !containsLine(stdout, want) {
			t.Errorf("--format completion misses %q\n%s", want, stdout)
		}
	}
	if !containsLine(stdout, ":4") {
		t.Errorf("--format completion misses the NoFileComp directive\n%s", stdout)
	}
}

// TestCompletionListsInitLanguages proves the init --languages flag
// completes both an empty value and the trailing item of a comma list.
func TestCompletionListsInitLanguages(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		toComplete string
		wantPrefix string
	}{
		{wantPrefix: ""},
		{toComplete: "go,", wantPrefix: "go,"},
	} {
		stdout, stderr, code := runBin(t, root, os.Environ(), "__complete", "init", "--languages", tc.toComplete)
		if code != 0 {
			t.Fatalf("__complete init --languages %q: exit %d\nstderr: %s", tc.toComplete, code, stderr)
		}
		for _, language := range []string{"go", "ts", "py"} {
			if want := tc.wantPrefix + language; !containsLine(stdout, want) {
				t.Errorf("--languages completion misses %q\n%s", want, stdout)
			}
		}
		if !containsLine(stdout, ":4") {
			t.Errorf("--languages completion misses the NoFileComp directive\n%s", stdout)
		}
	}
}

func TestCompletionListsInitPatterns(t *testing.T) {
	root := t.TempDir()
	stdout, stderr, code := runBin(t, root, os.Environ(), "__complete", "init", "--pattern", "")
	if code != 0 {
		t.Fatalf("__complete init --pattern: exit %d\nstderr: %s", code, stderr)
	}
	// bare, then every available Pattern by its exact reference: the
	// spelling that pins one version in extends.
	for _, name := range []string{"bare", "arclint/vertical@0.1.0"} {
		if !containsLine(stdout, name) {
			t.Errorf("--pattern completion misses %q\n%s", name, stdout)
		}
	}
	if !containsLine(stdout, ":4") {
		t.Errorf("--pattern completion misses the NoFileComp directive\n%s", stdout)
	}
}

// TestCompletionDegradesWithoutRuleset pins the contract: with no
// rules.arclint.yaml anywhere, completion exits 0 with no dynamic candidates
// and no error text; the shell must never see a failure on TAB.
// Static subcommand names (schema, test) still complete: they need no
// ruleset.
func TestCompletionDegradesWithoutRuleset(t *testing.T) {
	root := t.TempDir()
	stdout, stderr, code := runBin(t, root, os.Environ(), "__complete", "rules", "")
	if code != 0 {
		t.Fatalf("__complete rules without ruleset: exit %d\nstderr: %s", code, stderr)
	}
	// No dynamic candidates: only the static subcommands and the
	// directive may appear.
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if line == ":4" || strings.HasPrefix(line, "schema\t") || strings.HasPrefix(line, "test\t") {
			continue
		}
		t.Errorf("dynamic candidate without a ruleset: %q\n%s", line, stdout)
	}
	if !containsLine(stdout, ":4") {
		t.Errorf("missing the NoFileComp directive\n%s", stdout)
	}
	// Cobra prints an informational directive note to stderr, which
	// shells discard; error text must not appear.
	if strings.Contains(stderr, "arclint:") || strings.Contains(stderr, "Error") {
		t.Errorf("completion must degrade silently, stderr: %s", stderr)
	}
}
