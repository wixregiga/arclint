package application

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// RulesetPresence lets setup initialize only a repository without a ruleset.
type RulesetPresence interface{ Exists() (bool, error) }

// SetupAgent coordinates the existing setup operations; the host owns activation.
type SetupAgent struct {
	initialize InitializeRepository
	domain     InitDomain
	hooks      InstallAgentHooks
	artifacts  ArtifactWriter
	publisher  AgentsPublisher
	ruleset    RulesetPresence
}

// SetupAgentRequest carries the explicit host and subject/evidence choices.
type SetupAgentRequest struct {
	Host          string
	DomainFiles   []string
	DomainSources []string
	Sources       []string
	Languages     []string
	Pattern       string
}

// NewSetupAgent reuses the existing initialization and publication operations.
func NewSetupAgent(initialize InitializeRepository, domain InitDomain, hooks InstallAgentHooks, artifacts ArtifactWriter, publisher AgentsPublisher, ruleset RulesetPresence) (SetupAgent, error) {
	if artifacts == nil || publisher == nil || ruleset == nil || hooks.installer == nil {
		return SetupAgent{}, fmt.Errorf("agent setup: missing dependency")
	}
	return SetupAgent{initialize: initialize, domain: domain, hooks: hooks, artifacts: artifacts, publisher: publisher, ruleset: ruleset}, nil
}

// Execute never replaces existing rules or recordings and never grants host trust.
func (uc SetupAgent) Execute(req SetupAgentRequest) ([]string, error) {
	if req.Host != "omp" && req.Host != codexHost {
		return nil, fmt.Errorf("choose --host omp or --host codex")
	}
	existing, err := uc.ruleset.Exists()
	if err != nil {
		return nil, fmt.Errorf("agent setup ruleset: %w", err)
	}
	paths := []string{}
	if !existing {
		path, err := uc.initialize.Execute(InitializeRepositoryRequest{Languages: req.Languages, Pattern: req.Pattern})
		if err != nil {
			return nil, fmt.Errorf("agent setup: %w", err)
		}
		paths = append(paths, path)
	} else if req.Pattern != "" || len(req.Languages) != 0 {
		return nil, fmt.Errorf("existing ruleset preserved; use patterns install for a deliberate policy change, and omit --pattern/--languages here")
	}
	if len(req.DomainFiles) == 0 || len(req.DomainFiles) == 1 && req.DomainFiles[0] == vocab.UbiquitousLanguageFileName {
		result, err := uc.domain.Execute("")
		if err != nil {
			return nil, fmt.Errorf("agent setup: %w", err)
		}
		req.DomainFiles = []string{result.Source}
	}
	protocol, _ := NewPublishSkillProtocol(uc.artifacts)
	vocabulary, _ := NewPublishSkillVocabulary(uc.artifacts)
	schema, _ := NewPublishDomainSchema(uc.artifacts)
	_, workflow, err := uc.artifacts.Write(DomainLibrarianSkillDir, "ARCLINT.md", []byte("# ArcLint setup and review\n\n"+vocab.SkillAgentWorkflow+"\n"))
	if err != nil {
		return nil, fmt.Errorf("agent setup workflow: %w", err)
	}
	paths = append(paths, workflow)
	for _, publish := range []func(string) (bool, string, error){protocol.Execute, vocabulary.Execute} {
		_, path, err := publish(DomainLibrarianSkillDir)
		if err != nil {
			return nil, fmt.Errorf("agent setup skill: %w", err)
		}
		paths = append(paths, path)
	}
	_, path, err := schema.Execute(SchemaDirectory)
	if err != nil {
		return nil, fmt.Errorf("agent setup: %w", err)
	}
	paths = append(paths, path)
	hookPaths, err := uc.hooks.Execute(req.Host, req.DomainFiles, AgentSourceScope{DomainSources: req.DomainSources, Sources: req.Sources})
	if err != nil {
		return nil, fmt.Errorf("agent setup: %w", err)
	}
	paths = append(paths, hookPaths...)
	_, path, err = uc.publisher.Install(agentSetupGuidance(req.DomainFiles))
	if err != nil {
		return nil, fmt.Errorf("agent setup: %w", err)
	}
	return append(paths, path), nil
}

func agentSetupGuidance(domains []string) string {
	return AgentsBegin + "\n## Architecture guidance (ArcLint)\n\n" +
		"Use .arclint/bin/arclint when installed, otherwise arclint.\n" +
		"Run arclint context [paths...] before changing architecture or domain behavior.\n" +
		"Recordings: " + strings.Join(domains, ", ") + ".\n" +
		"Use .agents/skills/domain-librarian/SKILL.md for classification and recording.\n" +
		"Read its companion ARCLINT.md for setup and the review/repair workflow.\n" +
		"Read both before domain work; do not invent a second\n" +
		"taxonomy or rewrite the request. Review subjects and supporting evidence are\n" +
		"explicit in .arclint/domain-guard.json. Installation is not activation or proof\n" +
		"of conformance. Keep the host trust review intact. Repair findings within scope\n" +
		"or rebut them with evidence for fresh review; never weaken rules or baselines\n" +
		"to obtain approval. Run arclint check . and the project's functional tests\n" +
		"before completion. Report checked behavior, fresh review and uncertainty separately.\n" +
		AgentsEnd + "\n"
}
