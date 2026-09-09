package rule

import (
	"errors"
	"strings"
)

// Rationale is the author's explanation of why a Rule's Constraint is needed.
// An absent explanation is represented by the zero value, never generated.
type Rationale struct {
	explanation string
}

// NewRationale validates an explicitly supplied explanation.
func NewRationale(explanation string) (Rationale, error) {
	explanation = strings.TrimSpace(explanation)
	if explanation == "" {
		return Rationale{}, errors.New("rationale: empty explanation")
	}
	return Rationale{explanation: explanation}, nil
}

// IsZero reports that no rationale was supplied.
func (r Rationale) IsZero() bool { return r.explanation == "" }

// String returns the author's explanation, or empty when absent.
func (r Rationale) String() string { return r.explanation }
