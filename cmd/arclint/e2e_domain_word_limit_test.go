package main

// End-to-end coverage of the word limits on recorded text against the
// compiled binary: define refuses a text over its limit and writes
// nothing, check reports one already recorded at its line in the
// domain file, and a ruleset adopts the built-in Rule by its id.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wordText is a text of n words.
func wordText(n int) string { return strings.TrimSpace(strings.Repeat("word ", n)) }

func TestDefineRefusesTextOverItsWordLimit(t *testing.T) {
	root := domainFixture(t)
	mustRunDomain(t, root, "domain", "define", "bounded_context", "ordering", "--definition", "Taking and fulfilling customer orders.")
	before, err := os.ReadFile(filepath.Join(root, "domain.arclint.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"value object", []string{"domain", "define", "value_object", "Money", "--definition", wordText(41)},
			`value object "Money" in context "ordering": the definition has 41 words, and a definition holds at most 40. Keep what the word means`},
		{"context", []string{"domain", "define", "bounded_context", "ordering", "--definition", wordText(61)},
			`context "ordering": the definition has 61 words, and a definition holds at most 60. Keep what the word means`},
		{"description", []string{"domain", "define", "description", "--description", wordText(101)},
			`the description has 101 words, and a description holds at most 100. Keep what the project is about`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runBin(t, root, os.Environ(), tc.args...)
			if code != 1 || stdout != "" {
				t.Fatalf("exit %d, want 1 with nothing on stdout\nstdout: %s\nstderr: %s", code, stdout, stderr)
			}
			for _, want := range []string{tc.want, "arclint domain define invariant", "arclint domain define assertion", "put promises that are not domain rules in tests"} {
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr lacks %q:\n%s", want, stderr)
				}
			}
			after, err := os.ReadFile(filepath.Join(root, "domain.arclint.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Errorf("a refused define changed the domain file:\n%s", after)
			}
		})
	}
	mustRunDomain(t, root, "domain", "define", "value_object", "Money", "--definition", wordText(40))
	mustRunDomain(t, root, "domain", "define", "invariant", "never-negative", "--owner", "Money", "--statement", wordText(40))
}

func TestCheckReportsRecordedTextOverItsWordLimit(t *testing.T) {
	root := domainFixture(t)
	write(t, root, "domain.arclint.yaml", `version: 1
project: shop
contexts:
  ordering:
    definition: Taking and fulfilling customer orders.
    value_objects:
      Money:
        definition: `+wordText(41)+`
      Currency:
        definition: `+wordText(40)+`
`)
	stdout, stderr, code := runBin(t, root, os.Environ(), "check", "--only", "domain_recording/text-within-word-limit", "--format", "json")
	if code != 1 {
		t.Fatalf("check: exit %d, want 1\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	var diagnostics []diagnosticDoc
	if err := json.Unmarshal([]byte(stdout), &diagnostics); err != nil {
		t.Fatalf("check json: %v\n%s", err, stdout)
	}
	var violations []diagnosticDoc
	for _, d := range diagnostics {
		if d.Kind == "violation" {
			violations = append(violations, d)
		}
	}
	want := diagnosticDoc{
		Kind: "violation", RuleID: "domain_recording/text-within-word-limit", Path: "domain.arclint.yaml", Line: 7, Severity: "error",
		Message: `value object "Money" in context "ordering": the definition has 41 words, and a definition holds at most 40`,
	}
	if len(violations) != 1 || violations[0].RuleID != want.RuleID || violations[0].Path != want.Path ||
		violations[0].Line != want.Line || violations[0].Severity != want.Severity || violations[0].Message != want.Message {
		t.Fatalf("violations = %+v, want only %+v", violations, want)
	}

	write(t, root, "rules.arclint.yaml", minimalDomainRules+`rules:
  domain_recording/text-within-word-limit:
    severity: warning
`)
	stdout, stderr, code = runBin(t, root, os.Environ(), "check", "--only", "domain_recording/text-within-word-limit")
	if code != 0 || !strings.Contains(stdout, "domain.arclint.yaml:7: [warning] domain_recording/text-within-word-limit") {
		t.Fatalf("an override at warning severity reports without failing: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	write(t, root, "rules.arclint.yaml", minimalDomainRules+`rules:
  domain_recording/text-within-word-limit:
    content:
      forbid: x
`)
	_, stderr, code = runBin(t, root, os.Environ(), "check")
	if code == 0 || !strings.Contains(stderr, "built in") {
		t.Fatalf("a ruleset may not spell a Rule under the built-in id: exit %d\nstderr: %s", code, stderr)
	}
}
