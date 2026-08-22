// Package vocab holds the project's recorded Ubiquitous Language
// vocabulary: the domain definitions a project declares in
// ubiquitous-language.yaml, the ArcLint-owned concept kinds and their
// meanings, the mutation semantics for maintaining the vocabulary, its
// published JSON Schema, and the Repository port that persists it.
// UbiquitousLanguage is a value with invariants, not a second aggregate
// root: Rule stays the sole aggregate.
package vocab

import (
	"fmt"
	"strings"
)

// Concept is one value from the finite ArcLint-owned set of domain
// concept kinds recorded in a project's Ubiquitous Language.
type Concept string

// The published concept kinds. Aggregate is an Entity designation, not
// a separate stored object.
const (
	ConceptEntity       Concept = "entity"
	ConceptAggregate    Concept = "aggregate"
	ConceptValueObject  Concept = "value-object"
	ConceptBusinessRule Concept = "business-rule"
	ConceptEvent        Concept = "event"
)

// Concepts returns the published enum in stable explain order.
func Concepts() []Concept {
	return []Concept{
		ConceptEntity,
		ConceptAggregate,
		ConceptValueObject,
		ConceptBusinessRule,
		ConceptEvent,
	}
}

// ParseConcept accepts a singular concept spelling.
func ParseConcept(s string) (Concept, error) {
	for _, c := range Concepts() {
		if Concept(s) == c {
			return c, nil
		}
	}
	return "", fmt.Errorf("domain concept %q: not one of %s", s, joinConcepts(Concepts()))
}

// ParseListing accepts a plural listing spelling
// (entities|aggregates|value-objects|business-rules|events).
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
	case ConceptAggregate:
		return "aggregates"
	case ConceptValueObject:
		return "value-objects"
	case ConceptBusinessRule:
		return "business-rules"
	case ConceptEvent:
		return "events"
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
	// concept ("The project supplies the Entity's name, definition, and aliases.").
	Supplies string
}

// Doc returns the ArcLint-owned documentation for this Concept.
func (c Concept) Doc() ConceptDoc {
	switch c {
	case ConceptEntity:
		return ConceptDoc{
			Concept: ConceptEntity,
			Title:   "Entity",
			Meaning: "A domain concept whose identity matters as it changes over time.",
			Questions: []string{
				"What must the project distinguish from other similar things?",
				"What remains the same thing even when its attributes change?",
			},
			Supplies: "The project supplies the Entity's name, definition, and aliases.",
		}
	case ConceptAggregate:
		return ConceptDoc{
			Concept: ConceptAggregate,
			Title:   "Aggregate",
			Meaning: "An Entity the project treats as a consistency boundary: it is changed as one unit and other objects reach it through its identity.",
			Questions: []string{
				"Which Entity must stay internally consistent when the project changes it?",
				"Which Entity do other objects reference by identity rather than reach inside?",
			},
			Supplies: "The project supplies the Aggregate's name, definition, and aliases.",
		}
	case ConceptValueObject:
		return ConceptDoc{
			Concept: ConceptValueObject,
			Title:   "Value Object",
			Meaning: "A domain value defined entirely by its attributes, with no identity of its own.",
			Questions: []string{
				"Can two occurrences with the same attributes be used interchangeably?",
				"Does replacing it with an equal value change nothing?",
			},
			Supplies: "The project supplies the Value Object's name, definition, and aliases.",
		}
	case ConceptBusinessRule:
		return ConceptDoc{
			Concept: ConceptBusinessRule,
			Title:   "Business Rule",
			Meaning: "A statement the project requires to always or never be true about its domain.",
			Questions: []string{
				"What must always hold for the project's data or behavior?",
				"What must never happen, regardless of implementation?",
			},
			Supplies: "The project supplies the Business Rule's name, definition, and aliases.",
		}
	case ConceptEvent:
		return ConceptDoc{
			Concept: ConceptEvent,
			Title:   "Domain Event",
			Meaning: "Something that has completed in the domain and that the project cares to record.",
			Questions: []string{
				"What completed occurrence do other parts of the project react to?",
				"What would the project mention in its history of what happened?",
			},
			Supplies: "The project supplies the Domain Event's name, definition, and aliases.",
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
