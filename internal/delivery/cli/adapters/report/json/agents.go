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

type workflowStatusDoc struct {
	Project   string   `json:"project"`
	HooksPath string   `json:"hooksPath"`
	Command   string   `json:"command"`
	Version   string   `json:"version"`
	Installed bool     `json:"installed"`
	Intact    bool     `json:"intact"`
	Problems  []string `json:"problems"`
	Limits    string   `json:"limits"`
}

func workflowStatusDocOf(r cli.WorkflowStatusReport) workflowStatusDoc {
	s := r.Status
	return workflowStatusDoc{Project: s.Project, HooksPath: s.HooksPath, Command: s.Command, Version: s.Version, Installed: s.Installed, Intact: s.Intact, Problems: append([]string{}, s.Problems...), Limits: cli.WorkflowStatusLimits}
}
