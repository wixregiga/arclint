package output

import (
	"encoding/json"
	"io"

	"github.com/jofyi/arclint/internal/rules"
)

// jsonViolation mirrors the exact violation shape from cli.md/rules.md:
// line present only when line-anchored, fixHint always present (empty
// string when the rule defines none). The schema is stable — fields are
// added, never renamed or removed.
type jsonViolation struct {
	RuleID   string `json:"ruleId"`
	Category string `json:"category"`
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Line     *int   `json:"line,omitempty"`
	Message  string `json:"message"`
	FixHint  string `json:"fixHint"`
}

type jsonSummary struct {
	Total        int   `json:"total"`
	FilesScanned int   `json:"filesScanned"`
	DurationMs   int64 `json:"durationMs"`
}

type jsonReport struct {
	Violations []jsonViolation `json:"violations"`
	Summary    jsonSummary     `json:"summary"`
}

// JSON writes the single machine-readable result object from cli.md:
// {"violations": [...], "summary": {...}}. A clean run emits an empty
// violations array, never null.
func JSON(w io.Writer, vs []rules.Violation, sum Summary) error {
	report := jsonReport{
		Violations: make([]jsonViolation, 0, len(vs)),
		Summary: jsonSummary{
			Total:        sum.Total,
			FilesScanned: sum.FilesScanned,
			DurationMs:   sum.DurationMs,
		},
	}
	for _, v := range vs {
		report.Violations = append(report.Violations, jsonViolation{
			RuleID:   v.RuleID,
			Category: string(v.Category),
			Severity: string(v.Severity),
			Path:     v.Path,
			Line:     v.Line,
			Message:  v.Message,
			FixHint:  v.FixHint,
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
