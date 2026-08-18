package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// NewRulesCommand adapts the Rule listing and showing use cases into
// the rules command: no argument lists, one argument shows the named
// Rule completely. Subcommands print the published Rule Schema and run
// the authored Rule Tests.
func NewRulesCommand(list application.ListRules, show application.ShowRule,
	runTests application.RunRuleTests,
) Command {
	return Command{
		Name:         "rules",
		Short:        "list the configured Rules, or show those matching an id, prefix, or pattern",
		MaxArgs:      1,
		CompleteArgs: completeRuleIDs(list),
		Subcommands: []Command{
			newRuleSchemaCommand(),
			newRuleTestCommand(runTests),
		},
		Run: func(ctx Context) error {
			if len(ctx.Args) == 1 {
				return showMatchingRules(ctx, list, show, ctx.Args[0])
			}
			rows, err := list.Execute()
			if err != nil {
				return ConfigError(err)
			}
			if err := writeRuleRows(ctx.Stdout, rows); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}
}

// showMatchingRules resolves one selector: a single match shows the
// complete Rule, several render as a narrowed listing.
func showMatchingRules(ctx Context, list application.ListRules, show application.ShowRule, selector string) error {
	rows, err := list.Select(selector)
	if err != nil {
		return ConfigError(err)
	}
	if len(rows) == 1 {
		detail, err := show.Execute(rows[0].ID)
		if err != nil {
			return ConfigError(err)
		}
		if err := writeRuleDetail(ctx.Stdout, detail); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}
	if err := writeRuleRows(ctx.Stdout, rows); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

// writeRuleRows renders the one-line listing form of Rule summaries.
func writeRuleRows(w io.Writer, rows []application.RuleSummary) error {
	p := &printer{w: w}
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
	return p.err
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
	p.printf("\n%s", d.Schema)
	return p.err
}

// completeRuleIDs adapts the listing use case into Rule-id completion
// candidates for the rules command and the check selectors. Per the
// CompleteArgs contract a failing ruleset yields no candidates: the
// error is swallowed deliberately, because a completion callback may
// never print or fail.
func completeRuleIDs(list application.ListRules) func(args []string, toComplete string) []Candidate {
	return func([]string, string) []Candidate {
		rows, err := list.Execute()
		if err != nil {
			return nil
		}
		candidates := make([]Candidate, 0, len(rows))
		for _, row := range rows {
			candidates = append(candidates, Candidate{Value: row.ID, Doc: row.Claim})
		}
		return candidates
	}
}

// completeRuleSelectors completes one comma-separated selector list:
// the trailing segment completes against the configured rule ids while
// the already-typed segments stay as the inserted prefix.
func completeRuleSelectors(list application.ListRules) func(toComplete string) []Candidate {
	ids := completeRuleIDs(list)
	return func(toComplete string) []Candidate {
		return withListPrefix(toComplete, ids(nil, ""))
	}
}

// withListPrefix keeps the comma-joined segments already typed as the
// inserted prefix of every candidate, so completing the trailing
// segment of a comma list inserts the whole value.
func withListPrefix(toComplete string, candidates []Candidate) []Candidate {
	i := strings.LastIndexByte(toComplete, ',')
	if i < 0 {
		return candidates
	}
	prefix := toComplete[:i+1]
	out := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, Candidate{Value: prefix + c.Value, Doc: c.Doc})
	}
	return out
}

// newRuleSchemaCommand prints the published Rule Schema: the JSON
// Schema editors reference to validate and autocomplete rules.yaml.
// Runtime validation and this published schema accept the same values.
func newRuleSchemaCommand() Command {
	return Command{
		Name:  "schema",
		Short: "print the JSON Schema for rules.yaml",
		Run: func(ctx Context) error {
			data, err := rule.Schema()
			if err != nil {
				return fmt.Errorf("rule schema: %w", err)
			}
			if _, err := ctx.Stdout.Write(data); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}
}

// newRuleTestCommand runs the repository's authored Rule Tests; an
// optional argument selects one test by name. Any failing test exits
// with the findings code.
func newRuleTestCommand(runTests application.RunRuleTests) Command {
	return Command{
		Name:    "test",
		Short:   "run the Rule Tests under .arclint/tests",
		MaxArgs: 1,
		Run: func(ctx Context) error {
			results, err := runTests.Execute()
			if err != nil {
				return ConfigError(err)
			}
			if len(ctx.Args) == 1 {
				var kept []application.RuleTestResult
				for _, r := range results {
					if r.Name == ctx.Args[0] {
						kept = append(kept, r)
					}
				}
				if len(kept) == 0 {
					return &ExitError{
						Code:    ExitConfigError,
						Message: fmt.Sprintf("no rule test named %q under .arclint/tests", ctx.Args[0]),
					}
				}
				results = kept
			}
			if err := writeRuleTestResults(ctx.Stdout, results); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			for _, r := range results {
				if !r.Passed() {
					return ViolationsExit()
				}
			}
			return nil
		},
	}
}

// writeRuleTestResults renders one line per test plus, for failures,
// every difference in ready-to-paste expect-entry form — the authoring
// loop is: run with an empty expect list, then adopt the reported
// actuals that are intended.
func writeRuleTestResults(w io.Writer, results []application.RuleTestResult) error {
	p := &printer{w: w}
	if len(results) == 0 {
		p.println("no rule tests under .arclint/tests")
		return p.err
	}
	passed := 0
	for _, r := range results {
		if r.Passed() {
			passed++
			p.printf("ok   %s (%s)\n", r.Name, r.RuleID)
			continue
		}
		p.printf("FAIL %s (%s)\n", r.Name, r.RuleID)
		if r.Err != "" {
			p.printf("  error: %s\n", r.Err)
		}
		if len(r.Unexpected) > 0 {
			p.println("  unexpected findings (add intended ones to expect):")
			for _, f := range r.Unexpected {
				writeExpectEntry(p, f.Kind, f.Path, f.Line, f.Message)
			}
		}
		if len(r.Missing) > 0 {
			p.println("  expected findings that never occurred:")
			for _, e := range r.Missing {
				writeExpectEntry(p, e.Kind, e.Path, e.Line, e.Message)
			}
		}
	}
	p.printf("%d passed · %d failed\n", passed, len(results)-passed)
	return p.err
}

// writeExpectEntry prints one finding as a .arclint/tests expect list
// entry.
func writeExpectEntry(p *printer, kind rule.FindingKind, path string, line int, message string) {
	p.printf("    - kind: %s\n      path: %s\n", kind, path)
	if line > 0 {
		p.printf("      line: %d\n", line)
	}
	p.printf("      message: %q\n", message)
}
