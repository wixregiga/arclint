package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestAgentsPublicSurfaceRetiresRejectedGuardCommands(t *testing.T) {
	root := t.TempDir()
	rules := filepath.Join(root, "rules.arclint.yaml")
	write(t, root, "rules.arclint.yaml", "runtime: [go]\nzones: {}\n")
	guard := ".codex/hooks/arclint-domain-guard/guard.py"
	preserved := "owner-installed guard remains separate\n"
	write(t, root, guard, preserved)
	stdout, stderr, code := runBin(t, root, nil, "--rules", rules, "agents", "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("agents help: %d %s", code, stderr)
	}
	for _, retained := range []string{"reviewer", "workflow", "md", "skill"} {
		if !strings.Contains(stdout, retained) {
			t.Fatalf("retained command %s missing: %s", retained, stdout)
		}
	}
	if regexp.MustCompile(`(?m)^\s+(hooks|setup|status)\s+`).MatchString(stdout) {
		t.Fatalf("retired command remains in the public surface: %s", stdout)
	}
	for _, retired := range []string{"hooks", "setup", "status"} {
		retiredOutput, retiredError, retiredCode := runBin(t, root, nil, "--rules", rules, "agents", retired)
		// Pure command groups may print help for unknown arguments. That is not an installation route.
		if retiredCode == 0 && !strings.Contains(retiredOutput, "Available Commands:") {
			t.Fatalf("retired command %s executed instead of showing group help: %s", retired, retiredOutput)
		}
		if retiredCode != 0 && retiredCode != 2 {
			t.Fatalf("unexpected retired-command outcome: %d %s", retiredCode, retiredError)
		}
	}
	content, err := os.ReadFile(filepath.Join(root, guard))
	if err != nil || string(content) != preserved {
		t.Fatalf("retired commands changed existing installation: %s %v", content, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".arclint")); !os.IsNotExist(err) {
		t.Fatalf("retired commands created installation state: %v", err)
	}
}
