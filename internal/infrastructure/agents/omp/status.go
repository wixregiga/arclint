package omp

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wixregiga/arclint/internal/application"
	agentfiles "github.com/wixregiga/arclint/internal/infrastructure/agents/files"
)

// InspectHostInstallation recognizes ArcLint's native extension entrypoint.
// Recognition does not prove that OMP loaded or ran the extension.
func (i *Installer) InspectHostInstallation() (application.AgentHostInstallation, error) {
	result := application.AgentHostInstallation{Name: "OMP", Paths: []string{".omp/extensions/arclint-domain-guard/index.js", ".omp/extensions/arclint-domain-guard/guard.mjs"}}
	root, err := filepath.Abs(i.root)
	if err != nil {
		return result, fmt.Errorf("inspect OMP project root: %w", err)
	}
	for _, path := range result.Paths {
		if _, err := os.Lstat(filepath.Join(root, path)); err == nil {
			result.Present = true
		}
	}
	target := filepath.Join(root, result.Paths[0])
	if err := agentfiles.CheckPath(root, target); err != nil {
		result.Problems = append(result.Problems, err.Error())
		return result, nil
	}
	installed, err := os.ReadFile(target)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("read OMP extension entrypoint: %w", err)
	}
	expected, err := assets.ReadFile("assets/index.js")
	if err != nil {
		return result, fmt.Errorf("read authored OMP entrypoint: %w", err)
	}
	result.Registered = bytes.Equal(installed, expected)
	if !result.Registered {
		result.Problems = append(result.Problems, "OMP ArcLint extension entrypoint differs from its installed registration")
	}
	return result, nil
}
