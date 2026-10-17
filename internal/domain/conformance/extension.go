package conformance

import (
	"time"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// ExtensionFinding is one Diagnostic an Extension reports for a
// selected subject.
type ExtensionFinding struct {
	// SubjectPath defaults to Path; a distinct location must cite supplied evidence.
	SubjectPath string
	Path        string
	Line        int
	Message     string
	Remediation string
}

// ExtensionRun is the complete result of one Extension check: its
// findings, how long the check ran, and the source file of the Extension
// that ran it.
type ExtensionRun struct {
	Findings []ExtensionFinding
	Elapsed  time.Duration
	Source   string
}

// ExtensionEvaluator is the domain-owned port to Extension
// enforcement: run the named extension rule over exactly the selected
// subjects through the same Facts as native evaluators, with host-validated
// parameters, under the Rule's TimeLimit, and return its complete
// findings with the time its check ran and the Extension's source file.
// Extensions operate only through deterministic host capabilities and
// cannot bypass diagnostic truthfulness; a finding without a selected
// subject or its supplied evidence is a Scope breach the check contains
// without aborting the Assessment. Extension code that stops, at its
// time limit or on an error it raised, is returned as an
// ExtensionFailure. Knowledge is the project's recorded domain model,
// empty when none is recorded.
type ExtensionEvaluator interface {
	Evaluate(extension string, params map[string]any, limit rule.TimeLimit, facts Facts,
		knowledge vocab.UbiquitousLanguage) (ExtensionRun, error)
}
