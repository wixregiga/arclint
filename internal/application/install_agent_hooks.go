package application

import "fmt"

// AgentHooksInstaller installs a supported host integration without replacing
// user-owned files. Domain paths are explicit project-relative files.
type AgentHooksInstaller interface {
	Install(host string, domainFiles []string, scope ...AgentSourceScope) ([]string, error)
}

// AgentSourceScope separates domain code from supporting implementation evidence.
type AgentSourceScope struct {
	DomainSources []string
	Sources       []string
}

// InstallAgentHooks connects agent setup to the host-specific installer.
type InstallAgentHooks struct{ installer AgentHooksInstaller }

// NewInstallAgentHooks requires an installer for the supported host.
func NewInstallAgentHooks(installer AgentHooksInstaller) (InstallAgentHooks, error) {
	if installer == nil {
		return InstallAgentHooks{}, fmt.Errorf("install agent hooks: missing installer")
	}
	return InstallAgentHooks{installer: installer}, nil
}

// Execute installs hooks over the explicitly configured domain files.
func (uc InstallAgentHooks) Execute(host string, domainFiles []string, scope ...AgentSourceScope) ([]string, error) {
	if host != "omp" && host != "codex" {
		return nil, fmt.Errorf("unsupported agent host %q: supported hosts are omp and codex", host)
	}
	if len(domainFiles) == 0 {
		domainFiles = []string{"domain.arclint.yaml"}
	}
	paths, err := uc.installer.Install(host, domainFiles, scope...)
	if err != nil {
		return nil, fmt.Errorf("install agent hooks: %w", err)
	}
	return paths, nil
}
