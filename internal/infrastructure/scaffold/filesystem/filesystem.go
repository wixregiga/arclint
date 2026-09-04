// Package filesystemscaffold persists drafted repository rulesets. It
// refuses to overwrite an existing ruleset unless forced: adopting
// ArcLint must never silently destroy configured policy.
package filesystemscaffold

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wixregiga/arclint/internal/application"
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

// Write persists the ruleset under its published file name, refusing
// to overwrite an existing file unless forced. The drafted content is
// led by an editor modeline naming the Rule Schema: the project's copy
// when it exists under the directory, else the published $id.
func (w Writer) Write(content string, force bool) (string, error) {
	target := filepath.Join(w.dir, rule.RulesetFileName)
	if !force {
		if _, err := os.Stat(target); err == nil {
			return "", fmt.Errorf("%s already exists; pass --force to overwrite", target)
		}
	}
	document := w.schemaModeline() + "\n" + content
	if err := os.WriteFile(target, []byte(document), 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", target, err)
	}
	return target, nil
}

// schemaModeline chooses the editor schema hint for the drafted
// ruleset, exactly as the domain file chooses its own.
func (w Writer) schemaModeline() string {
	local := filepath.ToSlash(filepath.Join(application.SchemaDirectory, rule.SchemaFileName))
	if st, err := os.Stat(filepath.Join(w.dir, filepath.FromSlash(local))); err == nil && !st.IsDir() {
		return "# yaml-language-server: $schema=" + local
	}
	return "# yaml-language-server: $schema=" + rule.SchemaID
}
