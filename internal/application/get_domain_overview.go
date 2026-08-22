package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// DomainOverview is the plain result of loading the project's recorded
// Ubiquitous Language for the overview command.
type DomainOverview struct {
	Found    bool
	Source   string
	Counts   vocab.Counts
	Language vocab.UbiquitousLanguage
}

// GetDomainOverview loads the project's recorded domain model for the
// overview command.
type GetDomainOverview struct {
	knowledge vocab.Repository
}

// NewGetDomainOverview requires the Ubiquitous Language repository port.
func NewGetDomainOverview(knowledge vocab.Repository) (GetDomainOverview, error) {
	if knowledge == nil {
		return GetDomainOverview{}, fmt.Errorf("get domain overview: missing knowledge repository")
	}
	return GetDomainOverview{knowledge: knowledge}, nil
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
	return out, nil
}
