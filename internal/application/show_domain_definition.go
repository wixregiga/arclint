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
	Context    string
	Definition vocab.Definition
	// Aggregate is meaningful for entity and aggregate concepts:
	// true when the matched Entity is designated an Aggregate.
	// An aggregate-concept match implies true by the match itself.
	Aggregate bool
	// Owner is set for invariant kinds (statement lives in Name).
	Owner string
	// ID is the cluster or assertion identity when recorded.
	ID string
	// On is the assertion's operation when recorded.
	On string
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
func (uc ShowDomainDefinition) Execute(concept, context, name string) (DomainDefinitionView, error) {
	c, err := vocab.ParseConcept(concept)
	if err != nil {
		return DomainDefinitionView{}, fmt.Errorf("%w: %v", ErrDomainUsage, err)
	}
	name = strings.TrimSpace(name)
	if name == "" && c != vocab.ConceptBoundedContext {
		return DomainDefinitionView{}, fmt.Errorf("%w: name is required", ErrDomainUsage)
	}

	lang, _, err := uc.knowledge.RecordedLanguage()
	if err != nil {
		return DomainDefinitionView{}, fmt.Errorf("load domain model: %w", err)
	}

	ctxName, def, ok, err := findInContexts(lang, c, context, name)
	if err != nil {
		return DomainDefinitionView{}, err
	}
	if !ok {
		return DomainDefinitionView{}, definitionNotFound(c, ctxName, name)
	}

	view := DomainDefinitionView{
		Concept:    c,
		Context:    ctxName,
		Definition: def,
	}
	switch c {
	case vocab.ConceptAggregate, vocab.ConceptAggregateRoot:
		view.Aggregate = true
	case vocab.ConceptEntity:
		if ent, found := lang.FindEntity(ctxName, name); found {
			view.Aggregate = ent.Aggregate
		}
	case vocab.ConceptInvariant, vocab.ConceptBusinessRule:
		// Domain Find encodes owner in Definition.Definition.
		view.Owner = def.Definition
		view.Definition = vocab.Definition{Name: def.Name}
		if inv, ok := lang.FindInvariant(ctxName, name); ok {
			view.ID = inv.ID
		}
	case vocab.ConceptAssertion:
		view.Owner = def.Definition
		view.Definition = vocab.Definition{Name: def.Name}
		if a, ok := lang.FindAssertion(ctxName, name); ok {
			view.ID = a.ID
			view.On = a.On
		}
	case vocab.ConceptValueObject, vocab.ConceptDomainEvent, vocab.ConceptBoundedContext, vocab.ConceptSpecification:
		// The base view already carries everything these kinds record.
	}
	return view, nil
}
