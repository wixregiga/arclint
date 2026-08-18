package conformance

// Outcome is the honest conclusion of one Rule Evaluation. Exactly one
// value completes an evaluation, and only sufficient evidence may
// produce conforms or violates.
type Outcome string

const (
	// OutcomeConforms means sufficient evaluation found no violation
	// within its declared limit.
	OutcomeConforms Outcome = "conforms"
	// OutcomeViolates means sufficient evidence proves the Claim is
	// broken.
	OutcomeViolates Outcome = "violates"
	// OutcomeSuspectedViolation means heuristic evidence indicates a
	// possible violation.
	OutcomeSuspectedViolation Outcome = "suspected_violation"
	// OutcomeUndetermined means available evidence cannot justify
	// conformance or violation.
	OutcomeUndetermined Outcome = "undetermined"
	// OutcomeUnsupported means required enforcement or Language Facts
	// are unavailable.
	OutcomeUnsupported Outcome = "unsupported"
	// OutcomeNotApplicable means Applicability or exclusion removes the
	// subject.
	OutcomeNotApplicable Outcome = "not_applicable"
	// OutcomeFailed means evaluation could not complete correctly.
	OutcomeFailed Outcome = "failed"
)

// Valid reports whether the value is a defined enum member.
func (o Outcome) Valid() bool {
	switch o {
	case OutcomeConforms, OutcomeViolates, OutcomeSuspectedViolation,
		OutcomeUndetermined, OutcomeUnsupported, OutcomeNotApplicable, OutcomeFailed:
		return true
	}
	return false
}

// RequiresDiagnostic decides whether the outcome must be surfaced:
// silence may never read as conformance.
func (o Outcome) RequiresDiagnostic() bool {
	switch o {
	case OutcomeViolates, OutcomeSuspectedViolation, OutcomeUnsupported, OutcomeFailed:
		return true
	case OutcomeConforms, OutcomeUndetermined, OutcomeNotApplicable:
		return false
	}
	return false
}
