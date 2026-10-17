package application

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
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

	// Rules is what the entry records about its invariants and
	// assertions; nil for a concept that records none.
	Rules *DomainEntryRules
}

// ShowDomainDefinition presents one recorded entry of the project's
// Ubiquitous Language, and for an entry that records invariants or
// assertions, where the code enforces each one and how the current
// check judges it.
type ShowDomainDefinition struct {
	knowledge vocab.Repository
	rules     rule.Repository
	assess    *AssessConformance
}

// NewShowDomainDefinition requires the Ubiquitous Language repository
// port.
func NewShowDomainDefinition(knowledge vocab.Repository) (ShowDomainDefinition, error) {
	if knowledge == nil {
		return ShowDomainDefinition{}, fmt.Errorf("show domain definition: missing knowledge repository")
	}
	return ShowDomainDefinition{knowledge: knowledge}, nil
}

// WithChecks lends the configured Rules and the Conformance Check, so
// show reports where each recorded invariant and assertion is enforced
// and its status. Without them every rule reports that no check ran.
func (uc ShowDomainDefinition) WithChecks(rules rule.Repository, assess AssessConformance) ShowDomainDefinition {
	uc.rules = rules
	uc.assess = &assess
	return uc
}

// Execute finds the entry. concept is a spelling vocab.ParseConcept
// accepts, or empty to find the entry by its name, key, or alias
// alone; context and owner narrow the search and may be empty when the
// name finds the entry alone. A missing entry wraps
// vocab.ErrDefinitionNotFound; an entry the request cannot pin down
// (several concepts, contexts, or owners record the name) wraps
// ErrDomainUsage and names each candidate with the command showing it.
func (uc ShowDomainDefinition) Execute(concept, context, owner, name string) (DomainEntryView, error) {
	concept = strings.TrimSpace(concept)
	var c vocab.Concept
	if concept != "" {
		parsed, err := vocab.ParseConcept(concept)
		if err != nil {
			return DomainEntryView{}, fmt.Errorf("%w: %v", ErrDomainUsage, err)
		}
		c = parsed
	}
	name = strings.TrimSpace(name)
	if name == "" {
		if c == "" {
			return DomainEntryView{}, fmt.Errorf("%w: a name is required", ErrDomainUsage)
		}
		return DomainEntryView{}, fmt.Errorf("%w: %s: a name is required", ErrDomainUsage, c)
	}
	lang, found, err := uc.knowledge.RecordedLanguage()
	if err != nil {
		return DomainEntryView{}, fmt.Errorf("load domain model: %w", err)
	}
	if !found {
		what := "entry"
		if c != "" {
			what = string(c)
		}
		return DomainEntryView{}, fmt.Errorf("no %s named %q is recorded: %s is missing: %w",
			what, name, uc.knowledge.Recording().Path(), vocab.ErrDefinitionNotFound)
	}
	var view DomainEntryView
	if c == "" {
		view, err = locateNamedEntry(lang, context, owner, name)
	} else {
		view, err = locateEntry(lang, c, context, owner, name)
	}
	if err != nil {
		return DomainEntryView{}, err
	}
	view.Rules = recordedRules(lang, view)
	if view.Rules != nil && len(view.Rules.Checks) > 0 {
		uc.judge(view.Context, view.Rules)
	}
	return view, nil
}
