package application

// SkillArtifactWriter persists one generated skill artifact file under
// a target directory. Implementations live in infrastructure.
// Write creates the directory as needed, writes atomically, and reports
// whether the on-disk bytes changed.
type SkillArtifactWriter interface {
	Write(dir, filename string, content []byte) (changed bool, path string, err error)
}

// DomainLibrarianSkillDir is the default relative install directory for
// domain-librarian skill artifacts (SKILL.md, VOCAB.yaml, library.schema.json).
const DomainLibrarianSkillDir = ".agents/skills/domain-librarian"
