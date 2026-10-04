package agentfiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNullReceiptRejectedBeforeWrites(t *testing.T) {
	root := t.TempDir()
	receipt := filepath.Join(root, ".arclint/agent-assets.json")
	if err := os.MkdirAll(filepath.Dir(receipt), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receipt, []byte("null"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, ".codex/new-file")
	if _, err := Install(root, map[string][]byte{target: []byte("new")}); err == nil || !strings.Contains(err.Error(), "expected an object") {
		t.Fatalf("expected invalid receipt error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".codex")); !os.IsNotExist(err) {
		t.Fatal("wrote installation before rejecting null receipt")
	}
	got, err := os.ReadFile(receipt)
	if err != nil || string(got) != "null" {
		t.Fatalf("changed invalid receipt: %q, %v", got, err)
	}
}

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
