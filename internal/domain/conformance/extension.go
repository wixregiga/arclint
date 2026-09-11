package conformance

import (
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// ExtensionFinding is one Diagnostic an Extension reports for a
// selected subject.
type ExtensionFinding struct {
	// SubjectPath names the governed file; empty retains Path as the subject.
	SubjectPath string
	Path        string
	Line        int
	Message     string
	Remediation string
}

// ExtensionScope separates resolved governed files and exclusion policy from
// the repository observations an Extension may inspect.
type ExtensionScope struct {
	Files         []string
	ExcludedFiles []string
	Exclusions    []rule.Exclusion
}

// ExtensionEvaluator is the domain-owned port to Extension
// enforcement: run the named extension rule over the resolved Scope
// using repository-wide observations with host-validated parameters, and return its complete
// findings. Extensions operate only through deterministic host
// capabilities and cannot bypass diagnostic truthfulness; a finding
// whose subject is outside the selected files is a Scope breach the check
// contains without aborting the Assessment. Knowledge is the project's
// recorded domain model, empty when none is recorded.
type ExtensionEvaluator interface {
	Evaluate(extension string, params map[string]any, scope ExtensionScope,
		zones []rule.Zone, obs Observations, knowledge vocab.UbiquitousLanguage) ([]ExtensionFinding, error)
}
