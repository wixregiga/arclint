package application

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// DomainRemoveResult reports one removal: the concept removed, where
// it was, and what else the removal carried away or altered, one
// sentence each (an aggregate's members and rules go with it, an event
// it raised no longer names it, a value object named as an identity
// stays implied, a context's relations go with it).
type DomainRemoveResult struct {
	Concept vocab.Concept
	Context string
	Owner   string
	Name    string
	Also    []string
}

// RemoveDomainDefinition takes one recorded entry out of the project's
// Ubiquitous Language.
type RemoveDomainDefinition struct {
	knowledge vocab.Repository
}

// NewRemoveDomainDefinition requires the Ubiquitous Language repository
// port.
func NewRemoveDomainDefinition(knowledge vocab.Repository) (RemoveDomainDefinition, error) {
	if knowledge == nil {
		return RemoveDomainDefinition{}, fmt.Errorf("remove domain definition: missing knowledge repository")
	}
	return RemoveDomainDefinition{knowledge: knowledge}, nil
}

// Execute removes the entry the concept, context, owner, and name
// locate, the way `arclint domain show` locates it. An unknown concept
// or an entry the request cannot pin down wraps ErrDomainUsage; a
// missing file or entry wraps vocab.ErrDefinitionNotFound and never
// saves.
func (uc RemoveDomainDefinition) Execute(concept, context, owner, name string) (DomainRemoveResult, error) {
	c, err := vocab.ParseConcept(strings.TrimSpace(concept))
	if err != nil {
		return DomainRemoveResult{}, fmt.Errorf("%w: %v", ErrDomainUsage, err)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return DomainRemoveResult{}, fmt.Errorf("%w: %s: a name is required", ErrDomainUsage, c)
	}
	lang, found, err := uc.knowledge.RecordedLanguage()
	if err != nil {
		return DomainRemoveResult{}, fmt.Errorf("load domain model: %w", err)
	}
	if !found {
		return DomainRemoveResult{}, fmt.Errorf("no %s named %q is recorded: %s is missing: %w",
			c, name, vocab.UbiquitousLanguageFileName, vocab.ErrDefinitionNotFound)
	}
	entry, err := locateEntry(lang, c, context, owner, name)
	if err != nil {
		return DomainRemoveResult{}, err
	}
	at := vocab.Locator{Context: entry.Context, Owner: entry.Owner, Name: entry.Name}
	if entry.Concept == vocab.ConceptBoundedContext {
		at = vocab.Locator{Name: entry.Name}
	}
	next, res, err := lang.Remove(entry.Concept, at)
	if err != nil {
		return DomainRemoveResult{}, domainRequestError(err)
	}
	if err := uc.knowledge.Record(next); err != nil {
		return DomainRemoveResult{}, fmt.Errorf("save domain model: %w", err)
	}
	return DomainRemoveResult{
		Concept: res.Concept,
		Context: entry.Context,
		Owner:   res.Owner,
		Name:    entry.Name,
		Also:    res.Also,
	}, nil
}
