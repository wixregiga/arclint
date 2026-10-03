package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/agent"
)

// ReviewerInstaller delivers the independent reviewer to a supported host.
type ReviewerInstaller interface {
	InstallReviewer(agent.Host) ([]string, error)
}

// InstallReviewer installs one native reviewer without installing a guard.
type InstallReviewer struct{ installer ReviewerInstaller }

// NewInstallReviewer requires an installer supplied by the composition root.
func NewInstallReviewer(installer ReviewerInstaller) (InstallReviewer, error) {
	if installer == nil {
		return InstallReviewer{}, fmt.Errorf("install reviewer: missing installer")
	}
	return InstallReviewer{installer: installer}, nil
}

// Execute validates the selected host before performing any writes.
func (uc InstallReviewer) Execute(host string) ([]string, error) {
	selected, err := agent.NewHost(host)
	if err != nil {
		return nil, fmt.Errorf("install reviewer: %w", err)
	}
	paths, err := uc.installer.InstallReviewer(selected)
	if err != nil {
		return nil, fmt.Errorf("install reviewer: %w", err)
	}
	return paths, nil
}
