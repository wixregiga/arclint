package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// InitDomainResult reports whether initialization created the project's
// recorded Ubiquitous Language.
type InitDomainResult struct {
	Source  string
	Created bool
}

// InitDomain initializes the project's recorded Ubiquitous Language
// without replacing an existing model.
type InitDomain struct {
	knowledge vocab.Repository
}

// NewInitDomain requires the domain-model repository it initializes.
func NewInitDomain(knowledge vocab.Repository) (InitDomain, error) {
	if knowledge == nil {
		return InitDomain{}, fmt.Errorf("init domain: missing knowledge repository")
	}
	return InitDomain{knowledge: knowledge}, nil
}

// Execute creates an empty recorded Ubiquitous Language when none is
// present. Repeated execution leaves the existing model unchanged.
func (uc InitDomain) Execute() (InitDomainResult, error) {
	result := InitDomainResult{Source: vocab.UbiquitousLanguageFileName}
	_, found, err := uc.knowledge.RecordedLanguage()
	if err != nil {
		return InitDomainResult{}, fmt.Errorf("load domain model: %w", err)
	}
	if found {
		return result, nil
	}
	if err := uc.knowledge.Record(vocab.UbiquitousLanguage{}); err != nil {
		return InitDomainResult{}, fmt.Errorf("save domain model: %w", err)
	}
	result.Created = true
	return result, nil
}
