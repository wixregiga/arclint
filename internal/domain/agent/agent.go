// Package agent describes supplied assistants and the hosts that accept them.
package agent

import (
	"fmt"
	"regexp"
	"strings"
)

var installableName = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

// Agent is a supplied assistant definition, independent of any running conversation.
// Equal names, descriptions, and instructions describe the same value.
type Agent struct {
	name         string
	description  string
	instructions string
}

// New requires an installable name, a purpose description, and instructions.
// Authored descriptions and instructions are preserved exactly.
func New(name, description, instructions string) (Agent, error) {
	a := Agent{name: name, description: description, instructions: instructions}
	if err := a.Validate(); err != nil {
		return Agent{}, err
	}
	return a, nil
}

// Validate rejects incomplete definitions, including the zero value.
func (a Agent) Validate() error {
	if !installableName.MatchString(a.name) {
		return fmt.Errorf("agent name %q: use lowercase words or numbers separated by single hyphens, beginning with a letter", a.name)
	}
	if strings.TrimSpace(a.description) == "" {
		return fmt.Errorf("agent %q: missing purpose description", a.name)
	}
	if strings.TrimSpace(a.instructions) == "" {
		return fmt.Errorf("agent %q: missing instructions", a.name)
	}
	return nil
}

// Name returns the host-independent installation name.
func (a Agent) Name() string { return a.name }

// Description returns the supplied explanation of the agent's purpose.
func (a Agent) Description() string { return a.description }

// Instructions returns the authored guidance without host-specific serialization.
func (a Agent) Instructions() string { return a.instructions }
