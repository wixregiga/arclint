package application

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// resolveDefineContext picks the bounded context for a define write.
// An explicit context always wins. An empty context is accepted only
// when the model records exactly one context.
func resolveDefineContext(lang vocab.UbiquitousLanguage, context string) (string, error) {
	context = strings.TrimSpace(context)
	if context != "" {
		return context, nil
	}
	contexts := lang.ListContexts()
	switch len(contexts) {
	case 1:
		return contexts[0].Name, nil
	case 0:
		return "", fmt.Errorf("%w: --context is required (no bounded context is recorded yet)", ErrDomainUsage)
	default:
		return "", fmt.Errorf("%w: --context is required when the project has multiple bounded contexts", ErrDomainUsage)
	}
}

// findInContexts locates a named definition for show/remove.
// An explicit context always wins. With an empty context and exactly
// one recorded context, that context is used. With multiple contexts,
// every context is searched; a unique hit wins, and hits in more than
// one context are a usage error.
func findInContexts(lang vocab.UbiquitousLanguage, c vocab.Concept, context, name string) (string, vocab.Definition, bool, error) {
	context = strings.TrimSpace(context)
	name = strings.TrimSpace(name)

	if c == vocab.ConceptBoundedContext {
		key := name
		if key == "" {
			key = context
		}
		if key == "" {
			return "", vocab.Definition{}, false, fmt.Errorf("%w: name is required", ErrDomainUsage)
		}
		if def, ok := lang.Find(c, key, key); ok {
			return key, def, true, nil
		}
		return key, vocab.Definition{}, false, nil
	}

	if context != "" {
		def, ok := lang.Find(c, context, name)
		return context, def, ok, nil
	}

	contexts := lang.ListContexts()
	switch len(contexts) {
	case 0:
		return "", vocab.Definition{}, false, nil
	case 1:
		ctxName := contexts[0].Name
		def, ok := lang.Find(c, ctxName, name)
		return ctxName, def, ok, nil
	default:
		var hits []string
		var found vocab.Definition
		for _, ctx := range contexts {
			if def, ok := lang.Find(c, ctx.Name, name); ok {
				hits = append(hits, ctx.Name)
				found = def
			}
		}
		switch len(hits) {
		case 0:
			return "", vocab.Definition{}, false, nil
		case 1:
			return hits[0], found, true, nil
		default:
			return "", vocab.Definition{}, false, fmt.Errorf(
				"%w: %s %q is defined in multiple contexts (%s); pass --context",
				ErrDomainUsage, c, name, strings.Join(hits, ", "),
			)
		}
	}
}

func definitionNotFound(c vocab.Concept, context, name string) error {
	if context == "" {
		return fmt.Errorf("no %s named %q is defined in the project domain model: %w", c, name, vocab.ErrDefinitionNotFound)
	}
	return fmt.Errorf("no %s named %q is defined in context %q of the project domain model: %w", c, name, context, vocab.ErrDefinitionNotFound)
}

func isInvariantKind(c vocab.Concept) bool {
	switch c {
	case vocab.ConceptInvariant, vocab.ConceptAssertion, vocab.ConceptBusinessRule:
		return true
	default:
		return false
	}
}

func isEntityKind(c vocab.Concept) bool {
	switch c {
	case vocab.ConceptEntity, vocab.ConceptAggregate, vocab.ConceptAggregateRoot:
		return true
	default:
		return false
	}
}

func requiresDefinition(c vocab.Concept) bool {
	switch c {
	case vocab.ConceptEntity, vocab.ConceptAggregate, vocab.ConceptAggregateRoot,
		vocab.ConceptValueObject, vocab.ConceptDomainEvent:
		return true
	default:
		return false
	}
}
