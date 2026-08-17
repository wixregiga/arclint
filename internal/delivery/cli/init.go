package cli

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
)

// NewInitCommand adapts the repository-initialization use case: draft
// a commented starter ruleset from explicit choices.
func NewInitCommand(initialize application.InitializeRepository) Command {
	return Command{
		Name:  "init",
		Short: "draft a starter rules.yaml for this repository",
		Flags: []Flag{
			{Name: "languages", Default: "go", Doc: "comma-separated runtime targets: go, ts, py"},
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
				Force:     ctx.Bool("force"),
			})
			if err != nil {
				return ConfigError(err)
			}
			if _, err := fmt.Fprintf(ctx.Stdout, "wrote %s\nnext: declare your modules, then run `arclint check .`\n", path); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}
}
