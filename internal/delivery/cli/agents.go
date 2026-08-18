package cli

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/application"
)

// NewAgentsCommand adapts the agents-context use case: print the
// generated architecture block, or install it into AGENTS.md.
func NewAgentsCommand(publish application.PublishAgentsContext) Command {
	return Command{
		Name:  "agents",
		Short: "compile the ruleset into a generated AGENTS.md architecture block",
		Flags: []Flag{
			{Name: "write", Bool: true, Doc: "install or refresh the block in <repo-root>/AGENTS.md"},
		},
		Run: func(ctx Context) error {
			if !ctx.Bool("write") {
				block, err := publish.Render()
				if err != nil {
					return ConfigError(err)
				}
				if _, err := fmt.Fprint(ctx.Stdout, block); err != nil {
					return fmt.Errorf("write output: %w", err)
				}
				return nil
			}
			changed, path, err := publish.Execute()
			if err != nil {
				return ConfigError(err)
			}
			if changed {
				if _, err := fmt.Fprintf(ctx.Stdout, "wrote %s\n", path); err != nil {
					return fmt.Errorf("write output: %w", err)
				}
			} else {
				if _, err := fmt.Fprintf(ctx.Stdout, "%s already current\n", path); err != nil {
					return fmt.Errorf("write output: %w", err)
				}
			}
			return nil
		},
	}
}
