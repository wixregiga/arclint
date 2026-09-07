package application_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// domainFixture declares Zones m (m/**) and catalog (catalog/**) and
// records contexts catalog and billing. The catalog context is named
// for its Zone, so its terms are looked for there alone: the Event
// aggregate and Price under catalog/event/, the specifications under
// catalog/spec/. Billing has no Zone, so its Money is found wherever it
// is declared, here inside Zone m. Every anchor outcome and every
// scoping route (a file, a folder, a Zone named for a context, a Zone
// that is not) has one term exercising it.
func domainFixture(t *testing.T) (rule.Configured, *fakeKnowledge, *fakeObservations) {
	t.Helper()
	cfg := contextFixture(t)
	glob, err := rule.NewGlob("catalog/**")
	if err != nil {
		t.Fatalf("NewGlob: %v", err)
	}
	catalog, err := rule.NewZone("catalog", "the catalog zone", []rule.Glob{glob})
	if err != nil {
		t.Fatalf("NewZone: %v", err)
	}
	cfg.Zones = append(cfg.Zones, catalog)
	lang, err := vocab.NewUbiquitousLanguage("boxoffice", "", []vocab.BoundedContext{
		{
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
			Specifications: []vocab.Specification{
				{Name: "HighValueOrder", Definition: "Orders above a threshold."},
				{Name: "LateOrder", Definition: "Orders after the doors."},
			},
			Events: []vocab.DomainEvent{{Name: "EventPublished", Definition: "An Event went on sale.", RaisedBy: "Event"}},
		},
		{
			Name:       "billing",
			Definition: "Getting paid.",
			Aggregates: []vocab.Aggregate{{Name: "Invoice", Definition: "A bill.", Identity: "InvoiceID"}},
			ValueObjects: []vocab.ValueObject{
				{Name: "Money", Definition: "An amount.", Invariants: []vocab.Invariant{{Key: "never-negative", Statement: "Money is never negative."}}},
			},
		},
	}, []vocab.ContextRelation{{From: "catalog", To: "billing", Kind: vocab.RelationConformist}})
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	obs, err := conformance.NewObservations(
		[]conformance.ObservedFile{{Path: "catalog/event/event.go"}, {Path: "catalog/event/venue.go"}, {Path: "catalog/spec/spec.go"}, {Path: "m/money.go"}},
		map[string]conformance.LanguageFacts{
			"catalog/event/event.go": {
				Language:              rule.LanguageGo,
				Package:               "event",
				DeclarationsAvailable: true,
				Declarations: []conformance.Declaration{
					{Kind: "struct", Name: "Event", Exported: true, StartLine: 3},
					{Kind: "struct", Name: "EventPublished", Exported: true, StartLine: 5},
					{Kind: "struct", Name: "Price", Exported: true, StartLine: 6},
					{Kind: "func", Name: "NewPrice", Results: []string{"Price", "error"}, StartLine: 8},
					{Kind: "method", Name: "EnsurePublishedFrozen", Owner: "Event", Exported: true, StartLine: 90},
					{Kind: "method", Name: "AssertTiersPriced", Owner: "Event", Exported: true, StartLine: 120},
				},
			},
			"catalog/event/venue.go": {
				Language:              rule.LanguageGo,
				Package:               "event",
				DeclarationsAvailable: true,
				Declarations:          []conformance.Declaration{{Kind: "struct", Name: "Venue", Exported: true, StartLine: 1}},
			},
			"catalog/spec/spec.go": {
				Language:              rule.LanguageGo,
				Package:               "spec",
				DeclarationsAvailable: true,
				Declarations: []conformance.Declaration{
					{Kind: "struct", Name: "HighValueOrder", Exported: true, StartLine: 10},
					{Kind: "method", Name: "SatisfiedBy", Owner: "HighValueOrder", Exported: true, StartLine: 34},
					{Kind: "struct", Name: "LateOrder", Exported: true, StartLine: 50},
				},
			},
			"m/money.go": {
				Language:              rule.LanguageGo,
				Package:               "m",
				DeclarationsAvailable: true,
				Declarations: []conformance.Declaration{
					{Kind: "struct", Name: "Money", Exported: true, StartLine: 1},
					{Kind: "func", Name: "NewMoney", Results: []string{"Money", "error"}, StartLine: 4},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	return cfg, &fakeKnowledge{lang: lang, found: true}, &fakeObservations{obs: obs}
}

func domainContext(t *testing.T, req application.ContextRequest) application.ArchitecturalContext {
	t.Helper()
	cfg, knowledge, obs := domainFixture(t)
	uc, err := application.NewGetArchitecturalContext(fakeRepository{cfg}, knowledge)
	if err != nil {
		t.Fatalf("NewGetArchitecturalContext: %v", err)
	}
	ctx, err := uc.WithObservations(obs).Execute(req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if ctx.Domain == nil {
		t.Fatalf("a recorded model must project into context")
	}
	return ctx
}

func contextNamed(t *testing.T, dk *application.DomainKnowledge, name string) application.DomainContextKnowledge {
	t.Helper()
	for _, c := range dk.Contexts {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("context %q absent from %+v", name, dk.Contexts)
	return application.DomainContextKnowledge{}
}

func unanchoredLabels(dk *application.DomainKnowledge) []string {
	labels := []string{}
	for _, u := range dk.Unanchored {
		labels = append(labels, string(u.Kind)+":"+u.Owner+u.Name+":"+u.Expected)
	}
	return labels
}

func TestArchitecturalContextRepositoryCarriesWholeDomain(t *testing.T) {
	ctx := domainContext(t, application.ContextRequest{})
	dk := ctx.Domain
	if dk.Scoped || !dk.Located {
		t.Fatalf("repository scope: scoped=%v located=%v, want whole and located", dk.Scoped, dk.Located)
	}
	if dk.Counts.Contexts != 2 || dk.Shown != dk.Counts {
		t.Errorf("counts = %+v shown = %+v, want equal whole-model tallies", dk.Counts, dk.Shown)
	}
	if len(dk.Relations) != 1 || dk.Relations[0] != (application.DomainRelationRef{From: "catalog", To: "billing", Kind: "conformist"}) {
		t.Errorf("relations = %+v, want the recorded one", dk.Relations)
	}
	want := []string{
		"invariant:Event:method EnsureOneVenue on Event",
		"invariant:Discount:constructor of Discount",
		"specification:LateOrder:satisfaction method on LateOrder",
	}
	if got := unanchoredLabels(dk); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("unanchored = %v, want %v", got, want)
	}
	billing := contextNamed(t, dk, "billing")
	if len(billing.Invariants) != 1 || billing.Invariants[0].Anchor != application.AnchorFound || billing.Invariants[0].Source != "m/money.go:4" {
		t.Errorf("billing invariant = %+v, want found through NewMoney", billing.Invariants)
	}
}

func TestArchitecturalContextPathScopesDomainToDeclarations(t *testing.T) {
	ctx := domainContext(t, application.ContextRequest{Paths: []string{"catalog/event/event.go"}})
	dk := ctx.Domain
	if !dk.Scoped {
		t.Fatalf("a path worksite must scope the domain")
	}
	if dk.Counts.Contexts != 2 || dk.Counts.Invariants != 5 {
		t.Errorf("counts = %+v, want the whole model tallied", dk.Counts)
	}
	if len(dk.Contexts) != 1 {
		t.Fatalf("contexts = %+v, want only catalog", dk.Contexts)
	}
	catalog := contextNamed(t, dk, "catalog")
	if len(catalog.Aggregates) != 1 || catalog.Aggregates[0].Name != "Event" || len(catalog.Aggregates[0].Entities) != 1 {
		t.Errorf("aggregates = %+v, want Event whole with its member Venue", catalog.Aggregates)
	}
	if len(catalog.ValueObjects) != 1 || catalog.ValueObjects[0] != "Price" {
		t.Errorf("value objects = %v, want Price through its declaration; Discount is declared nowhere", catalog.ValueObjects)
	}
	owners := []string{}
	for _, inv := range catalog.Invariants {
		owners = append(owners, inv.Owner)
	}
	if strings.Join(owners, " ") != "Event Event Price" {
		t.Errorf("invariant owners = %v, want the two Event invariants and the Price one", owners)
	}
	if len(catalog.Assertions) != 1 || len(catalog.Specifications) != 0 {
		t.Errorf("assertions = %+v specifications = %+v, want the Event assertion and no specification", catalog.Assertions, catalog.Specifications)
	}
	if len(catalog.Events) != 1 || catalog.Events[0] != "EventPublished" {
		t.Errorf("events = %v, want EventPublished declared in the file", catalog.Events)
	}
	if len(dk.Relations) != 1 {
		t.Errorf("relations = %+v, want the one touching catalog", dk.Relations)
	}
	want := vocab.Counts{Contexts: 1, Aggregates: 1, Entities: 1, ValueObjects: 1, Invariants: 3, Assertions: 1, Events: 1, Relations: 1}
	if dk.Shown != want {
		t.Errorf("shown = %+v, want %+v", dk.Shown, want)
	}
	if got := unanchoredLabels(dk); strings.Join(got, " ") != "invariant:Event:method EnsureOneVenue on Event" {
		t.Errorf("unanchored = %v, want only the in-scope Event invariant without its method", got)
	}
}

func TestArchitecturalContextFolderPathScopesDomain(t *testing.T) {
	ctx := domainContext(t, application.ContextRequest{Paths: []string{"catalog/event/"}})
	catalog := contextNamed(t, ctx.Domain, "catalog")
	if len(catalog.Aggregates) != 1 || len(catalog.ValueObjects) != 1 {
		t.Errorf("catalog = %+v, want Event and Price under the folder", catalog)
	}
	if ctx.Domain.Shown.Invariants != 3 || ctx.Domain.Shown.Entities != 1 {
		t.Errorf("shown = %+v, want the Event and Price invariants with Venue", ctx.Domain.Shown)
	}
}

func TestArchitecturalContextMemberAnchorsItsAggregate(t *testing.T) {
	ctx := domainContext(t, application.ContextRequest{Paths: []string{"catalog/event/venue.go"}})
	catalog := contextNamed(t, ctx.Domain, "catalog")
	if len(catalog.Aggregates) != 1 || catalog.Aggregates[0].Name != "Event" {
		t.Errorf("aggregates = %+v, want Event kept whole through its member Venue", catalog.Aggregates)
	}
	if len(catalog.Invariants) != 2 || len(catalog.ValueObjects) != 0 {
		t.Errorf("catalog = %+v, want the Event invariants alone", catalog)
	}
}

func TestArchitecturalContextZoneNamedForContextKeepsItWhole(t *testing.T) {
	ctx := domainContext(t, application.ContextRequest{Zones: []string{"catalog"}})
	dk := ctx.Domain
	if len(dk.Contexts) != 1 {
		t.Fatalf("contexts = %+v, want only catalog", dk.Contexts)
	}
	catalog := contextNamed(t, dk, "catalog")
	if len(catalog.Aggregates) != 1 || len(catalog.ValueObjects) != 2 || len(catalog.Invariants) != 4 || len(catalog.Specifications) != 2 {
		t.Errorf("a Zone named for the context must keep it whole, got %+v", catalog)
	}
	if len(dk.Unanchored) != 3 {
		t.Errorf("unanchored = %+v, want every catalog contract that is not found", dk.Unanchored)
	}
}

// A Zone named for no context scopes to the terms declared inside it:
// billing's Money is declared in Zone m, so billing is shown through
// Money alone and Invoice, declared nowhere, is not.
func TestArchitecturalContextZoneScopesDomainToItsDeclarations(t *testing.T) {
	ctx := domainContext(t, application.ContextRequest{Zones: []string{"m"}})
	dk := ctx.Domain
	if len(dk.Contexts) != 1 {
		t.Fatalf("contexts = %+v, want only billing", dk.Contexts)
	}
	billing := contextNamed(t, dk, "billing")
	if len(billing.Aggregates) != 0 || len(billing.ValueObjects) != 1 || billing.ValueObjects[0] != "Money" || len(billing.Invariants) != 1 {
		t.Errorf("Zone m declares Money and nothing else of billing, got %+v", billing)
	}
	if len(dk.Relations) != 1 {
		t.Errorf("relations = %+v, want the one touching billing", dk.Relations)
	}
}

func TestArchitecturalContextPathOutsideDomainScopesToNothing(t *testing.T) {
	ctx := domainContext(t, application.ContextRequest{Paths: []string{"elsewhere/file.go"}})
	dk := ctx.Domain
	if !dk.Scoped || len(dk.Contexts) != 0 || len(dk.Relations) != 0 || len(dk.Unanchored) != 0 {
		t.Errorf("nothing anchors into an unrelated path, got %+v", dk)
	}
	if dk.Shown != (vocab.Counts{}) || dk.Counts.Contexts != 2 {
		t.Errorf("shown = %+v counts = %+v, want empty shown and whole counts", dk.Shown, dk.Counts)
	}
}

func TestArchitecturalContextFullKeepsWholeDomain(t *testing.T) {
	ctx := domainContext(t, application.ContextRequest{Paths: []string{"catalog/event/event.go"}, Full: true})
	dk := ctx.Domain
	if dk.Scoped || len(dk.Contexts) != 2 || dk.Shown != dk.Counts {
		t.Errorf("--full must carry the whole model, got scoped=%v contexts=%d", dk.Scoped, len(dk.Contexts))
	}
	if len(dk.Unanchored) != 3 {
		t.Errorf("unanchored = %+v, want the whole model's three", dk.Unanchored)
	}
}

func TestArchitecturalContextWithoutObservationsIsNeverScoped(t *testing.T) {
	cfg, knowledge, _ := domainFixture(t)
	uc, err := application.NewGetArchitecturalContext(fakeRepository{cfg}, knowledge)
	if err != nil {
		t.Fatalf("NewGetArchitecturalContext: %v", err)
	}
	ctx, err := uc.Execute(application.ContextRequest{Paths: []string{"catalog/event/event.go"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	dk := ctx.Domain
	if dk.Located || dk.Scoped || len(dk.Contexts) != 2 || len(dk.Unanchored) != 0 {
		t.Errorf("without declarations nothing can anchor, so the model stays whole and unlocated, got %+v", dk)
	}
	for _, c := range dk.Contexts {
		for _, inv := range c.Invariants {
			if inv.Anchor != "" || inv.Source != "" {
				t.Errorf("invariant %+v carries an anchor without observations", inv)
			}
		}
	}
}
