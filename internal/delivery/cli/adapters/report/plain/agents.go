package plain

import (
	"fmt"
	"io"

	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
)

func writeAgentInstall(w io.Writer, r cli.AgentInstallReport) error {
	for _, path := range r.Paths {
		if _, err := fmt.Fprintln(w, path); err != nil {
			return fmt.Errorf("write agent report: %w", err)
		}
	}
	for _, path := range r.Removed {
		if _, err := fmt.Fprintf(w, "removed the duplicate hooks from %s\n", path); err != nil {
			return fmt.Errorf("write agent report: %w", err)
		}
	}
	if r.Orientation != "" {
		if _, err := fmt.Fprintln(w, r.Orientation); err != nil {
			return fmt.Errorf("write agent report: %w", err)
		}
	}
	_, err := fmt.Fprintln(w, r.Activation)
	if err != nil {
		return fmt.Errorf("write agent report: %w", err)
	}
	return nil
}

func writeHooksStatus(w io.Writer, r cli.HooksStatusReport) error {
	s := r.Status
	if _, err := fmt.Fprintf(w, "Project: %s\nBinary: %s\nLast event: %s\n", s.Project, s.Binary, out.LastHookEvent(s.LastEvent)); err != nil {
		return fmt.Errorf("write workflow status: %w", err)
	}
	for _, host := range s.Files {
		if _, err := fmt.Fprintf(w, "%s %s: %s (installed: %t)\n", host.Host, out.HookScope(host), host.Path, host.Installed); err != nil {
			return fmt.Errorf("write workflow status: %w", err)
		}
		for _, problem := range host.Problems {
			if _, err := fmt.Fprintf(w, "  %s\n", problem); err != nil {
				return fmt.Errorf("write workflow status: %w", err)
			}
		}
	}
	if _, err := fmt.Fprintln(w, cli.HooksStatusLimits); err != nil {
		return fmt.Errorf("write workflow status: %w", err)
	}
	return nil
}
