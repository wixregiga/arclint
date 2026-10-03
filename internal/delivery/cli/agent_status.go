package cli

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/application"
)

func newAgentStatusCommand(status application.AgentSetupStatus, render Renderer) Command {
	return Command{Name: "status", Short: "show installed scope and assets without granting or inferring host approval", Run: func(ctx Context) error {
		value, err := status.Execute()
		if err != nil {
			return ConfigError(err)
		}
		if err := render.Render(ctx.Stdout, AgentStatusReport{Installation: value}); err != nil {
			return fmt.Errorf("write agent setup output: %w", err)
		}
		return nil
	}}
}
