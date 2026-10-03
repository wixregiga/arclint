package cli

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/application"
)

func newAgentReviewerCommand(install application.InstallReviewer, status application.ReviewerStatus) Command {
	return Command{Name: "reviewer", Short: "install or inspect the separate Codex domain reviewer", Subcommands: []Command{
		{Name: "install", Short: "install arclint-domain-reviewer without installing hooks", Flags: []Flag{
			{Name: agentHostFlag, Options: []string{agentHostCodex}, Doc: "required agent host (codex)"},
		}, Run: func(ctx Context) error {
			paths, err := install.Execute(ctx.String(agentHostFlag))
			if err != nil {
				return ConfigError(err)
			}
			for _, path := range paths {
				if _, err := fmt.Fprintln(ctx.Stdout, path); err != nil {
					return fmt.Errorf("write reviewer output: %w", err)
				}
			}
			_, err = fmt.Fprintln(ctx.Stdout, "Installed arclint-domain-reviewer. Start a new Codex session and ask it to use arclint-domain-reviewer with your request, affected paths and decisions. Installation does not prove host discovery or a successful review. Review records appear in the Codex conversation; Codex owns its session history.")
			if err != nil {
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
			_, err = fmt.Fprintf(ctx.Stdout, "Project: %s\nAgent: %s\nHost: %s\nPath: %s\nInstalled: %t\nInstalled ArcLint release: %s\nAvailable ArcLint release: %s\nAsset integrity: %t\n", value.Project, value.Name, value.Host, value.Path, value.Installed, value.InstalledVersion, value.AvailableVersion, value.Intact)
			if err != nil {
				return fmt.Errorf("write reviewer output: %w", err)
			}
			for _, problem := range value.Problems {
				if _, err := fmt.Fprintln(ctx.Stdout, problem); err != nil {
					return fmt.Errorf("write reviewer output: %w", err)
				}
			}
			_, err = fmt.Fprintln(ctx.Stdout, "Installation and integrity do not prove Codex discovery, activation or review quality. Activity records are in the Codex conversation; ArcLint does not manage their storage or retention.")
			if err != nil {
				return fmt.Errorf("write reviewer output: %w", err)
			}
			return nil
		}},
	}}
}
