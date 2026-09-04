package lipgloss

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

func writeRuleRows(p *out.Printer, th Theme, rows []application.RuleSummary) {
	for _, row := range rows {
		marker := ""
		if row.Disabled {
			marker = th.Muted.Render(fmt.Sprintf("  (disabled: %s)", row.DisabledReason))
		}
		provenance := ""
		if row.Provenance != "" {
			provenance = th.Muted.Render("  from " + row.Provenance)
		}
		meta := th.Muted.Render("[") +
			th.Muted.Render(row.Type+"/") +
			th.severity(row.Severity).Render(row.Severity) +
			th.Muted.Render("/"+row.Assurance+"]")
		p.Printf("%s  %s  %s%s%s\n", th.Muted.Render(row.ID), meta, row.Claim, provenance, marker)
	}
}

func writeRuleDetail(p *out.Printer, th Theme, d application.RuleDetail) {
	write := func(name, value string) {
		if value == "" {
			return
		}
		label := fmt.Sprintf("%-12s", name+":")
		p.Printf("%s %s\n", th.Bold.Render(label), value)
	}
	write("id", th.Muted.Render(d.Summary.ID))
	write("type", d.Summary.Type)
	if d.Summary.Severity != "" {
		label := fmt.Sprintf("%-12s", "severity:")
		p.Printf("%s %s\n", th.Bold.Render(label), th.severity(d.Summary.Severity).Render(d.Summary.Severity))
	}
	write("claim", d.Summary.Claim)
	write("asserts", d.Asserts)
	if d.EntireRepository {
		write("applies to", "the entire repository")
	} else {
		write("modules", strings.Join(d.Modules, ", "))
		write("files", strings.Join(d.Files, ", "))
	}
	write("evidence", d.Evidence)
	write("assurance", d.Summary.Assurance)
	if d.Summary.Severity == "error" {
		write("when violated", "fails the gate")
	} else {
		write("when violated", fmt.Sprintf("reported as %s without failing the gate", d.Summary.Severity))
	}
	write("languages", strings.Join(d.Languages, ", "))
	write("facts", strings.Join(d.Facts, ", "))
	write("limitations", strings.Join(d.Limitations, "; "))
	write("provenance", th.Muted.Render(d.Summary.Provenance))
	for _, e := range d.Exclusions {
		write("excluded", fmt.Sprintf("%s (%s)", strings.Join(e.Selectors, ", "), e.Reason))
	}
	for _, s := range d.Suppressions {
		write("suppressed", fmt.Sprintf("%s (%s)", strings.Join(s.Selectors, ", "), s.Reason))
	}
	if d.Summary.Disabled {
		write("disabled", d.Summary.DisabledReason)
	}
	p.Printf("\n%s", d.Schema)
}

func writeRuleTestResults(p *out.Printer, th Theme, results []application.RuleTestResult) {
	if len(results) == 0 {
		p.Println(th.Muted.Render("no rule tests under .arclint/tests"))
		return
	}
	passed := 0
	for _, r := range results {
		if r.Passed() {
			passed++
			p.Printf("%s   %s (%s)\n", th.OK.Render("ok"), r.Name, th.Muted.Render(r.RuleID))
			continue
		}
		p.Printf("%s %s (%s)\n", th.Fail.Render("FAIL"), r.Name, th.Muted.Render(r.RuleID))
		if r.Err != "" {
			p.Printf("  %s: %s\n", th.Error.Render("error"), r.Err)
		}
		if len(r.Unexpected) > 0 {
			p.Println("  unexpected findings (add intended ones to expect):")
			for _, f := range r.Unexpected {
				writeExpectEntry(p, f.Kind, f.Path, f.Line, f.Message)
			}
		}
		if len(r.Missing) > 0 {
			p.Println("  expected findings that never occurred:")
			for _, e := range r.Missing {
				writeExpectEntry(p, e.Kind, e.Path, e.Line, e.Message)
			}
		}
	}
	failed := len(results) - passed
	passTok := th.OK.Render(itoa(passed))
	failTok := itoa(failed)
	if failed > 0 {
		failTok = th.Error.Render(failTok)
	} else {
		failTok = th.Muted.Render(failTok)
	}
	p.Printf("%s passed · %s failed\n", passTok, failTok)
}

func writeExpectEntry(p *out.Printer, kind rule.FindingKind, path string, line int, message string) {
	p.Printf("    - kind: %s\n      path: %s\n", kind, path)
	if line > 0 {
		p.Printf("      line: %d\n", line)
	}
	p.Printf("      message: %q\n", message)
}
