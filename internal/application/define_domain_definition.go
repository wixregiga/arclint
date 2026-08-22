package application

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// DefineDomainRequest is the plain request for create-or-update of one
// domain definition. Definition is a pointer so callers can distinguish
// omitted from explicitly cleared (empty string). Aggregate is set only
// by guided authoring when designating an Entity in the same write.
type DefineDomainRequest struct {
	Concept      string
	Name         string
	Definition   *string
	Aliases      []string
	ClearAliases bool
	Aggregate    *bool
}

// DomainDefineResult is the plain result of a define operation.
type DomainDefineResult struct {
	Outcome vocab.Outcome
	Concept vocab.Concept
	Name    string
	Changed []string
	// Aliases is the final alias set after the define.
	Aliases []string
}

// DefineDomainDefinition create-or-updates one recorded domain
// definition through the repository port.
type DefineDomainDefinition struct {
	knowledge vocab.Repository
}

// NewDefineDomainDefinition requires the Ubiquitous Language repository
// port.
func NewDefineDomainDefinition(knowledge vocab.Repository) (DefineDomainDefinition, error) {
	if knowledge == nil {
		return DefineDomainDefinition{}, fmt.Errorf("define domain definition: missing knowledge repository")
	}
	return DefineDomainDefinition{knowledge: knowledge}, nil
}

// Execute create-or-updates one definition. Usage failures (unknown
// concept, empty name, mutually exclusive alias flags) wrap
// ErrDomainUsage. The model is saved only when the outcome is not
// unchanged.
func (uc DefineDomainDefinition) Execute(req DefineDomainRequest) (DomainDefineResult, error) {
	c, err := vocab.ParseConcept(req.Concept)
	if err != nil {
		return DomainDefineResult{}, fmt.Errorf("%w: %v", ErrDomainUsage, err)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return DomainDefineResult{}, fmt.Errorf("%w: name is required", ErrDomainUsage)
	}
	if len(req.Aliases) > 0 && req.ClearAliases {
		return DomainDefineResult{}, fmt.Errorf("%w: --alias and --clear-aliases are mutually exclusive", ErrDomainUsage)
	}

	lang, _, err := uc.knowledge.RecordedLanguage()
	if err != nil {
		return DomainDefineResult{}, fmt.Errorf("load domain model: %w", err)
	}

	ch := vocab.Change{}
	if req.Definition != nil {
		ch.SetDefinition = true
		ch.DefinitionText = *req.Definition
	}
	if len(req.Aliases) > 0 {
		ch.SetAliases = true
		ch.Aliases = append([]string(nil), req.Aliases...)
	}
	if req.ClearAliases {
		ch.ClearAliases = true
	}
	if req.Aggregate != nil {
		ch.SetAggregate = true
		ch.Aggregate = *req.Aggregate
	}

	next, result, err := lang.Define(c, name, ch)
	if err != nil {
		// No prefix: the domain vocabulary is the complete
		// user-facing message.
		return DomainDefineResult{}, fmt.Errorf("%w", err)
	}

	if result.Outcome != vocab.OutcomeUnchanged {
		if err := uc.knowledge.Record(next); err != nil {
			return DomainDefineResult{}, fmt.Errorf("save domain model: %w", err)
		}
	}

	// Final aliases come from the post-define model (aggregate defines
	// land on the entity record).
	aliases := finalAliases(next, c, name)

	return DomainDefineResult{
		Outcome: result.Outcome,
		Concept: c,
		Name:    name,
		Changed: result.Changed,
		Aliases: aliases,
	}, nil
}

// finalAliases reads the alias set after a define. Entity and aggregate
// concepts resolve through FindEntity; other concepts use Find.
func finalAliases(lang vocab.UbiquitousLanguage, c vocab.Concept, name string) []string {
	switch c {
	case vocab.ConceptEntity, vocab.ConceptAggregate:
		if ent, ok := lang.FindEntity(name); ok {
			return append([]string(nil), ent.Aliases...)
		}
	default:
		if def, ok := lang.Find(c, name); ok {
			return append([]string(nil), def.Aliases...)
		}
	}
	return nil
}
