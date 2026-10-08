package cli

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/application"
)

func newAgentReviewerCommand(install application.InstallReviewer, status application.ReviewerStatus, render Renderer) Command {
	return Command{Name: "reviewer", Short: "install or inspect the separate Codex domain reviewer", Subcommands: []Command{
		{Name: agentInstallCommand, Short: "install arclint-domain-reviewer without installing hooks", Flags: []Flag{
			{Name: agentHostFlag, Options: []string{agentHostCodex}, Doc: "required agent host (codex)"},
		}, Run: func(ctx Context) error {
			paths, err := install.Execute(ctx.String(agentHostFlag))
			if err != nil {
				return ConfigError(err)
			}
			if err := render.Render(ctx.Stdout, AgentInstallReport{Operation: "reviewer", Host: ctx.String(agentHostFlag), Paths: paths, Activation: "Installed arclint-domain-reviewer. Start a new Codex session and ask it to use arclint-domain-reviewer with your request, affected paths and decisions. Installation does not prove host discovery or a successful review. Review records appear in the Codex conversation; Codex owns its session history."}); err != nil {
				return fmt.Errorf("write reviewer output: %w", err)
			}
			return nil
		}},
		{Name: "status", Short: "show reviewer release and installed asset integrity", Flags: []Flag{
			{Name: agentHostFlag, Default: agentHostCodex, Options: []string{agentHostCodex}, Doc: "agent host (codex)"},
		}, Run: func(ctx Context) error {
			value, err := status.Execute(ctx.String(agentHostFlag))
			if err != nil {
				return ConfigError(err)
			}
			if err := render.Render(ctx.Stdout, ReviewerStatusReport{Installation: value}); err != nil {
				return fmt.Errorf("write reviewer output: %w", err)
			}
			return nil
		}},
	}}
}
