package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// PublishSkillVocabulary renders and writes the domain-librarian
// VOCAB.yaml from domain-owned taxonomy data. Emission is hand-assembled
// in the domain so comments and flow-style lines match the litmus file;
// yaml.Marshal is never used.
type PublishSkillVocabulary struct {
	writer ArtifactWriter
}

// NewPublishSkillVocabulary requires the artifact writer port.
func NewPublishSkillVocabulary(writer ArtifactWriter) (PublishSkillVocabulary, error) {
	if writer == nil {
		return PublishSkillVocabulary{}, fmt.Errorf("publish skill vocabulary: missing artifact writer")
	}
	return PublishSkillVocabulary{writer: writer}, nil
}

// Render returns the VOCAB.yaml content byte-exact to the litmus file.
func (uc PublishSkillVocabulary) Render() string {
	return vocab.VocabularyYAML()
}

// Execute writes VOCAB.yaml under dir (default DomainLibrarianSkillDir
// when dir is empty), reporting whether the file changed and its path.
func (uc PublishSkillVocabulary) Execute(dir string) (changed bool, path string, err error) {
	if dir == "" {
		dir = DomainLibrarianSkillDir
	}
	content := []byte(uc.Render())
	changed, path, err = uc.writer.Write(dir, vocab.SkillVocabularyFile, content)
	if err != nil {
		return false, path, fmt.Errorf("publish skill vocabulary: %w", err)
	}
	return changed, path, nil
}
