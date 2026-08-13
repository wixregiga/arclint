package main

import (
	"fmt"
	"io"
	"time"

	"github.com/wixregiga/arclint/internal/engine"
	"github.com/wixregiga/arclint/internal/report"
)

// writeHuman groups violations by contract clause, blame shown, per the
// proposal's CLI shape.
func writeHuman(w io.Writer, res *engine.Result, dur time.Duration) {
	groups := []struct {
		contract report.Contract
		header   string
	}{
		{report.ContractConsumes, "CONSUMES"},
		{report.ContractProvides, "PROVIDES"},
		{report.ContractInvariant, "INVARIANTS"},
	}
	counts := map[report.Severity]int{}
	for _, g := range groups {
		printed := false
		for _, v := range res.Violations {
			if v.Contract != g.contract {
				continue
			}
			if !printed {
				fmt.Fprintln(w, g.header)
				printed = true
			}
			loc := v.Path
			if v.Line != nil {
				loc = fmt.Sprintf("%s:%d", v.Path, *v.Line)
			}
			fmt.Fprintf(w, "  %s  %s  %s  blame:%s\n", loc, v.RuleID, v.Severity, v.Blame)
			fmt.Fprintf(w, "    %s\n", v.Message)
			if v.FixHint != "" {
				fmt.Fprintf(w, "    fix: %s\n", v.FixHint)
			}
		}
		if printed {
			fmt.Fprintln(w)
		}
	}
	for _, v := range res.Violations {
		counts[v.Severity]++
	}
	suppressed := ""
	if res.Suppressed > 0 {
		suppressed = fmt.Sprintf(" · %d suppressed by except", res.Suppressed)
	}
	if len(res.Violations) == 0 {
		fmt.Fprintf(w, "clean: 0 violations%s · %d files · %s\n", suppressed, res.FilesScanned, dur.Round(time.Millisecond))
		return
	}
	fmt.Fprintf(w, "%d violations (%d error, %d warn, %d info)%s · %d files · %s\n",
		len(res.Violations), counts[report.SeverityError], counts[report.SeverityWarn],
		counts[report.SeverityInfo], suppressed, res.FilesScanned, dur.Round(time.Millisecond))
}
