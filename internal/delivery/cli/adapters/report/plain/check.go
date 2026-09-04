package plain

import (
	"fmt"
	"io"

	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
	"github.com/wixregiga/arclint/internal/domain/conformance"
)

func writeCheck(w io.Writer, a conformance.Assessment) error {
	p := &out.Printer{W: w}
	for _, v := range a.ActiveViolations() {
		anchor := v.Path()
		if v.Line() > 0 {
			anchor = fmt.Sprintf("%s:%d", v.Path(), v.Line())
		}
		p.Printf("%s: [%s] %s%s %s\n", anchor, v.Severity(), v.Rule().Qualified(), provenanceTag(v), v.Message())
	}
	for _, d := range a.Diagnostics() {
		switch d.Kind() {
		case conformance.DiagnosticOperational:
			p.Printf("operational [%s]: %s\n", d.Severity(), atPath(d))
		case conformance.DiagnosticCoverage:
			p.Printf("coverage: %s\n", d.Message())
		case conformance.DiagnosticViolation:
			// Violations are rendered from ActiveViolations above.
		}
	}
	p.Printf("%d active finding(s) · %d suppressed · %d baselined · %d rule(s) applied\n",
		len(a.ActiveViolations()), len(a.SuppressedViolations()),
		len(a.BaselinedViolations()), len(a.AppliedRules()))
	return p.Err
}

// provenanceTag spells the distributing Pattern's version beside a
// shared Rule's id. The id already names the Pattern (namespace/name),
// so the tag adds only the version, and a local Rule carries no tag.
func provenanceTag(v conformance.Violation) string {
	ref, ok := v.Provenance()
	if !ok {
		return ""
	}
	return " (@" + ref.Version() + ")"
}

func atPath(d conformance.Diagnostic) string {
	if d.Path() == "" {
		return d.Message()
	}
	if d.Line() > 0 {
		return fmt.Sprintf("%s:%d: %s", d.Path(), d.Line(), d.Message())
	}
	return fmt.Sprintf("%s: %s", d.Path(), d.Message())
}
