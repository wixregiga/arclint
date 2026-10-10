package jsonreport

import (
	"time"

	"github.com/wixregiga/arclint/internal/delivery/cli"
)

type agentInstallDoc struct {
	Operation   string   `json:"operation"`
	Host        string   `json:"host"`
	Paths       []string `json:"paths"`
	Removed     []string `json:"removed"`
	Orientation string   `json:"orientation,omitempty"`
	Activation  string   `json:"activation"`
}

func agentInstallDocOf(r cli.AgentInstallReport) agentInstallDoc {
	return agentInstallDoc{Operation: r.Operation, Host: r.Host, Paths: append([]string{}, r.Paths...), Removed: append([]string{}, r.Removed...), Orientation: r.Orientation, Activation: r.Activation}
}

type hookFileDoc struct {
	Host      string   `json:"host"`
	Scope     string   `json:"scope"`
	Windows   bool     `json:"windows"`
	Path      string   `json:"path"`
	Installed bool     `json:"installed"`
	Problems  []string `json:"problems"`
}

type hooksStatusDoc struct {
	Project string        `json:"project"`
	Binary  string        `json:"binary"`
	Files   []hookFileDoc `json:"files"`
	// LastEvent is RFC 3339, empty when no hook event reached the project.
	LastEvent string `json:"lastEvent"`
	Limits    string `json:"limits"`
}

func hooksStatusDocOf(r cli.HooksStatusReport) hooksStatusDoc {
	s := r.Status
	files := make([]hookFileDoc, 0, len(s.Files))
	for _, host := range s.Files {
		files = append(files, hookFileDoc{Host: host.Host, Scope: host.Scope, Windows: host.Windows, Path: host.Path, Installed: host.Installed, Problems: append([]string{}, host.Problems...)})
	}
	last := ""
	if !s.LastEvent.IsZero() {
		last = s.LastEvent.Format(time.RFC3339)
	}
	return hooksStatusDoc{Project: s.Project, Binary: s.Binary, Files: files, LastEvent: last, Limits: cli.HooksStatusLimits}
}
