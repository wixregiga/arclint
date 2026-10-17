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
	return []SourcedStatement{DomainDecides(), ServiceDecides(), MovesBetweenLayers(), SharedMechanism()}
}

// The bibliography keys of the works the agent-facing guidance cites,
// and the sections of the Reference it names.
const (
	anemicDomainModel          = "fowler-anemic-domain-model"
	kindsOfLogic               = "stafford-business-logic"
	goSpec                     = "go-spec"
	iddSamples                 = "vernon-iddd-samples"
	layeredArchitectureSection = "Layered Architecture"
	valueObjectsSection        = "Value Objects"
	servicesSection            = "Services"
	sideEffectFreeSection      = "Side-Effect-Free Functions"
	assertionsSection          = "Assertions"
)

// DomainDecides says where a decision lives and where its change is
// made: the domain decides, the application carries the change out and
// holds no business rule. The librarian, the remediation of a service's
// unchecked assertion, and the AGENTS.md workflow state it.
func DomainDecides() SourcedStatement {
	return SourcedStatement{
		Text: "The domain decides; the application gathers what the decision needs and carries out the change. " +
			"The application layer holds no business rule: it only coordinates tasks and delegates work to the domain.",
		Sources: []Citation{
			{Work: evansReference, Page: "10", Section: layeredArchitectureSection, Note: "on isolating the domain"},
			{Work: anemicDomainModel, Note: "quoting Evans 2003, on no business rules or knowledge"},
			{Work: kindsOfLogic, Note: "on application logic"},
		},
	}
}

// DecideThenCarryOut states DomainDecides and then ServiceDecides, its
// application to a domain service: the librarian's distillation rule
// and the remediation of a service's unchecked assertion say it in
// these words.
func DecideThenCarryOut() string {
	return DomainDecides().String() + " " + ServiceDecides().String()
}

// ServiceDecides applies DomainDecides to a domain service whose
// operation would change something outside the model: the assertion
// goes on the operation that decides, its verdict is a value, the
// application service makes the change only with a verdict that allows
// it, and a test proves what the domain cannot see. It names no
// operation or type, and it guides without adding a rule.
func ServiceDecides() SourcedStatement {
	return SourcedStatement{
		Text: "When an operation of a domain service would change something outside the model, such as a file, a store or another system, " +
			"its assertion goes on the operation that decides. " +
			"That operation takes what the decision needs as values, changes nothing, and returns its verdict as a value; " +
			"a verdict that carries a rule, such as approving only when no participant refused, is recorded as a value object whose invariant states that rule, so `value_object/constructed-through-one-door` requires it to declare a constructor. " +
			"The application service gathers those values, asks for the verdict, and makes the change only with a verdict that allows it in hand. " +
			"Go makes that verdict hard to forge with unexported fields and a zero value that allows nothing, " +
			"TypeScript with a private constructor and a private field, and Python only by convention, with an underscore-prefixed name. " +
			"An automated test proves what the domain cannot see, such as a refusal changing nothing on disk.",
		Sources: []Citation{
			{Work: evansReference, Page: "12", Section: valueObjectsSection},
			{Work: evansReference, Page: "14", Section: servicesSection},
			{Work: evansReference, Page: "21", Section: sideEffectFreeSection},
			{Work: evansReference, Page: "22", Section: assertionsSection},
			{Work: goSpec, Section: "Exported identifiers"},
			{Work: goSpec, Section: "Struct literals"},
			{Work: "typescript-2-0", Section: "Private and Protected Constructors"},
			{Work: "typescript-handbook-type-compatibility", Section: "Private and protected members in classes"},
			{Work: "python-tutorial-classes", Section: "9.6 Private Variables"},
		},
	}
}

// MovesBetweenLayers is the placement the librarian recommends when a
// recording or a finding would move code between layers: a pure
// decision operation in the domain and the gathering and the changes in
// the application service.
func MovesBetweenLayers() SourcedStatement {
	return SourcedStatement{
		Text: "Then recommend the placement decide-then-carry-out names: a pure decision operation in the domain, values in and verdict out, " +
			"and the gathering and the changes in the application service.",
		Sources: []Citation{
			{Work: evansReference, Page: "10", Section: layeredArchitectureSection},
			{Work: evansReference, Page: "21", Section: sideEffectFreeSection},
			{Work: kindsOfLogic},
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
