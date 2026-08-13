package engine

import (
	"strings"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/report"
)

// fillCapabilities stamps every builtin violation with its rule's
// enforcement tier (extension violations arrive already stamped from the
// extension's declared capability).
func fillCapabilities(rs *config.RuleSet, vs []report.Violation) {
	byID := map[string]string{"scan.unknown-imports": config.CapabilityExact}
	for _, inst := range rs.Instances() {
		if inst.Capability != "" {
			byID[inst.ID] = inst.Capability
		}
	}
	for i := range vs {
		if vs[i].Capability != "" {
			continue
		}
		if c, ok := byID[vs[i].RuleID]; ok {
			vs[i].Capability = c
			continue
		}
		// Derived consumes ids carry a per-aspect suffix
		// (<module>.consumes.internal); the instance id is the clause
		// (<module>.consumes).
		for _, aspect := range []string{".internal", ".external", ".stdlib"} {
			if trimmed, ok := strings.CutSuffix(vs[i].RuleID, aspect); ok {
				if c, ok := byID[trimmed]; ok {
					vs[i].Capability = c
				}
				break
			}
		}
	}
}
