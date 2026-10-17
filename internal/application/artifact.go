package application

import (
	"path/filepath"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// ArtifactWriter persists one generated artifact file (a skill file, a
// published JSON Schema) under a target directory. Implementations
// live in infrastructure. Write creates the directory as needed,
// writes atomically, and reports whether the on-disk bytes changed.
type ArtifactWriter interface {
	Write(dir, filename string, content []byte) (changed bool, path string, err error)
}

// SkillDirectories returns the directories one run writes the
// domain-librarian skill artifacts (SKILL.md, VOCAB.yaml) into: the
// requested ones in order with repeats dropped, or every directory a
// supported agent harness loads the skill from when none is requested.
func SkillDirectories(requested []string) []string {
	if len(requested) == 0 {
		return vocab.SkillDirectories()
	}
	dirs := make([]string, 0, len(requested))
	seen := make(map[string]bool, len(requested))
	for _, dir := range requested {
		key := filepath.Clean(dir)
		if seen[key] {
			continue
		}
		seen[key] = true
		dirs = append(dirs, dir)
	}
	return dirs
}

// SchemaDirectory is the default relative directory the published JSON
// Schemas are written into. The vocabulary owns the literal because the
// VOCAB.yaml it renders tells the librarian where the schema lives.
const SchemaDirectory = vocab.SchemaDirectory
