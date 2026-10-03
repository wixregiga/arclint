package jsonreport

import "github.com/wixregiga/arclint/internal/delivery/cli"

type agentInstallDoc struct {
	Operation  string   `json:"operation"`
	Host       string   `json:"host"`
	Paths      []string `json:"paths"`
	Activation string   `json:"activation"`
}

func agentInstallDocOf(r cli.AgentInstallReport) agentInstallDoc {
	return agentInstallDoc{Operation: r.Operation, Host: r.Host, Paths: append([]string{}, r.Paths...), Activation: r.Activation}
}

type agentStatusDoc struct {
	Project              string   `json:"project"`
	DomainFiles          []string `json:"domainFiles"`
	DomainSourcePatterns []string `json:"domainSourcePatterns"`
	SourcePatterns       []string `json:"sourcePatterns"`
	InstalledHosts       []string `json:"installedHosts"`
	ChangedAssets        []string `json:"changedAssets"`
	Problems             []string `json:"problems"`
	LegacyRules          bool     `json:"legacyRules"`
	Intact               bool     `json:"intact"`
	Limits               string   `json:"limits"`
}

func agentStatusDocOf(r cli.AgentStatusReport) agentStatusDoc {
	s := r.Installation
	return agentStatusDoc{Project: s.Project, DomainFiles: append([]string{}, s.DomainFiles...), DomainSourcePatterns: append([]string{}, s.DomainSourcePatterns...), SourcePatterns: append([]string{}, s.SourcePatterns...), InstalledHosts: append([]string{}, s.InstalledHosts...), ChangedAssets: append([]string{}, s.ChangedAssets...), Problems: append([]string{}, s.Problems...), LegacyRules: s.LegacyRules, Intact: s.Intact, Limits: cli.AgentStatusLimits}
}

type reviewerStatusDoc struct {
	Project          string   `json:"project"`
	Name             string   `json:"name"`
	Host             string   `json:"host"`
	Path             string   `json:"path"`
	InstalledVersion string   `json:"installedVersion"`
	AvailableVersion string   `json:"availableVersion"`
	Installed        bool     `json:"installed"`
	Intact           bool     `json:"intact"`
	Problems         []string `json:"problems"`
	Limits           string   `json:"limits"`
}

func reviewerStatusDocOf(r cli.ReviewerStatusReport) reviewerStatusDoc {
	s := r.Installation
	return reviewerStatusDoc{Project: s.Project, Name: s.Name, Host: s.Host, Path: s.Path, InstalledVersion: s.InstalledVersion, AvailableVersion: s.AvailableVersion, Installed: s.Installed, Intact: s.Intact, Problems: append([]string{}, s.Problems...), Limits: cli.ReviewerStatusLimits}
}
