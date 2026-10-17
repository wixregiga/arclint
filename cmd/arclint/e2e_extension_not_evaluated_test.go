package main

// An Extension Rule whose check() never returns must not empty the
// report: check prints every other result plus one not-evaluated entry
// that says why, and exits 3, the code for an incomplete check.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// spinExtension reads one selected file, then loops forever.
const spinExtension = `
import { defineRule } from "arclint";
export default defineRule({
  type: "spin",
  check(ctx) {
    ctx.read("src/ok.go");
    for (;;) {}
  },
});
`

// spinRepository holds an extension that loops forever and a naming
// Rule with one finding. limit is the spin Rule's time_limit line, ""
// for the default.
func spinRepository(t *testing.T, limit string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, ".arclint/extensions/spin.ts", spinExtension)
	write(t, root, "rules.arclint.yaml", `runtime: [go]
zones:
  src: src/**
rules:
  src/spin:
    on: src
    files: "src/**/*.go"
    uses: spin
`+limit+`
  src/snake:
    on: src
    naming: snake_case
`)
	write(t, root, "src/BadName.go", "package src\n")
	write(t, root, "src/ok.go", "package src\n")
	return root
}

type notEvaluatedDoc struct {
	Kind         string `json:"kind"`
	RuleID       string `json:"ruleId"`
	Path         string `json:"path"`
	Message      string `json:"message"`
	NotEvaluated *struct {
		Extension string `json:"extension"`
		Source    string `json:"source"`
		Phase     string `json:"phase"`
		TimedOut  bool   `json:"timedOut"`
		LimitMs   int64  `json:"limitMs"`
		ElapsedMs int64  `json:"elapsedMs"`
		File      string `json:"file"`
		Error     string `json:"error"`
	} `json:"notEvaluated"`
}

func checkJSON(t *testing.T, root string) ([]notEvaluatedDoc, string, int) {
	t.Helper()
	stdout, stderr, code := runBin(t, root, os.Environ(), "check", "--format", "json")
	var docs []notEvaluatedDoc
	if err := json.Unmarshal([]byte(stdout), &docs); err != nil {
		t.Fatalf("complete JSON report required (exit %d): %v\nstdout: %q\nstderr: %s", code, err, stdout, stderr)
	}
	return docs, stderr, code
}

func kindOf(docs []notEvaluatedDoc, kind string) []notEvaluatedDoc {
	var out []notEvaluatedDoc
	for _, d := range docs {
		if d.Kind == kind {
			out = append(out, d)
		}
	}
	return out
}

