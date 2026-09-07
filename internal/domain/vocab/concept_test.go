package vocab_test

import (
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
		if doc.Meaning != block.Definition.Text {
			t.Errorf("%s Meaning = %q, want the block's definition", c, doc.Meaning)
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
	// business_rule doc must state it resolves to an invariant or an assertion with an owner.
	br := vocab.ConceptBusinessRule.Doc()
	if !strings.Contains(br.Meaning, "either an invariant") || !strings.Contains(br.Meaning, "or an assertion") {
		t.Errorf("business_rule meaning missing resolve clause: %q", br.Meaning)
	}
	if !strings.Contains(br.Supplies, "owner") {
		t.Errorf("business_rule Supplies should mention owner: %q", br.Supplies)
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
