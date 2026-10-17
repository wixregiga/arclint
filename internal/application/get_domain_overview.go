package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// DomainOverview is the plain result of loading the project's recorded
// Ubiquitous Language for the overview command.
type DomainOverview struct {
	Found    bool
	Source   string
	Counts   vocab.Counts
	Language vocab.UbiquitousLanguage
	// Matrix is the recorded contracts with source locations when
	// observations were supplied; nil otherwise.
	Matrix *DomainKnowledge
}

// GetDomainOverview loads the project's recorded domain model for the
// overview command.
type GetDomainOverview struct {
	knowledge    vocab.Repository
	rules        rule.Repository
	observations ObservationSource
}

// NewGetDomainOverview requires the Ubiquitous Language repository port.
func NewGetDomainOverview(knowledge vocab.Repository) (GetDomainOverview, error) {
	if knowledge == nil {
		return GetDomainOverview{}, fmt.Errorf("get domain overview: missing knowledge repository")
	}
	return GetDomainOverview{knowledge: knowledge}, nil
}

// WithObservations lends rules and observations so the overview can
// locate recorded contracts in source (file:line or missing).
func (uc GetDomainOverview) WithObservations(rules rule.Repository, observations ObservationSource) GetDomainOverview {
	uc.rules = rules
	uc.observations = observations
	return uc
}

// Execute loads the recorded model. A missing file is a normal result
// (Found=false), not an error.
func (uc GetDomainOverview) Execute() (DomainOverview, error) {
	lang, found, err := uc.knowledge.RecordedLanguage()
	if err != nil {
		return DomainOverview{}, fmt.Errorf("load domain model: %w", err)
	}
	out := DomainOverview{
		Found:  found,
		Source: uc.knowledge.Recording().Path(),
	}
	if !found {
		return out, nil
	}
	out.Language = lang
	out.Counts = lang.Counts()
	if uc.rules == nil || uc.observations == nil {
		return out, nil
	}
	cfg, err := uc.rules.ConfiguredRules()
	if err != nil {
		return DomainOverview{}, fmt.Errorf("load configured rules: %w", err)
	}
	required := carrierFacts(lang)
	obs, err := uc.observations.Observe(cfg.Languages, cfg.Scan, required)
	if err != nil {
		return DomainOverview{}, fmt.Errorf("observe contracts: %w", err)
	}
	matrix := domainKnowledgeOf(lang)
	facts, err := conformance.NewInspectionFacts(cfg.Zones, obs, required...)
	if err != nil {
		return DomainOverview{}, fmt.Errorf("prepare domain facts: %w", err)
	}
	carriers, err := conformance.NewCarriers(facts, lang)
	if err != nil {
		return DomainOverview{}, fmt.Errorf("locate contracts: %w", err)
	}
	if err := locateDomainContracts(matrix, carriers); err != nil {
		return DomainOverview{}, fmt.Errorf("locate contracts: %w", err)
	}
	matrix.Unanchored = unanchoredContracts(matrix, cfg.Languages)
	out.Matrix = matrix
	return out, nil
}

// carrierFacts are the facts locating the recorded language needs:
// declarations, and the resolved result types of operations when a
// domain service records an assertion, whose check may be the value
// its operation returns.
func carrierFacts(lang vocab.UbiquitousLanguage) []rule.Fact {
	facts := []rule.Fact{rule.FactDeclarations}
	for _, ctx := range lang.Contexts {
		for _, s := range ctx.Services {
			if len(s.Assertions) > 0 {
				return append(facts, rule.FactFieldTypes)
			}
		}
	}
	return facts
}
