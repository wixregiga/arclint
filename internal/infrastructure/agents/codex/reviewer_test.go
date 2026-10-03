package codex

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewerInstallPreservesGuardAndReportsOwnedRelease(t *testing.T) {
	root := t.TempDir()
	unchanged := map[string]string{
		".codex/hooks.json":                          "owner hooks",
		".codex/hooks/arclint-domain-guard/guard.py": "owner guard",
		".arclint/domain-guard.json":                 "owner scope",
		".codex/agents/owner.toml":                   "owner agent",
	}
	for path, content := range unchanged {
		target := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	installer := NewReviewer(root, "1.2.3\n")
	for range 2 {
		if _, err := installer.InstallReviewer(); err != nil {
			t.Fatal(err)
		}
	}
	status, err := installer.ReviewerStatus()
	if err != nil || !status.Installed || !status.Intact || status.InstalledVersion != "1.2.3" {
		t.Fatalf("status: %+v %v", status, err)
	}
	installed, err := os.ReadFile(filepath.Join(root, reviewerPath))
	if err != nil || !bytes.Equal(installed, reviewerDefinition) {
		t.Fatalf("embedded instructions changed: %v", err)
	}
	err = validateReviewerConfiguration(installed)
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	upgraded := NewReviewer(root, "1.2.4")
	if _, err := upgraded.InstallReviewer(); err != nil {
		t.Fatal(err)
	}
	status, err = upgraded.ReviewerStatus()
	if err != nil || !status.Intact || status.InstalledVersion != "1.2.4" {
		t.Fatalf("upgrade: %+v %v", status, err)
	}
	for path, content := range unchanged {
		got, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || string(got) != content {
			t.Fatalf("changed %s: %v", path, err)
		}
	}
}

func TestReviewerPreservesLocalInstructionsAndMetadataBeforeAnyUpgrade(t *testing.T) {
	for _, edited := range []string{reviewerPath, reviewerMetadataPath} {
		t.Run(edited, func(t *testing.T) {
			root := t.TempDir()
			installer := NewReviewer(root, "1.0.0")
			if _, err := installer.InstallReviewer(); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, edited), []byte("owner edit"), 0600); err != nil {
				t.Fatal(err)
			}
			before := map[string][]byte{}
			for _, path := range []string{reviewerPath, reviewerMetadataPath, ".arclint/agent-assets.json"} {
				before[path], _ = os.ReadFile(filepath.Join(root, path))
			}
			if _, err := NewReviewer(root, "1.0.1").InstallReviewer(); err == nil {
				t.Fatal("overwrote owner edit")
			}
			for path, want := range before {
				got, _ := os.ReadFile(filepath.Join(root, path))
				if !bytes.Equal(got, want) {
					t.Fatalf("partial upgrade of %s", path)
				}
			}
			status, err := installer.ReviewerStatus()
			if err != nil || status.Intact || len(status.Problems) == 0 {
				t.Fatalf("missed changed asset: %+v %v", status, err)
			}
		})
	}
}

func TestReviewerStatusMissing(t *testing.T) {
	installer := NewReviewer(t.TempDir(), "1.0.0")
	status, err := installer.ReviewerStatus()
	if err != nil || status.Installed || status.Intact {
		t.Fatalf("missing status: %+v %v", status, err)
	}
}

func TestReviewerStatusDoesNotCertifySymlinkedAssets(t *testing.T) {
	for _, path := range []string{reviewerPath, reviewerMetadataPath, ".arclint/agent-assets.json", ".codex/agents"} {
		t.Run(path, func(t *testing.T) {
			root := t.TempDir()
			installer := NewReviewer(root, "1.0.0")
			if _, err := installer.InstallReviewer(); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(root, path)
			outside := filepath.Join(t.TempDir(), "copied-asset")
			if err := os.Rename(target, outside); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, target); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			status, err := installer.ReviewerStatus()
			if err == nil && status.Intact {
				t.Fatalf("certified symlinked asset: %+v", status)
			}
			if _, err := installer.InstallReviewer(); err == nil {
				t.Fatal("installed through symlink")
			}
		})
	}
}

func TestReviewerRejectsMalformedOrIncompleteCodexConfiguration(t *testing.T) {
	valid := `name = "arclint-domain-reviewer"
description = "Review domain decisions"
developer_instructions = "Explain evidence and uncertainty"
sandbox_mode = "read-only"
`
	if err := validateReviewerConfiguration([]byte(valid)); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ name, content string }{
		{"malformed TOML", "name = ["},
		{"wrong name", strings.Replace(valid, "arclint-domain-reviewer", "other-reviewer", 1)},
		{"missing description", strings.Replace(valid, `description = "Review domain decisions"`, "", 1)},
		{"blank description", strings.Replace(valid, "Review domain decisions", " ", 1)},
		{"missing instructions", strings.Replace(valid, `developer_instructions = "Explain evidence and uncertainty"`, "", 1)},
		{"blank instructions", strings.Replace(valid, "Explain evidence and uncertainty", " ", 1)},
		{"write sandbox", strings.Replace(valid, "read-only", "workspace-write", 1)},
		{"wrong field type", strings.Replace(valid, `description = "Review domain decisions"`, "description = 42", 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateReviewerConfiguration([]byte(test.content)); err == nil {
				t.Fatal("accepted invalid Codex configuration")
			}
		})
	}
}

func TestReviewerMissingReleaseDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	if _, err := NewReviewer(root, " \n").InstallReviewer(); err == nil {
		t.Fatal("accepted empty release")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("invalid release wrote files: %v %v", entries, err)
	}
}
