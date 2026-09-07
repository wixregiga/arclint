package plain

import (
	"bytes"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// scopedKnowledge is a worksite projection carrying every anchor
// outcome: a found aggregate invariant, a missing aggregate invariant,
// a missing value object invariant, a found assertion, and a missing
// specification.
func scopedKnowledge() *application.DomainKnowledge {
	return &application.DomainKnowledge{
		Source:  "domain.arclint.yaml",
		Counts:  vocab.Counts{Contexts: 2, Aggregates: 2, Entities: 1, ValueObjects: 3, Invariants: 5, Assertions: 1, Specifications: 1, Relations: 1},
		Scoped:  true,
		Shown:   vocab.Counts{Contexts: 1, Aggregates: 1, Entities: 1, ValueObjects: 1, Invariants: 3, Assertions: 1, Specifications: 1, Relations: 1},
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
			Assertions: []application.DomainAssertionRef{
				{Key: "capacity-fits", Statement: "Capacity fits the venue.", Owner: "Event", On: "Publish", Source: "event/publish.go:12", Anchor: application.AnchorFound},
			},
			Specifications: []application.DomainSpecificationRef{{Name: "LateOrder", Anchor: application.AnchorMissing}},
		}},
		Relations: []application.DomainRelationRef{{From: "catalog", To: "billing", Kind: "conformist"}},
		Unanchored: []application.UnanchoredContract{
			{Kind: application.ContractInvariant, Context: "catalog", Owner: "Event", Key: "one-venue", Statement: "An Event has one Venue.", Expected: "method OneVenue on Event"},
			{Kind: application.ContractInvariant, Context: "catalog", Owner: "Price", Key: "never-negative", Statement: "A Price is never negative.", Expected: "constructor of Price"},
			{Kind: application.ContractSpecification, Context: "catalog", Name: "LateOrder", Expected: "satisfaction method on LateOrder"},
		},
	}
}

