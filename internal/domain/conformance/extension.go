package conformance

import (
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

// ExtensionEvaluator is the domain-owned port to Extension
// enforcement: run the named extension rule over exactly the selected
// subjects through the same Facts as native evaluators, with host-validated
// parameters, and return its complete
// findings. Extensions operate only through deterministic host
// capabilities and cannot bypass diagnostic truthfulness; a finding
// without a selected subject or its supplied evidence is a Scope breach the check
// contains without aborting the Assessment. Knowledge is the project's
// recorded domain model, empty when none is recorded.
type ExtensionEvaluator interface {
	Evaluate(extension string, params map[string]any, facts Facts,
		knowledge vocab.UbiquitousLanguage) ([]ExtensionFinding, error)
}
