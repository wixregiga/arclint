package agentfiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Status describes installed artifacts; only the host can report current activation and trust.
func (w Writer) Status() (string, error) {
	content, err := os.ReadFile(filepath.Join(w.Root, ".arclint/domain-guard.json"))
	if err != nil {
		return "", fmt.Errorf("agent setup not readable: %w", err)
	}
	var config struct{ DomainFiles, DomainSourcePatterns, SourcePatterns []string }
	if err := json.Unmarshal(content, &config); err != nil {
		return "", fmt.Errorf("agent status: %w", err)
	}
	var out strings.Builder
	if _, err := os.Stat(filepath.Join(w.Root, "rules.arclint.yaml")); os.IsNotExist(err) {
		if _, err := os.Stat(filepath.Join(w.Root, ".arclint/rules.yaml")); err == nil {
			out.WriteString("Legacy rules preserved: this build needs rules.arclint.yaml for structural conformance; no automatic migration or replacement was performed.\n")
		}
	}
	fmt.Fprintf(&out, "Project: %s\nRecordings: %s\nDomain source subjects: %s\nSupporting evidence: %s\n", w.Root, strings.Join(config.DomainFiles, ", "), strings.Join(config.DomainSourcePatterns, ", "), strings.Join(config.SourcePatterns, ", "))
	for host, entry := range map[string]string{"OMP": ".omp/extensions/arclint-domain-guard/index.js", "Codex": ".codex/hooks.json"} {
		_, err := os.Stat(filepath.Join(w.Root, entry))
		if err == nil {
			fmt.Fprintf(&out, "%s: installed; activation/trust must be checked in the host\n", host)
		}
	}
	receiptBytes, err := os.ReadFile(filepath.Join(w.Root, ".arclint/agent-assets.json"))
	if err != nil {
		return "", fmt.Errorf("asset receipt unavailable; integrity not verified: %w", err)
	}
	receipt := map[string]string{}
	if err := json.Unmarshal(receiptBytes, &receipt); err != nil {
		return "", fmt.Errorf("agent status: %w", err)
	}
	for name, expected := range receipt {
		content, err := os.ReadFile(filepath.Join(w.Root, name))
		if err != nil || digest(content) != expected {
			fmt.Fprintf(&out, "Changed or missing installed asset: %s\n", name)
		}
	}
	out.WriteString("Installation status is not a review verdict. Codex: review /hooks in CLI or Settings > Coding > Hooks in desktop. OMP: /arclint-domain-status.\n")
	return out.String(), nil
}
