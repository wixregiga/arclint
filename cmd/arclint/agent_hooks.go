package main

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/application"

	codexagents "github.com/wixregiga/arclint/internal/infrastructure/agents/codex"
	ompagents "github.com/wixregiga/arclint/internal/infrastructure/agents/omp"
)

// agentHooksInstaller selects the actual host adapter in the composition root.
type agentHooksInstaller struct{ root string }

func (i agentHooksInstaller) Install(host string, files []string, scope ...application.AgentSourceScope) ([]string, error) {
	var paths []string
	var err error
	switch host {
	case "omp":
		paths, err = ompagents.NewInstaller(i.root).Install(host, files, scope...)
	case "codex":
		paths, err = codexagents.NewInstaller(i.root).Install(host, files, scope...)
	default:
		return nil, fmt.Errorf("unsupported agent host %q", host)
	}
	if err != nil {
		return nil, fmt.Errorf("install %s hooks: %w", host, err)
	}
	return paths, nil
}
