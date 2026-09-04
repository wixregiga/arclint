package main

// An Extension that reports a logical path outside its Rule Applicability
// must not abort check: the Assessment stays complete, the gate fails via
// an operational Diagnostic (exit 1), and no Violation is invented.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestExtensionOutsideApplicabilityContained(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".arclint/extensions/report_outside.ts", `
import { defineRule } from "arclint";
export default defineRule({
  type: "report-outside",
  check(ctx) {
    // Hard-coded path outside the Rule's selected subjects: the sandbox
    // hides out-of-scope files, but report() still accepts any string.
    ctx.report({
      path: "elsewhere/secret.go",
      line: 1,
      message: "must not become a violation",
    });
  },
});
`)
	write(t, root, "rules.yaml", `runtime: [go]
modules:
  src: src/**
rules:
  src/report-outside:
    on: src
    files: "src/**/*.go"
    uses: report-outside
`)
	write(t, root, "src/ok.go", "package src\n")

	stdout, stderr, code := runBin(t, root, os.Environ(), "check", "--format", "json")
	// Pre-fix: Run returned an error → ConfigError → exit 2, no JSON Assessment.
	if code != 1 {
		t.Fatalf("exit %d, want 1 (gate via operational diagnostic, not config abort)\nstdout: %s\nstderr: %s",
			code, stdout, stderr)
	}
	var diagnostics []diagnosticDoc
	if err := json.Unmarshal([]byte(stdout), &diagnostics); err != nil {
		t.Fatalf("complete JSON Assessment required: %v\n%s", err, stdout)
	}

	var active []diagnosticDoc
	var operational []diagnosticDoc
	for _, d := range diagnostics {
		if d.Kind == "violation" && d.Status == "active" {
			active = append(active, d)
		}
		if d.Kind == "operational" {
			operational = append(operational, d)
		}
	}
	if len(active) != 0 {
		t.Errorf("active violations = %+v, want none (breach must not invent Violations)", active)
	}
	if len(operational) != 1 {
		t.Fatalf("operational diagnostics = %+v, want exactly one error-severity Applicability breach", operational)
	}
	op := operational[0]
	if op.Severity != "error" || op.RuleID != "src/report-outside" || op.Path != "elsewhere/secret.go" || op.Line != 1 {
		t.Errorf("operational = %+v, want error on src/report-outside at elsewhere/secret.go:1", op)
	}
	wantMsg := `rule src/report-outside: extension "report-outside" reported "elsewhere/secret.go", which is outside the rule's applicability`
	if op.Message != wantMsg {
		t.Errorf("message = %q\nwant    %q", op.Message, wantMsg)
	}
	// Coverage still explains the failed selected subject; silence must not
	// read as a clean Evaluation for the in-scope file.
	var sawFailedCoverage bool
	for _, d := range diagnostics {
		if d.Kind == "coverage" && d.RuleID == "src/report-outside" &&
			strings.Contains(d.Message, "failed evaluation") {
			sawFailedCoverage = true
		}
	}
	if !sawFailedCoverage {
		t.Errorf("expected failed-evaluation coverage for the selected subject; diagnostics = %+v", diagnostics)
	}
}
