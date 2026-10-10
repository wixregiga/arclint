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
