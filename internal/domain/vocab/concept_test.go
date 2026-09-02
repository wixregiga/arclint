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
		vocab.ConceptBoundedContext,
		vocab.ConceptBusinessRule,
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
		vocab.ConceptDomainEvent:    "domain_events",
		vocab.ConceptBoundedContext: "bounded_contexts",
		vocab.ConceptBusinessRule:   "business_rules",
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

func TestConceptDocMeaningsFromVocabularyTerms(t *testing.T) {
	// Meanings are the VOCAB.yaml one-liners for the matching term.
	want := map[vocab.Concept]string{
		vocab.ConceptEntity:         vocab.TermDefinition(vocab.TermEntity),
		vocab.ConceptValueObject:    vocab.TermDefinition(vocab.TermValueObject),
		vocab.ConceptInvariant:      vocab.TermDefinition(vocab.TermInvariant),
		vocab.ConceptAssertion:      vocab.TermDefinition(vocab.TermAssertion),
		vocab.ConceptSpecification:  vocab.TermDefinition(vocab.TermSpecification),
		vocab.ConceptAggregate:      vocab.TermDefinition(vocab.TermAggregate),
		vocab.ConceptAggregateRoot:  vocab.TermDefinition(vocab.TermAggregateRoot),
		vocab.ConceptDomainEvent:    vocab.TermDefinition(vocab.TermDomainEvent),
		vocab.ConceptBoundedContext: vocab.TermDefinition(vocab.TermBoundedContext),
		vocab.ConceptBusinessRule:   vocab.TermDefinition(vocab.TermBusinessRule),
	}
	for c, meaning := range want {
		doc := c.Doc()
		if doc.Meaning != meaning {
			t.Errorf("%s Meaning = %q, want %q", c, doc.Meaning, meaning)
		}
	}
	// business_rule doc must state it always resolves to invariant or assertion with an owner.
	br := vocab.ConceptBusinessRule.Doc()
	if !strings.Contains(br.Meaning, "always resolves to invariant or assertion") {
		t.Errorf("business_rule meaning missing resolve clause: %q", br.Meaning)
	}
	if !strings.Contains(br.Supplies, "owner") {
		t.Errorf("business_rule Supplies should mention owner: %q", br.Supplies)
	}
}

func TestRelationKindsOrderAndSharedKernelDivergence(t *testing.T) {
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

	docs := vocab.RelationKindDocs()
	if len(docs) != 8 {
		t.Fatalf("RelationKindDocs len = %d", len(docs))
	}
	for _, d := range docs {
		if d.Kind == vocab.RelationSharedKernel {
			if d.Meaning != "small jointly-owned subset" {
				t.Errorf("shared_kernel Meaning = %q", d.Meaning)
			}
			if d.SchemaMeaning != "small jointly-owned model subset" {
				t.Errorf("shared_kernel SchemaMeaning = %q", d.SchemaMeaning)
			}
			continue
		}
		if d.Meaning != d.SchemaMeaning {
			t.Errorf("%s Meaning %q != SchemaMeaning %q", d.Kind, d.Meaning, d.SchemaMeaning)
		}
	}

	flow := vocab.ContextRelationFlowYAML()
	if !strings.Contains(flow, "shared_kernel: small jointly-owned subset") {
		t.Errorf("VOCAB flow missing shared_kernel meaning: %s", flow)
	}
	if strings.Contains(flow, "jointly-owned model subset") {
		t.Errorf("VOCAB flow must not use schema-only shared_kernel phrasing: %s", flow)
	}

	schemaDesc := vocab.SchemaKindDescription()
	if !strings.Contains(schemaDesc, "shared_kernel: small jointly-owned model subset") {
		t.Errorf("schema kind description missing model subset: %s", schemaDesc)
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
