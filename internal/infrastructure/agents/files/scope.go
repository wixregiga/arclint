package agentfiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

const domainSourcePatternsField = "domainSourcePatterns"

// Scope preserves unrelated settings and only changes source selection when explicitly supplied.
func Scope(root string, domains []string, selections ...application.AgentSourceScope) ([]byte, error) {
	return scope(root, domains, false, selections...)
}

// PlannedScope permits only the default recording that setup itself will create.
func PlannedScope(root string, domains []string, selections ...application.AgentSourceScope) ([]byte, error) {
	return scope(root, domains, true, selections...)
}

func scope(root string, domains []string, planned bool, selections ...application.AgentSourceScope) ([]byte, error) {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("project root: %w", err)
	}
	for _, name := range domains {
		if !filepath.IsLocal(name) || strings.ContainsAny(name, "\\:") {
			return nil, fmt.Errorf("invalid domain path: %s", name)
		}
		for _, part := range strings.Split(name, "/") {
			if part == ".." {
				return nil, fmt.Errorf("invalid domain traversal: %s", name)
			}
		}
		full, err := filepath.EvalSymlinks(filepath.Join(root, name))
		if err != nil {
			if planned && len(domains) == 1 && name == vocab.UbiquitousLanguageFileName && os.IsNotExist(err) {
				if _, statErr := os.Lstat(filepath.Join(root, name)); os.IsNotExist(statErr) {
					continue
				}
			}
			return nil, fmt.Errorf("domain file %s: %w", name, err)
		}
		rel, err := filepath.Rel(root, full)
		if err != nil || filepath.IsAbs(name) || !filepath.IsLocal(rel) {
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
		for field, patterns := range map[string][]string{"sourcePatterns": selection.Sources, domainSourcePatternsField: selection.DomainSources} {
			if len(patterns) == 0 {
				continue
			}
			for _, pattern := range patterns {
				if err := sourcePattern(root, pattern, field == domainSourcePatternsField); err != nil {
					return nil, fmt.Errorf("agent setup: %w", err)
				}
			}
			config[field] = patterns
			changed = true
		}
	}
	for _, field := range []string{"sourcePatterns", domainSourcePatternsField} {
		if raw, found := config[field]; found {
			encoded, err := json.Marshal(raw)
			var patterns []string
			if err != nil || json.Unmarshal(encoded, &patterns) != nil || patterns == nil {
				return nil, fmt.Errorf("%s must be a list of patterns", field)
			}
			for _, pattern := range patterns {
				if err := sourcePattern(root, pattern, field == domainSourcePatternsField); err != nil {
					return nil, err
				}
			}
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
	if pattern == "" || filepath.IsAbs(pattern) || strings.ContainsAny(pattern, "[]\\:") || strings.ContainsAny(parts[0], "*?") {
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
	if err != nil || !filepath.IsLocal(rel) {
		return fmt.Errorf("source scope escapes project: %s", pattern)
	}
	glob, err := rule.NewGlob(pattern)
	if err != nil {
		return fmt.Errorf("source pattern: %w", err)
	}
	projectRoot, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("open source project root: %w", err)
	}
	selected := 0
	prefixName := strings.Join(prefix, "/")
	err = filepath.WalkDir(path, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk source selection: %w", walkErr)
		}
		suffix, err := filepath.Rel(path, current)
		if err != nil {
			return fmt.Errorf("source selection relative path: %w", err)
		}
		name := prefixName
		if suffix != "." {
			name += "/" + filepath.ToSlash(suffix)
		}
		if entry.IsDir() || !glob.Match(name) {
			return nil
		}
		resolved, err := filepath.EvalSymlinks(current)
		if err != nil {
			return fmt.Errorf("resolve selected source: %w", err)
		}
		relative, err := filepath.Rel(root, resolved)
		if err != nil || !filepath.IsLocal(relative) {
			return fmt.Errorf("selected source escapes project: %s", name)
		}
		file, err := projectRoot.Open(relative)
		if err != nil {
			return fmt.Errorf("open selected source within project: %w", err)
		}
		info, statErr := file.Stat()
		closeErr := file.Close()
		if statErr != nil {
			return fmt.Errorf("inspect selected source: %w", statErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close selected source: %w", closeErr)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if domain && !strings.HasSuffix(name, ".go") {
			return fmt.Errorf("domain source comment checks support .go only: %s", name)
		}

		selected++
		return nil
	})
	if closeErr := projectRoot.Close(); closeErr != nil {
		return fmt.Errorf("close source project root: %w", closeErr)
	}
	if err != nil {
		return fmt.Errorf("source scope: %w", err)
	}
	if selected == 0 {
		return fmt.Errorf("source scope has no files: %s", pattern)
	}
	return nil
}
