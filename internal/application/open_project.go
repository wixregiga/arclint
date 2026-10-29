package application

import (
	"fmt"
	"path/filepath"
	"strings"
)

// OpenProject finds the project a path lies in and remembers it, so the
// page serves and lists it.
type OpenProject struct {
	directories ProjectDirectories
	registry    ProjectRegistry
}

// NewOpenProject requires the port that finds a project and the
// registry that remembers it.
func NewOpenProject(directories ProjectDirectories, registry ProjectRegistry) (OpenProject, error) {
	if directories == nil || registry == nil {
		return OpenProject{}, fmt.Errorf("open project: missing project directories or registry")
	}
	return OpenProject{directories: directories, registry: registry}, nil
}

// Execute returns the directory of the project path lies in, the one
// whose rules.arclint.yaml is found upward from it, as every command
// finds its project, after remembering it.
func (uc OpenProject) Execute(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("open project: name a path inside the project")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("open project: %q is not an absolute path", path)
	}
	dir, err := uc.directories.Locate(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("open project: %w", err)
	}
	if err := uc.registry.Remember(dir); err != nil {
		return "", fmt.Errorf("open project %s: remember it: %w", dir, err)
	}
	return dir, nil
}
