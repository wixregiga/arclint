package application

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// DomainEntryView is one recorded entry as `arclint domain show`
// presents it: the concept it is recorded as (an aggregate root shows
// as its aggregate, a business rule as the invariant or assertion it
// is), the bounded context it is in, the owner it sits under when
// nested, and the entry itself in the field of its concept. A bounded
// context also carries the relations of the context map that name it.
type DomainEntryView struct {
	Concept vocab.Concept
	Context string
	Owner   string
	Name    string

	BoundedContext vocab.BoundedContext
	Relations      []vocab.ContextRelation
	Aggregate      vocab.Aggregate
	Entity         vocab.Entity
	ValueObject    vocab.ValueObject
	Invariant      vocab.Invariant
	Assertion      vocab.Assertion
	Event          vocab.DomainEvent
	Service        vocab.DomainService
	Specification  vocab.Specification
	Question       vocab.Question
}

// ShowDomainDefinition presents one recorded entry of the project's
// Ubiquitous Language.
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

// Execute finds the entry. concept is a spelling vocab.ParseConcept
// accepts; context and owner narrow the search and may be empty when
// the name finds the entry alone. A missing entry wraps
// vocab.ErrDefinitionNotFound; an entry the request cannot pin down
// (several contexts or owners record it) wraps ErrDomainUsage.
func (uc ShowDomainDefinition) Execute(concept, context, owner, name string) (DomainEntryView, error) {
	c, err := vocab.ParseConcept(strings.TrimSpace(concept))
	if err != nil {
		return DomainEntryView{}, fmt.Errorf("%w: %v", ErrDomainUsage, err)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return DomainEntryView{}, fmt.Errorf("%w: %s: a name is required", ErrDomainUsage, c)
	}
	lang, found, err := uc.knowledge.RecordedLanguage()
	if err != nil {
		return DomainEntryView{}, fmt.Errorf("load domain model: %w", err)
	}
	if !found {
		return DomainEntryView{}, fmt.Errorf("no %s named %q is recorded: %s is missing: %w",
			c, name, vocab.UbiquitousLanguageFileName, vocab.ErrDefinitionNotFound)
	}
	return locateEntry(lang, c, context, owner, name)
}
