package vocab

import (
	"fmt"
	"regexp"
	"strings"
)

// contextName is ContextNamePattern compiled: the spelling a context
// name shares with a Zone name, so a Zone of rules.arclint.yaml can be
// spelled with it to narrow the context's code.
var contextName = regexp.MustCompile(ContextNamePattern)

// BoundedContext is one model: the boundary within which every recorded
// name has one meaning. It holds the context's aggregates, value
// objects, events, services, specifications, and the open questions
// the team recorded in place of a guess. The context's code is not
// recorded here: it is located from the declarations that spell the
// recorded terms, narrowed to the Zone spelled with the context's name
// when rules.arclint.yaml declares one. Line is where the context is
// written down, 0 when it is not written down yet.
type BoundedContext struct {
	Name           string
	Definition     string
	Aggregates     []Aggregate
	ValueObjects   []ValueObject
	Events         []DomainEvent
	Services       []DomainService
	Specifications []Specification
	Questions      []Question
	Line           int
}

// Question is one open question about the model, keyed so it can be
// referred to and answered. A question is never a rule; nothing is
// enforced from it.
type Question struct {
	Key  string
	Text string
	Line int
}

// Term is one recorded name of a context with the concept it is
// recorded as. An identity named by an aggregate or a member entity is
// a value object of the context whether or not it has an entry of its
// own; such a Term is marked Implied and carries no definition of its
// own. Aggregate names the aggregate a member entity belongs to, or
// the entity whose identity an implied value object is.
type Term struct {
	Name       string
	Concept    Concept
	Definition string
	Aggregate  string
	Implied    bool
	Line       int
}

// Terms returns every recorded name of the context, one Term each, in
// file order: each aggregate, then its member entities, value objects
// (recorded, then the identities implied by aggregates and members),
// events, services, specifications.
func (c BoundedContext) Terms() []Term {
	var out []Term
	for _, a := range c.Aggregates {
		out = append(out, Term{Name: a.Name, Concept: ConceptAggregate, Definition: a.Definition, Line: a.Line})
		for _, e := range a.Entities {
			out = append(out, Term{Name: e.Name, Concept: ConceptEntity, Definition: e.Definition, Aggregate: a.Name, Line: e.Line})
		}
	}
	for _, v := range c.ValueObjects {
		out = append(out, Term{Name: v.Name, Concept: ConceptValueObject, Definition: v.Definition, Line: v.Line})
	}
	for _, a := range c.Aggregates {
		if _, recorded := c.recordedAs(a.Identity); !recorded && a.Identity != "" {
			out = append(out, Term{Name: a.Identity, Concept: ConceptValueObject, Aggregate: a.Name, Implied: true, Line: a.Line})
		}
		for _, e := range a.Entities {
			if _, recorded := c.recordedAs(e.Identity); recorded || e.Identity == "" {
				continue
			}
			out = append(out, Term{Name: e.Identity, Concept: ConceptValueObject, Aggregate: a.Name, Implied: true, Line: e.Line})
		}
	}
	for _, e := range c.Events {
		out = append(out, Term{Name: e.Name, Concept: ConceptDomainEvent, Definition: e.Definition, Line: e.Line})
	}
	for _, s := range c.Services {
		out = append(out, Term{Name: s.Name, Concept: ConceptDomainService, Definition: s.Definition, Line: s.Line})
	}
	for _, s := range c.Specifications {
		out = append(out, Term{Name: s.Name, Concept: ConceptSpecification, Definition: s.Definition, Line: s.Line})
	}
	return dedupeTerms(out)
}

// dedupeTerms keeps the first Term per name so an identity two entities
// share is one implied value object.
func dedupeTerms(in []Term) []Term {
	seen := map[string]bool{}
	out := make([]Term, 0, len(in))
	for _, t := range in {
		if seen[t.Name] {
			continue
		}
		seen[t.Name] = true
		out = append(out, t)
	}
	return out
}

// Term finds one recorded name of the context.
func (c BoundedContext) Term(name string) (Term, bool) {
	for _, t := range c.Terms() {
		if t.Name == name {
			return t, true
		}
	}
	return Term{}, false
}

// Aggregate finds one aggregate by name.
func (c BoundedContext) Aggregate(name string) (Aggregate, bool) {
	for _, a := range c.Aggregates {
		if a.Name == name {
			return a, true
		}
	}
	return Aggregate{}, false
}

// Entity finds one member entity by name, with the aggregate it
// belongs to.
func (c BoundedContext) Entity(name string) (Entity, Aggregate, bool) {
	for _, a := range c.Aggregates {
		if e, ok := a.Entity(name); ok {
			return e, a, true
		}
	}
	return Entity{}, Aggregate{}, false
}

// ValueObject finds one recorded value object by name. An implied
// identity has no entry and is not found here; Term finds it.
func (c BoundedContext) ValueObject(name string) (ValueObject, bool) {
	for _, v := range c.ValueObjects {
		if v.Name == name {
			return v, true
		}
	}
	return ValueObject{}, false
}

// Event finds one domain event by name.
func (c BoundedContext) Event(name string) (DomainEvent, bool) {
	for _, e := range c.Events {
		if e.Name == name {
			return e, true
		}
	}
	return DomainEvent{}, false
}

// Service finds one domain service by name.
func (c BoundedContext) Service(name string) (DomainService, bool) {
	for _, s := range c.Services {
		if s.Name == name {
			return s, true
		}
	}
	return DomainService{}, false
}

// Specification finds one specification by name.
func (c BoundedContext) Specification(name string) (Specification, bool) {
	for _, s := range c.Specifications {
		if s.Name == name {
			return s, true
		}
	}
	return Specification{}, false
}

