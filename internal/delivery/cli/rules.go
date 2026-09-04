package cli

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// NewRulesCommand adapts the Rule listing and showing use cases into
// the rules command: no argument lists, one argument shows the named
// Rule completely. Subcommands print the published Rule Schema and run
// the authored Rule Tests.
func NewRulesCommand(list application.ListRules, show application.ShowRule,
	runTests application.RunRuleTests, publishSchema application.PublishRuleSchema, render Renderer,
) Command {
	return Command{
		Name:         "rules",
		Short:        "list the configured Rules, or show those matching an id, prefix, or pattern",
		MaxArgs:      1,
		CompleteArgs: completeRuleIDs(list),
		Subcommands: []Command{
			newRuleSchemaCommand(publishSchema, render),
			newRuleTestCommand(runTests, render),
		},
		Run: func(ctx Context) error {
			if len(ctx.Args) == 1 {
				return showMatchingRules(ctx, list, show, render, ctx.Args[0])
			}
			rows, err := list.Execute()
			if err != nil {
				return ConfigError(err)
			}
			if err := render.Render(ctx.Stdout, RuleListReport{Rules: rows}); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}
}

// showMatchingRules resolves one selector: a single match shows the
// complete Rule, several render as a narrowed listing.
func showMatchingRules(ctx Context, list application.ListRules, show application.ShowRule, render Renderer, selector string) error {
	rows, err := list.Select(selector)
	if err != nil {
		return ConfigError(err)
	}
	if len(rows) == 1 {
		detail, err := show.Execute(rows[0].ID)
		if err != nil {
			return ConfigError(err)
		}
		if err := render.Render(ctx.Stdout, RuleDetailReport{Detail: detail}); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}
	if err := render.Render(ctx.Stdout, RuleListReport{Rules: rows}); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

// completeRuleIDs adapts the listing use case into Rule-id completion
// candidates for the rules command and the check selectors. Per the
// CompleteArgs contract a failing ruleset yields no candidates: the
// error is swallowed deliberately, because a completion callback may
// never print or fail.
func completeRuleIDs(list application.ListRules) func(args []string, toComplete string) []AutoCompleteCandidate {
	return func([]string, string) []AutoCompleteCandidate {
		rows, err := list.Execute()
		if err != nil {
			return nil
		}
		candidates := make([]AutoCompleteCandidate, 0, len(rows))
		for _, row := range rows {
			candidates = append(candidates, AutoCompleteCandidate{Value: row.ID, Doc: row.Claim})
		}
		return candidates
	}
}

// completeRuleSelectors completes one comma-separated selector list:
// the trailing segment completes against the configured rule ids while
// the already-typed segments stay as the inserted prefix.
func completeRuleSelectors(list application.ListRules) func(toComplete string) []AutoCompleteCandidate {
	ids := completeRuleIDs(list)
	return func(toComplete string) []AutoCompleteCandidate {
		return withListPrefix(toComplete, ids(nil, ""))
	}
}

// withListPrefix keeps the comma-joined segments already typed as the
// inserted prefix of every candidate, so completing the trailing
// segment of a comma list inserts the whole value.
func withListPrefix(toComplete string, candidates []AutoCompleteCandidate) []AutoCompleteCandidate {
	i := strings.LastIndexByte(toComplete, ',')
	if i < 0 {
		return candidates
	}
	prefix := toComplete[:i+1]
	out := make([]AutoCompleteCandidate, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, AutoCompleteCandidate{Value: prefix + c.Value, Doc: c.Doc})
	}
	return out
}

// newRuleSchemaCommand prints or writes the published Rule Schema:
// the JSON Schema editors reference to validate and autocomplete the
// ruleset. Runtime validation and this published schema accept the
// same values.
func newRuleSchemaCommand(publish application.PublishRuleSchema, render Renderer) Command {
	return newSchemaCommand(
		"print or write the JSON Schema for "+rule.RulesetFileName,
		ruleSchemaLong,
		ruleSchemaExample,
		publish,
		render,
	)
}

const ruleSchemaLong = `Print the JSON Schema accepted for ` + rule.RulesetFileName + `, or write it under
the project's schema directory with --write so the ruleset's modeline can
name a local copy.

Runtime validation and the published schema accept the same values.`

const ruleSchemaExample = `  arclint rules schema
  arclint rules schema --write
  arclint rules schema --write --dir docs/schemas`

// newRuleTestCommand runs the repository's authored Rule Tests; an
// optional argument selects one test by name. Any failing test exits
// with the findings code.
func newRuleTestCommand(runTests application.RunRuleTests, render Renderer) Command {
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
			if err := render.Render(ctx.Stdout, RuleTestReport{Results: results}); err != nil {
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
