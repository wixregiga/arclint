package application

import "fmt"

// AgentInstallation is observed installation scope and asset integrity.
// It carries no host activation or review verdict.
type AgentInstallation struct {
	Project                                           string
	DomainFiles, DomainSourcePatterns, SourcePatterns []string
	InstalledHosts, ChangedAssets, Problems           []string
	LegacyRules, Intact                               bool
}

// AgentHostInstallation carries adapter-observed registration and required paths.
// Presence and registration describe files, never host loading or approval.
type AgentHostInstallation struct {
	Name                string
	Paths               []string
	Present, Registered bool
	Problems            []string
}

// AgentSetupInspector reads scope and integrity without approving work.
type AgentSetupInspector interface {
	Status() (AgentInstallation, error)
}

// AgentSetupStatus separates installation from host activation.
type AgentSetupStatus struct{ inspector AgentSetupInspector }

// NewAgentSetupStatus requires a read-only installation inspector.
func NewAgentSetupStatus(inspector AgentSetupInspector) (AgentSetupStatus, error) {
	if inspector == nil {
		return AgentSetupStatus{}, fmt.Errorf("agent status: missing inspector")
	}
	return AgentSetupStatus{inspector: inspector}, nil
}

// Execute returns installed scope and asset integrity without inferring host activation.
func (uc AgentSetupStatus) Execute() (AgentInstallation, error) {
	status, err := uc.inspector.Status()
	if err != nil {
		return AgentInstallation{}, fmt.Errorf("agent status: %w", err)
	}
	return status, nil
}
