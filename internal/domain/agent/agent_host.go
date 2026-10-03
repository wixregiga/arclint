package agent

import "fmt"

// Host selects a supported host for separately installed agents.
// Existing hook integrations have their own host support.
type Host struct {
	name string
}

// NewHost selects Codex, the currently supported named-agent host.
func NewHost(name string) (Host, error) {
	h := Host{name: name}
	if err := h.Validate(); err != nil {
		return Host{}, err
	}
	return h, nil
}

// Validate rejects unsupported hosts and the zero value without choosing a fallback.
func (h Host) Validate() error {
	if h.name != "codex" {
		return fmt.Errorf("agent host %q: separately installed agents support codex", h.name)
	}
	return nil
}

// String returns the selected host's installation name.
func (h Host) String() string { return h.name }
