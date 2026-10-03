package cli

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/application"
)

func newAgentHooksCommand(install application.InstallAgentHooks, render Renderer) Command {
	return Command{
		Name:  "hooks",
		Short: "install a project-local domain guard for OMP or Codex",
		Flags: []Flag{
			{Name: agentHostFlag, Default: agentHostOMP, Options: []string{agentHostOMP, agentHostCodex}, Doc: "supported agent host"},
			{Name: "domain-source", Repeat: true, Doc: "Go domain-source subject path/glob; repeat for several"},
			{Name: "source", Repeat: true, Doc: "supporting source evidence path/glob"},
			{Name: agentDomainFlag, Repeat: true, Doc: "project-relative domain file; repeat for multiple files (default domain.arclint.yaml)"},
		},
		Run: func(ctx Context) error {
			paths, err := install.Execute(ctx.String(agentHostFlag), ctx.Strings(agentDomainFlag), application.AgentSourceScope{DomainSources: ctx.Strings("domain-source"), Sources: ctx.Strings("source")})
			if err != nil {
				return ConfigError(err)
			}
			if err := render.Render(ctx.Stdout, AgentInstallReport{Operation: "hooks", Host: ctx.String(agentHostFlag), Paths: paths, Activation: agentActivation(ctx.String(agentHostFlag))}); err != nil {
				return fmt.Errorf("write hook installation output: %w", err)
			}
			return nil
		},
	}
}
