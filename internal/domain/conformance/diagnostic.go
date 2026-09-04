package conformance

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// DiagnosticKind distinguishes the Diagnostic variants.
type DiagnosticKind string

const (
	// DiagnosticViolation reports a broken or suspected-broken Claim.
	DiagnosticViolation DiagnosticKind = "violation"
	// DiagnosticOperational reports a problem encountered while loading
	// or evaluating: unreadable input, parse failure, unknown import.
	DiagnosticOperational DiagnosticKind = "operational"
	// DiagnosticCoverage reports what was not evaluated and why:
	// disabled Rules, unsupported enforcement, stale Baseline entries.
	DiagnosticCoverage DiagnosticKind = "coverage"
)

// Diagnostic is one user-visible result produced while loading,
// evaluating, or explaining conformance. It identifies its kind,
// message, and provenance, and provides enough context to understand
// cause and effect.
type Diagnostic struct {
	kind              DiagnosticKind
	ruleID            string // qualified Rule ID, "" for rule-less operational notes
	pattern           string // PatternReference of the distributing Pattern, "" for local Rules
	path              string
	line              int
	severity          rule.Severity // "" for coverage notes
	message           string
	remediation       string
	status            Status // violation kind only
	suppressionReason string
}

// NewOperational reports an operational problem. Severity states its
// gate effect: an error-severity operational Diagnostic fails the gate.
// ruleID may be empty for rule-less notes; set it when the problem is
// tied to one Rule so Diagnostics carry provenance.
func NewOperational(ruleID, path string, line int, severity rule.Severity, message string) (Diagnostic, error) {
	if strings.TrimSpace(message) == "" {
		return Diagnostic{}, fmt.Errorf("operational diagnostic: missing message")
	}
	if !severity.Valid() {
		return Diagnostic{}, fmt.Errorf("operational diagnostic: severity %q invalid", severity)
	}
	if line < 0 {
		return Diagnostic{}, fmt.Errorf("operational diagnostic: negative line")
	}
	return Diagnostic{
		kind: DiagnosticOperational, ruleID: ruleID, path: path, line: line,
		severity: severity, message: message,
	}, nil
}

// NewCoverage reports why something was not evaluated.
func NewCoverage(ruleID, message string) (Diagnostic, error) {
	if strings.TrimSpace(message) == "" {
		return Diagnostic{}, fmt.Errorf("coverage diagnostic: missing message")
	}
	return Diagnostic{kind: DiagnosticCoverage, ruleID: ruleID, message: message}, nil
}

// Diagnostic lifts a Violation into the unified Diagnostic view.
func (v Violation) Diagnostic() Diagnostic {
	pattern := ""
	if v.provenance != nil {
		pattern = v.provenance.String()
	}
	return Diagnostic{
		kind:              DiagnosticViolation,
		ruleID:            v.ruleID.Qualified(),
		pattern:           pattern,
		path:              v.path,
		line:              v.line,
		severity:          v.severity,
		message:           v.message,
		remediation:       v.remediation,
		status:            v.status,
		suppressionReason: v.suppressionReason,
	}
}

// Kind returns the Diagnostic variant.
func (d Diagnostic) Kind() DiagnosticKind { return d.kind }

// RuleID returns the qualified Rule identity, "" when no Rule is
// involved.
func (d Diagnostic) RuleID() string { return d.ruleID }

// Pattern returns the reference of the Pattern that distributed the
// Rule, "" when the Rule is local or no Rule is involved.
func (d Diagnostic) Pattern() string { return d.pattern }

// Path is the repo-relative anchor, possibly empty for repo-wide
// notes.
func (d Diagnostic) Path() string { return d.path }

// Line is the anchor line, 0 when not line-anchored.
func (d Diagnostic) Line() int { return d.line }

// Severity states the gate effect; "" for coverage notes.
func (d Diagnostic) Severity() rule.Severity { return d.severity }

// Message states the cause.
func (d Diagnostic) Message() string { return d.message }

// Remediation gives fixing context, possibly empty.
func (d Diagnostic) Remediation() string { return d.remediation }

// Status reports the violation status; "" for other kinds.
func (d Diagnostic) Status() Status { return d.status }

// SuppressionReason returns the suppressing reason, when suppressed.
func (d Diagnostic) SuppressionReason() string { return d.suppressionReason }

// Explain describes cause and remediation context.
func (d Diagnostic) Explain() string {
	var b strings.Builder
	b.WriteString(d.message)
	if d.remediation != "" {
		b.WriteString(": ")
		b.WriteString(d.remediation)
	}
	if d.status == StatusSuppressed && d.suppressionReason != "" {
		fmt.Fprintf(&b, " (suppressed: %s)", d.suppressionReason)
	}
	return b.String()
}
