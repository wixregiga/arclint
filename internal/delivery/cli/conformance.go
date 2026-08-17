package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// The output formats format-flagged commands accept.
const (
	formatHuman = "human"
	formatJSON  = "json"
)

// NewCheckCommand adapts the conformance use case into the check
// command.
func NewCheckCommand(assess application.AssessConformance) Command {
	return Command{
		Name:  "check",
		Short: "evaluate the configured Rules against the repository",
		// The optional path selects the repository; the composition root
		// resolves it before any adapter exists.
		MaxArgs: 1,
		Flags: []Flag{
			{Name: "format", Default: formatHuman, Doc: "output format: human, json"},
			{Name: "no-baseline", Bool: true, Doc: "evaluate without subtracting the committed baseline"},
		},
		Run: func(ctx Context) error {
			format := ctx.String("format")
			if format != formatHuman && format != formatJSON {
				return &ExitError{
					Code:    ExitConfigError,
					Message: fmt.Sprintf("unknown format %q (human, json)", format),
				}
			}
			assessment, err := assess.Execute(application.AssessConformanceRequest{
				SkipBaseline: ctx.Bool("no-baseline"),
			})
			if err != nil {
				return ConfigError(err)
			}
			switch format {
			case formatJSON:
				if err := writeJSON(ctx.Stdout, assessment); err != nil {
					return ConfigError(err)
				}
			default:
				if err := writeHuman(ctx.Stdout, assessment); err != nil {
					return fmt.Errorf("write output: %w", err)
				}
			}
			if assessment.HasErrors() {
				return ViolationsExit()
			}
			return nil
		},
	}
}

// diagnosticDoc is the stable JSON shape of one Diagnostic in the
// target engine's output.
type diagnosticDoc struct {
	Kind        string `json:"kind"`
	RuleID      string `json:"ruleId,omitempty"`
	Path        string `json:"path,omitempty"`
	Line        int    `json:"line,omitempty"`
	Severity    string `json:"severity,omitempty"`
	Status      string `json:"status,omitempty"`
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
}

func writeJSON(w io.Writer, a conformance.Assessment) error {
	diags := a.Diagnostics()
	docs := make([]diagnosticDoc, 0, len(diags))
	for _, d := range diags {
		docs = append(docs, diagnosticDoc{
			Kind:        string(d.Kind()),
			RuleID:      d.RuleID(),
			Path:        d.Path(),
			Line:        d.Line(),
			Severity:    string(d.Severity()),
			Status:      string(d.Status()),
			Message:     d.Message(),
			Remediation: d.Remediation(),
		})
	}
	data, err := json.MarshalIndent(docs, "", "  ")
	if err != nil {
		return fmt.Errorf("encode diagnostics: %w", err)
	}
	if _, err := fmt.Fprintln(w, string(data)); err != nil {
		return fmt.Errorf("write diagnostics: %w", err)
	}
	return nil
}

func writeHuman(w io.Writer, a conformance.Assessment) error {
	p := &printer{w: w}
	for _, v := range a.ActiveViolations() {
		anchor := v.Path()
		if v.Line() > 0 {
			anchor = fmt.Sprintf("%s:%d", v.Path(), v.Line())
		}
		p.printf("%s: [%s] %s %s\n", anchor, v.Severity(), v.Rule().Qualified(), v.Message())
	}
	for _, d := range a.Diagnostics() {
		switch d.Kind() {
		case conformance.DiagnosticOperational:
			p.printf("operational [%s]: %s\n", d.Severity(), atPath(d))
		case conformance.DiagnosticCoverage:
			p.printf("coverage: %s\n", d.Message())
		case conformance.DiagnosticViolation:
			// Violations are rendered from ActiveViolations above.
		}
	}
	p.printf("%d active finding(s) · %d suppressed · %d baselined · %d rule(s) applied\n",
		len(a.ActiveViolations()), len(a.SuppressedViolations()),
		len(a.BaselinedViolations()), len(a.AppliedRules()))
	return p.err
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
