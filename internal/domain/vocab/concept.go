// Package vocab holds the project's recorded Ubiquitous Language
// vocabulary: the domain definitions a project declares in
// domain.arclint.yaml, the Domain-Driven Design meta-model that defines
// every concept arclint speaks about, the mutation semantics for
// maintaining the vocabulary, its published JSON Schema, the
// domain-librarian skill taxonomy, and the Repository port that persists
// it. UbiquitousLanguage is a value with invariants, not a second
// aggregate root: Rule stays the sole aggregate.
package vocab

import (
	"fmt"
	"strings"
)

// Concept is one value from the finite ArcLint-owned set of domain
// concept kinds a project records. Every Concept is the term of a
// building block of the meta-model, which is where its meaning lives.
// Spellings use underscores (hyphen forms are rejected).
type Concept string

// The published concept kinds. An aggregate is recorded as its root;
// aggregate_root is the same entry. An entity is a member of an
// aggregate. business_rule always resolves to an invariant or an
// assertion, never to its own section. bounded_context is the context
// itself. question is an open question of a context, never a rule.
const (
	ConceptEntity         Concept = "entity"
	ConceptValueObject    Concept = "value_object"
	ConceptInvariant      Concept = "invariant"
	ConceptAssertion      Concept = "assertion"
	ConceptSpecification  Concept = "specification"
	ConceptAggregate      Concept = "aggregate"
	ConceptAggregateRoot  Concept = "aggregate_root"
	ConceptDomainEvent    Concept = "domain_event"
	ConceptDomainService  Concept = "domain_service"
	ConceptBoundedContext Concept = "bounded_context"
	ConceptBusinessRule   Concept = "business_rule"
	ConceptQuestion       Concept = "question"
)

// Concepts returns the published enum in stable explain order.
func Concepts() []Concept {
	return []Concept{
		ConceptEntity,
		ConceptValueObject,
		ConceptInvariant,
		ConceptAssertion,
		ConceptSpecification,
		ConceptAggregate,
		ConceptAggregateRoot,
		ConceptDomainEvent,
		ConceptDomainService,
		ConceptBoundedContext,
		ConceptBusinessRule,
		ConceptQuestion,
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

// Listing returns the plural spelling used by list filters and
// headers: the domain file's section name wherever the concept has a
// section.
func Listing(c Concept) string {
	switch c {
	case ConceptEntity:
		return string(SectionEntities)
	case ConceptValueObject:
		return string(SectionValueObjects)
	case ConceptInvariant:
		return string(SectionInvariants)
	case ConceptAssertion:
		return string(SectionAssertions)
	case ConceptSpecification:
		return string(SectionSpecifications)
	case ConceptAggregate:
		return string(SectionAggregates)
	case ConceptAggregateRoot:
		return "aggregate_roots"
	case ConceptDomainEvent:
		return string(SectionEvents)
	case ConceptDomainService:
		return string(SectionServices)
	case ConceptBoundedContext:
		return string(SectionContexts)
	case ConceptBusinessRule:
		return "business_rules"
	case ConceptQuestion:
		return string(SectionQuestions)
	default:
		return string(c)
	}
}

// ConceptDoc is the ArcLint-owned meaning of one Concept: the single
// source of truth reused by help, explain, guided authoring, JSON
// output, docs, and the extension SDK. Title, Meaning, and Sources are
// the meta-model's building block of the same term; Questions and
// Supplies are arclint's authoring guidance for recording one.
type ConceptDoc struct {
	Concept   Concept
	Title     string
	Meaning   string
	Sources   []Reference
	Questions []string
	// Supplies is the closer naming what the project records for this
	// concept.
	Supplies string
}

// Doc returns the ArcLint-owned documentation for this Concept.
func (c Concept) Doc() ConceptDoc {
	doc := ConceptDoc{Concept: c, Title: string(c)}
	model := DDD()
	if block, ok := model.Block(string(c)); ok {
		doc.Title = block.Title
		doc.Meaning = block.Definition.Text
		doc.Sources = model.References(block.Definition.Sources)
	}
	switch c {
	case ConceptEntity:
		doc.Questions = []string{
			"Does this have an identity that survives attribute changes?",
			"Which aggregate does it belong to, and what tells two of them apart inside it?",
		}
		doc.Supplies = "The project supplies the member entity's name, definition, and the aggregate it belongs to; its local identity and aliases are optional."
	case ConceptValueObject:
		doc.Questions = []string{
			"Are two instances with identical values interchangeable?",
			"Does replacing it with an equal value change nothing?",
		}
		doc.Supplies = "The project supplies the value object's name, definition, and aliases, and the invariants every value of its kind satisfies."
	case ConceptInvariant:
		doc.Questions = []string{
			"What must never be violated, even for an instant?",
			"What concrete violation does this forbid a naive implementation from doing?",
		}
		doc.Supplies = "The project supplies the invariant's key, its statement, and the one aggregate or value object it is recorded under."
	case ConceptAssertion:
		doc.Questions = []string{
			"Does this hold when a named operation occurs, rather than at all times?",
			"Which operation must call the method that checks it?",
		}
		doc.Supplies = "The project supplies the assertion's key, its statement, the aggregate it is recorded under, and the root operation it is on."
	case ConceptSpecification:
		doc.Questions = []string{
			"Do experts pass this predicate around as a thing, not just a rule that holds?",
			"Would you say this to an expert who never saw the language?",
		}
		doc.Supplies = "The project supplies the specification's name and definition; source shows a type of that name with a satisfaction method."
	case ConceptAggregate:
		doc.Questions = []string{
			"What is the smallest cluster that must stay consistent in one transaction?",
			"Which entity do other objects reference by identity rather than reach inside?",
		}
		doc.Supplies = "The project supplies the aggregate's name, definition, and identity; its member entities, invariants, assertions, repository, and factory are recorded under it."
	case ConceptAggregateRoot:
		doc.Questions = []string{
			"Which single entity is the entry point of the aggregate?",
			"What must stay internally consistent when the project changes this cluster?",
		}
		doc.Supplies = "The project records the aggregate root as the aggregate's own entry: the aggregate's name is the root's name and its identity is the root's."
	case ConceptDomainEvent:
		doc.Questions = []string{
			"What completed occurrence do experts name in past tense?",
			"What would the project mention in its history of what happened?",
		}
		doc.Supplies = "The project supplies the domain event's name and definition, and the aggregate that raises it when the model records that."
	case ConceptDomainService:
		doc.Questions = []string{
			"Is this an operation no aggregate or value object is the natural home of?",
			"Which aggregates does it span?",
		}
		doc.Supplies = "The project supplies the domain service's name and definition."
	case ConceptBoundedContext:
		doc.Questions = []string{
			"Which people or teams use this term, and do they mean the same thing?",
			"Is a party that must be informed its own context?",
		}
		doc.Supplies = "The project supplies the bounded context's name and definition and the terms inside it; its code is located from the declarations that spell those terms."
	case ConceptBusinessRule:
		doc.Questions = []string{
			"Does this resolve to an invariant or an assertion?",
			"Which aggregate or value object owns enforcement?",
		}
		// business_rule always resolves to invariant or assertion; never specification.
		doc.Supplies = "The project records a business_rule as an invariant or an assertion under one owner; it is never stored as its own section, and it never becomes a specification."
	case ConceptQuestion:
		doc.Questions = []string{
			"What about the model is unsettled, and who can settle it?",
			"Is this a question, or a decision nobody has written down?",
		}
		doc.Supplies = "The project supplies the question's key and its text under the context it is about; nothing is enforced from it."
	}
	return doc
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
