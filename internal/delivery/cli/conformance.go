package cli

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// NewCheckCommand adapts the conformance use case into the check
// command; the listing use case exists only to complete rule
// selectors. Presentation is closed over the injected Renderer.
func NewCheckCommand(assess application.AssessConformance, list application.ListRules, inspectAgents application.InspectAgentsContext, render Renderer) Command {
	return Command{
		Name:  checkCommand,
		Short: "evaluate the configured Rules against the repository",
		// The optional path selects the repository; the composition root
		// resolves it before any adapter exists. The path keeps the
		// adapter's default file completion, so no CompleteArgs here.
		MaxArgs: 1,
		Flags: []Flag{
			{Name: "no-baseline", Bool: true, Doc: "evaluate without subtracting the committed baseline"},
			{
				Name:     "only",
				Doc:      "evaluate only rules matching these ids, prefixes, or patterns (comma or space separated)",
				Complete: completeRuleSelectors(list),
			},
			{
				Name:     "exclude",
				Doc:      "skip rules matching these ids, prefixes, or patterns (comma or space separated)",
				Complete: completeRuleSelectors(list),
			},
		},
		Run: func(ctx Context) error {
			assessment, err := assess.Execute(application.AssessConformanceRequest{
				SkipBaseline: ctx.Bool("no-baseline"),
				Only:         splitSelectors(ctx.String("only")),
				Exclude:      splitSelectors(ctx.String("exclude")),
			})
			if err != nil {
				return ConfigError(err)
			}
			agentsContext, err := inspectAgents.Execute()
			if err != nil && !reportedAsNotEvaluated(err, assessment) {
				return ConfigError(err)
			}
			if err := render.Render(ctx.Stdout, AssessmentReport{Assessment: assessment, AgentsContext: agentsContext}); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			if !assessment.Complete() {
				return NotEvaluatedExit()
			}
			if assessment.HasErrors() {
				return ViolationsExit()
			}
			return nil
		},
	}
}

// reportedAsNotEvaluated reports whether an error is an extension that
// failed while registering, which the Assessment already names in its
// not-evaluated entries. The AGENTS.md comparison needs the same
// extension inventory, so it is skipped rather than discarding the
// report.
func reportedAsNotEvaluated(err error, assessment conformance.Assessment) bool {
	var failure conformance.ExtensionFailure
	return errors.As(err, &failure) && !assessment.Complete()
}

// splitSelectors splits one flag value into rule selectors; commas and
// whitespace both separate.
func splitSelectors(v string) []string {
	return strings.FieldsFunc(v, func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
}
