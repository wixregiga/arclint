package sobekextension_test

import (
	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
	sobekextension "github.com/wixregiga/arclint/internal/infrastructure/extension/sobek"
)

// testLimit is the check time limit the tests run under: the default.
var testLimit = rule.DefaultTimeLimit.Duration()

// evaluate runs one extension rule under the default time limit and
// returns its findings.
func evaluate(e *sobekextension.Evaluator, extension string, params map[string]any,
	facts conformance.Facts, knowledge vocab.UbiquitousLanguage,
) ([]conformance.ExtensionFinding, error) {
	run, err := e.Evaluate(extension, params, rule.DefaultTimeLimit, facts, knowledge)
	return run.Findings, err
}
