package omp

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallPreservesFilesAndConfiguresDomain(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "language.yaml"), []byte("contexts: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	installer := NewInstaller(root)
	paths, err := installer.Install("omp", []string{"language.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 3 {
		t.Fatalf("installed %d paths", len(paths))
	}
	if _, err := installer.Install("omp", []string{"language.yaml"}); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(root, ".omp/extensions/arclint-domain-guard/index.js")
	if err := os.WriteFile(entry, []byte("user change"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := installer.Install("omp", []string{"language.yaml"}); err == nil {
		t.Fatal("overwrote user change")
	}
	got, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "user change" {
		t.Fatal("lost user change")
	}
}

func TestInstallRejectsEscapingDomainAndOutputSymlinks(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "domain.yaml"), []byte("contexts: []"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../domain.yaml", "/tmp/domain.yaml", "missing.yaml"} {
		if _, err := NewInstaller(root).Install("omp", []string{name}); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, ".omp")); err != nil {
		t.Skip(err)
	}
	if _, err := NewInstaller(root).Install("omp", []string{"domain.yaml"}); err == nil {
		t.Fatal("accepted symlinked output")
	}
	if _, err := os.Stat(filepath.Join(root, ".arclint/domain-guard.json")); !os.IsNotExist(err) {
		t.Fatal("wrote partial config before preflight")
	}
}

func TestHostContract(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is needed for OMP extension contract tests")
	}
	cmd := exec.Command(node, "--test", "--test-reporter=tap", "assets/guard.test.mjs")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("guard contract: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "# fail 0") {
		t.Fatalf("missing test result: %s", out)
	}
}
