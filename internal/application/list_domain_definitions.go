package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// DomainListing is the plain result of listing domain definitions,
// either the full model or one filtered concept group.
type DomainListing struct {
	Found    bool
	Source   string
	Language vocab.UbiquitousLanguage
	Concept  vocab.Concept
	Filtered bool
}

// ListDomainDefinitions lists the project's recorded domain definitions.
type ListDomainDefinitions struct {
	knowledge vocab.Repository
}

// NewListDomainDefinitions requires the Ubiquitous Language repository
// port.
func NewListDomainDefinitions(knowledge vocab.Repository) (ListDomainDefinitions, error) {
	if knowledge == nil {
		return ListDomainDefinitions{}, fmt.Errorf("list domain definitions: missing knowledge repository")
	}
	return ListDomainDefinitions{knowledge: knowledge}, nil
}

// Execute lists definitions. An empty listing returns every group;
// a non-empty listing must be a plural spelling accepted by
// vocab.ParseListing. A missing file behaves as an empty model.
func (uc ListDomainDefinitions) Execute(listing string) (DomainListing, error) {
	out := DomainListing{Source: vocab.UbiquitousLanguageFileName}
	if listing != "" {
		c, err := vocab.ParseListing(listing)
		if err != nil {
			return DomainListing{}, fmt.Errorf("%w: %v", ErrDomainUsage, err)
		}
		out.Concept = c
		out.Filtered = true
	}

	lang, found, err := uc.knowledge.RecordedLanguage()
	if err != nil {
		return DomainListing{}, fmt.Errorf("load domain model: %w", err)
	}
	out.Found = found
	if found {
		out.Language = lang
	}
	return out, nil
}
