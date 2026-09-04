package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// PublishDomainSchema renders and writes the Ubiquitous Language
// schema, domain.arclint.schema.json, from vocab.Schema(), the single
// source of the library document grammar.
type PublishDomainSchema struct {
	writer ArtifactWriter
}

// NewPublishDomainSchema requires the artifact writer port.
func NewPublishDomainSchema(writer ArtifactWriter) (PublishDomainSchema, error) {
	if writer == nil {
		return PublishDomainSchema{}, fmt.Errorf("publish domain schema: missing artifact writer")
	}
	return PublishDomainSchema{writer: writer}, nil
}

// Render returns the schema bytes from vocab.Schema().
func (uc PublishDomainSchema) Render() ([]byte, error) {
	out, err := vocab.Schema()
	if err != nil {
		return nil, fmt.Errorf("publish domain schema: %w", err)
	}
	return out, nil
}

// Execute writes the schema under dir (default SchemaDirectory when
// dir is empty), reporting whether the file changed and its path.
func (uc PublishDomainSchema) Execute(dir string) (changed bool, path string, err error) {
	if dir == "" {
		dir = SchemaDirectory
	}
	content, err := uc.Render()
	if err != nil {
		return false, "", err
	}
	changed, path, err = uc.writer.Write(dir, vocab.SchemaFileName, content)
	if err != nil {
		return false, path, fmt.Errorf("publish domain schema: %w", err)
	}
	return changed, path, nil
}
