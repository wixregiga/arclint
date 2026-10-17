package main

// An Extension Rule whose check() never returns must not empty the
// report: check prints every other result plus one not-evaluated entry
// and exits 3, the code for an incomplete check.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const notEvaluatedMessage = `rule src/spin: extension "spin": .arclint/extensions/spin.ts exceeded its 5s time limit`

// spinRepository holds an extension that loops forever and a naming
// Rule with one finding.
func spinRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, ".arclint/extensions/spin.ts", `
import { defineRule } from "arclint";
export default defineRule({
  type: "spin",
  check() {
    for (;;) {}
  },
});
`)
	write(t, root, "rules.arclint.yaml", `runtime: [go]
zones:
  src: src/**
rules:
  src/spin:
    on: src
    files: "src/**/*.go"
    uses: spin
  src/snake:
    on: src
    naming: snake_case
`)
	write(t, root, "src/BadName.go", "package src\n")
	return root
}

func TestExtensionThatNeverReturnsIsNotEvaluated(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		t.Parallel()
		stdout, stderr, code := runBin(t, spinRepository(t), os.Environ(), "check", "--format", "json")
		if code != 3 {
			t.Fatalf("exit %d, want 3 (not evaluated)\nstdout: %s\nstderr: %s", code, stdout, stderr)
		}
		if stderr != "" {
			t.Errorf("stderr = %q, want the report on stdout alone", stderr)
		}
		var docs []diagnosticDoc
		if err := json.Unmarshal([]byte(stdout), &docs); err != nil {
			t.Fatalf("complete JSON report required: %v\n%q", err, stdout)
		}
		var stopped, findings []diagnosticDoc
		for _, d := range docs {
			switch d.Kind {
			case "not_evaluated":
				stopped = append(stopped, d)
			case "violation":
				findings = append(findings, d)
			}
		}
		if len(stopped) != 1 || stopped[0].RuleID != "src/spin" || stopped[0].Message != notEvaluatedMessage {
			t.Errorf("not_evaluated = %+v, want one entry for src/spin: %s", stopped, notEvaluatedMessage)
		}
		if len(findings) != 1 || findings[0].RuleID != "src/snake" || findings[0].Path != "src/BadName.go" {
			t.Errorf("violations = %+v, want the src/snake finding still reported", findings)
		}
	})
	t.Run("human", func(t *testing.T) {
		t.Parallel()
		stdout, stderr, code := runBin(t, spinRepository(t), os.Environ(), "check", "--no-color")
		if code != 3 {
			t.Fatalf("exit %d, want 3\nstdout: %s\nstderr: %s", code, stdout, stderr)
		}
		want := "not evaluated: " + notEvaluatedMessage + "\n"
		if strings.Count(stdout, "src/spin") != 1 || !strings.Contains(stdout, want) {
			t.Errorf("stdout does not say it in one line %q:\n%s", want, stdout)
		}
		if !strings.Contains(stdout, `src/BadName.go: [error] src/snake`) {
			t.Errorf("the other Rule's finding is missing:\n%s", stdout)
		}
		if !strings.Contains(stdout, "1 active finding(s) · 0 suppressed · 0 baselined · 1 rule(s) applied") {
			t.Errorf("summary counts the stopped Rule as applied:\n%s", stdout)
		}
	})
	t.Run("baseline capture", func(t *testing.T) {
		t.Parallel()
		root := spinRepository(t)
		stdout, stderr, code := runBin(t, root, os.Environ(), "baseline", "capture")
		if code != 2 || !strings.Contains(stderr, "src/spin") {
			t.Fatalf("exit %d, want 2 refusing over src/spin\nstdout: %s\nstderr: %s", code, stdout, stderr)
		}
		if _, err := os.Stat(root + "/.arclint/baseline.v2.json"); !os.IsNotExist(err) {
			t.Errorf("a baseline was written over a Rule not evaluated: %v", err)
		}
	})
}
