package conformance

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// Evaluation is one evidence-qualified attempt to evaluate a Rule for
// one Rule Subject: exactly one Rule, exactly one Subject, exactly one
// Outcome, with the Evidence Method, Assurance, limitations, and
// Diagnostics recorded.
type Evaluation struct {
	ruleID      rule.ID
	subject     rule.Subject
	outcome     Outcome
	evidence    rule.EvidenceMethod
	assurance   rule.Assurance
	limitations []string
	violations  []Violation
}

// NewEvaluation constructs a valid Evaluation or rejects it. The
// Outcome must be justified: conforms requires Assurance that permits
// conformance, violation outcomes require at least one Violation, and
// every other outcome requires none.
func NewEvaluation(id rule.ID, subject rule.Subject, outcome Outcome,
	enforcement rule.Enforcement, violations []Violation,
) (Evaluation, error) {
	if id.IsZero() {
		return Evaluation{}, fmt.Errorf("evaluation: missing rule id")
	}
	fail := func(err error) (Evaluation, error) {
		return Evaluation{}, fmt.Errorf("evaluation of %s: %v", id, err)
	}
	if subject.IsZero() {
		return fail(fmt.Errorf("missing subject"))
	}
	if !outcome.Valid() {
		return fail(fmt.Errorf("outcome %q invalid", outcome))
	}
	if enforcement.IsZero() {
		return fail(fmt.Errorf("missing enforcement"))
	}
	switch outcome {
	case OutcomeConforms:
		if !enforcement.Assurance().PermitsConformance() {
			return fail(fmt.Errorf("assurance %q cannot justify conformance", enforcement.Assurance()))
		}
		fallthrough
	case OutcomeUndetermined, OutcomeUnsupported, OutcomeNotApplicable, OutcomeFailed:
		if len(violations) > 0 {
			return fail(fmt.Errorf("outcome %q carries violations", outcome))
		}
	case OutcomeViolates, OutcomeSuspectedViolation:
		if len(violations) == 0 {
			return fail(fmt.Errorf("outcome %q without a violation", outcome))
		}
	}
	for _, v := range violations {
		if !v.Rule().Equals(id) {
			return fail(fmt.Errorf("violation references rule %s", v.Rule()))
		}
		if v.Outcome() != outcome {
			return fail(fmt.Errorf("violation outcome %q differs from evaluation outcome %q", v.Outcome(), outcome))
		}
	}
	return Evaluation{
		ruleID:      id,
		subject:     subject,
		outcome:     outcome,
		evidence:    enforcement.Evidence(),
		assurance:   enforcement.Assurance(),
		limitations: enforcement.Limitations(),
		violations:  append([]Violation(nil), violations...),
	}, nil
}

// Rule returns the evaluated Rule's identity.
func (e Evaluation) Rule() rule.ID { return e.ruleID }

// Subject returns the one evaluated Rule Subject.
func (e Evaluation) Subject() rule.Subject { return e.subject }

// Outcome returns the honest conclusion.
func (e Evaluation) Outcome() Outcome { return e.outcome }

// Evidence names how the conclusion was reached.
func (e Evaluation) Evidence() rule.EvidenceMethod { return e.evidence }

// Assurance is the declared conclusion strength.
func (e Evaluation) Assurance() rule.Assurance { return e.assurance }

// Limitations returns the documented analysis limits.
func (e Evaluation) Limitations() []string { return append([]string(nil), e.limitations...) }

// Violations returns the Violations produced by this evaluation.
func (e Evaluation) Violations() []Violation { return append([]Violation(nil), e.violations...) }

// Diagnostics returns the evaluation's Violations in the unified
// Diagnostic view.
func (e Evaluation) Diagnostics() []Diagnostic {
	out := make([]Diagnostic, 0, len(e.violations))
	for _, v := range e.violations {
		out = append(out, v.Diagnostic())
	}
	return out
}

// withViolations rebuilds the evaluation with relabeled violations,
// preserving everything else. The set must be the same findings; only
// status may differ.
func (e Evaluation) withViolations(vs []Violation) Evaluation {
	e.violations = append([]Violation(nil), vs...)
	return e
}
