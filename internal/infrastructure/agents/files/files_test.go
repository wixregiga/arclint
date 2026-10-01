package agentfiles

import (
	"github.com/wixregiga/arclint/internal/application"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOwnedUpgradeAndEditedAssetPreservation(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, ".omp/extensions/arclint-domain-guard/index.js")
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

func TestScopeUpdatesOnlyExplicitSelections(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0750); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"domain.yaml", "src/order.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("data"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, ".arclint"), 0750); err != nil {
		t.Fatal(err)
	}
	config := []byte("{\"version\":1,\"domainFiles\":[\"domain.yaml\"],\"sourcePatterns\":[\"src/order.go\"],\"ownerOption\":true}")
	if err := os.WriteFile(filepath.Join(root, ".arclint/domain-guard.json"), config, 0600); err != nil {
		t.Fatal(err)
	}
	unchanged, err := Scope(root, []string{"domain.yaml"})
	if err != nil || string(unchanged) != string(config) {
		t.Fatalf("changed scope: %s %v", unchanged, err)
	}
	updated, err := Scope(root, []string{"domain.yaml"}, application.AgentSourceScope{DomainSources: []string{"src/*.go"}})
	if err != nil || !strings.Contains(string(updated), "ownerOption") || !strings.Contains(string(updated), "domainSourcePatterns") || !strings.Contains(string(updated), "sourcePatterns") {
		t.Fatalf("scope lost: %s %v", updated, err)
	}
	for _, pattern := range []string{"../outside.go", "/tmp/outside.go", "*.go", "src/*.ts", ".codex/x.go"} {
		if _, err := Scope(root, []string{"domain.yaml"}, application.AgentSourceScope{DomainSources: []string{pattern}}); err == nil {
			t.Fatalf("accepted unsafe or unsupported %s", pattern)
		}
	}
}
