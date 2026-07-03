// Package output renders check results in the two formats from
// docs/design/cli.md: human text (grouped by category) and machine JSON
// (stable schema). Violations and results go to the writer the caller
// picks (stdout); commentary belongs on stderr and is not this package's
// business.
package output

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/jofyi/arclint/internal/config"
	"github.com/jofyi/arclint/internal/rules"
)

// Summary is the run tail: totals for the summary line / JSON summary
// object.
type Summary struct {
	Total        int
	FilesScanned int
	DurationMs   int64
}

// Text renders the default human format per cli.md: violations grouped by
// category (fixed order), rule id always visible, fix hint on its own
// indented line, then a one-line summary. Under quiet, only violation
// lines and the summary print — and nothing at all when clean.
func Text(w io.Writer, vs []rules.Violation, sum Summary, quiet bool) {
	byCat := make(map[config.Category][]rules.Violation)
	for _, v := range vs {
		byCat[v.Category] = append(byCat[v.Category], v)
	}

	for _, cat := range rules.CategoryOrder {
		items := byCat[cat]
		if len(items) == 0 {
			continue
		}
		if !quiet {
			fmt.Fprintf(w, "%s (%d)\n", cat, len(items))
		}
		idW, locW := 0, 0
		locs := make([]string, len(items))
		for i, v := range items {
			loc := v.Path
			if v.Line != nil {
				loc += ":" + strconv.Itoa(*v.Line)
			}
			locs[i] = loc
			idW = max(idW, len(v.RuleID))
			locW = max(locW, len(loc))
		}
		for i, v := range items {
			fmt.Fprintf(w, "  %-*s  %-*s  %s\n", idW, v.RuleID, locW, locs[i], v.Message)
			if !quiet && v.FixHint != "" {
				fmt.Fprintf(w, "          fix: %s\n", v.FixHint)
			}
		}
		if !quiet {
			fmt.Fprintln(w)
		}
	}

	if sum.Total == 0 {
		if !quiet {
			fmt.Fprintf(w, "0 violations in %d files, %dms\n", sum.FilesScanned, sum.DurationMs)
		}
		return
	}

	var parts []string
	for _, cat := range rules.CategoryOrder {
		if n := len(byCat[cat]); n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, cat))
		}
	}
	noun := "violations"
	if sum.Total == 1 {
		noun = "violation"
	}
	fmt.Fprintf(w, "%d %s (%s) in %d files, %dms\n",
		sum.Total, noun, strings.Join(parts, ", "), sum.FilesScanned, sum.DurationMs)
}
