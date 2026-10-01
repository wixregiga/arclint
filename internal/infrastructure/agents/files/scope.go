package agentfiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
)

// Scope preserves unrelated settings and only changes source selection when explicitly supplied.
func Scope(root string, domains []string, selections ...application.AgentSourceScope) ([]byte, error) {
	for _, name := range domains {
		full, err := filepath.EvalSymlinks(filepath.Join(root, name))
		if err != nil {
			return nil, fmt.Errorf("domain file %s: %w", name, err)
		}
		rel, err := filepath.Rel(root, full)
		if err != nil || filepath.IsAbs(name) || rel == ".." || strings.HasPrefix(rel, "../") {
			return nil, fmt.Errorf("domain path escapes project: %s", name)
		}
		info, err := os.Stat(full)
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("domain file is not regular: %s", name)
		}
	}
	config := map[string]any{"version": 1, "domainFiles": domains}
	target := filepath.Join(root, ".arclint/domain-guard.json")
	before, err := os.ReadFile(target)
	if err == nil {
		var saved struct {
			Version     int
			DomainFiles []string
		}
		if json.Unmarshal(before, &saved) != nil || saved.Version != 1 || strings.Join(saved.DomainFiles, "\x00") != strings.Join(domains, "\x00") {
			return nil, fmt.Errorf("preserving different domain scope in %s", target)
		}
		if err := json.Unmarshal(before, &config); err != nil {
			return nil, fmt.Errorf("agent setup: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("agent setup: %w", err)
	}
	changed := false
	for _, selection := range selections {
		for field, patterns := range map[string][]string{"sourcePatterns": selection.Sources, "domainSourcePatterns": selection.DomainSources} {
			if len(patterns) == 0 {
				continue
			}
			for _, pattern := range patterns {
				if err := sourcePattern(root, pattern, field == "domainSourcePatterns"); err != nil {
					return nil, fmt.Errorf("agent setup: %w", err)
				}
			}
			config[field] = patterns
			changed = true
		}
	}
	if before != nil && !changed {
		return before, nil
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode scope: %w", err)
	}
	return append(data, '\n'), nil
}

func sourcePattern(root, pattern string, domain bool) error {
	parts := strings.Split(pattern, "/")
	if pattern == "" || filepath.IsAbs(pattern) || strings.ContainsAny(pattern, "[]\\") || strings.ContainsAny(parts[0], "*?") {
		return fmt.Errorf("source pattern needs a project-relative literal prefix: %q", pattern)
	}
	for _, part := range parts {
		if part == ".." || part == "" {
			return fmt.Errorf("invalid source pattern: %q", pattern)
		}
	}
	switch parts[0] {
	case ".git", ".arclint", ".omp", ".codex":
		return fmt.Errorf("source pattern selects guard metadata: %q", pattern)
	}
	if domain && !strings.HasSuffix(pattern, ".go") {
		return fmt.Errorf("domain source comment checks support .go only: %s", pattern)
	}
	prefix := []string{}
	for _, part := range parts {
		if strings.ContainsAny(part, "*?") {
			break
		}
		prefix = append(prefix, part)
	}
	path, err := filepath.EvalSymlinks(filepath.Join(append([]string{root}, prefix...)...))
	if err != nil {
		return fmt.Errorf("source scope: %w", err)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return fmt.Errorf("source scope escapes project: %s", pattern)
	}
	return nil
}
