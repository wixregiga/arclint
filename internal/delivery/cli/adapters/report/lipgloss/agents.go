package lipgloss

import (
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
)

func writeAgentInstall(p *out.Printer, th Theme, r cli.AgentInstallReport) {
	for _, path := range r.Paths {
		p.Println(th.Path.Render(path))
	}
	p.Println(th.Info.Render(r.Activation))
}

func writeReviewerStatus(p *out.Printer, th Theme, r cli.ReviewerStatusReport) {
	s := r.Installation
	p.Printf("Project: %s\nAgent: %s\nHost: %s\nPath: %s\nInstalled: %t\nInstalled ArcLint release: %s\nAvailable ArcLint release: %s\nAsset integrity: %t\n", th.Path.Render(s.Project), s.Name, s.Host, th.Path.Render(s.Path), s.Installed, s.InstalledVersion, s.AvailableVersion, s.Intact)
	for _, problem := range s.Problems {
		p.Println(th.Warning.Render(problem))
	}
	p.Println(th.Muted.Render(cli.ReviewerStatusLimits))
}

func writeWorkflowStatus(p *out.Printer, th Theme, r cli.WorkflowStatusReport) {
	s := r.Status
	p.Printf("Project: %s\nHooks: %s\nCommand: %s\nRelease: %s\nInstalled: %t\nAsset integrity: %t\n", th.Path.Render(s.Project), th.Path.Render(s.HooksPath), s.Command, s.Version, s.Installed, s.Intact)
	for _, problem := range s.Problems {
		p.Println(th.Warning.Render(problem))
	}
	p.Println(th.Muted.Render(cli.WorkflowStatusLimits))
}
