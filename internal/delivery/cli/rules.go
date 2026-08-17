package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
)

// NewRulesCommand adapts the Rule listing and showing use cases into
// the rules command: no argument lists, one argument shows the named
// Rule completely.
func NewRulesCommand(list application.ListRules, show application.ShowRule) Command {
	return Command{
		Name:    "rules",
		Short:   "list the configured Rules, or show one by id",
		MaxArgs: 1,
		Run: func(ctx Context) error {
			if len(ctx.Args) == 1 {
				detail, err := show.Execute(ctx.Args[0])
				if err != nil {
					return ConfigError(err)
				}
				if err := writeRuleDetail(ctx.Stdout, detail); err != nil {
					return fmt.Errorf("write output: %w", err)
				}
				return nil
			}
			rows, err := list.Execute()
			if err != nil {
				return ConfigError(err)
			}
			p := &printer{w: ctx.Stdout}
			for _, row := range rows {
				marker := ""
				if row.Disabled {
					marker = fmt.Sprintf("  (disabled: %s)", row.DisabledReason)
				}
				provenance := ""
				if row.Provenance != "" {
					provenance = "  from " + row.Provenance
				}
				p.printf("%s  [%s/%s/%s]  %s%s%s\n",
					row.ID, row.Type, row.Severity, row.Assurance, row.Claim, provenance, marker)
			}
			if p.err != nil {
				return fmt.Errorf("write output: %w", p.err)
			}
			return nil
		},
	}
}

func writeRuleDetail(w io.Writer, d application.RuleDetail) error {
	p := &printer{w: w}
	write := func(name, value string) {
		if value != "" {
			p.printf("%-12s %s\n", name+":", value)
		}
	}
	write("id", d.Summary.ID)
	write("type", d.Summary.Type)
	write("severity", d.Summary.Severity)
	write("claim", d.Summary.Claim)
	if d.EntireRepository {
		write("applies to", "the entire repository")
	} else {
		write("modules", strings.Join(d.Modules, ", "))
		write("files", strings.Join(d.Files, ", "))
	}
	write("evidence", d.Evidence)
	write("assurance", d.Summary.Assurance)
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
	p.printf("\n%s", d.Schema)
	return p.err
}

// NewExplainCommand adapts the explanation use case into the explain
// command.
func NewExplainCommand(explain application.ExplainRule) Command {
	return Command{
		Name:    "explain",
		Short:   "explain one Rule in human- and agent-readable form",
		MaxArgs: 1,
		Run: func(ctx Context) error {
			if len(ctx.Args) != 1 {
				return &ExitError{Code: ExitConfigError, Message: "explain requires exactly one rule id"}
			}
			text, err := explain.Execute(ctx.Args[0])
			if err != nil {
				return ConfigError(err)
			}
			p := &printer{w: ctx.Stdout}
			p.print(text)
			if p.err != nil {
				return fmt.Errorf("write output: %w", p.err)
			}
			return nil
		},
	}
}
