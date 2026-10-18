package conformance

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// ContractCoverage retains the structural checks and gaps for one recorded
// invariant or assertion. No result here establishes statement correctness.
type ContractCoverage struct {
	context, owner, key, statement string
	kind                           vocab.Concept
	path                           string
	line                           int
	checks                         []Evaluation
	unperformed                    []string
	carrier                        Carrier
	located                        bool
	bodilessOwner                  string
}

// ContractCoverageSpec is the input to the validated coverage value.
type ContractCoverageSpec struct {
	Context, Owner, Key, Statement string
	Kind                           vocab.Concept
	// Path is the domain file the contract is recorded in; Line is where.
	Path        string
	Line        int
	Checks      []Evaluation
	Unperformed []string
	// Carrier is the declaration carrying the contract, as a check
	// located it; nil when no check located one.
	Carrier *Carrier
	// BodilessOwner names how an assertion's owner is declared when it
	// has no body to call a checking method from, an interface or an
	// abstract class; nothing then carries the assertion.
	BodilessOwner string
}

// NewContractCoverage validates the recording and preserves each check unchanged.
func NewContractCoverage(s ContractCoverageSpec) (ContractCoverage, error) {
	if strings.TrimSpace(s.Owner) == "" || strings.TrimSpace(s.Key) == "" || strings.TrimSpace(s.Statement) == "" || strings.TrimSpace(s.Path) == "" || s.Line < 0 {
		return ContractCoverage{}, fmt.Errorf("contract coverage: owner, key, statement, path, and nonnegative line are required")
	}
	subject, err := rule.ContextSubject(s.Context)
	if err != nil {
		return ContractCoverage{}, fmt.Errorf("contract coverage: %w", err)
	}
	if s.Kind != vocab.ConceptInvariant && s.Kind != vocab.ConceptAssertion {
		return ContractCoverage{}, fmt.Errorf("contract coverage: %q is not an invariant or assertion", s.Kind)
	}
	for _, check := range s.Checks {
		if check.Rule().IsZero() || !check.Subject().Equals(subject) {
			return ContractCoverage{}, fmt.Errorf("contract coverage: check does not belong to context %s", s.Context)
		}
	}
	for _, reason := range s.Unperformed {
		if strings.TrimSpace(reason) == "" {
			return ContractCoverage{}, fmt.Errorf("contract coverage: an unperformed check needs a reason")
		}
	}
	out := ContractCoverage{
		context: s.Context, owner: s.Owner, key: s.Key, statement: s.Statement,
		kind: s.Kind, path: s.Path, line: s.Line, checks: append([]Evaluation(nil), s.Checks...),
		unperformed: append([]string(nil), s.Unperformed...), bodilessOwner: s.BodilessOwner,
	}
	if s.BodilessOwner != "" && s.Kind != vocab.ConceptAssertion {
		return ContractCoverage{}, fmt.Errorf("contract coverage: only an assertion's owner is judged by how it is declared")
	}
	if s.Carrier != nil {
		if strings.TrimSpace(s.Carrier.Path) == "" || s.Carrier.Line <= 0 {
			return ContractCoverage{}, fmt.Errorf("contract coverage: a carrier needs a path and a positive line")
		}
		out.carrier, out.located = *s.Carrier, true
	}
	return out, nil
}

// Context names the bounded context holding the contract.
func (c ContractCoverage) Context() string { return c.context }

// Owner names the recorded aggregate, value object, or service.
func (c ContractCoverage) Owner() string { return c.owner }

// Key returns the recorded invariant or assertion key.
func (c ContractCoverage) Key() string { return c.key }

// Statement returns the recorded guarantee without reinterpretation.
func (c ContractCoverage) Statement() string { return c.statement }

// Kind distinguishes an invariant from an assertion.
func (c ContractCoverage) Kind() vocab.Concept { return c.kind }

// Path is the domain file the contract is recorded in.
func (c ContractCoverage) Path() string { return c.path }

// Line anchors the contract in its domain file.
func (c ContractCoverage) Line() int { return c.line }

// Checks returns detached evaluations of the applicable structural rules.
func (c ContractCoverage) Checks() []Evaluation { return append([]Evaluation(nil), c.checks...) }

// Carrier is the declaration carrying the contract; false when no
// check located one.
func (c ContractCoverage) Carrier() (Carrier, bool) { return c.carrier, c.located }

// BodilessOwner names how an assertion's owner is declared when it has
// no body to call a checking method from, an interface or an abstract
// class, so nothing carries the assertion; "" when the owner has a body
// or was not located.
func (c ContractCoverage) BodilessOwner() string { return c.bodilessOwner }

// Unperformed explains why applicable checks were not attempted.
func (c ContractCoverage) Unperformed() []string { return append([]string(nil), c.unperformed...) }

// Status describes structural checking only. Limits stay on each Evaluation.
func (c ContractCoverage) Status() string {
	if len(c.checks) == 0 {
		return "unchecked"
	}
	for _, outcome := range []Outcome{OutcomeFailed, OutcomeViolates, OutcomeSuspectedViolation, OutcomeUnsupported, OutcomeUndetermined, OutcomeNotApplicable} {
		for _, e := range c.checks {
			if e.Outcome() == outcome {
				return string(outcome)
			}
		}
	}
	if len(c.unperformed) > 0 {
		return "incomplete"
	}
	return "structurally_checked"
}
