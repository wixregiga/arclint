package rule

import "fmt"

// Severity is the configured linter importance of a Rule Violation.
// It is independent from Assurance and Evidence Method: how important a
// break is says nothing about how sure ArcLint is that it happened.
type Severity string

const (
	// SeverityError fails the configured gate.
	SeverityError Severity = "error"
	// SeverityWarning requires attention without failing the default gate.
	SeverityWarning Severity = "warning"
	// SeverityInfo communicates lower-priority guidance.
	SeverityInfo Severity = "info"
)

// DefaultSeverity is the declared default when a Rule states none.
const DefaultSeverity = SeverityError

// ParseSeverity accepts a defined enum value; the empty string resolves
// to the declared default.
func ParseSeverity(s string) (Severity, error) {
	switch Severity(s) {
	case SeverityError, SeverityWarning, SeverityInfo:
		return Severity(s), nil
	}
	if s == "" {
		return DefaultSeverity, nil
	}
	return "", fmt.Errorf("severity %q: not one of error, warning, info", s)
}

// FailsGate decides whether an active Violation of this Severity fails
// the gate.
func (s Severity) FailsGate() bool { return s == SeverityError }

// Valid reports whether the value is a defined enum member.
func (s Severity) Valid() bool {
	switch s {
	case SeverityError, SeverityWarning, SeverityInfo:
		return true
	}
	return false
}
