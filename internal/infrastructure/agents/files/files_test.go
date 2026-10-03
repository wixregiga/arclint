package agentfiles

import (
	"github.com/wixregiga/arclint/internal/application"
	"os"
	"path/filepath"
	"runtime"
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

func TestScopeRejectsResolvedEscapesOnNativePlatform(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.yaml"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(root, filepath.Join(outside, "secret.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Scope(root, []string{relative}); err == nil {
		t.Fatal("accepted platform-native parent escape")
	}
	if _, err := Scope(root, []string{filepath.Join(outside, "secret.yaml")}); err == nil {
		t.Fatal("accepted absolute recording")
	}
	if err := os.WriteFile(filepath.Join(root, "domain.yaml"), []byte("domain"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("native symlinks unavailable: %v", err)
	}
	if _, err := Scope(root, []string{"linked/secret.yaml"}); err == nil {
		t.Fatal("accepted escaped domain symlink")
	}
	if _, err := Scope(root, []string{"domain.yaml"}, application.AgentSourceScope{Sources: []string{"linked/*.go"}}); err == nil {
		t.Fatal("accepted escaped source-prefix symlink")
	}
}

func TestPlannedScopePermitsOnlyAbsentDefaultRecording(t *testing.T) {
	root := t.TempDir()
	if _, err := PlannedScope(root, []string{"domain.arclint.yaml"}); err != nil {
		t.Fatal(err)
	}
	if _, err := PlannedScope(root, []string{"domain.arclint.yaml", "other.yaml"}); err == nil {
		t.Fatal("preflight accepted missing default that setup will not create")
	}
	if _, err := Scope(root, []string{"domain.arclint.yaml"}); err == nil {
		t.Fatal("normal install accepted absent recording")
	}
	if _, err := PlannedScope(root, []string{"custom.yaml"}); err == nil {
		t.Fatal("preflight accepted absent custom recording")
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "missing.yaml"), filepath.Join(root, "domain.arclint.yaml")); err != nil {
		t.Skipf("native symlinks unavailable: %v", err)
	}
	if _, err := PlannedScope(root, []string{"domain.arclint.yaml"}); err == nil {
		t.Fatal("preflight accepted dangling recording symlink")
	}
}

func TestScopeWindowsBackslashTraversal(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("requires native Windows filepath semantics")
	}
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	if err := os.Mkdir(root, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "secret.yaml"), []byte("outside evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Scope(root, []string{`..\secret.yaml`}); err == nil {
		t.Fatal("accepted explicit Windows parent path")
	}
	if _, err := PlannedScope(root, []string{`..\secret.yaml`}); err == nil {
		t.Fatal("planned scope accepted explicit Windows parent path")
	}
}

func TestScopeDoubleStarRequiresActualReadableMatches(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "domain.yaml"), []byte("domain"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src", "nested"), 0750); err != nil {
		t.Fatal(err)
	}
	selection := application.AgentSourceScope{Sources: []string{"src/**/*.go"}}
	if _, err := Scope(root, []string{"domain.yaml"}, selection); err == nil {
		t.Fatal("accepted empty recursive glob")
	}
	for _, name := range []string{"src/direct.go", "src/nested/deep.go"} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte("package source"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Scope(root, []string{"domain.yaml"}, selection); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret.go")
	if err := os.WriteFile(outside, []byte("outside source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "src", "secret.go")); err != nil {
		t.Skipf("native symlinks unavailable: %v", err)
	}
	if _, err := Scope(root, []string{"domain.yaml"}, selection); err == nil {
		t.Fatal("accepted escaping matched symlink alongside valid files")
	}
}
