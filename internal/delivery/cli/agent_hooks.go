package cli

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/application"
)

func newAgentHooksCommand(install application.InstallAgentHooks) Command {
	return Command{
		Name:  "hooks",
		Short: "install a project-local domain guard for OMP or Codex",
		Flags: []Flag{
			{Name: "host", Default: "omp", Options: []string{"omp", "codex"}, Doc: "supported agent host"},
			{Name: "domain", Repeat: true, Doc: "project-relative domain file; repeat for multiple files (default domain.arclint.yaml)"},
		},
		Run: func(ctx Context) error {
			paths, err := install.Execute(ctx.String("host"), ctx.Strings("domain"))
			if err != nil {
				return ConfigError(err)
			}
			for _, path := range paths {
				if _, err := fmt.Fprintln(ctx.Stdout, path); err != nil {
					return fmt.Errorf("write hook installation output: %w", err)
				}
			}
			message := "OMP domain guard installed. Start a new OMP session or /reload; /arclint-domain-status reports activation and review status."
			if ctx.String("host") == "codex" {
				message = "Codex domain guard installed. Start a new local Codex session and use /hooks to review and trust the five ArcLint hooks. Until trusted they do not run."
			}
			_, err = fmt.Fprintln(ctx.Stdout, message)
			if err != nil {
				return fmt.Errorf("write hook installation output: %w", err)
			}
			return nil
		},
	}
}
