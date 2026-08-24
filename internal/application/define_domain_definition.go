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
// Name is the statement for invariant, assertion, and business_rule.
// Owner is required at create for those kinds.
type DefineDomainRequest struct {
	Concept      string
	Context      string
	Name         string
	Definition   *string
	Aliases      []string
	ClearAliases bool
	Aggregate    *bool
	Owner        string
}

// DomainDefineResult is the plain result of a define operation.
type DomainDefineResult struct {
	Outcome vocab.Outcome
	Concept vocab.Concept
	Context string
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

// Execute create-or-updates one definition. Usage failures wrap
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
	if req.Aggregate != nil && !isEntityKind(c) {
		return DomainDefineResult{}, fmt.Errorf("%w: --aggregate is only meaningful for entity kinds", ErrDomainUsage)
	}

	lang, _, err := uc.knowledge.RecordedLanguage()
	if err != nil {
		return DomainDefineResult{}, fmt.Errorf("load domain model: %w", err)
	}

	if c == vocab.ConceptBoundedContext {
		return uc.defineBoundedContext(lang, name, req)
	}

	contextName, err := resolveDefineContext(lang, req.Context)
	if err != nil {
		return DomainDefineResult{}, err
	}

	exists, existing := lookupExisting(lang, c, contextName, name)
	if err := validateDefineUsage(c, exists, existing, req); err != nil {
		return DomainDefineResult{}, err
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
	owner := strings.TrimSpace(req.Owner)
	if owner != "" {
		ch.SetOwner = true
		ch.Owner = owner
	}

	next, result, err := lang.Define(c, contextName, name, ch)
	if err != nil {
		// Surface domain create-time owner/name failures as usage.
		msg := err.Error()
		if strings.Contains(msg, "owner must be non-empty") ||
			strings.Contains(msg, "name must be non-empty") ||
			strings.Contains(msg, "context name must be non-empty") {
			return DomainDefineResult{}, fmt.Errorf("%w: %v", ErrDomainUsage, err)
		}
		return DomainDefineResult{}, fmt.Errorf("%w", err)
	}

	if result.Outcome != vocab.OutcomeUnchanged {
		if err := uc.knowledge.Record(next); err != nil {
			return DomainDefineResult{}, fmt.Errorf("save domain model: %w", err)
		}
	}

	aliases := finalAliases(next, c, contextName, name)
	return DomainDefineResult{
		Outcome: result.Outcome,
		Concept: c,
		Context: contextName,
		Name:    name,
		Changed: result.Changed,
		Aliases: aliases,
	}, nil
}

func (uc DefineDomainDefinition) defineBoundedContext(lang vocab.UbiquitousLanguage, name string, req DefineDomainRequest) (DomainDefineResult, error) {
	if req.Definition != nil || len(req.Aliases) > 0 || req.ClearAliases || req.Aggregate != nil || strings.TrimSpace(req.Owner) != "" {
		return DomainDefineResult{}, fmt.Errorf("%w: bounded_context takes only a name", ErrDomainUsage)
	}
	// bounded_context ignores the ordinary context-resolution rule:
	// the name is the context.
	next, result, err := lang.Define(vocab.ConceptBoundedContext, name, name, vocab.Change{})
	if err != nil {
		return DomainDefineResult{}, fmt.Errorf("%w", err)
	}
	if result.Outcome != vocab.OutcomeUnchanged {
		if err := uc.knowledge.Record(next); err != nil {
			return DomainDefineResult{}, fmt.Errorf("save domain model: %w", err)
		}
	}
	return DomainDefineResult{
		Outcome: result.Outcome,
		Concept: vocab.ConceptBoundedContext,
		Context: name,
		Name:    name,
		Changed: result.Changed,
	}, nil
}

// lookupExisting reports whether the underlying stored object already
// exists. Aggregate kinds look up the Entity (not only designated
// aggregates) so designating an existing Entity is an update.
func lookupExisting(lang vocab.UbiquitousLanguage, c vocab.Concept, contextName, name string) (bool, vocab.Definition) {
	if isEntityKind(c) {
		if ent, ok := lang.FindEntity(contextName, name); ok {
			return true, ent.Definition
		}
		return false, vocab.Definition{}
	}
	def, ok := lang.Find(c, contextName, name)
	return ok, def
}

func validateDefineUsage(c vocab.Concept, exists bool, existing vocab.Definition, req DefineDomainRequest) error {
	if requiresDefinition(c) {
		if !exists {
			if req.Definition == nil || strings.TrimSpace(*req.Definition) == "" {
				return fmt.Errorf("%w: --definition is required when creating a %s", ErrDomainUsage, c)
			}
		} else if req.Definition != nil && strings.TrimSpace(*req.Definition) == "" {
			return fmt.Errorf("%w: definition cannot be cleared to empty", ErrDomainUsage)
		}
	}
	if isInvariantKind(c) {
		if !exists && strings.TrimSpace(req.Owner) == "" {
			return fmt.Errorf("%w: --owner is required when creating a %s", ErrDomainUsage, c)
		}
		if req.Definition != nil {
			return fmt.Errorf("%w: %s uses the name argument as its statement; --definition is not accepted", ErrDomainUsage, c)
		}
		if len(req.Aliases) > 0 || req.ClearAliases {
			return fmt.Errorf("%w: aliases are not accepted for %s", ErrDomainUsage, c)
		}
	}
	_ = existing
	return nil
}

// finalAliases reads the alias set after a define.
func finalAliases(lang vocab.UbiquitousLanguage, c vocab.Concept, contextName, name string) []string {
	switch c {
	case vocab.ConceptEntity, vocab.ConceptAggregate, vocab.ConceptAggregateRoot:
		if ent, ok := lang.FindEntity(contextName, name); ok {
			return append([]string(nil), ent.Aliases...)
		}
	default:
		if def, ok := lang.Find(c, contextName, name); ok {
			return append([]string(nil), def.Aliases...)
		}
	}
	return nil
}
