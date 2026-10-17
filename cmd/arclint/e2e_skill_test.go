package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// skillArtifactNames are the domain-librarian skill files the agents
// skill command must emit byte-exact to the committed litmus.
var skillArtifactNames = []string{
	"SKILL.md",
	"VOCAB.yaml",
}

const (
	// skillLitmusDir is the committed copy every generated copy is
	// compared with; TestAgentsSkillArtifactsCurrent keeps every other
	// committed copy equal to the generator.
	skillLitmusDir = vocab.AgentsSkillDirectory
	// domainSchemaProjectPath is where agents skill lands the schema for
	// human YAML editors, relative to the project root; the
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

// skillFixture is a project used to compare the published skill bundle.
func skillFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", "runtime: [go]\ndomain:\n  file: domain.arclint.yaml\nzones:\n  src:\n    paths: [\"src/**\"]\n")
	write(t, root, "src/ok.go", "package src\n")
	return root
}

// assertDefaultSkillCopies byte-compares the skill files in every
// directory an agent harness loads the skill from with the litmus.
func assertDefaultSkillCopies(t *testing.T, root, project string) {
	t.Helper()
	for _, dir := range vocab.SkillDirectories() {
		assertSkillArtifacts(t, root, project, filepath.Join(project, filepath.FromSlash(dir)))
	}
}

