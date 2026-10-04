package agentfiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOwnedUpgradeAndEditedAssetPreservation(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, ".codex/agents/arclint-domain-reviewer.toml")
	for _, content := range []string{"first version", "second version"} {
		if _, err := Install(root, map[string][]byte{target: []byte(content)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(target, []byte("user change"), 0600); err != nil {
		t.Fatal(err)
	}
	extra := filepath.Join(root, ".codex/new-file")
	if _, err := Install(root, map[string][]byte{target: []byte("third version"), extra: []byte("new")}); err == nil {
		t.Fatal("overwrote local edit")
	}
	got, _ := os.ReadFile(target)
	if string(got) != "user change" {
		t.Fatal("lost edit")
	}
	if _, err := os.Stat(extra); !os.IsNotExist(err) {
		t.Fatal("partial write before conflict check")
	}
}
