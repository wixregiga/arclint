package codex

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallMergesHooksAndPreservesProjectSettings(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".codex"), 0750); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"domain.yaml":        "contexts: []\n",
		".codex/config.toml": "sandbox_mode = \"workspace-write\"\n",
		".codex/hooks.json":  `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"existing-check"}]}]}}`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	installer := NewInstaller(root)
	if _, err := installer.Install("codex", []string{"domain.yaml"}); err != nil {
		t.Fatal(err)
	}
	if _, err := installer.Install("codex", []string{"domain.yaml"}); err != nil {
		t.Fatal(err)
	}
	hooks, err := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(hooks), "existing-check") || !strings.Contains(string(hooks), "PreToolUse") {
		t.Fatal("lost hooks")
	}
	config, err := os.ReadFile(filepath.Join(root, ".codex/config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(config) != "sandbox_mode = \"workspace-write\"\n" {
		t.Fatal("changed security settings")
	}
	script := filepath.Join(root, ".codex/hooks/arclint-domain-guard/guard.py")
	if err := os.WriteFile(script, []byte("user edit"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := installer.Install("codex", []string{"domain.yaml"}); err == nil {
		t.Fatal("overwrote user edit")
	}
}

func TestGuardContract(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python 3 required for Codex guard tests")
	}
	command := exec.Command(python, "-B", "assets/guard_test.py")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("guard contract: %v\n%s", err, out)
	}
}
