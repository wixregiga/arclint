package application

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// DomainListing is the plain result of listing domain definitions,
// either the full model or one filtered concept group. Optional Context
// narrows the listing to one bounded context.
type DomainListing struct {
	Found    bool
	Source   string
	Language vocab.UbiquitousLanguage
	Concept  vocab.Concept
	Filtered bool
	// Context is the optional --context filter (empty means all).
	Context string
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
// vocab.ParseListing. context, when non-empty, limits the view to that
// bounded context. A missing file behaves as an empty model.
func (uc ListDomainDefinitions) Execute(listing, context string) (DomainListing, error) {
	out := DomainListing{
		Source:  vocab.UbiquitousLanguageFileName,
		Context: strings.TrimSpace(context),
	}
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
	if !found {
		return out, nil
	}

	if out.Context != "" {
		// A context nobody recorded is a typo, told apart from a
		// recorded context that lists nothing.
		if _, ok := lang.Context(out.Context); !ok {
			return DomainListing{}, fmt.Errorf("%w: unknown context %q; recorded: %s", ErrDomainUsage, out.Context, strings.Join(lang.ContextNames(), ", "))
		}
	}

	out.Language = lang
	return out, nil
}
