package cli

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
)

// NewContextCommand adapts the architectural-context use case: with no
// scope it explains the repository — every Module, the Rule kinds in
// use, the enforcement posture; with paths or named Modules it is the
// worksite call, answering in one payload what governs the given set.
func NewContextCommand(context application.GetArchitecturalContext, render Renderer) Command {
	return Command{
		Name:    "context",
		Short:   "explain the architecture: the repository, or everything binding the given paths",
		MaxArgs: -1,
		Flags: []Flag{
			{
				Name:     "module",
				Doc:      "declared modules to include in the scope (comma or space separated)",
				Complete: completeModuleNames(context),
			},
		},
		Run: func(ctx Context) error {
			paths := make([]string, 0, len(ctx.Args))
			for _, a := range ctx.Args {
				paths = append(paths, strings.TrimPrefix(a, "./"))
			}
			result, err := context.Execute(application.ContextRequest{
				Paths:   paths,
				Modules: splitSelectors(ctx.String("module")),
			})
			if err != nil {
				return ConfigError(err)
			}
			if err := render.Render(ctx.Stdout, ContextReport{Context: result}); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}
}

// completeModuleNames completes declared Module names for the context
// --module flag, keeping typed comma segments as the inserted prefix.
// Per the completion contract a failing ruleset yields no candidates.
func completeModuleNames(context application.GetArchitecturalContext) func(toComplete string) []AutoCompleteCandidate {
	return func(toComplete string) []AutoCompleteCandidate {
		result, err := context.Execute(application.ContextRequest{})
		if err != nil {
			return nil
		}
		candidates := make([]AutoCompleteCandidate, 0, len(result.Modules))
		for _, m := range result.Modules {
			candidates = append(candidates, AutoCompleteCandidate{Value: m.Name, Doc: m.Description})
		}
		return withListPrefix(toComplete, candidates)
	}
}
