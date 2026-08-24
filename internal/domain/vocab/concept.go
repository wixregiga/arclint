// Package vocab holds the project's recorded Ubiquitous Language
// vocabulary: the domain definitions a project declares in
// ubiquitous-language.yaml, the ArcLint-owned concept kinds and their
// meanings, the mutation semantics for maintaining the vocabulary, its
// published JSON Schema, the domain-librarian skill taxonomy, and the
// Repository port that persists it. UbiquitousLanguage is a value with
// invariants, not a second aggregate root: Rule stays the sole aggregate.
package vocab

import (
	"fmt"
	"strings"
)

// Concept is one value from the finite ArcLint-owned set of domain
// concept kinds. Spellings use underscores (hyphen forms are rejected).
type Concept string

// The published concept kinds. Aggregate and AggregateRoot are Entity
// designations, not separate stored objects. business_rule and
// assertion always resolve to an Invariant entry. domain_event records
// into the events section. bounded_context is the context itself.
const (
	ConceptEntity         Concept = "entity"
	ConceptValueObject    Concept = "value_object"
	ConceptInvariant      Concept = "invariant"
	ConceptAssertion      Concept = "assertion"
	ConceptAggregate      Concept = "aggregate"
	ConceptAggregateRoot  Concept = "aggregate_root"
	ConceptDomainEvent    Concept = "domain_event"
	ConceptBoundedContext Concept = "bounded_context"
	ConceptBusinessRule   Concept = "business_rule"
)

// Concepts returns the published enum in stable explain order.
func Concepts() []Concept {
	return []Concept{
		ConceptEntity,
		ConceptValueObject,
		ConceptInvariant,
		ConceptAssertion,
		ConceptAggregate,
		ConceptAggregateRoot,
		ConceptDomainEvent,
		ConceptBoundedContext,
		ConceptBusinessRule,
	}
}

// ParseConcept accepts a singular concept spelling (underscore form).
func ParseConcept(s string) (Concept, error) {
	for _, c := range Concepts() {
		if Concept(s) == c {
			return c, nil
		}
	}
	return "", fmt.Errorf("domain concept %q: not one of %s", s, joinConcepts(Concepts()))
}

// ParseListing accepts a plural listing spelling.
func ParseListing(s string) (Concept, error) {
	for _, c := range Concepts() {
		if Listing(c) == s {
			return c, nil
		}
	}
	return "", fmt.Errorf("domain listing %q: not one of %s", s, joinListings(Concepts()))
}

// Listing returns the plural spelling used by list filters and headers.
func Listing(c Concept) string {
	switch c {
	case ConceptEntity:
		return "entities"
	case ConceptValueObject:
		return "value_objects"
	case ConceptInvariant:
		return "invariants"
	case ConceptAssertion:
		return "assertions"
	case ConceptAggregate:
		return "aggregates"
	case ConceptAggregateRoot:
		return "aggregate_roots"
	case ConceptDomainEvent:
		return "domain_events"
	case ConceptBoundedContext:
		return "bounded_contexts"
	case ConceptBusinessRule:
		return "business_rules"
	default:
		return string(c)
	}
}

// ConceptDoc is the ArcLint-owned meaning of one Concept: the single
// source of truth reused by help, explain, guided authoring, JSON
// output, docs, and the extension SDK.
type ConceptDoc struct {
	Concept   Concept
	Title     string
	Meaning   string
	Questions []string
	// Supplies is the closer naming what the project records for this
	// concept.
	Supplies string
}

