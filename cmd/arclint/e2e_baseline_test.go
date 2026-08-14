package main

// The baseline adoption story, proven through the binary: adopt debt,
// stay clean, catch only NEW breaks, surface the debt on request, and
// warn when adopted findings disappear.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBaselineAdoptionFlow(t *testing.T) {
	dir := copyFixture(t, "extension-handler-naming")

	// Debt exists: exit 1 with two findings.
	if _, _, code := runBin(t, dir, os.Environ(), "check", "."); code != 1 {
		t.Fatalf("pre-baseline check: exit %d, want 1", code)
	}

	// Adopt. The command exits 0: adopting debt is not a failure.
	stdout, stderr, code := runBin(t, dir, os.Environ(), "baseline")
	if code != 0 {
		t.Fatalf("baseline: exit %d, stderr %s", code, stderr)
	}
	if !strings.Contains(stdout, "baseline written: 2 findings (2 error, 0 warn, 0 info)") {
		t.Errorf("baseline summary:\n%s", stdout)
	}
	baselinePath := filepath.Join(dir, ".arclint", "baseline.json")
	first, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatal(err)
	}

	// Determinism: adopting again produces byte-identical content.
	if _, _, code := runBin(t, dir, os.Environ(), "baseline"); code != 0 {
		t.Fatal("second baseline run failed")
	}
	second, _ := os.ReadFile(baselinePath)
	if string(first) != string(second) {
		t.Errorf("baseline regeneration is not deterministic:\n%s\nvs\n%s", first, second)
	}

	// Clean now, with the debt counted visibly.
	stdout, _, code = runBin(t, dir, os.Environ(), "check", ".")
	if code != 0 {
		t.Fatalf("post-baseline check: exit %d\n%s", code, stdout)
	}
	if !strings.Contains(stdout, "clean: 0 violations · 2 baselined") {
		t.Errorf("baselined count not visible:\n%s", stdout)
	}

	// The debt is listable, and marked in JSON.
	stdout, _, _ = runBin(t, dir, os.Environ(), "check", ".", "--show-baselined")
	if !strings.Contains(stdout, "BASELINED (adopted debt)") || !strings.Contains(stdout, "handler files must end in") {
		t.Errorf("--show-baselined output:\n%s", stdout)
	}
	stdout, _, _ = runBin(t, dir, os.Environ(), "check", ".", "--format", "json", "--show-baselined")
	var listed []map[string]any
	if err := json.Unmarshal([]byte(stdout), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[0]["baselined"] != true {
		t.Errorf("json baselined marks: %v", listed)
	}

	// Without the flag the JSON array is empty: only NEW findings are
	// problems.
	stdout, _, _ = runBin(t, dir, os.Environ(), "check", ".", "--format", "json")
	if err := json.Unmarshal([]byte(stdout), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Errorf("baselined findings leaked into plain json: %v", listed)
	}

	// --no-baseline shows everything and fails again.
	if _, _, code := runBin(t, dir, os.Environ(), "check", ".", "--no-baseline"); code != 1 {
		t.Errorf("--no-baseline: exit %d, want 1", code)
	}

	// A NEW violation is reported alone, exit 1, while the debt stays
	// baselined.
	newFile := filepath.Join(dir, "internal", "api", "handlers", "also_broken.go")
	if err := os.WriteFile(newFile, []byte("package handlers\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, code = runBin(t, dir, os.Environ(), "check", ".", "--format", "json")
	if code != 1 {
		t.Fatalf("new violation: exit %d\n%s", code, stdout)
	}
	if err := json.Unmarshal([]byte(stdout), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0]["path"] != "internal/api/handlers/also_broken.go" {
		t.Errorf("new-finding detection: %v", listed)
	}
	os.Remove(newFile)

	// Fix one adopted finding (drop the wiring-audit instance): check
	// warns that the baseline is stale, and stays clean.
	rules, err := os.ReadFile(filepath.Join(dir, "rules.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	patched := strings.Replace(string(rules), "  - type: wiring-audit\n", "", 1)
	if patched == string(rules) {
		t.Fatal("fixture rules.yaml shape changed; update the patch")
	}
	if err := os.WriteFile(filepath.Join(dir, "rules.yaml"), []byte(patched), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = runBin(t, dir, os.Environ(), "check", ".")
	if code != 0 {
		t.Fatalf("after fix: exit %d\n%s", code, stdout)
	}
	if !strings.Contains(stderr, "baseline: 1 adopted findings no longer occur") {
		t.Errorf("stale warning missing:\n%s", stderr)
	}
}