// Question finds one open question by key.
func (c BoundedContext) Question(key string) (Question, bool) {
	for _, q := range c.Questions {
		if q.Key == key {
			return q, true
		}
	}
	return Question{}, false
}

// Invariants returns every invariant of the context with its owner:
// each aggregate's, then each value object's, in file order.
func (c BoundedContext) Invariants() []OwnedInvariant {
	var out []OwnedInvariant
	for _, a := range c.Aggregates {
		for _, inv := range a.Invariants {
			out = append(out, OwnedInvariant{Context: c.Name, Owner: a.Name, OwnerConcept: ConceptAggregate, Invariant: inv})
		}
	}
	for _, v := range c.ValueObjects {
		for _, inv := range v.Invariants {
			out = append(out, OwnedInvariant{Context: c.Name, Owner: v.Name, OwnerConcept: ConceptValueObject, Invariant: inv})
		}
	}
	return out
}

// Assertions returns every assertion of the context with the aggregate
// that owns it, in file order.
func (c BoundedContext) Assertions() []OwnedAssertion {
	var out []OwnedAssertion
	for _, a := range c.Aggregates {
		for _, as := range a.Assertions {
			out = append(out, OwnedAssertion{Context: c.Name, Owner: a.Name, Assertion: as})
		}
	}
	return out
}

// validate applies the context's block invariants and each nested
// block's, as one consistency unit.
func (c BoundedContext) validate() error {
	if err := requireName(c.Line, "context", c.Name); err != nil {
		return fmt.Errorf("bounded_context/named-once: %scontext has an empty name", at(c.Line))
	}
	if !contextName.MatchString(c.Name) {
		return fmt.Errorf("bounded_context/named-once: %scontext %q: a context name is spelled like a Zone name: lowercase, starting with a letter, then letters, digits, _ or -", at(c.Line), c.Name)
	}
	if err := requireDefinition(c.Line, "context", c.Name, c.Definition); err != nil {
		return err
	}
	names := map[string]string{}
	claim := func(line int, what, name string) error {
		if err := requireName(line, what, name); err != nil {
			return err
		}
		if prev, dup := names[name]; dup {
			return fmt.Errorf("ubiquitous_language/one-meaning-per-name: %scontext %q: %q is recorded as %s and again as %s",
				at(line), c.Name, name, prev, what)
		}
		names[name] = what
		return nil
	}
	for _, a := range c.Aggregates {
		if err := claim(a.Line, "an aggregate", a.Name); err != nil {
			return err
		}
	}
	for _, a := range c.Aggregates {
		for _, e := range a.Entities {
			if err := claim(e.Line, "an entity of "+a.Name, e.Name); err != nil {
				return err
			}
		}
	}
	for _, v := range c.ValueObjects {
		if err := claim(v.Line, "a value object", v.Name); err != nil {
			return err
		}
	}
	for _, e := range c.Events {
		if err := claim(e.Line, "an event", e.Name); err != nil {
			return err
		}
	}
	for _, s := range c.Services {
		if err := claim(s.Line, "a service", s.Name); err != nil {
			return err
		}
	}
	for _, s := range c.Specifications {
		if err := claim(s.Line, "a specification", s.Name); err != nil {
			return err
		}
	}
	for _, a := range c.Aggregates {
		if err := a.validate(c); err != nil {
			return err
		}
	}
	for _, v := range c.ValueObjects {
		if err := v.validate(c); err != nil {
			return err
		}
	}
	for _, e := range c.Events {
		if err := e.validate(c); err != nil {
			return err
		}
	}
	for _, s := range c.Services {
		if err := s.validate(); err != nil {
			return err
		}
	}
	for _, s := range c.Specifications {
		if err := s.validate(); err != nil {
			return err
		}
	}
	keys := map[string]bool{}
	for _, q := range c.Questions {
		if wrong := keySpelling(q.Key); wrong != "" {
			return fmt.Errorf("%scontext %q: question key %q %s", at(q.Line), c.Name, q.Key, wrong)
		}
		if keys[q.Key] {
			return fmt.Errorf("%scontext %q: question %q is recorded twice", at(q.Line), c.Name, q.Key)
		}
		keys[q.Key] = true
		if strings.TrimSpace(q.Text) == "" {
			return fmt.Errorf("%scontext %q: question %q has no text", at(q.Line), c.Name, q.Key)
		}
	}
	return nil
}

// recordedAs reports what the context records under a name: an entry
// of its own, never an implied identity.
func (c BoundedContext) recordedAs(name string) (Concept, bool) {
	for _, a := range c.Aggregates {
		if a.Name == name {
			return ConceptAggregate, true
		}
		for _, e := range a.Entities {
			if e.Name == name {
				return ConceptEntity, true
			}
		}
	}
	if _, ok := c.ValueObject(name); ok {
		return ConceptValueObject, true
	}
	if _, ok := c.Event(name); ok {
		return ConceptDomainEvent, true
	}
	if _, ok := c.Service(name); ok {
		return ConceptDomainService, true
	}
	if _, ok := c.Specification(name); ok {
		return ConceptSpecification, true
	}
	return "", false
}

func (c BoundedContext) clone() BoundedContext {
	out := c
	out.Aggregates = nil
	for _, a := range c.Aggregates {
		out.Aggregates = append(out.Aggregates, a.clone())
	}
	out.ValueObjects = nil
	for _, v := range c.ValueObjects {
		out.ValueObjects = append(out.ValueObjects, v.clone())
	}
	out.Events = cloneSlice(c.Events)
	out.Services = cloneSlice(c.Services)
	out.Specifications = cloneSlice(c.Specifications)
	out.Questions = cloneSlice(c.Questions)
	return out
}

func cloneSlice[T any](in []T) []T {
	if in == nil {
		return nil
	}
	return append([]T(nil), in...)
}
