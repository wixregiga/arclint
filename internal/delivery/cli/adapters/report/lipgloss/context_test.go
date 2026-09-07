package lipgloss

import (
	"bytes"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// The Lipgloss renderer speaks the same grammar as the plain one and
// colors the anchors: a found source as a path, a missing one at
// warning weight.
func TestLipglossContextScopedDomainGrammarAndAnchorColors(t *testing.T) {
	dk := &application.DomainKnowledge{
		Source:  "domain.arclint.yaml",
		Counts:  vocab.Counts{Contexts: 2, Aggregates: 1, Entities: 1, ValueObjects: 2, Invariants: 3},
		Scoped:  true,
		Shown:   vocab.Counts{Contexts: 1, Aggregates: 1, Entities: 1, ValueObjects: 1, Invariants: 3},
		Located: true,
		Contexts: []application.DomainContextKnowledge{{
			Name:         "catalog",
			Aggregates:   []application.DomainAggregateRef{{Name: "Event", Identity: "EventID", Entities: []string{"Organizer"}}},
			ValueObjects: []string{"Price"},
			Invariants: []application.DomainInvariantRef{
				{Key: "published-frozen", Statement: "A published Event never changes.", Owner: "Event", OwnerConcept: vocab.ConceptAggregate, Source: "event/event.go:90", Anchor: application.AnchorFound},
				{Key: "one-venue", Statement: "An Event has one Venue.", Owner: "Event", OwnerConcept: vocab.ConceptAggregate, Anchor: application.AnchorMissing},
				{Key: "never-negative", Statement: "A Price is never negative.", Owner: "Price", OwnerConcept: vocab.ConceptValueObject, Anchor: application.AnchorMissing},
			},
		}},
		Unanchored: []application.UnanchoredContract{
			{Kind: application.ContractInvariant, Context: "catalog", Owner: "Event", Key: "one-venue", Statement: "An Event has one Venue.", Expected: "method OneVenue on Event"},
			{Kind: application.ContractInvariant, Context: "catalog", Owner: "Price", Key: "never-negative", Statement: "A Price is never negative.", Expected: "constructor of Price"},
		},
	}
	var buf bytes.Buffer
	err := ansiRenderer().Render(&buf, cli.ContextReport{Context: application.ArchitecturalContext{Scope: "m/event.go", Domain: dk}})
	if err != nil {
		t.Fatal(err)
	}
	raw := buf.String()
	if !strings.Contains(raw, "\x1b[33mmissing\x1b[0m") {
		t.Errorf("missing is not rendered in the warning color: %q", raw)
	}
	if !strings.Contains(raw, "\x1b[36;2mevent/event.go:90\x1b[0m") && !strings.Contains(raw, "\x1b[2;36mevent/event.go:90\x1b[0m") {
		t.Errorf("a found source is not rendered as a path: %q", raw)
	}
	out := stripANSI(raw)
	for _, want := range []string{
		"project domain (domain.arclint.yaml): 1 of 2 contexts, 1 of 1 aggregate, 1 of 1 entity, 1 of 2 value objects, 3 of 3 invariants anchor into this scope; --full shows the whole model\n",
		"    aggregates: Event (EventID; Organizer)\n",
		"      published-frozen (Event): A published Event never changes. event/event.go:90\n",
		"      one-venue (Event): An Event has one Venue. missing\n",
		"      never-negative (Price): A Price is never negative. missing\n",
		"  unanchored contracts: 2 missing\n",
		"    missing: invariant one-venue of Event (context catalog)\n",
		"      expected method OneVenue on Event\n",
		"    missing: invariant never-negative of Price (context catalog)\n",
		"      expected constructor of Price\n",
		"    arclint check reports each as a Violation of the built-in rule of its block\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("grammar lacks %q:\n%s", want, out)
		}
	}
}
