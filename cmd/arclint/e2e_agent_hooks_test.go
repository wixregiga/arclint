package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentHooksInstallWithoutRuleset(t *testing.T) {
	root := t.TempDir()
	write(t, root, "language.yaml", "contexts: []\n")
	stdout, stderr, code := runBin(t, root, nil, "agents", "hooks", "--domain", "language.yaml")
	if code != 0 {
		t.Fatalf("install: %d\n%s\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "/reload") {
		t.Fatal("missing activation instruction")
	}
	for _, path := range []string{".arclint/domain-guard.json", ".omp/extensions/arclint-domain-guard/index.js", ".omp/extensions/arclint-domain-guard/guard.mjs"} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "rules.arclint.yaml")); !os.IsNotExist(err) {
		t.Fatal("setup created a ruleset")
	}
	stdout, stderr, code = runBin(t, root, nil, "agents", "hooks", "--domain", "language.yaml")
	if code != 0 {
		t.Fatalf("idempotent install: %d %s %s", code, stdout, stderr)
	}
	_, _, code = runBin(t, root, nil, "agents", "hooks", "--host", "unknown", "--domain", "language.yaml")
	if code != 2 {
		t.Fatalf("unsupported host exit: %d", code)
	}
}

func TestCodexAgentHooksInstallWithoutRuleset(t *testing.T) {
	root := t.TempDir()
	write(t, root, "language.yaml", "contexts: []\n")
	out, stderr, code := runBin(t, root, nil, "agents", "hooks", "--host", "codex", "--domain", "language.yaml")
	if code != 0 {
		t.Fatalf("install: %d %s %s", code, out, stderr)
	}
	if !strings.Contains(out, "/hooks") || !strings.Contains(out, "Until trusted") {
		t.Fatal("missing trust requirement")
	}
	hooks, err := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(hooks), "PreToolUse") || !strings.Contains(string(hooks), "Stop") {
		t.Fatal("missing native hooks")
	}
}
