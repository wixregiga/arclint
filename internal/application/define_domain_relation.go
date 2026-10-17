package application

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

const (
	domainRelation    = "relation"
	domainDescription = "description"
)

// DefineRelationRequest identifies a context pair and the properties to
// record. Omitted properties stay as recorded; description may be cleared.
type DefineRelationRequest struct {
	From, To       string
	SetKind        bool
	Kind           string
	SetDescription bool
	Description    string
}

// ExecuteRelation records the context map through the same repository
// as definitions. The language constructor enforces the map's constraints.
func (uc DefineDomainDefinition) ExecuteRelation(req DefineRelationRequest) (DomainDefineResult, error) {
	lang, found, err := uc.knowledge.RecordedLanguage()
	if err != nil {
		return DomainDefineResult{}, fmt.Errorf("load domain model: %w", err)
	}
	if !found {
		lang = vocab.UbiquitousLanguage{Project: uc.project}
	}
	from, to := strings.TrimSpace(req.From), strings.TrimSpace(req.To)
	relation, exists := lang.Relation(from, to)
	out := DomainDefineResult{Target: domainRelation, Name: from + " -> " + to, Outcome: vocab.OutcomeUnchanged, Values: map[string]string{}}
	if !exists {
		if !req.SetKind {
			return DomainDefineResult{}, fmt.Errorf("%w: relation: --kind is required on first recording", ErrDomainUsage)
		}
		relation = vocab.ContextRelation{From: from, To: to}
		out.Outcome = vocab.OutcomeCreated
		out.Changed = append(out.Changed, "from", "to")
		out.Values["from"], out.Values["to"] = from, to
	} else if relation.From != from || relation.To != to {
		relation.From, relation.To = from, to
		out.Changed = append(out.Changed, "from", "to")
		out.Values["from"], out.Values["to"] = from, to
	}
	if req.SetKind && string(relation.Kind) != strings.TrimSpace(req.Kind) {
		relation.Kind = vocab.RelationKind(strings.TrimSpace(req.Kind))
		out.Changed = append(out.Changed, "kind")
		out.Values["kind"] = string(relation.Kind)
	}
	if req.SetDescription && relation.Description != strings.TrimSpace(req.Description) {
		relation.Description = strings.TrimSpace(req.Description)
		out.Changed = append(out.Changed, domainDescription)
		out.Values[domainDescription] = relation.Description
	}
	relations := slices.Clone(lang.Relations)
	if exists {
		for i, r := range relations {
			if (r.From == from && r.To == to) || (r.From == to && r.To == from) {
				relations[i] = relation
				break
			}
		}
	} else {
		relations = append(relations, relation)
	}
	next, err := vocab.NewUbiquitousLanguage(lang.Project, lang.Description, lang.Contexts, relations)
	if err != nil {
		return DomainDefineResult{}, fmt.Errorf("%w: %w", ErrDomainUsage, err)
	}
	if len(out.Changed) == 0 {
		return out, nil
	}
	if out.Outcome != vocab.OutcomeCreated {
		out.Outcome = vocab.OutcomeUpdated
	}
	if err := uc.knowledge.Record(next); err != nil {
		return DomainDefineResult{}, fmt.Errorf("save domain model: %w", err)
	}
	return out, nil
}

// ExecuteDescription records the project's top-level description. A
// description over its WordLimit is refused as the recorded language
// refuses it, never as a usage error.
func (uc DefineDomainDefinition) ExecuteDescription(description string) (DomainDefineResult, error) {
	lang, found, err := uc.knowledge.RecordedLanguage()
	if err != nil {
		return DomainDefineResult{}, fmt.Errorf("load domain model: %w", err)
	}
	if !found {
		lang = vocab.UbiquitousLanguage{Project: uc.project}
	}
	description = strings.TrimSpace(description)
	out := DomainDefineResult{Target: domainDescription, Name: lang.Project, Outcome: vocab.OutcomeUnchanged}
	// Describe judges the description before an unchanged one returns,
	// so a description already over its limit is refused when written
	// again, as an entry's definition is.
	next, err := lang.Describe(description)
	if err != nil {
		return DomainDefineResult{}, domainDescriptionError(err)
	}
	if description == lang.Description {
		return out, nil
	}
	out.Outcome = vocab.OutcomeUpdated
	out.Changed = []string{domainDescription}
	out.Values = map[string]string{domainDescription: description}
	if err := uc.knowledge.Record(next); err != nil {
		return DomainDefineResult{}, fmt.Errorf("save domain model: %w", err)
	}
	return out, nil
}

// domainDescriptionError classifies a refused description: over its
// WordLimit, the recorded language refuses it; otherwise the request
// is unusable as spelled.
func domainDescriptionError(err error) error {
	var over vocab.OverlongText
	if errors.As(err, &over) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrDomainUsage, err)
}
