package application

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// DomainDefinitionView is one recorded definition resolved by concept
// and name for the show command.
type DomainDefinitionView struct {
	Concept    vocab.Concept
	Definition vocab.Definition
	// Aggregate is meaningful for entity and aggregate concepts:
	// true when the matched Entity is designated an Aggregate.
	// An aggregate-concept match implies true by the match itself.
	Aggregate bool
}

// ShowDomainDefinition shows one recorded domain definition.
type ShowDomainDefinition struct {
	knowledge vocab.Repository
}

// NewShowDomainDefinition requires the Ubiquitous Language repository
// port.
func NewShowDomainDefinition(knowledge vocab.Repository) (ShowDomainDefinition, error) {
	if knowledge == nil {
		return ShowDomainDefinition{}, fmt.Errorf("show domain definition: missing knowledge repository")
	}
	return ShowDomainDefinition{knowledge: knowledge}, nil
}

// Execute resolves one definition by singular concept spelling and
// exact name. Unknown concepts and empty names are usage errors;
// missing definitions wrap vocab.ErrDefinitionNotFound.
func (uc ShowDomainDefinition) Execute(concept, name string) (DomainDefinitionView, error) {
	c, err := vocab.ParseConcept(concept)
	if err != nil {
		return DomainDefinitionView{}, fmt.Errorf("%w: %v", ErrDomainUsage, err)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return DomainDefinitionView{}, fmt.Errorf("%w: name is required", ErrDomainUsage)
	}

	lang, _, err := uc.knowledge.RecordedLanguage()
	if err != nil {
		return DomainDefinitionView{}, fmt.Errorf("load domain model: %w", err)
	}
	def, ok := lang.Find(c, name)
	if !ok {
		return DomainDefinitionView{}, fmt.Errorf("no %s named %q is defined in the project domain model: %w", c, name, vocab.ErrDefinitionNotFound)
	}
	view := DomainDefinitionView{Concept: c, Definition: def}
	switch c {
	case vocab.ConceptAggregate:
		// Aggregate-concept match only succeeds for designated Entities.
		view.Aggregate = true
	case vocab.ConceptEntity:
		if ent, found := lang.FindEntity(name); found {
			view.Aggregate = ent.Aggregate
		}
	default:
		// The identity-less concepts carry no Aggregate designation.
	}
	return view, nil
}
