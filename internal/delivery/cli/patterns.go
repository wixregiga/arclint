package cli

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/application"
)

// NewPatternsCommand adapts the Pattern listing use case into the
// patterns command.
func NewPatternsCommand(list application.ListPatterns, render Renderer) Command {
	return Command{
		Name:  "patterns",
		Short: "list available Pattern distribution packages",
		Run: func(ctx Context) error {
			rows, err := list.Execute()
			if err != nil {
				return ConfigError(err)
			}
			if err := render.Render(ctx.Stdout, PatternsReport{Patterns: rows}); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}
}
