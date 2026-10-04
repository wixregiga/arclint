package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowInstallPreservesOtherHooksAndLocalEdits(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".codex"), 0o750); err != nil {
		t.Fatal(err)
	}
	prior := []byte(`{"custom":"keep","hooks":{"Stop":[{"hooks":[{"type":"command","command":"existing-guard"}]}]}}`)
	path := filepath.Join(root, ".codex/hooks.json")
	if err := os.WriteFile(path, prior, 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "arclint")
	if err := os.WriteFile(binary, []byte("test executable bytes"), 0o750); err != nil {
		t.Fatal(err)
	}
	installer := NewInstaller(root, binary, "test-version")
	if _, err := installer.Install(); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(first, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["custom"] != "keep" || !bytes.Contains(first, []byte("existing-guard")) {
		t.Fatal("replaced existing configuration")
	}
	if _, err := installer.Install(); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	if !bytes.Equal(first, second) {
		t.Fatal("repeat installation changed registration")
	}
	status, err := installer.Status()
	if err != nil || !status.Installed || !status.Intact {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	instructions := filepath.Join(root, workflowDirectory, "instructions.md")
	if err := os.WriteFile(instructions, []byte("user-edited instructions"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := installer.Install(); err == nil {
		t.Fatal("overwrote user-edited instructions")
	}
	status, err = installer.Status()
	if err != nil || status.Intact {
		t.Fatalf("edited asset status=%+v err=%v", status, err)
	}
	data, _ := os.ReadFile(instructions)
	if string(data) != "user-edited instructions" {
		t.Fatal("lost local edit")
	}
}

func TestWorkflowInstallRejectsInvalidRegistrationBeforeWrites(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".codex"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".codex/hooks.json"), []byte("bad json"), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "arclint")
	if err := os.WriteFile(binary, []byte("executable"), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := NewInstaller(root, binary, "test").Install(); err == nil {
		t.Fatal("accepted invalid registration")
	}
	if _, err := os.Stat(filepath.Join(root, workflowDirectory)); !os.IsNotExist(err) {
		t.Fatal("wrote before validation")
	}
}

func TestWorkflowStatusDoesNotTreatEmptyReceiptAsIntact(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".arclint"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, workflowReceipt), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := NewInstaller(root, "", "").Status()
	if err != nil || s.Intact {
		t.Fatalf("empty receipt status=%+v err=%v", s, err)
	}
}

func installedWorkflowFixture(t *testing.T) (*Installer, string) {
	t.Helper()
	root := t.TempDir()
	binary := filepath.Join(t.TempDir(), "arclint")
	if err := os.WriteFile(binary, []byte("test executable bytes"), 0o700); err != nil {
		t.Fatal(err)
	}
	installer := NewInstaller(root, binary, "test-version")
	if _, err := installer.Install(); err != nil {
		t.Fatal(err)
	}
	status, err := installer.Status()
	if err != nil || !status.Installed || !status.Intact {
		t.Fatalf("initial status=%+v err=%v", status, err)
	}
	return installer, root
}

func TestWorkflowStatusRejectsCommandWithoutOwnerExecutePermission(t *testing.T) {
	installer, root := installedWorkflowFixture(t)
	path := filepath.Join(root, workflowDirectory, "arclint")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	status, err := installer.Status()
	if err != nil || !status.Installed || status.Intact {
		t.Fatalf("nonexecutable command status=%+v err=%v", status, err)
	}
	if !strings.Contains(strings.Join(status.Problems, "\n"), "not executable") {
		t.Fatalf("missing execution permission explanation: %+v", status.Problems)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("status changed executable content: err=%v", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("status changed executable permissions: info=%v err=%v", info, err)
	}
}

func TestWorkflowStatusRejectsSymlinkedReceiptsAndAssets(t *testing.T) {
	for _, name := range []string{
		workflowReceipt,
		".arclint/agent-assets.json",
		".codex/hooks.json",
		workflowDirectory + "/arclint",
		workflowDirectory + "/instructions.md",
		workflowDirectory + "/start-codex.sh",
	} {
		t.Run(name, func(t *testing.T) {
			installer, root := installedWorkflowFixture(t)
			path := filepath.Join(root, name)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// Keep the exact installed bytes and permissions: only containment changes.
			target := filepath.Join(t.TempDir(), "original")
			if err := os.Rename(path, target); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
			status, err := installer.Status()
			if err != nil || status.Intact {
				t.Fatalf("symlinked path status=%+v err=%v", status, err)
			}
			if !strings.Contains(strings.Join(status.Problems, "\n"), "symlink") {
				t.Fatalf("missing containment explanation: %+v", status.Problems)
			}
			after, err := os.ReadFile(target)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("status changed symlink target: err=%v", err)
			}
			link, err := os.Readlink(path)
			if err != nil || link != target {
				t.Fatalf("status changed symlink: target=%q err=%v", link, err)
			}
		})
	}
}

func TestWorkflowInstallSelectedRulesAndRejectsNullHooks(t *testing.T) {
	for _, nullHooks := range []bool{false, true} {
		t.Run(fmt.Sprint(nullHooks), func(t *testing.T) {
			t.Setenv("WSL_DISTRO_NAME", "test-distro")
			root := t.TempDir()
			binary := filepath.Join(t.TempDir(), "arclint")
			if err := os.WriteFile(binary, []byte("fixture executable"), 0o600); err != nil {
				t.Fatal(err)
			}
			rulesPath := filepath.Join(root, "chosen policy.yaml")
			if nullHooks {
				if err := os.MkdirAll(filepath.Join(root, ".codex"), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".codex/hooks.json"), []byte("null\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			_, err := NewInstaller(root, binary, "test").WithRulesPath(rulesPath).Install()
			if nullHooks {
				if err == nil {
					t.Fatal("accepted null hooks document")
				}
				for _, name := range []string{workflowDirectory, workflowReceipt, ".arclint/agent-assets.json"} {
					if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
						t.Fatalf("wrote %s after rejection: %v", name, err)
					}
				}
				data, _ := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
				if string(data) != "null\n" {
					t.Fatal("changed rejected hooks file")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(root, workflowReceipt))
			if err != nil {
				t.Fatal(err)
			}
			var receipt installation
			if err := json.Unmarshal(data, &receipt); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(receipt.Command, "--rules "+shellQuote(rulesPath)) {
				t.Fatalf("wrong POSIX policy: %s", receipt.Command)
			}
			windows, _ := receipt.Handler["commandWindows"].(string)
			if !strings.Contains(windows, "--rules \""+rulesPath+"\"") {
				t.Fatalf("wrong Windows policy: %s", windows)
			}
		})
	}
}
