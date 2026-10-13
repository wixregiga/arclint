package application

import (
	"fmt"
	"strings"
)

// AgentsContextReader reads only the marked block, without changing the
// document. A missing document or a document without a block is not stale.
type AgentsContextReader interface {
	GeneratedBlock() (block string, found bool, err error)
}

// Refresh command and advice for a stale generated snapshot.
const (
	AgentsContextRefreshCommand = "arclint agents md --write"
	AgentsContextStaleMessage   = "AGENTS.md generated block is stale; refresh with `" + AgentsContextRefreshCommand + "`"
)

// InspectAgentsContext compares the installed snapshot with the block
// the publisher would write from the current rules and recorded language.
type InspectAgentsContext struct {
	publish PublishAgentsContext
	reader  AgentsContextReader
}

// NewInspectAgentsContext requires the publisher and document reader ports.
func NewInspectAgentsContext(publish PublishAgentsContext, reader AgentsContextReader) (InspectAgentsContext, error) {
	if reader == nil {
		return InspectAgentsContext{}, fmt.Errorf("inspect agents context: missing document reader")
	}
	return InspectAgentsContext{publish: publish, reader: reader}, nil
}

// Execute returns advice only for a stale block. It never writes the
// document and does not contribute a conformance finding.
func (uc InspectAgentsContext) Execute() (string, error) {
	block, found, err := uc.reader.GeneratedBlock()
	if err != nil {
		return "", fmt.Errorf("inspect agents context: %w", err)
	}
	if !found {
		return "", nil
	}
	current, err := uc.publish.Render()
	if err != nil {
		return "", fmt.Errorf("inspect agents context: %w", err)
	}
	// A checkout may write AGENTS.md with CRLF line endings; the block is
	// stale only when its text differs, not its line endings.
	if strings.ReplaceAll(block, "\r\n", "\n") == strings.TrimSuffix(strings.ReplaceAll(current, "\r\n", "\n"), "\n") {
		return "", nil
	}
	return AgentsContextStaleMessage, nil
}
