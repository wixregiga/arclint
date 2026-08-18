package cli

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/application"
)

// NewBaselineCommand adapts the Baseline lifecycle use cases into the
// baseline command group: capture adopts current findings, refresh
// replaces the snapshot after comparison with a later assessment.
func NewBaselineCommand(capture application.CaptureBaseline, refresh application.RefreshBaseline) Command {
	return Command{
		Name:  "baseline",
		Short: "manage the committed baseline of adopted findings",
		Subcommands: []Command{
			{
				Name:  "capture",
				Short: "capture the committed baseline from one complete assessment",
				Run: func(ctx Context) error {
					result, err := capture.Execute()
					if err != nil {
						return ConfigError(err)
					}
					if _, err := fmt.Fprintf(ctx.Stdout, "baseline captured: %d finding(s) across %d applied rule(s)\n",
						result.Findings, result.Rules); err != nil {
						return fmt.Errorf("write output: %w", err)
					}
					return nil
				},
			},
			{
				Name:  "refresh",
				Short: "replace the baseline after comparison, dropping stale entries",
				Run: func(ctx Context) error {
					result, err := refresh.Execute()
					if err != nil {
						return ConfigError(err)
					}
					if _, err := fmt.Fprintf(ctx.Stdout, "baseline refreshed: %d finding(s) across %d applied rule(s), %d stale entr(ies) dropped\n",
						result.Findings, result.Rules, result.RemovedStale); err != nil {
						return fmt.Errorf("write output: %w", err)
					}
					return nil
				},
			},
		},
	}
}
