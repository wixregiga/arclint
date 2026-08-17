// Package application holds ArcLint's action-named use cases. A use
// case accepts ports and plain request values, coordinates domain
// objects, and returns plain result values. Use cases never import
// delivery or infrastructure and never construct outbound Adapters; a
// port begins beside the use case that owns its requirements.
package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// RuleSummary is the plain result value describing one configured
// Rule.
type RuleSummary struct {
	ID             string
	Type           string
	Severity       string
	Claim          string
	Assurance      string
	Provenance     string // "namespace/name@version", or "" for repository-local Rules
	Disabled       bool
	DisabledReason string
}

// ListRules lists the repository's configured Rules through the
// domain-owned read port.
type ListRules struct {
	rules rule.Repository
}

// NewListRules requires the Rule repository port.
func NewListRules(rules rule.Repository) (ListRules, error) {
	if rules == nil {
		return ListRules{}, fmt.Errorf("list rules: missing rule repository")
	}
	return ListRules{rules: rules}, nil
}

// Execute returns one summary per configured Rule, in configuration
// order.
func (uc ListRules) Execute() ([]RuleSummary, error) {
	cfg, err := uc.rules.ConfiguredRules()
	if err != nil {
		return nil, fmt.Errorf("load configured rules: %w", err)
	}
	out := make([]RuleSummary, 0, len(cfg.Rules))
	for _, r := range cfg.Rules {
		out = append(out, summarize(r))
	}
	return out, nil
}