func TestExtensionThatNeverReturnsIsNotEvaluated(t *testing.T) {
	t.Run("json at the default limit", func(t *testing.T) {
		t.Parallel()
		docs, stderr, code := checkJSON(t, spinRepository(t, ""))
		if code != 3 {
			t.Fatalf("exit %d, want 3 (not evaluated)\nstderr: %s", code, stderr)
		}
		if stderr != "" {
			t.Errorf("stderr = %q, want the report on stdout alone", stderr)
		}
		stopped := kindOf(docs, "not_evaluated")
		if len(stopped) != 1 || stopped[0].RuleID != "src/spin" || stopped[0].NotEvaluated == nil {
			t.Fatalf("not_evaluated = %+v, want one entry for src/spin with its circumstances", stopped)
		}
		got := *stopped[0].NotEvaluated
		if got.Extension != "spin" || got.Source != ".arclint/extensions/spin.ts" || got.Phase != "check" ||
			!got.TimedOut || got.LimitMs != 5000 || got.ElapsedMs < 5000 || got.File != "src/ok.go" || got.Error != "" {
			t.Errorf("circumstances = %+v, want a check timeout at 5000ms while reading src/ok.go", got)
		}
		message := regexp.MustCompile(`^rule src/spin: extension "spin" \(\.arclint/extensions/spin\.ts\) reached its 5s time limit in check and was stopped after 5\.\d+s, last reading src/ok\.go$`)
		if !message.MatchString(stopped[0].Message) {
			t.Errorf("message = %q", stopped[0].Message)
		}
		findings := kindOf(docs, "violation")
		if len(findings) != 1 || findings[0].RuleID != "src/snake" || findings[0].Path != "src/BadName.go" {
			t.Errorf("violations = %+v, want the src/snake finding still reported", findings)
		}
	})
	t.Run("human", func(t *testing.T) {
		t.Parallel()
		stdout, stderr, code := runBin(t, spinRepository(t, "    time_limit: 300ms"), os.Environ(), "check", "--no-color")
		if code != 3 {
			t.Fatalf("exit %d, want 3\nstdout: %s\nstderr: %s", code, stdout, stderr)
		}
		line := regexp.MustCompile(`(?m)^not evaluated: rule src/spin: extension "spin" \(\.arclint/extensions/spin\.ts\) reached its 300ms time limit in check and was stopped after \d+ms, last reading src/ok\.go$`)
		if strings.Count(stdout, "src/spin") != 1 || !line.MatchString(stdout) {
			t.Errorf("stdout does not say it in one line:\n%s", stdout)
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
		root := spinRepository(t, "    time_limit: 300ms")
		stdout, stderr, code := runBin(t, root, os.Environ(), "baseline", "capture")
		if code != 2 || !strings.Contains(stderr, "src/spin") {
			t.Fatalf("exit %d, want 2 refusing over src/spin\nstdout: %s\nstderr: %s", code, stdout, stderr)
		}
		if _, err := os.Stat(filepath.Join(root, ".arclint", "baseline.v2.json")); !os.IsNotExist(err) {
			t.Errorf("a baseline was written over a Rule not evaluated: %v", err)
		}
	})
	t.Run("rule detail shows the limit", func(t *testing.T) {
		t.Parallel()
		stdout, stderr, code := runBin(t, spinRepository(t, "    time_limit: 1m30s"), os.Environ(), "rules", "src/spin", "--no-color")
		if code != 0 || !strings.Contains(stdout, "time limit:  1m30s\n") {
			t.Errorf("exit %d, want the rule's time limit\nstdout: %s\nstderr: %s", code, stdout, stderr)
		}
	})
}

// An extension that throws while registering stops every extension
// Rule, even with a generated AGENTS.md block whose comparison reads the
// same extension inventory.
func TestExtensionRegistrationErrorIsNotEvaluated(t *testing.T) {
	root := spinRepository(t, "")
	write(t, root, ".arclint/extensions/spin.ts", `
import { defineRule } from "arclint";
export default defineRule({ type: "spin", check() {} });
`)
	if stdout, stderr, code := runBin(t, root, os.Environ(), "agents", "md", "--write"); code != 0 {
		t.Fatalf("agents md --write: exit %d\n%s\n%s", code, stdout, stderr)
	}
	write(t, root, ".arclint/extensions/spin.ts", `
import { defineRule } from "arclint";
throw new Error("broken at load");
export default defineRule({ type: "spin", check() {} });
`)
	docs, stderr, code := checkJSON(t, root)
	if code != 3 {
		t.Fatalf("exit %d, want 3\nstderr: %s", code, stderr)
	}
	stopped := kindOf(docs, "not_evaluated")
	if len(stopped) != 1 || stopped[0].NotEvaluated == nil {
		t.Fatalf("not_evaluated = %+v, want one entry", stopped)
	}
	got := *stopped[0].NotEvaluated
	if got.Phase != "registration" || got.TimedOut || got.File != "" || !strings.Contains(got.Error, "broken at load") || got.LimitMs != 5000 {
		t.Errorf("circumstances = %+v, want the registration error under the 5000ms limit", got)
	}
	if len(kindOf(docs, "violation")) != 1 {
		t.Errorf("the other Rule's finding is missing: %+v", docs)
	}
}

// A malformed definition is the author's configuration error, not code
// that failed while running: check exits 2 and names it.
func TestMalformedExtensionDefinitionIsAConfigurationError(t *testing.T) {
	root := spinRepository(t, "")
	write(t, root, ".arclint/extensions/spin.ts", `
import { defineRule } from "arclint";
export default defineRule({ type: "spin", check: "not a function" } as any);
`)
	stdout, stderr, code := runBin(t, root, os.Environ(), "check", "--format", "json")
	if code != 2 || !strings.Contains(stderr, "defineRule: check must be a function") {
		t.Errorf("exit %d, want 2 naming the malformed definition\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
}
