package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// PublishLibrarySchema renders and writes library.schema.json from
// vocab.Schema(), the single source of the domain-librarian library
// document schema.
type PublishLibrarySchema struct {
	writer SkillArtifactWriter
}

// NewPublishLibrarySchema requires the skill artifact writer port.
func NewPublishLibrarySchema(writer SkillArtifactWriter) (PublishLibrarySchema, error) {
	if writer == nil {
		return PublishLibrarySchema{}, fmt.Errorf("publish library schema: missing skill artifact writer")
	}
	return PublishLibrarySchema{writer: writer}, nil
}

// Render returns library.schema.json bytes from vocab.Schema().
func (uc PublishLibrarySchema) Render() ([]byte, error) {
	out, err := vocab.Schema()
	if err != nil {
		return nil, fmt.Errorf("publish library schema: %w", err)
	}
	return out, nil
}

// Execute writes library.schema.json under dir (default
// DomainLibrarianSkillDir when dir is empty), reporting whether the
// file changed and its path.
func (uc PublishLibrarySchema) Execute(dir string) (changed bool, path string, err error) {
	if dir == "" {
		dir = DomainLibrarianSkillDir
	}
	content, err := uc.Render()
	if err != nil {
		return false, "", err
	}
	changed, path, err = uc.writer.Write(dir, vocab.SkillLibrarySchemaFile, content)
	if err != nil {
		return false, path, fmt.Errorf("publish library schema: %w", err)
	}
	return changed, path, nil
}
