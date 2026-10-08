package plain

import (
	"fmt"
	"io"

	"github.com/wixregiga/arclint/internal/delivery/cli"
)

func writeAgentInstall(w io.Writer, r cli.AgentInstallReport) error {
	for _, path := range r.Paths {
		if _, err := fmt.Fprintln(w, path); err != nil {
			return fmt.Errorf("write agent report: %w", err)
		}
	}
	_, err := fmt.Fprintln(w, r.Activation)
	if err != nil {
		return fmt.Errorf("write agent report: %w", err)
	}
	return nil
}

func writeReviewerStatus(w io.Writer, r cli.ReviewerStatusReport) error {
	s := r.Installation
	if _, err := fmt.Fprintf(w, "Project: %s\nAgent: %s\nHost: %s\nPath: %s\nInstalled: %t\nInstalled ArcLint release: %s\nAvailable ArcLint release: %s\nAsset integrity: %t\n", s.Project, s.Name, s.Host, s.Path, s.Installed, s.InstalledVersion, s.AvailableVersion, s.Intact); err != nil {
		return fmt.Errorf("write agent report: %w", err)
	}
	for _, problem := range s.Problems {
		if _, err := fmt.Fprintln(w, problem); err != nil {
			return fmt.Errorf("write agent report: %w", err)
		}
	}
	_, err := fmt.Fprintln(w, cli.ReviewerStatusLimits)
	if err != nil {
		return fmt.Errorf("write agent report: %w", err)
	}
	return nil
}

func writeWorkflowStatus(w io.Writer, r cli.WorkflowStatusReport) error {
	s := r.Status
	if _, err := fmt.Fprintf(w, "Project: %s\nCommand: %s\n", s.Project, s.Command); err != nil {
		return fmt.Errorf("write workflow status: %w", err)
	}
	for _, host := range s.Hosts {
		if _, err := fmt.Fprintf(w, "%s: %s (installed: %t)\n", host.Host, host.Path, host.Installed); err != nil {
			return fmt.Errorf("write workflow status: %w", err)
		}
		for _, problem := range host.Problems {
			if _, err := fmt.Fprintf(w, "  %s\n", problem); err != nil {
				return fmt.Errorf("write workflow status: %w", err)
			}
		}
	}
	if _, err := fmt.Fprintln(w, cli.WorkflowStatusLimits); err != nil {
		return fmt.Errorf("write workflow status: %w", err)
	}
	return nil
}
