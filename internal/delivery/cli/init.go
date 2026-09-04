package cli

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
)

// NewInitCommand adapts the repository-initialization use case: draft
// a commented starter ruleset from explicit choices.
func NewInitCommand(initialize application.InitializeRepository, render Renderer) Command {
	return Command{
		Name:  commandInit,
		Short: "draft a starter rules.yaml for this repository",
		Flags: []Flag{
			{Name: "languages", Default: "go", Doc: "comma-separated runtime targets: go, ts, py", Complete: completeLanguages},
			{Name: "pattern", Default: "bare", Doc: "starter ruleset: bare, or a Pattern to extend by reference or name (arclint patterns lists them)", Complete: completePatterns(initialize)},
			{Name: "force", Bool: true, Doc: "overwrite an existing rules.yaml"},
		},
		Run: func(ctx Context) error {
			var languages []string
			for _, l := range strings.Split(ctx.String("languages"), ",") {
				if l = strings.TrimSpace(l); l != "" {
					languages = append(languages, l)
				}
			}
			path, err := initialize.Execute(application.InitializeRepositoryRequest{
				Languages: languages,
				Pattern:   ctx.String("pattern"),
				Force:     ctx.Bool("force"),
			})
			if err != nil {
				return ConfigError(err)
			}
			if err := render.Render(ctx.Stdout, InitReport{Path: path}); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}
}

func completeLanguages(toComplete string) []AutoCompleteCandidate {
	languages := application.SupportedLanguages()
	candidates := make([]AutoCompleteCandidate, 0, len(languages))
	for _, language := range languages {
		candidates = append(candidates, AutoCompleteCandidate{Value: language})
	}
	return withListPrefix(toComplete, candidates)
}

func completePatterns(initialize application.InitializeRepository) func(toComplete string) []AutoCompleteCandidate {
	return func(_ string) []AutoCompleteCandidate {
		// Completion degrades to no candidates when a Pattern source
		// fails: the shell callback has nowhere to report the error.
		names, err := initialize.Patterns()
		if err != nil {
			return nil
		}
		candidates := make([]AutoCompleteCandidate, 0, len(names))
		for _, name := range names {
			candidates = append(candidates, AutoCompleteCandidate{Value: name})
		}
		return candidates
	}
}
