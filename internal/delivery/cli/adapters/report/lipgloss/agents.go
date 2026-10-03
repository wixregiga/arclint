package lipgloss

import (
	"strings"

	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
)

func writeAgentInstall(p *out.Printer, th Theme, r cli.AgentInstallReport) {
	for _, path := range r.Paths {
		p.Println(th.Path.Render(path))
	}
	p.Println(th.Info.Render(r.Activation))
}

func writeAgentStatus(p *out.Printer, th Theme, r cli.AgentStatusReport) {
	s := r.Installation
	if s.LegacyRules {
		p.Println(th.Warning.Render("Legacy rules preserved: this build needs rules.arclint.yaml for structural conformance; no automatic migration or replacement was performed."))
	}
	p.Printf("Project: %s\nRecordings: %s\nDomain source subjects: %s\nSupporting evidence: %s\n", th.Path.Render(s.Project), strings.Join(s.DomainFiles, ", "), strings.Join(s.DomainSourcePatterns, ", "), strings.Join(s.SourcePatterns, ", "))
	for _, host := range s.InstalledHosts {
		p.Printf("%s: installed; activation/trust must be checked in the host\n", host)
	}
	for _, name := range s.ChangedAssets {
		p.Printf("Changed or missing installed asset: %s\n", th.Path.Render(name))
	}
	for _, problem := range s.Problems {
		p.Println(th.Warning.Render(problem))
	}
	p.Println(th.Muted.Render(cli.AgentStatusLimits))
}

func writeReviewerStatus(p *out.Printer, th Theme, r cli.ReviewerStatusReport) {
	s := r.Installation
	p.Printf("Project: %s\nAgent: %s\nHost: %s\nPath: %s\nInstalled: %t\nInstalled ArcLint release: %s\nAvailable ArcLint release: %s\nAsset integrity: %t\n", th.Path.Render(s.Project), s.Name, s.Host, th.Path.Render(s.Path), s.Installed, s.InstalledVersion, s.AvailableVersion, s.Intact)
	for _, problem := range s.Problems {
		p.Println(th.Warning.Render(problem))
	}
	p.Println(th.Muted.Render(cli.ReviewerStatusLimits))
}
