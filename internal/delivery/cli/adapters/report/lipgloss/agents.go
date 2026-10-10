package lipgloss

import (
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
)

func writeAgentInstall(p *out.Printer, th Theme, r cli.AgentInstallReport) {
	for _, path := range r.Paths {
		p.Println(th.Path.Render(path))
	}
	for _, path := range r.Removed {
		p.Println("removed the duplicate hooks from " + th.Path.Render(path))
	}
	if r.Orientation != "" {
		p.Println(r.Orientation)
	}
	p.Println(th.Info.Render(r.Activation))
}

func writeHooksStatus(p *out.Printer, th Theme, r cli.HooksStatusReport) {
	s := r.Status
	p.Printf("Project: %s\nBinary: %s\nLast event: %s\n", th.Path.Render(s.Project), th.Path.Render(s.Binary), out.LastHookEvent(s.LastEvent))
	for _, host := range s.Files {
		p.Printf("%s %s: %s (installed: %t)\n", host.Host, out.HookScope(host), th.Path.Render(host.Path), host.Installed)
		for _, problem := range host.Problems {
			p.Println("  " + th.Warning.Render(problem))
		}
	}
	p.Println(th.Muted.Render(cli.HooksStatusLimits))
}
