package plain

import (
	"fmt"
	"io"
	"strings"

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

func writeAgentStatus(w io.Writer, r cli.AgentStatusReport) error {
	s := r.Installation
	if s.LegacyRules {
		if _, err := fmt.Fprintln(w, "Legacy rules preserved: this build needs rules.arclint.yaml for structural conformance; no automatic migration or replacement was performed."); err != nil {
			return fmt.Errorf("write agent report: %w", err)
		}
	}
	if _, err := fmt.Fprintf(w, "Project: %s\nRecordings: %s\nDomain source subjects: %s\nSupporting evidence: %s\n", s.Project, strings.Join(s.DomainFiles, ", "), strings.Join(s.DomainSourcePatterns, ", "), strings.Join(s.SourcePatterns, ", ")); err != nil {
		return fmt.Errorf("write agent report: %w", err)
	}
	for _, host := range s.InstalledHosts {
		if _, err := fmt.Fprintf(w, "%s: installed; activation/trust must be checked in the host\n", host); err != nil {
			return fmt.Errorf("write agent report: %w", err)
		}
	}
	for _, name := range s.ChangedAssets {
		if _, err := fmt.Fprintf(w, "Changed or missing installed asset: %s\n", name); err != nil {
			return fmt.Errorf("write agent report: %w", err)
		}
	}
	for _, problem := range s.Problems {
		if _, err := fmt.Fprintln(w, problem); err != nil {
			return fmt.Errorf("write agent report: %w", err)
		}
	}
	_, err := fmt.Fprintln(w, cli.AgentStatusLimits)
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
