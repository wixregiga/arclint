// Package omp installs the project-local domain guard for OMP.
package omp

import (
	"embed"
	"fmt"
	"path/filepath"

	"github.com/wixregiga/arclint/internal/application"
	agentfiles "github.com/wixregiga/arclint/internal/infrastructure/agents/files"
)

//go:embed assets/index.js assets/guard.mjs
var assets embed.FS

// Installer writes the native project-local OMP extension.
type Installer struct{ root string }

// NewInstaller binds installation to one project root.
func NewInstaller(root string) *Installer { return &Installer{root: root} }

// Install preserves configuration and upgrades only unchanged owned assets.
func (i *Installer) Install(host string, domains []string, scope ...application.AgentSourceScope) ([]string, error) {
	if host != "omp" {
		return nil, fmt.Errorf("unsupported host %q", host)
	}
	root, err := filepath.Abs(i.root)
	if err != nil {
		return nil, fmt.Errorf("agent setup: %w", err)
	}
	config, err := agentfiles.Scope(root, domains, scope...)
	if err != nil {
		return nil, fmt.Errorf("agent setup: %w", err)
	}
	configPath := filepath.Join(root, ".arclint/domain-guard.json")
	files := map[string][]byte{configPath: config}
	for _, name := range []string{"index.js", "guard.mjs"} {
		content, err := assets.ReadFile("assets/" + name)
		if err != nil {
			return nil, fmt.Errorf("agent setup: %w", err)
		}
		files[filepath.Join(root, ".omp/extensions/arclint-domain-guard", name)] = content
	}
	paths, err := agentfiles.Install(root, files, configPath)
	if err != nil {
		return nil, fmt.Errorf("install hooks: %w", err)
	}
	return paths, nil
}
