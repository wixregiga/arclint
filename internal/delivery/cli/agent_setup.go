package cli

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
)

const (
	agentHostFlag   = "host"
	agentHostCodex  = "codex"
	agentHostOMP    = "omp"
	agentDomainFlag = "domain"
)

func newAgentSetupCommand(setup application.SetupAgent, render Renderer) Command {
	return Command{
		Name:  "setup",
		Short: "prepare rules, domain guidance, skill and native hooks for one chosen host",
		Flags: []Flag{
			{Name: agentHostFlag, Options: []string{agentHostOMP, agentHostCodex}, Doc: "required agent host; installation does not grant trust"},
			{Name: agentDomainFlag, Repeat: true, Doc: "existing project-relative recording; repeat for several (default domain.arclint.yaml)"},
			{Name: "domain-source", Repeat: true, Doc: "Go domain-source path/glob subject to the comment policy; repeat for several"},
			{Name: "source", Repeat: true, Doc: "supporting source path/glob; comments remain allowed"},
			{Name: "languages", Doc: "runtime targets for a new ruleset only (default go)"},
			{Name: "pattern", Doc: "Pattern for a new ruleset only (default bare)"},
		},
		Run: func(ctx Context) error {
			var languages []string
			if value := ctx.String("languages"); value != "" {
				for _, value := range strings.Split(value, ",") {
					languages = append(languages, strings.TrimSpace(value))
				}
			}
			paths, err := setup.Execute(application.SetupAgentRequest{Host: ctx.String(agentHostFlag), DomainFiles: ctx.Strings(agentDomainFlag), DomainSources: ctx.Strings("domain-source"), Sources: ctx.Strings("source"), Languages: languages, Pattern: ctx.String("pattern")})
			if err != nil {
				return ConfigError(err)
			}
			if err := render.Render(ctx.Stdout, AgentInstallReport{Operation: "setup", Host: ctx.String(agentHostFlag), Paths: paths, Activation: agentActivation(ctx.String(agentHostFlag))}); err != nil {
				return fmt.Errorf("write agent setup output: %w", err)
			}
			return nil
		},
	}
}

func agentActivation(host string) string {
	if host == agentHostCodex {
		return "Codex setup installed, not trusted. Open a new local Codex session, review project trust and the five definitions in /hooks (CLI) or Settings > Coding > Hooks (desktop), then start a new session. Until trusted they do not run. Use arclint agents status to inspect installed scope; host trust is shown by Codex."
	}
	return "OMP setup installed. Start a new OMP session or /reload. /arclint-domain-status reports loaded scope and review status. Use arclint agents status to inspect installed files."
}
