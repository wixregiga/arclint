package vocab

import (
	"fmt"
	"strings"
)

// SourcedStatement is guidance arclint repeats word for word at each
// moment an agent decides where behavior goes, with the works that
// support it, so every place that states it states the same thing.
// Sources cite the meta-model's bibliography by key, so the bibliography
// check covers them and every reader sees each work's standing.
type SourcedStatement struct {
	Text    string
	Sources []Citation
}

// String spells the statement with its sources before the final period.
func (s SourcedStatement) String() string {
	return strings.TrimSuffix(s.Text, ".") + " (" + s.Cited() + ")."
}

// Cited spells the statement's sources for a sentence that cites them:
// each work once, inline, with the narrowing parts of every consecutive
// citation of it and its standing.
func (s SourcedStatement) Cited() string {
	return DDD().Inline(s.Sources)
}

// SourcedStatements lists every SourcedStatement arclint repeats, so the
// bibliography check covers each one.
func SourcedStatements() []SourcedStatement {
	return []SourcedStatement{DomainDecides(), SharedMechanism()}
}

// The bibliography keys of the works the agent-facing guidance cites,
// and the sections of the Reference it names.
const (
	anemicDomainModel          = "fowler-anemic-domain-model"
	layeredArchitectureSection = "Layered Architecture"
)

// DomainDecides says where a decision lives, in Evans's own words: the
// domain model is isolated, and the application layer holds no business
// rule. The AGENTS.md workflow states it.
func DomainDecides() SourcedStatement {
	return SourcedStatement{
		Text: `Evans: "Isolate the expression of the domain model and the business logic, and eliminate any dependency on infrastructure, user interface, or even application logic that is not business logic." ` +
			`The application layer "does not contain business rules or knowledge, but only coordinates tasks and delegates work to collaborations of domain objects in the next layer down."`,
		Sources: []Citation{
			{Work: evansReference, Page: "10", Section: layeredArchitectureSection},
			{Work: anemicDomainModel, Note: "quoting Evans 2003, Application Layer"},
		},
	}
}

// SharedMechanism is the design move the remediation of a refused
// import names: a shared mechanism gets a Zone of its own, allowed where
// it is needed and free of product decisions.
func SharedMechanism() SourcedStatement {
	return SourcedStatement{
		Text:    "A shared mechanism gets a Zone of its own, allowed where it is needed and free of product decisions.",
		Sources: []Citation{{Work: evansReference, Page: "44", Section: "Cohesive Mechanisms"}},
	}
}

// DomainService is one recorded domain service of a context: a
// stateless operation that makes a business decision no single
// aggregate or value object can make alone. It may read through
// repositories, but, by ArcLint's own convention, which no cited source
// states, it never commits and never publishes; the application service
// that calls it does. Its assertions are its
// contract: the guarantees its operations give, each checked by the
// operation it constrains.
type DomainService struct {
	Name       string
	Definition string
	Assertions []Assertion
	Line       int
}

// Assertion finds one assertion of the service by key.
func (s DomainService) Assertion(key string) (Assertion, bool) {
	for _, as := range s.Assertions {
		if as.Key == key {
			return as, true
		}
	}
	return Assertion{}, false
}

// validate applies the domain service block's loader invariants and
// those of the assertions it owns.
func (s DomainService) validate(c BoundedContext) error {
	if err := requireDefinition(s.Line, "service", s.Name, s.Definition); err != nil {
		return err
	}
	where := fmt.Sprintf("context %q: service %q", c.Name, s.Name)
	keys := map[string]bool{}
	for _, as := range s.Assertions {
		if err := as.validate(where); err != nil {
			return err
		}
		if keys[as.Key] {
			return fmt.Errorf("invariant/keyed-within-owner: %s%s: assertion %q is recorded twice", at(as.Line), where, as.Key)
		}
		keys[as.Key] = true
	}
	return nil
}

func (s DomainService) clone() DomainService {
	out := s
	out.Assertions = cloneAssertions(s.Assertions)
	return out
}
