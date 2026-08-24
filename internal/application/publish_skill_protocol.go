package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// PublishSkillProtocol renders and writes the domain-librarian SKILL.md
// protocol file from domain-owned skill taxonomy constants.
type PublishSkillProtocol struct {
	writer SkillArtifactWriter
}

// NewPublishSkillProtocol requires the skill artifact writer port.
func NewPublishSkillProtocol(writer SkillArtifactWriter) (PublishSkillProtocol, error) {
	if writer == nil {
		return PublishSkillProtocol{}, fmt.Errorf("publish skill protocol: missing skill artifact writer")
	}
	return PublishSkillProtocol{writer: writer}, nil
}

// Render returns the SKILL.md content byte-exact to the litmus file.
func (uc PublishSkillProtocol) Render() string {
	return vocab.SkillMarkdown()
}

// Execute writes SKILL.md under dir (default DomainLibrarianSkillDir
// when dir is empty), reporting whether the file changed and its path.
func (uc PublishSkillProtocol) Execute(dir string) (changed bool, path string, err error) {
	if dir == "" {
		dir = DomainLibrarianSkillDir
	}
	content := []byte(uc.Render())
	changed, path, err = uc.writer.Write(dir, vocab.SkillProtocolFile, content)
	if err != nil {
		return false, path, fmt.Errorf("publish skill protocol: %w", err)
	}
	return changed, path, nil
}