// Doc returns the ArcLint-owned documentation for this Concept.
// Meaning text is the vocabulary term one-liner from VOCAB.yaml.
func (c Concept) Doc() ConceptDoc {
	switch c {
	case ConceptEntity:
		return ConceptDoc{
			Concept: ConceptEntity,
			Title:   "Entity",
			Meaning: TermDefinition(TermEntity),
			Questions: []string{
				"Does this have an identity that survives attribute changes?",
				"What must the project distinguish from other similar things?",
			},
			Supplies: "The project supplies the Entity's name, definition, and aliases.",
		}
	case ConceptValueObject:
		return ConceptDoc{
			Concept: ConceptValueObject,
			Title:   "Value Object",
			Meaning: TermDefinition(TermValueObject),
			Questions: []string{
				"Are two instances with identical values interchangeable?",
				"Does replacing it with an equal value change nothing?",
			},
			Supplies: "The project supplies the Value Object's name, definition, and aliases.",
		}
	case ConceptInvariant:
		return ConceptDoc{
			Concept: ConceptInvariant,
			Title:   "Invariant",
			Meaning: TermDefinition(TermInvariant),
			Questions: []string{
				"What must never be violated, even for an instant?",
				"What concrete violation does this forbid a naive implementation from doing?",
			},
			Supplies: "The project supplies the Invariant's statement and exactly one owner.",
		}
	case ConceptAssertion:
		return ConceptDoc{
			Concept: ConceptAssertion,
			Title:   "Assertion",
			Meaning: TermDefinition(TermAssertion),
			Questions: []string{
				"What post-condition must hold after an operation?",
				"Is this an invariant restated at an operation boundary?",
			},
			Supplies: "The project supplies the Assertion as an invariant statement with exactly one owner.",
		}
	case ConceptAggregate:
		return ConceptDoc{
			Concept: ConceptAggregate,
			Title:   "Aggregate",
			Meaning: TermDefinition(TermAggregate),
			Questions: []string{
				"What is the smallest cluster that must stay consistent in one transaction?",
				"Which Entity do other objects reference by identity rather than reach inside?",
			},
			Supplies: "The project supplies the Aggregate as an Entity designation (aggregate: true).",
		}
	case ConceptAggregateRoot:
		return ConceptDoc{
			Concept: ConceptAggregateRoot,
			Title:   "Aggregate Root",
			Meaning: TermDefinition(TermAggregateRoot),
			Questions: []string{
				"Which single entity is the entry point of the aggregate?",
				"What must stay internally consistent when the project changes this cluster?",
			},
			Supplies: "The project supplies the Aggregate Root as an Entity designation (aggregate: true).",
		}
	case ConceptDomainEvent:
		return ConceptDoc{
			Concept: ConceptDomainEvent,
			Title:   "Domain Event",
			Meaning: TermDefinition(TermDomainEvent),
			Questions: []string{
				"What completed occurrence do experts name in past tense?",
				"What would the project mention in its history of what happened?",
			},
			Supplies: "The project supplies the Domain Event's name and definition.",
		}
	case ConceptBoundedContext:
		return ConceptDoc{
			Concept: ConceptBoundedContext,
			Title:   "Bounded Context",
			Meaning: TermDefinition(TermBoundedContext),
			Questions: []string{
				"Which people or teams use this term, and do they mean the same thing?",
				"Is a party that must be informed its own context?",
			},
			Supplies: "The project supplies the Bounded Context's name and the terms inside it.",
		}
	case ConceptBusinessRule:
		return ConceptDoc{
			Concept: ConceptBusinessRule,
			Title:   "Business Rule",
			Meaning: TermDefinition(TermBusinessRule),
			Questions: []string{
				"Does this resolve to an invariant or an assertion?",
				"Which entity, aggregate root, or value object owns enforcement?",
			},
			// business_rule always resolves to invariant or assertion with an owner.
			Supplies: "The project records a business_rule as an invariant or assertion with exactly one owner; it is never stored as its own section.",
		}
	default:
		return ConceptDoc{Concept: c, Title: string(c)}
	}
}

func joinConcepts(cs []Concept) string {
	parts := make([]string, len(cs))
	for i, c := range cs {
		parts[i] = string(c)
	}
	return strings.Join(parts, ", ")
}

func joinListings(cs []Concept) string {
	parts := make([]string, len(cs))
	for i, c := range cs {
		parts[i] = Listing(c)
	}
	return strings.Join(parts, ", ")
}
