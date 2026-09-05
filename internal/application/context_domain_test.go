package application_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// domainFixture declares Modules m (m/**) and catalog (catalog/**),
// records contexts catalog and billing, and observes declarations
// under m/, catalog/, and billing/ so every anchor outcome and every
// scoping route (path, declaration-carrying Module, context-named
// Module) has one term exercising it.
func domainFixture(t *testing.T) (rule.Configured, *fakeKnowledge, *fakeObservations) {
	t.Helper()
	cfg := contextFixture(t)
	glob, err := rule.NewGlob("catalog/**")
	if err != nil {
		t.Fatalf("NewGlob: %v", err)
	}
	catalog, err := rule.NewModule("catalog", "the catalog module", []rule.Glob{glob})
	if err != nil {
		t.Fatalf("NewModule: %v", err)
	}
	cfg.Modules = append(cfg.Modules, catalog)
	lang, err := vocab.NewUbiquitousLanguage([]vocab.BoundedContext{
		{
			Name: "catalog",
			Entities: []vocab.Entity{
				{Definition: vocab.Definition{Name: "Event", Definition: "A show."}, Aggregate: true},
				{Definition: vocab.Definition{Name: "Venue", Definition: "A hall."}},
			},
			ValueObjects: []vocab.Definition{
				{Name: "Price", Definition: "Whole cents."},
				{Name: "Discount", Definition: "A percentage off."},
			},
			Invariants: []vocab.Invariant{
				{Statement: "A published Event never changes.", Owner: "Event", ID: "published-frozen"},
				{Statement: "An Event has at most one Venue.", Owner: "Event"},
				{Statement: "A Price is never negative.", Owner: "Price"},
				{Statement: "A Discount never exceeds the whole.", Owner: "Discount"},
				{Statement: "A Venue is named.", Owner: "Venue"},
			},
			Assertions: []vocab.Assertion{
				{Statement: "Tiers are priced.", Owner: "Event", ID: "tiers-priced", On: "Publish"},
			},
			Specifications: []vocab.Specification{
				{Name: "HighValueOrder", Definition: "Orders above a threshold."},
				{Name: "LateOrder", Definition: "Orders after the doors."},
			},
			Events: []vocab.Definition{{Name: "EventPublished", Definition: "An Event went on sale."}},
		},
		{
			Name: "billing",
			Entities: []vocab.Entity{
				{Definition: vocab.Definition{Name: "Invoice", Definition: "A bill."}, Aggregate: true},
			},
			ValueObjects: []vocab.Definition{{Name: "Money", Definition: "An amount."}},
			Invariants: []vocab.Invariant{
				{Statement: "Money is never negative.", Owner: "Money"},
			},
		},
	}, []vocab.ContextRelation{{From: "catalog", To: "billing", Kind: vocab.RelationConformist}})
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	obs, err := conformance.NewObservations(
		[]conformance.ObservedFile{{Path: "m/event.go"}, {Path: "m/venue.go"}, {Path: "catalog/spec.go"}, {Path: "billing/money.go"}},
		map[string]conformance.LanguageFacts{
			"m/event.go": {
				Language:              rule.LanguageGo,
				DeclarationsAvailable: true,
				Declarations: []conformance.Declaration{
					{Kind: "struct", Name: "Event", Exported: true, StartLine: 3},
					{Kind: "struct", Name: "EventPublished", Exported: true, StartLine: 5},
					{Kind: "func", Name: "NewPrice", Results: []string{"Price", "error"}, StartLine: 8},
					{Kind: "method", Name: "PublishedFrozen", Owner: "Event", Exported: true, StartLine: 90},
					{Kind: "method", Name: "TiersPriced", Owner: "Event", Exported: true, StartLine: 120},
				},
			},
			"m/venue.go": {
				Language:              rule.LanguageGo,
				DeclarationsAvailable: true,
				Declarations:          []conformance.Declaration{{Kind: "struct", Name: "Venue", Exported: true, StartLine: 1}},
			},
			"catalog/spec.go": {
				Language:              rule.LanguageGo,
				DeclarationsAvailable: true,
				Declarations: []conformance.Declaration{
					{Kind: "struct", Name: "HighValueOrder", Exported: true, StartLine: 10},
					{Kind: "method", Name: "SatisfiedBy", Owner: "HighValueOrder", Exported: true, StartLine: 34},
					{Kind: "struct", Name: "LateOrder", Exported: true, StartLine: 50},
				},
			},
			"billing/money.go": {
				Language:              rule.LanguageGo,
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

func TestArchitecturalContextRepositoryCarriesWholeDomain(t *testing.T) {
	ctx := domainContext(t, application.ContextRequest{})
	dk := ctx.Domain
	if dk.Scoped || !dk.Located {
		t.Fatalf("repository scope: scoped=%v located=%v, want whole and located", dk.Scoped, dk.Located)
	}
	if dk.Counts.Contexts != 2 || dk.Shown != dk.Counts {
		t.Errorf("counts = %+v shown = %+v, want equal whole-model tallies", dk.Counts, dk.Shown)
	}
	if len(dk.Relations) != 1 {
		t.Errorf("relations = %+v, want the recorded one", dk.Relations)
	}
	kinds := []string{}
	for _, u := range dk.Unanchored {
		kinds = append(kinds, string(u.Kind)+":"+u.Owner+u.Name+":"+string(u.Anchor))
	}
	want := []string{
		"invariant:Event:unanchorable",
		"invariant:Venue:unanchorable",
		"invariant:Discount:missing",
		"specification:LateOrder:missing",
	}
	if strings.Join(kinds, " ") != strings.Join(want, " ") {
		t.Errorf("unanchored = %v, want %v", kinds, want)
	}
	for _, u := range dk.Unanchored {
		if u.Anchor == application.AnchorUnanchorable && u.Reason == "" {
			t.Errorf("unanchorable %+v carries no reason", u)
		}
	}
}

func TestArchitecturalContextPathScopesDomainToDeclarations(t *testing.T) {
	ctx := domainContext(t, application.ContextRequest{Paths: []string{"m/event.go"}})
	dk := ctx.Domain
	if !dk.Scoped {
		t.Fatalf("a path worksite must scope the domain")
	}
	if dk.Counts.Contexts != 2 || dk.Counts.Invariants != 6 {
		t.Errorf("counts = %+v, want the whole model tallied", dk.Counts)
	}
	if len(dk.Contexts) != 1 {
		t.Fatalf("contexts = %+v, want only catalog", dk.Contexts)
	}
	catalog := contextNamed(t, dk, "catalog")
	if len(catalog.Entities) != 1 || catalog.Entities[0].Name != "Event" || !catalog.Entities[0].Aggregate {
		t.Errorf("entities = %+v, want Event alone; Venue is declared elsewhere", catalog.Entities)
	}
	if len(catalog.ValueObjects) != 1 || catalog.ValueObjects[0] != "Price" {
		t.Errorf("value objects = %v, want Price through its constructor; Discount is declared nowhere", catalog.ValueObjects)
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
	want := vocab.Counts{Contexts: 1, Entities: 1, Aggregates: 1, ValueObjects: 1, Invariants: 3, Assertions: 1, Events: 1, Relations: 1}
	if dk.Shown != want {
		t.Errorf("shown = %+v, want %+v", dk.Shown, want)
	}
	if len(dk.Unanchored) != 1 || dk.Unanchored[0].Owner != "Event" || dk.Unanchored[0].Anchor != application.AnchorUnanchorable {
		t.Errorf("unanchored = %+v, want only the in-scope Event invariant without an id", dk.Unanchored)
	}
}

func TestArchitecturalContextFolderPathScopesDomain(t *testing.T) {
	ctx := domainContext(t, application.ContextRequest{Paths: []string{"m/"}})
	catalog := contextNamed(t, ctx.Domain, "catalog")
	if len(catalog.Entities) != 2 {
		t.Errorf("entities = %+v, want Event and Venue under the folder", catalog.Entities)
	}
	if ctx.Domain.Shown.Invariants != 4 {
		t.Errorf("shown invariants = %d, want the Event, Price, and Venue ones", ctx.Domain.Shown.Invariants)
	}
}

func TestArchitecturalContextModuleNamedForContextKeepsItWhole(t *testing.T) {
	ctx := domainContext(t, application.ContextRequest{Modules: []string{"catalog"}})
	dk := ctx.Domain
	if len(dk.Contexts) != 1 {
		t.Fatalf("contexts = %+v, want only catalog", dk.Contexts)
	}
	catalog := contextNamed(t, dk, "catalog")
	if len(catalog.Entities) != 2 || len(catalog.ValueObjects) != 2 || len(catalog.Invariants) != 5 || len(catalog.Specifications) != 2 {
		t.Errorf("a Module named for the context must keep it whole, got %+v", catalog)
	}
	if len(dk.Unanchored) != 4 {
		t.Errorf("unanchored = %+v, want every catalog contract that is not found", dk.Unanchored)
	}
}

func TestArchitecturalContextModuleScopesDomainToItsDeclarations(t *testing.T) {
	ctx := domainContext(t, application.ContextRequest{Modules: []string{"m"}})
	dk := ctx.Domain
	if len(dk.Contexts) != 1 {
		t.Fatalf("contexts = %+v, want only catalog", dk.Contexts)
	}
	catalog := contextNamed(t, dk, "catalog")
	if len(catalog.Entities) != 2 || len(catalog.Specifications) != 0 {
		t.Errorf("Module m declares Event and Venue and no specification, got %+v", catalog)
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
	ctx := domainContext(t, application.ContextRequest{Paths: []string{"m/event.go"}, Full: true})
	dk := ctx.Domain
	if dk.Scoped || len(dk.Contexts) != 2 || dk.Shown != dk.Counts {
		t.Errorf("--full must carry the whole model, got scoped=%v contexts=%d", dk.Scoped, len(dk.Contexts))
	}
	if len(dk.Unanchored) != 4 {
		t.Errorf("unanchored = %+v, want the whole model's four", dk.Unanchored)
	}
}

func TestArchitecturalContextWithoutObservationsIsNeverScoped(t *testing.T) {
	cfg, knowledge, _ := domainFixture(t)
	uc, err := application.NewGetArchitecturalContext(fakeRepository{cfg}, knowledge)
	if err != nil {
		t.Fatalf("NewGetArchitecturalContext: %v", err)
	}
	ctx, err := uc.Execute(application.ContextRequest{Paths: []string{"m/event.go"}})
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
