package plain

import (
	"fmt"
	"io"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

func writeRuleRows(w io.Writer, rows []application.RuleSummary) error {
	p := &out.Printer{W: w}
	for _, row := range rows {
		marker := ""
		if row.Disabled {
			marker = fmt.Sprintf("  (disabled: %s)", row.DisabledReason)
		}
		provenance := ""
		if row.Provenance != "" {
			provenance = "  from " + row.Provenance
		}
		p.Printf("%s  [%s/%s/%s]  %s%s%s\n",
			row.ID, row.Type, row.Severity, row.Assurance, row.Claim, provenance, marker)
	}
	return p.Err
}

func writeRuleDetail(w io.Writer, d application.RuleDetail) error {
	p := &out.Printer{W: w}
	write := func(name, value string) {
		if value != "" {
			p.Printf("%-12s %s\n", name+":", value)
		}
	}
	write("id", d.Summary.ID)
	write("type", d.Summary.Type)
	write("severity", d.Summary.Severity)
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
	write("provenance", d.Summary.Provenance)
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
	return p.Err
}

func writeRuleTestResults(w io.Writer, results []application.RuleTestResult) error {
	p := &out.Printer{W: w}
	if len(results) == 0 {
		p.Println("no rule tests under .arclint/tests")
		return p.Err
	}
	passed := 0
	for _, r := range results {
		if r.Passed() {
			passed++
			p.Printf("ok   %s (%s)\n", r.Name, r.RuleID)
			continue
		}
		p.Printf("FAIL %s (%s)\n", r.Name, r.RuleID)
		if r.Err != "" {
			p.Printf("  error: %s\n", r.Err)
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
	p.Printf("%d passed · %d failed\n", passed, len(results)-passed)
	return p.Err
}

func writeExpectEntry(p *out.Printer, kind rule.FindingKind, path string, line int, message string) {
	p.Printf("    - kind: %s\n      path: %s\n", kind, path)
	if line > 0 {
		p.Printf("      line: %d\n", line)
	}
	p.Printf("      message: %q\n", message)
}
