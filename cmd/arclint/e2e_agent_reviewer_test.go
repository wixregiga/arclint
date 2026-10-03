package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewerCLIInstallsWithoutRulesOrGuard(t *testing.T) {
	root := t.TempDir()
	out, stderr, code := runBin(t, root, nil, "agents", "reviewer", "status")
	if code != 0 || !strings.Contains(out, "Installed: false") {
		t.Fatalf("missing status: %d %s %s", code, out, stderr)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("status wrote files")
	}
	out, stderr, code = runBin(t, root, nil, "agents", "reviewer", "install", "--host", "codex")
	if code != 0 || !strings.Contains(out, "arclint-domain-reviewer") {
		t.Fatalf("install: %d %s %s", code, out, stderr)
	}
	for _, path := range []string{".codex/agents/arclint-domain-reviewer.toml", ".arclint/reviewer.json", ".arclint/agent-assets.json"} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"rules.arclint.yaml", "domain.arclint.yaml", ".codex/hooks.json", ".codex/config.toml", ".arclint/domain-guard.json"} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatalf("unexpected %s", path)
		}
	}
	out, stderr, code = runBin(t, root, nil, "agents", "reviewer", "status")
	if code != 0 || !strings.Contains(out, "Asset integrity: true") || !strings.Contains(out, "Installed ArcLint release: "+strings.TrimSpace(version)) {
		t.Fatalf("installed status: %d %s %s", code, out, stderr)
	}
	path := filepath.Join(root, ".codex/agents/arclint-domain-reviewer.toml")
	before, _ := os.ReadFile(path)
	out, stderr, code = runBin(t, root, nil, "agents", "reviewer", "install", "--host", "codex")
	after, _ := os.ReadFile(path)
	if code != 0 || !bytes.Equal(before, after) {
		t.Fatalf("repeat install: %d %s %s", code, out, stderr)
	}
}

func TestReviewerCLIRejectsUnsupportedHostBeforeWriting(t *testing.T) {
	for _, args := range [][]string{{"agents", "reviewer", "install"}, {"agents", "reviewer", "install", "--host", "omp"}, {"agents", "reviewer", "status", "--host", "omp"}} {
		root := t.TempDir()
		out, stderr, code := runBin(t, root, nil, args...)
		if code != 2 {
			t.Fatalf("expected usage error: %d %s %s", code, out, stderr)
		}
		entries, _ := os.ReadDir(root)
		if len(entries) != 0 {
			t.Fatal("unsupported host wrote files")
		}
	}
}
