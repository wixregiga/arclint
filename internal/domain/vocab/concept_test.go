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
		vocab.ConceptAggregate,
		vocab.ConceptValueObject,
		vocab.ConceptBusinessRule,
		vocab.ConceptEvent,
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

func TestParseConceptRejectsUnknown(t *testing.T) {
	_, err := vocab.ParseConcept("bounded-context")
	if err == nil {
		t.Fatal("ParseConcept(bounded-context) succeeded, want error")
	}
	if !strings.Contains(err.Error(), "entity") {
		t.Errorf("error %q should list accepted concepts", err)
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
		vocab.ConceptEntity:       "entities",
		vocab.ConceptAggregate:    "aggregates",
		vocab.ConceptValueObject:  "value-objects",
		vocab.ConceptBusinessRule: "business-rules",
		vocab.ConceptEvent:        "events",
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

func TestConceptDocMeaningsVerbatim(t *testing.T) {
	// Fixed ArcLint-owned meanings from the recommendation doc / plan.
	want := map[vocab.Concept]struct {
		title     string
		meaning   string
		questions []string
	}{
		vocab.ConceptEntity: {
			title:   "Entity",
			meaning: "A domain concept whose identity matters as it changes over time.",
			questions: []string{
				"What must the project distinguish from other similar things?",
				"What remains the same thing even when its attributes change?",
			},
		},
		vocab.ConceptAggregate: {
			title:   "Aggregate",
			meaning: "An Entity the project treats as a consistency boundary: it is changed as one unit and other objects reach it through its identity.",
			questions: []string{
				"Which Entity must stay internally consistent when the project changes it?",
				"Which Entity do other objects reference by identity rather than reach inside?",
			},
		},
		vocab.ConceptValueObject: {
			title:   "Value Object",
			meaning: "A domain value defined entirely by its attributes, with no identity of its own.",
			questions: []string{
				"Can two occurrences with the same attributes be used interchangeably?",
				"Does replacing it with an equal value change nothing?",
			},
		},
		vocab.ConceptBusinessRule: {
			title:   "Business Rule",
			meaning: "A statement the project requires to always or never be true about its domain.",
			questions: []string{
				"What must always hold for the project's data or behavior?",
				"What must never happen, regardless of implementation?",
			},
		},
		vocab.ConceptEvent: {
			title:   "Domain Event",
			meaning: "Something that has completed in the domain and that the project cares to record.",
			questions: []string{
				"What completed occurrence do other parts of the project react to?",
				"What would the project mention in its history of what happened?",
			},
		},
	}
	for c, w := range want {
		doc := c.Doc()
		if doc.Title != w.title {
			t.Errorf("%s Title = %q, want %q", c, doc.Title, w.title)
		}
		if doc.Meaning != w.meaning {
			t.Errorf("%s Meaning = %q, want %q", c, doc.Meaning, w.meaning)
		}
		if len(doc.Questions) != len(w.questions) {
			t.Errorf("%s Questions len = %d, want %d", c, len(doc.Questions), len(w.questions))
			continue
		}
		for i := range w.questions {
			if doc.Questions[i] != w.questions[i] {
				t.Errorf("%s Questions[%d] = %q, want %q", c, i, doc.Questions[i], w.questions[i])
			}
		}
		wantSupplies := "The project supplies the " + w.title + "'s name, definition, and aliases."
		if doc.Supplies != wantSupplies {
			t.Errorf("%s Supplies = %q, want %q", c, doc.Supplies, wantSupplies)
		}
	}
}
