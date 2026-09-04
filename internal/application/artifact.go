package application

import "github.com/wixregiga/arclint/internal/domain/vocab"

// ArtifactWriter persists one generated artifact file (a skill file, a
// published JSON Schema) under a target directory. Implementations
// live in infrastructure. Write creates the directory as needed,
// writes atomically, and reports whether the on-disk bytes changed.
type ArtifactWriter interface {
	Write(dir, filename string, content []byte) (changed bool, path string, err error)
}

// DomainLibrarianSkillDir is the default relative install directory for
// the domain-librarian skill artifacts (SKILL.md, VOCAB.yaml).
const DomainLibrarianSkillDir = vocab.SkillDirectory

// SchemaDirectory is the default relative directory the published JSON
// Schemas are written into. The vocabulary owns the literal because the
// VOCAB.yaml it renders tells the librarian where the schema lives.
const SchemaDirectory = vocab.SchemaDirectory
