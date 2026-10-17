package vocab_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

func TestConceptsOrder(t *testing.T) {
	got := vocab.Concepts()
	want := []vocab.Concept{
		vocab.ConceptEntity,
		vocab.ConceptValueObject,
		vocab.ConceptInvariant,
		vocab.ConceptAssertion,
		vocab.ConceptSpecification,
		vocab.ConceptAggregate,
		vocab.ConceptAggregateRoot,
		vocab.ConceptDomainEvent,
		vocab.ConceptDomainService,
		vocab.ConceptBoundedContext,
		vocab.ConceptBusinessRule,
		vocab.ConceptQuestion,
	}
	if len(got) != len(want) {
		t.Fatalf("Concepts() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Concepts()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseConceptRoundTrip(t *testing.T) {
	for _, c := range vocab.Concepts() {
		got, err := vocab.ParseConcept(string(c))
		if err != nil {
			t.Errorf("ParseConcept(%q): %v", c, err)
			continue
		}
		if got != c {
			t.Errorf("ParseConcept(%q) = %q, want %q", c, got, c)
		}
	}
}

func TestParseConceptRejectsHyphenAndUnknown(t *testing.T) {
	for _, s := range []string{"value-object", "business-rule", "domain-event", "bounded-context", "nope"} {
		_, err := vocab.ParseConcept(s)
		if err == nil {
			t.Fatalf("ParseConcept(%q) succeeded, want error", s)
		}
		if !strings.Contains(err.Error(), "entity") {
			t.Errorf("error %q should list accepted concepts", err)
		}
	}
}

func TestParseListingRoundTrip(t *testing.T) {
	for _, c := range vocab.Concepts() {
		listing := vocab.Listing(c)
		got, err := vocab.ParseListing(listing)
		if err != nil {
			t.Errorf("ParseListing(%q): %v", listing, err)
			continue
		}
		if got != c {
			t.Errorf("ParseListing(%q) = %q, want %q", listing, got, c)
		}
	}
}

func TestParseListingRejectsSingular(t *testing.T) {
	_, err := vocab.ParseListing("entity")
	if err == nil {
		t.Fatal("ParseListing(entity) succeeded, want error")
	}
}

func TestListingSpellings(t *testing.T) {
	cases := map[vocab.Concept]string{
		vocab.ConceptEntity:         "entities",
		vocab.ConceptValueObject:    "value_objects",
		vocab.ConceptInvariant:      "invariants",
		vocab.ConceptAssertion:      "assertions",
		vocab.ConceptSpecification:  "specifications",
		vocab.ConceptAggregate:      "aggregates",
		vocab.ConceptAggregateRoot:  "aggregate_roots",
		vocab.ConceptDomainEvent:    "events",
		vocab.ConceptDomainService:  "services",
		vocab.ConceptBoundedContext: "contexts",
		vocab.ConceptBusinessRule:   "business_rules",
		vocab.ConceptQuestion:       "questions",
	}
	for c, want := range cases {
		if got := vocab.Listing(c); got != want {
			t.Errorf("Listing(%q) = %q, want %q", c, got, want)
		}
	}
}

func TestConceptDocNonEmpty(t *testing.T) {
	for _, c := range vocab.Concepts() {
		doc := c.Doc()
		if doc.Concept != c {
			t.Errorf("%s Doc.Concept = %q", c, doc.Concept)
		}
		if doc.Title == "" {
			t.Errorf("%s Doc.Title empty", c)
		}
		if doc.Meaning == "" {
			t.Errorf("%s Doc.Meaning empty", c)
		}
		if len(doc.Questions) == 0 {
			t.Errorf("%s Doc.Questions empty", c)
		}
		for _, q := range doc.Questions {
			if strings.TrimSpace(q) == "" {
				t.Errorf("%s Doc has blank question", c)
			}
		}
		if doc.Supplies == "" {
			t.Errorf("%s Doc.Supplies empty", c)
		}
	}
}

// Every Concept is the term of a meta-model building block, and its
// documentation is that block's title, definition, and sources.
func TestConceptDocsComeFromTheMetaModel(t *testing.T) {
	model := vocab.DDD()
	for _, c := range vocab.Concepts() {
		block, ok := model.Block(string(c))
		if !ok {
			t.Errorf("%s: no building block of that term in the meta-model", c)
			continue
		}
		doc := c.Doc()
		if doc.Title != block.Title {
			t.Errorf("%s Title = %q, want the block's %q", c, doc.Title, block.Title)
		}
		if doc.Meaning != block.Definition.Labelled() {
			t.Errorf("%s Meaning = %q, want the block's definition with its conventions labelled", c, doc.Meaning)
		}
		if len(doc.Sources) != len(block.Definition.Sources) {
			t.Errorf("%s: %d sources, the block cites %d", c, len(doc.Sources), len(block.Definition.Sources))
		}
		for _, s := range doc.Sources {
			if s.Work.Title == "" {
				t.Errorf("%s: a source resolved to no work", c)
			}
		}
	}
	// business_rule doc must state it resolves to an invariant, an assertion, or a specification with an owner.
	br := vocab.ConceptBusinessRule.Doc()
	if !strings.Contains(br.Meaning, "an invariant") || !strings.Contains(br.Meaning, "an assertion") || !strings.Contains(br.Meaning, "a specification") {
		t.Errorf("business_rule meaning missing resolve clause: %q", br.Meaning)
	}
	if !strings.Contains(br.Supplies, "owner") {
		t.Errorf("business_rule Supplies should mention owner: %q", br.Supplies)
	}
	// The authoring guidance places a rule where the meaning does: a
	// specification is one of the places, never an excluded one.
	if !strings.Contains(br.Supplies, "as a specification") || strings.Contains(br.Supplies, "never becomes a specification") {
		t.Errorf("business_rule Supplies contradicts the meaning about specifications: %q", br.Supplies)
	}
	if !strings.Contains(strings.Join(br.Questions, " "), "a specification") {
		t.Errorf("business_rule Questions never ask whether the rule is a specification: %q", br.Questions)
	}
}

func TestConceptDocSourcesAreReadable(t *testing.T) {
	doc := vocab.ConceptAggregate.Doc()
	if len(doc.Sources) == 0 {
		t.Fatal("aggregate cites nothing")
	}
	first := doc.Sources[0].String()
	for _, want := range []string{"Eric Evans", "Domain-Driven Design Reference", "(2015)", "p. "} {
		if !strings.Contains(first, want) {
			t.Errorf("reference %q lacks %q", first, want)
		}
	}
}

// The RelationKind enum is the meta-model's context_relation kinds, in
// order, and every kind is documented by its recorded definition.
func TestRelationKindsMatchTheMetaModel(t *testing.T) {
	kinds := vocab.RelationKinds()
	want := []vocab.RelationKind{
		vocab.RelationPartnership,
		vocab.RelationSharedKernel,
		vocab.RelationCustomerSupplier,
		vocab.RelationConformist,
		vocab.RelationAnticorruptionLayer,
		vocab.RelationOpenHostService,
		vocab.RelationPublishedLanguage,
		vocab.RelationSeparateWays,
	}
	if len(kinds) != len(want) {
		t.Fatalf("RelationKinds len = %d, want %d", len(kinds), len(want))
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Errorf("RelationKinds[%d] = %q, want %q", i, kinds[i], want[i])
		}
	}

	block, ok := vocab.DDD().Block("context_relation")
	if !ok {
		t.Fatal("the meta-model has no context_relation block")
	}
	if len(block.Records.Kinds) != len(kinds) {
		t.Fatalf("meta-model records %d kinds, the enum has %d", len(block.Records.Kinds), len(kinds))
	}
	for i, k := range block.Records.Kinds {
		if k.Name != string(kinds[i]) {
			t.Errorf("meta-model kind %d is %q, the enum has %q", i, k.Name, kinds[i])
		}
	}

	docs := vocab.RelationKindDocs()
	if len(docs) != len(kinds) {
		t.Fatalf("RelationKindDocs len = %d", len(docs))
	}
	for i, d := range docs {
		if d.Kind != kinds[i] {
			t.Errorf("RelationKindDocs[%d] = %q, want %q", i, d.Kind, kinds[i])
		}
		if d.Meaning != block.Records.Kinds[i].Definition {
			t.Errorf("%s Meaning = %q, want the recorded definition", d.Kind, d.Meaning)
		}
		if len(d.Sources) == 0 {
			t.Errorf("%s cites nothing", d.Kind)
		}
	}

	schemaDesc := vocab.SchemaKindDescription()
	for _, d := range docs {
		if !strings.Contains(schemaDesc, "\n"+string(d.Kind)+": "+d.Meaning) {
			t.Errorf("schema kind description lacks %s", d.Kind)
		}
	}
}

func TestParseRelationKindRoundTrip(t *testing.T) {
	for _, k := range vocab.RelationKinds() {
		got, err := vocab.ParseRelationKind(string(k))
		if err != nil {
			t.Errorf("ParseRelationKind(%q): %v", k, err)
			continue
		}
		if got != k {
			t.Errorf("ParseRelationKind(%q) = %q", k, got)
		}
	}
	if _, err := vocab.ParseRelationKind("shared-kernel"); err == nil {
		t.Fatal("hyphen kind should be rejected")
	}
}

// When an assertion goes on a domain service, the librarian asks
// whether its operation changes anything outside the model, in the
// question bank and in the assertion and domain service Ask lists, and
// its distillation rule states where the decision lives and where its
// change is made, then how a domain service applies it, with the
// sources of both statements.
func TestLibrarianSaysTheDomainDecides(t *testing.T) {
	const question = "Does <operation> change anything outside the model, such as a file, a store or another system? Then record the assertion on the operation that decides, and let the application service make the change."
	const service = "When an operation of a domain service would change something outside the model, such as a file, a store or another system, its assertion goes on the operation that decides. " +
		"That operation takes what the decision needs as values, changes nothing, and returns its verdict as a value; " +
		"a verdict that carries a rule, such as approving only when no participant refused, is recorded as a value object whose invariant states that rule, so `value_object/constructed-through-one-door` requires it to declare a constructor. " +
		"The application service gathers those values, asks for the verdict, and makes the change only with a verdict that allows it in hand. " +
		"Go makes that verdict hard to forge with unexported fields and a zero value that allows nothing, " +
		"TypeScript with a private constructor and a private field, and Python only by convention, with an underscore-prefixed name. " +
		"An automated test proves what the domain cannot see, such as a refusal changing nothing on disk " +
		"(Evans, DDD Reference 2015, p. 12, Value Objects, p. 14, Services, p. 21, Side-Effect-Free Functions, and p. 22, Assertions; " +
		"The Go Programming Language Specification, Exported identifiers and Struct literals; " +
		"TypeScript 2.0 release notes, Private and Protected Constructors, and TypeScript Handbook, Type Compatibility; " +
		"The Python Tutorial, 9.6 Private Variables)."
	if got := vocab.ServiceDecides().String(); got != service {
		t.Errorf("ServiceDecides:\n got %q\nwant %q", got, service)
	}
	if _, ok := vocab.DDD().Invariant("value_object/constructed-through-one-door"); !ok {
		t.Error("ServiceDecides names value_object/constructed-through-one-door, which is no meta-model invariant")
	}

	var asked bool
	for _, q := range vocab.InsufficientInfoQuestions() {
		if q.Question == question {
			asked = q.Decides == "deciding operation (domain_service) vs carrying out the change (application_service)"
		}
	}
	if !asked {
		t.Errorf("the insufficient_info bank does not ask %q to decide the deciding operation", question)
	}
	for _, c := range []vocab.Concept{vocab.ConceptAssertion, vocab.ConceptDomainService} {
		if !slices.Contains(c.Doc().Questions, question) {
			t.Errorf("%s Doc.Questions lacks %q", c, question)
		}
	}

	rules := vocab.DistillationRules()
	at := slices.IndexFunc(rules, func(r vocab.DistillationRule) bool { return r.ID == "decide-then-carry-out" })
	if at < 1 || rules[at-1].ID != "contract-of-a-service" {
		t.Fatalf("decide-then-carry-out is not the rule after contract-of-a-service (at %d)", at)
	}
	if want := vocab.DomainDecides().String() + " " + service; rules[at].Rule != want {
		t.Errorf("decide-then-carry-out:\n got %q\nwant %q", rules[at].Rule, want)
	}
}

// Before the librarian recommends moving code between layers it states
// the strongest case against the move from the meta-model's
// bibliography, then recommends decide-then-carry-out; the protocol
// step comes after Behavior, and every work it names is one the
// meta-model cites, so the librarian finds it there.
func TestLibrarianWeighsAMoveBetweenLayers(t *testing.T) {
	rules := vocab.SkillProtocolRules()
	at := slices.IndexFunc(rules, func(r string) bool { return strings.HasPrefix(r, "**Moves between layers.**") })
	if at < 1 || !strings.HasPrefix(rules[at-1], "**Behavior.**") {
		t.Fatalf("the Moves between layers step is not the one after Behavior (at %d)", at)
	}
	step := rules[at]
	for _, want := range []string{
		"`assertion/checked-by-its-operation`",
		"state the strongest case against the move before recommending it, citing works from the meta-model's bibliography",
		"decide-then-carry-out",
		"values in and verdict out",
	} {
		if !strings.Contains(step, want) {
			t.Errorf("the step lacks %q:\n%s", want, step)
		}
	}
	if _, ok := vocab.DistillationRuleByID("decide-then-carry-out"); !ok {
		t.Error("the step names decide-then-carry-out, which is no distillation rule")
	}

	model := vocab.DDD()
	block, ok := model.Block("domain_service")
	if !ok {
		t.Fatal("no domain_service block")
	}
	cited := map[string]bool{}
	for _, g := range block.Guidance {
		for _, c := range g.Sources {
			cited[c.Work] = true
		}
	}
	for work, named := range map[string]string{
		"evans2015":               "Evans, DDD Reference 2015, p. 10, Layered Architecture, and p. 21, Side-Effect-Free Functions",
		"stafford-business-logic": "Stafford, Kinds of Business Logic",
	} {
		if !strings.Contains(step, named) {
			t.Errorf("the step does not cite %s as %q", work, named)
		}
		if !cited[work] {
			t.Errorf("no domain_service guidance cites %s, so the bibliography does not reach it", work)
		}
	}
}

// The domain_service guidance that says the service decides and the
// application carries out the change is grounded on the canonical
// reference and on Stafford's published pattern: Evans on layers, side
// effects, values, and assertions, and Stafford on application logic.
// A practitioner blog may illustrate but never grounds what agents are
// told, so the entry cites nothing else.
func TestDecisionGuidanceIsGroundedOnCanonicalSources(t *testing.T) {
	block, ok := vocab.DDD().Block("domain_service")
	if !ok {
		t.Fatal("no domain_service block")
	}
	at := slices.IndexFunc(block.Guidance, func(g vocab.Guidance) bool {
		return strings.HasPrefix(g.Text, "When an operation would change something outside the model")
	})
	if at < 0 {
		t.Fatal("no domain_service guidance says the service decides and the application carries out the change")
	}
	var got []string
	for _, c := range block.Guidance[at].Sources {
		got = append(got, c.Work+" "+c.Page+" "+c.Section)
	}
	want := []string{
		"evans2015 10 Layered Architecture",
		"evans2015 12 Value Objects",
		"evans2015 21 Side-Effect-Free Functions",
		"evans2015 22 Assertions",
		"stafford-business-logic  ",
	}
	if !slices.Equal(got, want) {
		t.Errorf("sources:\n got %q\nwant %q", got, want)
	}
}
