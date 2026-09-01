package pattern

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// Reference identifies one exact distributed version of a named Pattern.
// It aliases the Rule context's origin value so Pattern provenance is one
// shared value, not a parallel identity representation.
type Reference = rule.PatternOrigin

// NewReference requires the exact identity of a distributed Pattern version.
func NewReference(namespace, name, version string) (Reference, error) {
	reference, err := rule.NewPatternOrigin(namespace, name, version)
	if err != nil {
		return Reference{}, fmt.Errorf("pattern reference: %w", err)
	}
	return reference, nil
}
