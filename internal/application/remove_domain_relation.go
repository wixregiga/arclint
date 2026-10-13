package application

import (
	"fmt"
	"slices"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// ExecuteRelation removes the one relation between the named contexts,
// in either direction, leaving both contexts intact.
func (uc RemoveDomainDefinition) ExecuteRelation(from, to string) (DomainRemoveResult, error) {
	from, to = strings.TrimSpace(from), strings.TrimSpace(to)
	if from == "" || to == "" {
		return DomainRemoveResult{}, fmt.Errorf("%w: relation: --from and --to are required", ErrDomainUsage)
	}
	lang, found, err := uc.knowledge.RecordedLanguage()
	if err != nil {
		return DomainRemoveResult{}, fmt.Errorf("load domain model: %w", err)
	}
	if _, exists := lang.Relation(from, to); !found || !exists {
		return DomainRemoveResult{}, fmt.Errorf("relation %s -> %s: %w", from, to, vocab.ErrDefinitionNotFound)
	}
	relations := slices.DeleteFunc(slices.Clone(lang.Relations), func(r vocab.ContextRelation) bool {
		return (r.From == from && r.To == to) || (r.From == to && r.To == from)
	})
	next, err := vocab.NewUbiquitousLanguage(lang.Project, lang.Description, lang.Contexts, relations)
	if err != nil {
		return DomainRemoveResult{}, fmt.Errorf("%w: %w", ErrDomainUsage, err)
	}
	if err := uc.knowledge.Record(next); err != nil {
		return DomainRemoveResult{}, fmt.Errorf("save domain model: %w", err)
	}
	return DomainRemoveResult{Target: domainRelation, Name: from + " -> " + to}, nil
}
