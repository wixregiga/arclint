package application

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// DomainRemoveResult is the plain result of removing one domain
// definition or clearing an Aggregate designation.
type DomainRemoveResult struct {
	Concept         vocab.Concept
	Context         string
	Name            string
	EntityPreserved bool
}

// RemoveDomainDefinition removes one recorded domain definition or
// clears an Aggregate designation.
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

// Execute removes one definition. Unknown concepts and empty names are
// usage errors. A missing file or missing name wraps
// vocab.ErrDefinitionNotFound and never saves.
func (uc RemoveDomainDefinition) Execute(concept, context, name string) (DomainRemoveResult, error) {
	c, err := vocab.ParseConcept(concept)
	if err != nil {
		return DomainRemoveResult{}, fmt.Errorf("%w: %v", ErrDomainUsage, err)
	}
	name = strings.TrimSpace(name)
	if name == "" && c != vocab.ConceptBoundedContext {
		return DomainRemoveResult{}, fmt.Errorf("%w: name is required", ErrDomainUsage)
	}

	lang, _, err := uc.knowledge.RecordedLanguage()
	if err != nil {
		return DomainRemoveResult{}, fmt.Errorf("load domain model: %w", err)
	}

	ctxName, _, ok, err := findInContexts(lang, c, context, name)
	if err != nil {
		return DomainRemoveResult{}, err
	}
	if !ok {
		return DomainRemoveResult{}, definitionNotFound(c, ctxName, name)
	}

	// For bounded_context the domain Remove keys on name.
	removeContext := ctxName
	removeName := name
	if c == vocab.ConceptBoundedContext {
		removeName = ctxName
		removeContext = ctxName
	}

	next, result, err := lang.Remove(c, removeContext, removeName)
	if err != nil {
		// No prefix: the domain vocabulary is the complete
		// user-facing message (wraps vocab.ErrDefinitionNotFound).
		return DomainRemoveResult{}, fmt.Errorf("%w", err)
	}
	if err := uc.knowledge.Record(next); err != nil {
		return DomainRemoveResult{}, fmt.Errorf("save domain model: %w", err)
	}
	return DomainRemoveResult{
		Concept:         c,
		Context:         ctxName,
		Name:            name,
		EntityPreserved: result.EntityPreserved,
	}, nil
}
