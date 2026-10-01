package cli

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/application"
)

func newAgentStatusCommand(status application.AgentSetupStatus) Command {
	return Command{Name: "status", Short: "show installed scope and assets without granting or inferring host approval", Run: func(ctx Context) error {
		value, err := status.Execute()
		if err != nil {
			return ConfigError(err)
		}
		_, err = fmt.Fprintln(ctx.Stdout, value)
		if err != nil {
			return fmt.Errorf("write agent setup output: %w", err)
		}
		return nil
	}}
}
