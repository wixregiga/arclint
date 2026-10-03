package application

import "fmt"

const codexHost = "codex"

// ReviewerInstaller delivers the independent reviewer to a supported host.
type ReviewerInstaller interface {
	InstallReviewer() ([]string, error)
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
	if host != codexHost {
		return nil, fmt.Errorf("install reviewer: host %q is unsupported; use codex", host)
	}
	paths, err := uc.installer.InstallReviewer()
	if err != nil {
		return nil, fmt.Errorf("install reviewer: %w", err)
	}
	return paths, nil
}
