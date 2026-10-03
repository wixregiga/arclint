package main

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/application"
	codexagents "github.com/wixregiga/arclint/internal/infrastructure/agents/codex"
	agentfiles "github.com/wixregiga/arclint/internal/infrastructure/agents/files"
	markdownagents "github.com/wixregiga/arclint/internal/infrastructure/agents/markdown"
	ompagents "github.com/wixregiga/arclint/internal/infrastructure/agents/omp"
)

// agentSetupPreflight connects read-only validation through the real adapters.
type agentSetupPreflight struct{ root string }

func (p agentSetupPreflight) Validate(req application.SetupAgentRequest) error {
	scope := application.AgentSourceScope{DomainSources: req.DomainSources, Sources: req.Sources}
	var err error
	if req.Host == "codex" {
		err = codexagents.NewInstaller(p.root).Preflight(req.Host, req.DomainFiles, scope)
	} else {
		err = ompagents.NewInstaller(p.root).Preflight(req.Host, req.DomainFiles, scope)
	}
	if err != nil {
		return fmt.Errorf("setup preflight: %w", err)
	}
	writer := agentfiles.Writer{Root: p.root, PreserveSkills: true, CheckOnly: true}
	if _, err := application.PublishAgentSetupArtifacts(writer); err != nil {
		return fmt.Errorf("setup preflight: %w", err)
	}
	publisher, err := markdownagents.NewPublisher(p.root)
	if err != nil {
		return fmt.Errorf("setup preflight: %w", err)
	}
	if err := publisher.Preflight(application.AgentSetupGuidance(req.DomainFiles)); err != nil {
		return fmt.Errorf("setup guidance preflight: %w", err)
	}
	return nil
}
