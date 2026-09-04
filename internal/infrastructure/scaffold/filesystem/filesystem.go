// Package filesystemscaffold persists drafted repository rulesets. It
// refuses to overwrite an existing ruleset unless forced: adopting
// ArcLint must never silently destroy configured policy.
package filesystemscaffold

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileName is the repository ruleset file init writes.
const FileName = "rules.yaml"

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

// Write persists rules.yaml, refusing to overwrite an existing file
// unless forced.
func (w Writer) Write(content string, force bool) (string, error) {
	target := filepath.Join(w.dir, FileName)
	if !force {
		if _, err := os.Stat(target); err == nil {
			return "", fmt.Errorf("%s already exists; pass --force to overwrite", target)
		}
	}
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", target, err)
	}
	return target, nil
}
