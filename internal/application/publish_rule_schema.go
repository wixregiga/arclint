package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// PublishRuleSchema renders and writes the Rule Schema,
// rules.arclint.schema.json, from rule.Schema(), the single source of
// the ruleset grammar editors validate and autocomplete against.
type PublishRuleSchema struct {
	writer ArtifactWriter
}

// NewPublishRuleSchema requires the artifact writer port.
func NewPublishRuleSchema(writer ArtifactWriter) (PublishRuleSchema, error) {
	if writer == nil {
		return PublishRuleSchema{}, fmt.Errorf("publish rule schema: missing artifact writer")
	}
	return PublishRuleSchema{writer: writer}, nil
}

// Render returns the schema bytes from rule.Schema().
func (uc PublishRuleSchema) Render() ([]byte, error) {
	out, err := rule.Schema()
	if err != nil {
		return nil, fmt.Errorf("publish rule schema: %w", err)
	}
	return out, nil
}

// Execute writes the schema under dir (default SchemaDirectory when
// dir is empty), reporting whether the file changed and its path.
func (uc PublishRuleSchema) Execute(dir string) (changed bool, path string, err error) {
	if dir == "" {
		dir = SchemaDirectory
	}
	content, err := uc.Render()
	if err != nil {
		return false, "", err
	}
	changed, path, err = uc.writer.Write(dir, rule.SchemaFileName, content)
	if err != nil {
		return false, path, fmt.Errorf("publish rule schema: %w", err)
	}
	return changed, path, nil
}
