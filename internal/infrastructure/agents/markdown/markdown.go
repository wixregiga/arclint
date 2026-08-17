// Package markdownagents installs the generated architecture block
// into the repository's AGENTS.md between ArcLint's markers. Content
// outside the markers is never touched, so the file carries
// hand-written guidance alongside the generated block.
package markdownagents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
)

// Publisher implements the application's AgentsPublisher port over one
// repository root.
type Publisher struct {
	root string
}

// NewPublisher binds the publisher to a repository root.
func NewPublisher(root string) (Publisher, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Publisher{}, fmt.Errorf("agents publisher root: %w", err)
	}
	return Publisher{root: abs}, nil
}

// Install writes the block into <root>/AGENTS.md. A file with both
// markers has the marked range replaced; a file without markers gets
// the block appended; a missing file is created holding only the
// block. One marker without the other is a corruption error.
func (p Publisher) Install(block string) (bool, string, error) {
	target := filepath.Join(p.root, "AGENTS.md")
	existing, err := os.ReadFile(target)
	switch {
	case os.IsNotExist(err):
		if err := os.WriteFile(target, []byte(block), 0o600); err != nil {
			return false, target, fmt.Errorf("write %s: %w", target, err)
		}
		return true, target, nil
	case err != nil:
		return false, target, fmt.Errorf("read %s: %w", target, err)
	}
	content := string(existing)
	begin := strings.Index(content, application.AgentsBegin)
	end := strings.Index(content, application.AgentsEnd)
	var next string
	switch {
	case begin >= 0 && end > begin:
		next = content[:begin] + strings.TrimSuffix(block, "\n") + content[end+len(application.AgentsEnd):]
	case begin < 0 && end < 0:
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		next = content + "\n" + block
	default:
		return false, target, fmt.Errorf("%s: found one arclint marker without the other; repair or delete the block", target)
	}
	if next == content {
		return false, target, nil
	}
	if err := os.WriteFile(target, []byte(next), 0o600); err != nil {
		return false, target, fmt.Errorf("write %s: %w", target, err)
	}
	return true, target, nil
}
