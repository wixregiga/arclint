package rule

import "fmt"

// Suppression is a Pattern Consumer decision retaining a produced
// Violation while removing its configured reporting or gate effect for
// selected subjects. It never changes Applicability or the Evaluation
// Outcome, and never turns a Violation into conformance.
type Suppression struct {
	paths  []Glob
	reason string
}

// NewSuppression requires at least one path selector and a reason.
func NewSuppression(paths []Glob, reason string) (Suppression, error) {
	if len(paths) == 0 {
		return Suppression{}, fmt.Errorf("suppression: no diagnostic scope")
	}
	if reason == "" {
		return Suppression{}, fmt.Errorf("suppression: missing reason")
	}
	for _, g := range paths {
		if g.IsZero() {
			return Suppression{}, fmt.Errorf("suppression: unconstructed path glob")
		}
	}
	return Suppression{paths: append([]Glob(nil), paths...), reason: reason}, nil
}

// MatchesPath decides whether a Violation anchored at the path is
// suppressed.
func (s Suppression) MatchesPath(path string) bool {
	for _, g := range s.paths {
		if g.Match(path) {
			return true
		}
	}
	return false
}

// Paths returns the diagnostic scope.
func (s Suppression) Paths() []Glob { return append([]Glob(nil), s.paths...) }

// Reason returns why matching Violations are suppressed.
func (s Suppression) Reason() string { return s.reason }
