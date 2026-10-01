package application

import "fmt"

type (
	// AgentSetupInspector reads scope and integrity without approving work.
	AgentSetupInspector interface{ Status() (string, error) }
	// AgentSetupStatus separates installation from host activation.
	AgentSetupStatus struct{ inspector AgentSetupInspector }
)

// NewAgentSetupStatus requires a read-only installation inspector.
func NewAgentSetupStatus(inspector AgentSetupInspector) (AgentSetupStatus, error) {
	if inspector == nil {
		return AgentSetupStatus{}, fmt.Errorf("agent status: missing inspector")
	}
	return AgentSetupStatus{inspector: inspector}, nil
}

// Execute returns the installed scope and remaining host activation steps.
func (uc AgentSetupStatus) Execute() (string, error) {
	status, err := uc.inspector.Status()
	if err != nil {
		return "", fmt.Errorf("agent status: %w", err)
	}
	return status, nil
}
