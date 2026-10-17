package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// The directories an agent harness loads this repository's skills from:
// Codex reads the source, Claude Code reads the copy.
const (
	repositorySkillSource = ".agents/skills"
	repositorySkillCopy   = ".claude/skills"
)

// TestRepositorySkillCopiesCurrent is the drift gate for this
// repository's own skills: every skill directory under .agents/skills
// has a copy under .claude/skills that is a real directory, not a link,
// holding the same files with the same bytes and nothing else. On
// failure run `make generate ONLY=skill-copies`; never hand-edit a copy.
func TestRepositorySkillCopiesCurrent(t *testing.T) {
	root := repoRoot(t)
	skills, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(repositorySkillSource)))
	if err != nil {
		t.Fatalf("read %s: %v", repositorySkillSource, err)
	}
	checked := 0
	for _, skill := range skills {
		if !skill.IsDir() {
			continue
		}
		checked++
		source := filepath.Join(root, filepath.FromSlash(repositorySkillSource), skill.Name())
		copied := filepath.Join(root, filepath.FromSlash(repositorySkillCopy), skill.Name())
		info, err := os.Lstat(copied)
		if err != nil {
			t.Errorf("skill %s has no copy under %s: %v; run make generate ONLY=skill-copies", skill.Name(), repositorySkillCopy, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s/%s is not a directory (mode %s); make generate ONLY=skill-copies writes it", repositorySkillCopy, skill.Name(), info.Mode())
			continue
		}
		want := skillTree(t, source)
		got := skillTree(t, copied)
		for name, content := range want {
			copiedContent, ok := got[name]
			switch {
			case !ok:
				t.Errorf("%s/%s/%s is missing; run make generate ONLY=skill-copies", repositorySkillCopy, skill.Name(), name)
			case !bytes.Equal(copiedContent, content):
				t.Errorf("%s/%s/%s differs from %s; run make generate ONLY=skill-copies (never hand-edit)", repositorySkillCopy, skill.Name(), name, repositorySkillSource)
			}
		}
		for name := range got {
			if _, ok := want[name]; !ok {
				t.Errorf("%s/%s/%s has no source under %s; run make generate ONLY=skill-copies", repositorySkillCopy, skill.Name(), name, repositorySkillSource)
			}
		}
	}
	if checked == 0 {
		t.Fatalf("no skill directories under %s", repositorySkillSource)
	}
}

// skillTree reads every file under dir, keyed by its slash path relative
// to dir. A link inside a skill is reported, since a copy must hold the
// bytes itself.
func skillTree(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			t.Errorf("%s is a link; a skill holds its files", path)
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = content
		return nil
	})
	if err != nil {
		t.Fatalf("read skill %s: %v", dir, err)
	}
	return files
}
