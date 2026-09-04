package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// skillArtifactNames are the three domain-librarian skill files the
// agents skill command must emit byte-exact to the committed litmus.
var skillArtifactNames = []string{
	"SKILL.md",
	"VOCAB.yaml",
	"library.schema.json",
}

const skillLitmusDir = ".agents/skills/domain-librarian"

// TestAgentsSkillWritesLitmusFiles runs `arclint agents skill --dir <tmp>`
// and byte-compares each produced file with the committed litmus under
// .agents/skills/domain-librarian/.
func TestAgentsSkillWritesLitmusFiles(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	stdout, stderr, code := runBin(t, root, os.Environ(), "agents", "skill", "--dir", dir)
	if code != 0 {
		t.Fatalf("agents skill exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	for _, name := range skillArtifactNames {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read generated %s: %v", name, err)
		}
		want, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(skillLitmusDir), name))
		if err != nil {
			t.Fatalf("read litmus %s: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s bytes differ from litmus; regenerate via `arclint agents skill` (never hand-edit)", name)
		}
	}
}

// TestAgentsSkillDefaultDirWritesLitmus asserts bare `agents skill`
// (no --dir) writes into the canonical .agents/skills/domain-librarian
// relative to the resolved repository root.
func TestAgentsSkillDefaultDirWritesLitmus(t *testing.T) {
	root := repoRoot(t)
	work := domainFixture(t)
	stdout, stderr, code := runBin(t, work, os.Environ(), "agents", "skill")
	if code != 0 {
		t.Fatalf("agents skill exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	for _, name := range skillArtifactNames {
		got, err := os.ReadFile(filepath.Join(work, filepath.FromSlash(skillLitmusDir), name))
		if err != nil {
			t.Fatalf("read default-dir %s: %v", name, err)
		}
		want, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(skillLitmusDir), name))
		if err != nil {
			t.Fatalf("read litmus %s: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s default-dir bytes differ from litmus", name)
		}
	}
}

// TestAgentsSkillArtifactsCurrent is the drift gate: the three committed
// .agents/skills/domain-librarian files must equal what the binary generates.
// On failure run `arclint agents skill`; never hand-edit the fixtures.
func TestAgentsSkillArtifactsCurrent(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	stdout, stderr, code := runBin(t, root, os.Environ(), "agents", "skill", "--dir", dir)
	if code != 0 {
		t.Fatalf("agents skill exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	for _, name := range skillArtifactNames {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read generated %s: %v", name, err)
		}
		want, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(skillLitmusDir), name))
		if err != nil {
			t.Fatalf("read committed %s: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is stale; run `arclint agents skill`", name)
		}
	}
}
