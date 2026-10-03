package codex

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/agent"
)

func reviewerHost(t *testing.T) agent.Host {
	t.Helper()
	host, err := agent.NewHost("codex")
	if err != nil {
		t.Fatal(err)
	}
	return host
}

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
	host := reviewerHost(t)
	installer := NewReviewer(root, "1.2.3\n")
	for range 2 {
		if _, err := installer.InstallReviewer(host); err != nil {
			t.Fatal(err)
		}
	}
	status, err := installer.ReviewerStatus(host)
	if err != nil || !status.Installed || !status.Intact || status.InstalledVersion != "1.2.3" {
		t.Fatalf("status: %+v %v", status, err)
	}
	installed, err := os.ReadFile(filepath.Join(root, reviewerPath))
	if err != nil || !bytes.Equal(installed, reviewerDefinition) {
		t.Fatalf("embedded instructions changed: %v", err)
	}
	definition, err := shippedReviewer()
	if err != nil || definition.Name() != "arclint-domain-reviewer" {
		t.Fatalf("definition: %v", err)
	}
	upgraded := NewReviewer(root, "1.2.4")
	if _, err := upgraded.InstallReviewer(host); err != nil {
		t.Fatal(err)
	}
	status, err = upgraded.ReviewerStatus(host)
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
			host := reviewerHost(t)
			installer := NewReviewer(root, "1.0.0")
			if _, err := installer.InstallReviewer(host); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, edited), []byte("owner edit"), 0600); err != nil {
				t.Fatal(err)
			}
			before := map[string][]byte{}
			for _, path := range []string{reviewerPath, reviewerMetadataPath, ".arclint/agent-assets.json"} {
				before[path], _ = os.ReadFile(filepath.Join(root, path))
			}
			if _, err := NewReviewer(root, "1.0.1").InstallReviewer(host); err == nil {
				t.Fatal("overwrote owner edit")
			}
			for path, want := range before {
				got, _ := os.ReadFile(filepath.Join(root, path))
				if !bytes.Equal(got, want) {
					t.Fatalf("partial upgrade of %s", path)
				}
			}
			status, err := installer.ReviewerStatus(host)
			if err != nil || status.Intact || len(status.Problems) == 0 {
				t.Fatalf("missed changed asset: %+v %v", status, err)
			}
		})
	}
}

func TestReviewerStatusMissingAndZeroHost(t *testing.T) {
	installer := NewReviewer(t.TempDir(), "1.0.0")
	status, err := installer.ReviewerStatus(reviewerHost(t))
	if err != nil || status.Installed || status.Intact {
		t.Fatalf("missing status: %+v %v", status, err)
	}
	if _, err := installer.InstallReviewer(agent.Host{}); err == nil {
		t.Fatal("accepted zero host")
	}
	if _, err := installer.ReviewerStatus(agent.Host{}); err == nil {
		t.Fatal("accepted zero host")
	}
}

func TestReviewerStatusDoesNotCertifySymlinkedAssets(t *testing.T) {
	for _, path := range []string{reviewerPath, reviewerMetadataPath, ".arclint/agent-assets.json", ".codex/agents"} {
		t.Run(path, func(t *testing.T) {
			root := t.TempDir()
			host := reviewerHost(t)
			installer := NewReviewer(root, "1.0.0")
			if _, err := installer.InstallReviewer(host); err != nil {
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
			status, err := installer.ReviewerStatus(host)
			if err == nil && status.Intact {
				t.Fatalf("certified symlinked asset: %+v", status)
			}
			if _, err := installer.InstallReviewer(host); err == nil {
				t.Fatal("installed through symlink")
			}
		})
	}
}
