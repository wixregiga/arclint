// Package filesystemscaffold persists drafted repository rulesets. It
// refuses to overwrite an existing ruleset unless forced: adopting
// ArcLint must never silently destroy configured policy.
package filesystemscaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// Writer implements the application's RulesetScaffold port over one
// directory.
type Writer struct {
	dir string
}

// NewWriter binds the writer to the directory the ruleset lands in.
func NewWriter(dir string) (Writer, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Writer{}, fmt.Errorf("scaffold dir: %w", err)
	}
	return Writer{dir: abs}, nil
}

type writeTarget struct {
	path string
	data []byte
}

// Write persists rules.yaml and any Pattern Extension entries, refusing
// to overwrite existing targets unless forced. Extension files are
// written before the ruleset so a new repository never references
// uninstalled entries.
func (w Writer) Write(content string, extensions []rule.Extension, force bool) (string, error) {
	exts := append([]rule.Extension(nil), extensions...)
	sort.Slice(exts, func(i, j int) bool {
		return exts[i].FileName() < exts[j].FileName()
	})
	targets := make([]writeTarget, 0, len(exts)+1)
	for _, e := range exts {
		targets = append(targets, writeTarget{
			path: filepath.Join(w.dir, ".arclint", "extensions", e.FileName()),
			data: []byte(e.Source()),
		})
	}
	rulesTarget := filepath.Join(w.dir, "rules.yaml")
	targets = append(targets, writeTarget{path: rulesTarget, data: []byte(content)})

	if !force {
		for _, t := range targets {
			if _, err := os.Stat(t.path); err == nil {
				return "", fmt.Errorf("%s already exists; pass --force to overwrite", t.path)
			}
		}
	}
	if len(exts) > 0 {
		dir := filepath.Join(w.dir, ".arclint", "extensions")
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return "", fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	for _, t := range targets {
		if err := os.WriteFile(t.path, t.data, 0o600); err != nil {
			return "", fmt.Errorf("write %s: %w", t.path, err)
		}
	}
	return rulesTarget, nil
}
