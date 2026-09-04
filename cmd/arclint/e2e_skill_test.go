package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// skillArtifactNames are the domain-librarian skill files the agents
// skill command must emit byte-exact to the committed litmus.
var skillArtifactNames = []string{
	"SKILL.md",
	"VOCAB.yaml",
}

const (
	skillLitmusDir = ".agents/skills/domain-librarian"
	// domainSchemaProjectPath is where agents skill lands the schema the
	// skill vocabulary points at, relative to the project root; the
	// litmus is the release copy under docs/schemas.
	domainSchemaProjectPath = ".arclint/schemas/domain.arclint.schema.json"
	domainSchemaLitmus      = "docs/schemas/domain.arclint.schema.json"
)

// assertSkillArtifacts byte-compares each skill file under dir, and the
// domain schema under the project root, with the committed litmus.
func assertSkillArtifacts(t *testing.T, root, project, dir string) {
	t.Helper()
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
	got, err := os.ReadFile(filepath.Join(project, filepath.FromSlash(domainSchemaProjectPath)))
	if err != nil {
		t.Fatalf("read generated domain schema: %v", err)
	}
	want, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(domainSchemaLitmus)))
	if err != nil {
		t.Fatalf("read litmus domain schema: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s bytes differ from %s; run make schemas (never hand-edit)", domainSchemaProjectPath, domainSchemaLitmus)
	}
}

// TestAgentsSkillWritesLitmusFiles runs `arclint agents skill --dir <tmp>`
// from a fixture project and byte-compares each produced file with the
// committed litmus; the schema lands under the fixture's .arclint/schemas
// regardless of --dir.
func TestAgentsSkillWritesLitmusFiles(t *testing.T) {
	root := repoRoot(t)
	work := domainFixture(t)
	dir := t.TempDir()
	stdout, stderr, code := runBin(t, work, os.Environ(), "agents", "skill", "--dir", dir)
	if code != 0 {
		t.Fatalf("agents skill exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	assertSkillArtifacts(t, root, work, dir)
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
	assertSkillArtifacts(t, root, work, filepath.Join(work, filepath.FromSlash(skillLitmusDir)))
}

// TestAgentsSkillArtifactsCurrent is the drift gate: the committed
// .agents/skills/domain-librarian files and the dogfood domain schema
// must equal what the binary generates. On failure run `arclint agents
// skill`; never hand-edit the fixtures.
func TestAgentsSkillArtifactsCurrent(t *testing.T) {
	root := repoRoot(t)
	work := domainFixture(t)
	stdout, stderr, code := runBin(t, work, os.Environ(), "agents", "skill")
	if code != 0 {
		t.Fatalf("agents skill exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	assertSkillArtifacts(t, root, work, filepath.Join(work, filepath.FromSlash(skillLitmusDir)))
	got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(domainSchemaProjectPath)))
	if err != nil {
		t.Fatalf("read committed %s: %v", domainSchemaProjectPath, err)
	}
	want, err := os.ReadFile(filepath.Join(work, filepath.FromSlash(domainSchemaProjectPath)))
	if err != nil {
		t.Fatalf("read generated domain schema: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s is stale; run make schemas", domainSchemaProjectPath)
	}
}
