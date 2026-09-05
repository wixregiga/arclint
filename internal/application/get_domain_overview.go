package application

import (
	"fmt"

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
		Source: vocab.UbiquitousLanguageFileName,
	}
	if !found {
		return out, nil
	}
	out.Language = lang
	out.Counts = lang.Counts()
	if uc.rules != nil && uc.observations != nil {
		cfg, err := uc.rules.ConfiguredRules()
		if err != nil {
			return DomainOverview{}, fmt.Errorf("load configured rules: %w", err)
		}
		obs, err := uc.observations.Observe(cfg.Languages, cfg.Scan, []rule.Fact{rule.FactDeclarations})
		if err != nil {
			return DomainOverview{}, fmt.Errorf("observe contracts: %w", err)
		}
		matrix := domainKnowledgeOf(lang)
		locateDomainContracts(matrix, lang, indexDeclarations(obs))
		out.Matrix = matrix
	}
	return out, nil
}