// TestAgentsSkillWritesLitmusFiles runs `arclint agents skill --dir <tmp>`
// from a fixture project and byte-compares each produced file with the
// committed litmus; the schema lands under the fixture's .arclint/schemas
// regardless of --dir, and the requested directory replaces the defaults.
func TestAgentsSkillWritesLitmusFiles(t *testing.T) {
	root := repoRoot(t)
	work := skillFixture(t)
	dir := t.TempDir()
	stdout, stderr, code := runBin(t, work, os.Environ(), "agents", "skill", "--dir", dir)
	if code != 0 {
		t.Fatalf("agents skill exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	assertSkillArtifacts(t, root, work, dir)
	for _, def := range vocab.SkillDirectories() {
		if _, err := os.Lstat(filepath.Join(work, filepath.FromSlash(def))); !os.IsNotExist(err) {
			t.Errorf("--dir %s also wrote the default %s (stat err %v)", dir, def, err)
		}
	}
}

// TestAgentsSkillWritesEveryRequestedDir writes one copy per --dir and
// one line per file, dropping a repeated directory.
func TestAgentsSkillWritesEveryRequestedDir(t *testing.T) {
	root := repoRoot(t)
	work := skillFixture(t)
	first, second := t.TempDir(), t.TempDir()
	stdout, stderr, code := runBin(t, work, os.Environ(), "agents", "skill", "--dir", first, "--dir", second, "--dir", first+"/")
	if code != 0 {
		t.Fatalf("agents skill exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	assertSkillArtifacts(t, root, work, first)
	assertSkillArtifacts(t, root, work, second)
	if lines := strings.Count(stdout, "\n"); lines != 2*len(skillArtifactNames)+1 {
		t.Errorf("want one line per skill file in two directories plus the schema, got %d:\n%s", lines, stdout)
	}
}

// TestAgentsSkillDefaultDirWritesLitmus asserts bare `agents skill`
// (no --dir) writes one identical copy into .agents/skills/domain-librarian
// for Codex and one into .claude/skills/domain-librarian for Claude
// Code, and that running it again changes nothing.
func TestAgentsSkillDefaultDirWritesLitmus(t *testing.T) {
	root := repoRoot(t)
	work := skillFixture(t)
	stdout, stderr, code := runBin(t, work, os.Environ(), "agents", "skill")
	if code != 0 {
		t.Fatalf("agents skill exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	assertDefaultSkillCopies(t, root, work)
	for _, dir := range vocab.SkillDirectories() {
		for _, name := range skillArtifactNames {
			want := "wrote " + filepath.Join(work, filepath.FromSlash(dir), name) + "\n"
			if !strings.Contains(stdout, want) {
				t.Errorf("first run does not report %q:\n%s", want, stdout)
			}
		}
	}

	stdout, stderr, code = runBin(t, work, os.Environ(), "agents", "skill")
	if code != 0 {
		t.Fatalf("second agents skill exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if strings.Contains(stdout, "wrote ") || strings.Count(stdout, "already current\n") != 2*len(skillArtifactNames)+1 {
		t.Errorf("second run changed files:\n%s", stdout)
	}
	assertDefaultSkillCopies(t, root, work)
}

// TestAgentsSkillWritesThroughALinkedClaudeCopy keeps a project whose
// .claude skill directory links to the .agents copy working: both
// names resolve to one set of current files and the link stays.
func TestAgentsSkillWritesThroughALinkedClaudeCopy(t *testing.T) {
	root := repoRoot(t)
	work := skillFixture(t)
	link := filepath.Join(work, filepath.FromSlash(vocab.ClaudeSkillDirectory))
	if err := os.MkdirAll(filepath.Dir(link), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", filepath.FromSlash(vocab.AgentsSkillDirectory)), link); err != nil {
		t.Fatal(err)
	}
	for run := range 2 {
		stdout, stderr, code := runBin(t, work, os.Environ(), "agents", "skill")
		if code != 0 {
			t.Fatalf("run %d: agents skill exit %d\nstdout: %s\nstderr: %s", run, code, stdout, stderr)
		}
	}
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the linked Claude copy is no longer a link: %v %v", info, err)
	}
	assertDefaultSkillCopies(t, root, work)
}

// TestAgentsSkillArtifactsCurrent is the drift gate: every committed
// copy of the domain-librarian skill, in the root and in the
// testing/boxoffice test bed, and the dogfood domain schema must equal
// what the binary generates. Each copy is a directory arclint writes,
// not a link. On failure run `make generate ONLY=skill`; never
// hand-edit the copies.
func TestAgentsSkillArtifactsCurrent(t *testing.T) {
	root := repoRoot(t)
	work := skillFixture(t)
	stdout, stderr, code := runBin(t, work, os.Environ(), "agents", "skill")
	if code != 0 {
		t.Fatalf("agents skill exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	assertDefaultSkillCopies(t, root, work)
	for _, project := range []string{root, filepath.Join(root, "testing", "boxoffice")} {
		for _, dir := range vocab.SkillDirectories() {
			committed := filepath.Join(project, filepath.FromSlash(dir))
			info, err := os.Lstat(committed)
			if err != nil {
				t.Errorf("committed skill copy %s: %v; run make generate ONLY=skill", committed, err)
				continue
			}
			if !info.IsDir() {
				t.Errorf("committed skill copy %s is not a directory (mode %s); arclint agents skill writes it", committed, info.Mode())
				continue
			}
			for _, name := range skillArtifactNames {
				got, err := os.ReadFile(filepath.Join(committed, name))
				if err != nil {
					t.Errorf("read committed %s: %v", name, err)
					continue
				}
				want, err := os.ReadFile(filepath.Join(work, filepath.FromSlash(dir), name))
				if err != nil {
					t.Fatalf("read generated %s: %v", name, err)
				}
				if !bytes.Equal(got, want) {
					t.Errorf("%s is stale; run make generate ONLY=skill (never hand-edit)", filepath.Join(committed, name))
				}
			}
		}
	}
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

func TestAgentsSkillIsIndependentOfRecordingFormat(t *testing.T) {
	for _, format := range []string{"yaml", "cml"} {
		t.Run(format, func(t *testing.T) {
			work := t.TempDir()
			write(t, work, "rules.arclint.yaml", "runtime: [go]\ndomain:\n  file: language."+format+"\nzones:\n  src:\n    paths: [\"src/**\"]\n")
			write(t, work, "src/ok.go", "package src\n")
			stdout, stderr, code := runBin(t, work, os.Environ(), "agents", "skill")
			if code != 0 {
				t.Fatalf("agents skill exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
			}
			assertDefaultSkillCopies(t, repoRoot(t), work)
		})
	}
}
