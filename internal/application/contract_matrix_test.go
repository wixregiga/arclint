package application

import (
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// A value object's invariant is carried by its constructor and an
// aggregate's by the root method its key names; with nothing observed,
// both are missing.
func TestLocateInvariantDispatchesOnTheOwnersConcept(t *testing.T) {
	carriers := catalogCarriers(t)
	price := DomainInvariantRef{Owner: "Price", OwnerConcept: vocab.ConceptValueObject, Key: "never-negative"}
	src, found, err := locateInvariant(carriers, "catalog", price)
	if err != nil || src != "event/event.go:8" || !found {
		t.Fatalf("locateInvariant Price = %q, %v, %v; want the constructor at event/event.go:8", src, found, err)
	}
	event := DomainInvariantRef{Owner: "Event", OwnerConcept: vocab.ConceptAggregate, Key: "published-frozen"}
	src, found, err = locateInvariant(carriers, "catalog", event)
	if err != nil || src != "event/event.go:90" || !found {
		t.Fatalf("locateInvariant Event = %q, %v, %v; want the method at event/event.go:90", src, found, err)
	}
	for _, inv := range []DomainInvariantRef{price, event} {
		src, found, err := locateInvariant(conformance.Carriers{}, "catalog", inv)
		if err != nil || src != "" || found {
			t.Fatalf("locateInvariant %s without declarations = %q, %v, %v; want missing", inv.Owner, src, found, err)
		}
	}
}

// A key no case can spell surfaces as an error naming the contract, so
// the listing never reports a spelling failure as a missing anchor.
func TestLocateDomainContractsReportsAnUnspellableKey(t *testing.T) {
	dk := &DomainKnowledge{Contexts: []DomainContextKnowledge{{
		Name:       "catalog",
		Invariants: []DomainInvariantRef{{Owner: "Event", OwnerConcept: vocab.ConceptAggregate, Key: ""}},
	}}}
	err := locateDomainContracts(dk, catalogCarriers(t))
	if err == nil {
		t.Fatalf("locateDomainContracts accepted an empty key")
	}
}

// The projection lists every contract with its owner and the owner's
// concept, in file order: the aggregate's invariants and assertions,
// then each value object's invariants.
func TestDomainKnowledgeOfProjectsEveryContract(t *testing.T) {
	dk := domainKnowledgeOf(catalogLanguage(t))
	if dk.Located || dk.Source != vocab.UbiquitousLanguageFileName {
		t.Fatalf("projection = %+v", dk)
	}
	if len(dk.Contexts) != 1 || dk.Contexts[0].Name != "catalog" {
		t.Fatalf("contexts = %+v", dk.Contexts)
	}
	ctx := dk.Contexts[0]
	if len(ctx.Aggregates) != 1 || ctx.Aggregates[0].Name != "Event" || ctx.Aggregates[0].Identity != "EventID" || len(ctx.Aggregates[0].Entities) != 1 || ctx.Aggregates[0].Entities[0] != "Venue" {
		t.Fatalf("aggregates = %+v", ctx.Aggregates)
	}
	if len(ctx.ValueObjects) != 2 || ctx.ValueObjects[0] != "Price" || ctx.ValueObjects[1] != "Discount" {
		t.Fatalf("value objects = %+v", ctx.ValueObjects)
	}
	want := []DomainInvariantRef{
		{Key: "published-frozen", Statement: "A published Event never changes.", Owner: "Event", OwnerConcept: vocab.ConceptAggregate},
		{Key: "one-venue", Statement: "An Event has at most one Venue.", Owner: "Event", OwnerConcept: vocab.ConceptAggregate},
		{Key: "never-negative", Statement: "A Price is never negative.", Owner: "Price", OwnerConcept: vocab.ConceptValueObject},
		{Key: "within-the-whole", Statement: "A Discount never exceeds the whole.", Owner: "Discount", OwnerConcept: vocab.ConceptValueObject},
	}
	if len(ctx.Invariants) != len(want) {
		t.Fatalf("invariants = %+v", ctx.Invariants)
	}
	for i, inv := range ctx.Invariants {
		if inv != want[i] {
			t.Errorf("invariant %d = %+v, want %+v", i, inv, want[i])
		}
	}
	if len(ctx.Assertions) != 1 || ctx.Assertions[0] != (DomainAssertionRef{Key: "tiers-priced", Statement: "Tiers are priced.", Owner: "Event", On: "Publish"}) {
		t.Fatalf("assertions = %+v", ctx.Assertions)
	}
	if len(ctx.Specifications) != 2 || len(ctx.Events) != 1 || ctx.Events[0] != "Published" || len(ctx.Services) != 1 || ctx.Services[0] != "Pricing" {
		t.Fatalf("specifications %+v events %+v services %+v", ctx.Specifications, ctx.Events, ctx.Services)
	}
	if dk.Counts != catalogLanguage(t).Counts() || dk.Shown != dk.Counts {
		t.Fatalf("counts = %+v shown = %+v", dk.Counts, dk.Shown)
	}
	if len(dk.Relations) != 0 {
		t.Fatalf("relations = %+v", dk.Relations)
	}
}

func TestLocateDomainContractsFillsSourcesAndAnchors(t *testing.T) {
	dk := domainKnowledgeOf(catalogLanguage(t))
	if err := locateDomainContracts(dk, catalogCarriers(t)); err != nil {
		t.Fatalf("locateDomainContracts: %v", err)
	}
	if !dk.Located {
		t.Fatalf("projection does not report Located after locating")
	}
	inv := dk.Contexts[0].Invariants
	if inv[0].Source != "event/event.go:90" || inv[0].Anchor != AnchorFound {
		t.Fatalf("aggregate invariant with its method = %+v", inv[0])
	}
	if inv[1].Source != "" || inv[1].Anchor != AnchorMissing {
		t.Fatalf("aggregate invariant without its method = %+v, want missing", inv[1])
	}
	if inv[2].Source != "event/event.go:8" || inv[2].Anchor != AnchorFound {
		t.Fatalf("value object with a constructor = %+v", inv[2])
	}
	if inv[3].Source != "" || inv[3].Anchor != AnchorMissing {
		t.Fatalf("value object without constructor = %+v, want missing", inv[3])
	}
	if a := dk.Contexts[0].Assertions[0]; a.Source != "event/event.go:120" || a.Anchor != AnchorFound {
		t.Fatalf("assertion = %+v", a)
	}
	if s := dk.Contexts[0].Specifications[0]; s.Source != "order/spec.go:34" || s.Anchor != AnchorFound {
		t.Fatalf("specification = %+v", s)
	}
	if s := dk.Contexts[0].Specifications[1]; s.Source != "" || s.Anchor != AnchorMissing {
		t.Fatalf("specification without SatisfiedBy = %+v, want missing", s)
	}
}

func TestLocateDomainContractsNilIsSafe(t *testing.T) {
	if err := locateDomainContracts(nil, conformance.Carriers{}); err != nil {
		t.Fatalf("locateDomainContracts(nil) = %v", err)
	}
}

// The unanchored listing names every missing contract with the
// declaration the recording expects, spelled the way the project's
// languages spell a method, in listing order, and never a found one; a
// projection that was never located lists nothing, since nothing was
// looked for.
func TestUnanchoredContractsNameTheExpectedCarrier(t *testing.T) {
	dk := domainKnowledgeOf(catalogLanguage(t))
	if got := unanchoredContracts(dk, []rule.Language{rule.LanguageGo}); got != nil {
		t.Fatalf("unanchored before locating = %+v, want none", got)
	}
	if err := locateDomainContracts(dk, catalogCarriers(t)); err != nil {
		t.Fatalf("locateDomainContracts: %v", err)
	}
	got := unanchoredContracts(dk, []rule.Language{rule.LanguageGo})
	want := []UnanchoredContract{
		{Kind: ContractInvariant, Context: "catalog", Owner: "Event", Key: "one-venue", Statement: "An Event has at most one Venue.", Expected: "method EnsureOneVenue on Event"},
		{Kind: ContractInvariant, Context: "catalog", Owner: "Discount", Key: "within-the-whole", Statement: "A Discount never exceeds the whole.", Expected: "constructor of Discount"},
		{Kind: ContractSpecification, Context: "catalog", Name: "LateOrder", Expected: "satisfaction method on LateOrder"},
	}
	if len(got) != len(want) {
		t.Fatalf("unanchored = %+v, want %d", got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("unanchored %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// A project in several languages is told each spelling with its
// language; one with no language configured is told the key itself.
func TestExpectedMethodSpellsPerLanguage(t *testing.T) {
	cases := []struct {
		languages []rule.Language
		want      string
	}{
		{[]rule.Language{rule.LanguageGo}, "method EnsureOneVenue on Event"},
		{[]rule.Language{rule.LanguageTypeScript}, "method ensureOneVenue on Event"},
		{[]rule.Language{rule.LanguagePython}, "method ensure_one_venue on Event"},
		{[]rule.Language{rule.LanguageGo, rule.LanguageTypeScript}, "method EnsureOneVenue (go) or ensureOneVenue (typescript) on Event"},
		{nil, "method ensure-one-venue on Event"},
	}
	for _, c := range cases {
		if got := expectedMethod(conformance.EnsureKey("one-venue"), "Event", c.languages); got != c.want {
			t.Errorf("expectedMethod(%v) = %q, want %q", c.languages, got, c.want)
		}
	}
}

// catalogCarriers locates the catalog language in the catalog
// observations with no Zone declared.
func catalogCarriers(t *testing.T) conformance.Carriers {
	t.Helper()
	carriers, err := conformance.NewCarriers(catalogObservations(t), catalogLanguage(t), nil)
	if err != nil {
		t.Fatalf("NewCarriers: %v", err)
	}
	return carriers
}

// catalogLanguage records one context whose contracts cover every
// anchor outcome: an aggregate invariant with and without its method,
// a value object invariant with and without a constructor, a found
// assertion, and one found and one missing specification.
func catalogLanguage(t *testing.T) vocab.UbiquitousLanguage {
	t.Helper()
	lang, err := vocab.NewUbiquitousLanguage("boxoffice", "", []vocab.BoundedContext{{
		Name:       "catalog",
		Definition: "What is on sale.",
		Aggregates: []vocab.Aggregate{{
			Name:       "Event",
			Definition: "A show.",
			Identity:   "EventID",
			Entities:   []vocab.Entity{{Name: "Venue", Definition: "A hall."}},
			Invariants: []vocab.Invariant{
				{Key: "published-frozen", Statement: "A published Event never changes."},
				{Key: "one-venue", Statement: "An Event has at most one Venue."},
			},
			Assertions: []vocab.Assertion{
				{Key: "tiers-priced", On: "Publish", Statement: "Tiers are priced."},
			},
		}},
		ValueObjects: []vocab.ValueObject{
			{Name: "Price", Definition: "Whole cents.", Invariants: []vocab.Invariant{{Key: "never-negative", Statement: "A Price is never negative."}}},
			{Name: "Discount", Definition: "A percentage off.", Invariants: []vocab.Invariant{{Key: "within-the-whole", Statement: "A Discount never exceeds the whole."}}},
		},
		Events:   []vocab.DomainEvent{{Name: "Published", Definition: "An Event went on sale.", RaisedBy: "Event"}},
		Services: []vocab.DomainService{{Name: "Pricing", Definition: "Prices tiers."}},
		Specifications: []vocab.Specification{
			{Name: "HighValueOrder", Definition: "Orders above a threshold."},
			{Name: "LateOrder", Definition: "Orders after the doors."},
		},
	}}, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	return lang
}

// catalogObservations declares the types the catalog context records,
// the constructor of Price, the methods carrying one invariant and the
// assertion of Event, and the satisfaction method of HighValueOrder;
// Discount and LateOrder are declared without their carriers.
func catalogObservations(t *testing.T) conformance.Observations {
	t.Helper()
	obs, err := conformance.NewObservations(
		[]conformance.ObservedFile{{Path: "event/event.go"}, {Path: "order/spec.go"}},
		map[string]conformance.LanguageFacts{
			"event/event.go": {
				Language:              rule.LanguageGo,
				DeclarationsAvailable: true,
				Declarations: []conformance.Declaration{
					{Kind: "struct", Name: "Event", Exported: true, StartLine: 3},
					{Kind: "struct", Name: "Price", Exported: true, StartLine: 6},
					{Kind: "func", Name: "NewPrice", Results: []string{"Price", "error"}, StartLine: 8},
					{Kind: "struct", Name: "Discount", Exported: true, StartLine: 40},
					{Kind: "method", Name: "EnsurePublishedFrozen", Owner: "Event", Exported: true, StartLine: 90},
					{Kind: "method", Name: "AssertTiersPriced", Owner: "Event", Exported: true, StartLine: 120},
				},
			},
			"order/spec.go": {
				Language:              rule.LanguageGo,
				DeclarationsAvailable: true,
				Declarations: []conformance.Declaration{
					{Kind: "struct", Name: "HighValueOrder", Exported: true, StartLine: 10},
					{Kind: "method", Name: "SatisfiedBy", Owner: "HighValueOrder", Exported: true, StartLine: 34},
					{Kind: "struct", Name: "LateOrder", Exported: true, StartLine: 50},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	return obs
}