func TestPlainContextScopedDomainBytes(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.ContextReport{Context: application.ArchitecturalContext{
		Scope:     "m/event.go",
		Paths:     []application.PathBinding{{Path: "m/event.go", Zones: []string{"m"}}},
		Languages: []string{"go"},
		RuleCount: 1,
		Zones:     []application.ZonePolicy{{Name: "m", Paths: []string{"m/**"}, External: "allow", Stdlib: "allow"}},
		Domain:    scopedKnowledge(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"scope: m/event.go",
		"  m/event.go → m",
		"languages: go",
		"configured rules: 1",
		"",
		"zone m",
		"  paths: m/**",
		"",
		"project domain (domain.arclint.yaml): 1 of 2 contexts, 1 of 2 aggregates, 1 of 1 entity, 1 of 3 value objects, 3 of 5 invariants, 1 of 1 assertion, 1 of 1 specification anchor into this scope; --full shows the whole model",
		"  context catalog:",
		"    aggregates: Event (EventID; Organizer)",
		"    value objects: Price",
		"    invariants:",
		"      published-frozen (Event): A published Event never changes. event/event.go:90",
		"      one-venue (Event): An Event has one Venue. missing",
		"      never-negative (Price): A Price is never negative. missing",
		"    assertions:",
		"      capacity-fits (Event, on Publish): Capacity fits the venue. event/publish.go:12",
		"    specifications:",
		"      LateOrder missing",
		"  relation: catalog -[conformist]-> billing",
		"  unanchored contracts: 3 missing",
		"    missing: invariant one-venue of Event (context catalog)",
		"      expected method OneVenue on Event",
		"    missing: invariant never-negative of Price (context catalog)",
		"      expected constructor of Price",
		"    missing: specification LateOrder (context catalog)",
		"      expected satisfaction method on LateOrder",
		"    arclint check reports each as a Violation of the built-in rule of its block",
		"",
	}, "\n")
	if buf.String() != want {
		t.Fatalf("bytes =\n%s\nwant\n%s", buf.String(), want)
	}
}

func TestPlainContextEmptyScopeAndWholeModelHeadlines(t *testing.T) {
	empty := &application.DomainKnowledge{
		Source:  "domain.arclint.yaml",
		Counts:  vocab.Counts{Contexts: 2, Aggregates: 1, Entities: 2, ValueObjects: 3, Invariants: 5},
		Scoped:  true,
		Located: true,
	}
	var buf bytes.Buffer
	if err := New().Render(&buf, cli.ContextReport{Context: application.ArchitecturalContext{Scope: "elsewhere.go", Domain: empty}}); err != nil {
		t.Fatal(err)
	}
	want := "project domain (domain.arclint.yaml): nothing recorded anchors into this scope; --full shows the whole model (2 contexts · 1 aggregate · 2 entities · 3 value objects · 5 invariants)\n"
	if !strings.Contains(buf.String(), want) {
		t.Fatalf("empty-scope headline absent from %q", buf.String())
	}

	whole := scopedKnowledge()
	whole.Scoped = false
	whole.Shown = whole.Counts
	buf.Reset()
	if err := New().Render(&buf, cli.ContextReport{Context: application.ArchitecturalContext{Scope: "repository", Domain: whole}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "project domain (domain.arclint.yaml): 2 contexts · 2 aggregates · 1 entity · 3 value objects · 5 invariants · 1 assertion · 1 specification · 1 relation\n") {
		t.Fatalf("whole-model headline absent from %q", buf.String())
	}
	if strings.Contains(buf.String(), "--full") {
		t.Fatalf("a whole listing must not point at --full: %q", buf.String())
	}
}

func TestPlainContextUnlocatedDomainCarriesNoAnchors(t *testing.T) {
	dk := &application.DomainKnowledge{
		Source: "domain.arclint.yaml",
		Counts: vocab.Counts{Contexts: 1, ValueObjects: 1, Invariants: 1},
		Shown:  vocab.Counts{Contexts: 1, ValueObjects: 1, Invariants: 1},
		Contexts: []application.DomainContextKnowledge{{
			Name:         "catalog",
			ValueObjects: []string{"Price"},
			Invariants:   []application.DomainInvariantRef{{Key: "never-negative", Statement: "A Price is never negative.", Owner: "Price", OwnerConcept: vocab.ConceptValueObject}},
		}},
	}
	var buf bytes.Buffer
	if err := New().Render(&buf, cli.ContextReport{Context: application.ArchitecturalContext{Scope: "repository", Domain: dk}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "      never-negative (Price): A Price is never negative.\n") {
		t.Fatalf("an unlocated contract must print bare: %q", buf.String())
	}
	if strings.Contains(buf.String(), "unanchored") || strings.Contains(buf.String(), "missing") {
		t.Fatalf("an unlocated projection must not report anchors: %q", buf.String())
	}
}

// The overview prints each contract's anchor under it: the source when
// a declaration carries it, "missing" when none does.
func TestPlainDomainOverviewSourcePhrases(t *testing.T) {
	lang, err := vocab.NewUbiquitousLanguage("boxoffice", "", []vocab.BoundedContext{{
		Name:       "catalog",
		Definition: "What is on sale.",
		Aggregates: []vocab.Aggregate{{
			Name: "Event", Definition: "A show.", Identity: "EventID",
			Invariants: []vocab.Invariant{
				{Key: "published-frozen", Statement: "A published Event never changes."},
				{Key: "one-venue", Statement: "An Event has one Venue."},
			},
			Assertions: []vocab.Assertion{{Key: "capacity-fits", On: "Publish", Statement: "Capacity fits the venue."}},
		}},
		ValueObjects: []vocab.ValueObject{{
			Name: "Price", Definition: "Whole cents.",
			Invariants: []vocab.Invariant{{Key: "never-negative", Statement: "A Price is never negative."}},
		}},
		Specifications: []vocab.Specification{{Name: "LateOrder", Definition: "An order placed after doors open."}},
	}}, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	var buf bytes.Buffer
	err = New().Render(&buf, cli.DomainOverviewReport{Overview: application.DomainOverview{
		Found:    true,
		Source:   "domain.arclint.yaml",
		Counts:   lang.Counts(),
		Language: lang,
		Matrix:   scopedKnowledge(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"Project domain: boxoffice\n",
		"1 context · 1 aggregate · 1 value object · 3 invariants · 1 assertion · 1 specification\n",
		"      published-frozen  A published Event never changes.\n        source: event/event.go:90\n",
		"      one-venue  An Event has one Venue.\n        source: missing\n",
		"      capacity-fits (on Publish)  Capacity fits the venue.\n        source: event/publish.go:12\n",
		"      invariant never-negative  A Price is never negative.\n        source: missing\n",
		"    LateOrder  An order placed after doors open.\n      source: missing\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("overview lacks %q:\n%s", want, out)
		}
	}
}
