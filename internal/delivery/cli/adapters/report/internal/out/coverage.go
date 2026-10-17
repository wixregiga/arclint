package out

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// ContractCoverage prints structural coverage separately from rule findings.
func ContractCoverage(p *Printer, a conformance.Assessment) {
	contracts := a.Contracts()
	for _, context := range a.ContractContexts() {
		invariants, assertions, checked := 0, 0, 0
		for _, c := range contracts {
			if c.Context() != context {
				continue
			}
			if c.Kind() == vocab.ConceptInvariant {
				invariants++
			} else {
				assertions++
			}
			if c.Status() == "structurally_checked" {
				checked++
			}
		}
		p.Printf("contracts: %s: %d invariant(s), %d assertion(s); %d structurally checked\n", context, invariants, assertions, checked)
		if invariants+assertions == 0 {
			p.Printf("  no named contracts to check; coverage is not established\n")
		}
	}
	for _, c := range contracts {
		if c.Status() == "structurally_checked" {
			continue
		}
		reasons := c.Unperformed()
		for _, e := range c.Checks() {
			if e.Outcome() != conformance.OutcomeConforms {
				reasons = append(reasons, fmt.Sprintf("%s: %s", e.Rule(), e.Outcome()))
			}
		}
		p.Printf("  %s.%s/%s: %s (%s)\n", c.Context(), c.Owner(), c.Key(), c.Status(), strings.Join(reasons, "; "))
	}
	if len(a.ContractContexts()) > 0 {
		p.Printf("contracts: structural checks do not verify that check bodies enforce the recorded statements\n")
	}
}

// PatternBoundMessage is the coverage statement for one applied Rule
// whose Assurance holds only for its pattern.
func PatternBoundMessage(ruleID string) string {
	return "rule " + ruleID + " is exact for its pattern; its rationale is not checked"
}

// PatternBoundSummary states, on one line, that the applied Rules
// whose Assurance holds only for their pattern leave their Rationale
// unchecked; "" when no applied Rule is pattern-bound.
func PatternBoundSummary(a conformance.Assessment) string {
	ids := a.PatternBoundRules()
	if len(ids) == 0 {
		return ""
	}
	return "these rules are exact for their pattern; their rationale is not checked: " + strings.Join(ids, ", ")
}

// WithoutFailingCaseHeading introduces the content Rules no Rule Test
// exercises with a failing case.
const WithoutFailingCaseHeading = "content rules without a failing case (add a test whose expect holds a violation drawn from the rule's rationale):"
