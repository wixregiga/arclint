package lipgloss

import (
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
	"github.com/wixregiga/arclint/internal/domain/conformance"
)

func writeCheck(p *out.Printer, th Theme, a conformance.Assessment) {
	for _, v := range a.ActiveViolations() {
		// Grammar: path[:line]: [sev] rule msg
		anchor := th.pathAnchor(v.Path(), v.Line())
		line := anchor + ": " + th.severityBracket(string(v.Severity())) + " " +
			th.Muted.Render(v.Rule().Qualified()) + " " + v.Message()
		p.Printf("%s\n", line)
	}
	for _, d := range a.Diagnostics() {
		switch d.Kind() {
		case conformance.DiagnosticOperational:
			p.Printf("%s %s: %s\n",
				th.Muted.Render("operational"),
				th.severityBracket(string(d.Severity())),
				th.atPath(d.Path(), d.Line(), d.Message()),
			)
		case conformance.DiagnosticCoverage:
			p.Printf("%s: %s\n", th.Muted.Render("coverage"), d.Message())
		case conformance.DiagnosticViolation:
			// Violations are rendered from ActiveViolations above.
		}
	}
	active := len(a.ActiveViolations())
	suppressed := len(a.SuppressedViolations())
	baselined := len(a.BaselinedViolations())
	applied := len(a.AppliedRules())
	p.Printf("%s active finding(s) · %s suppressed · %s baselined · %s rule(s) applied\n",
		countToken(th, active),
		th.Muted.Render(itoa(suppressed)),
		th.Muted.Render(itoa(baselined)),
		th.Bold.Render(itoa(applied)),
	)
}

func countToken(th Theme, n int) string {
	s := itoa(n)
	if n > 0 {
		return th.Error.Render(s)
	}
	return th.OK.Render(s)
}
