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
// outcome: a found cluster invariant, an unanchorable aggregate
// invariant without an id, a missing value integrity, and a missing
// specification.
func scopedKnowledge() *application.DomainKnowledge {
	return &application.DomainKnowledge{
		Source:  "domain.arclint.yaml",
		Counts:  vocab.Counts{Contexts: 2, Entities: 2, Aggregates: 1, ValueObjects: 3, Invariants: 5, Specifications: 1, Relations: 1},
		Scoped:  true,
		Shown:   vocab.Counts{Contexts: 1, Entities: 1, Aggregates: 1, ValueObjects: 1, Invariants: 3, Specifications: 1, Relations: 1},
		Located: true,
		Contexts: []application.DomainContextKnowledge{{
			Name:         "catalog",
			Entities:     []application.DomainEntityRef{{Name: "Event", Aggregate: true}},
			ValueObjects: []string{"Price"},
			Invariants: []application.DomainInvariantRef{
				{Statement: "A published Event never changes.", Owner: "Event", ID: "published-frozen", Source: "event/event.go:90", Anchor: application.AnchorFound},
				{Statement: "An Event has one Venue.", Owner: "Event", Anchor: application.AnchorUnanchorable, Reason: "owner Event is an aggregate and the invariant has no id, so no method is named to carry it"},
				{Statement: "A Price is never negative.", Owner: "Price", Anchor: application.AnchorMissing},
			},
			Specifications: []application.DomainSpecificationRef{{Name: "LateOrder", Anchor: application.AnchorMissing}},
		}},
		Relations: []application.DomainRelationRef{{From: "catalog", To: "billing", Kind: "conformist"}},
		Unanchored: []application.UnanchoredContract{
			{Kind: application.ContractInvariant, Context: "catalog", Owner: "Event", Statement: "An Event has one Venue.", Anchor: application.AnchorUnanchorable, Reason: "owner Event is an aggregate and the invariant has no id, so no method is named to carry it"},
			{Kind: application.ContractInvariant, Context: "catalog", Owner: "Price", Statement: "A Price is never negative.", Anchor: application.AnchorMissing},
			{Kind: application.ContractSpecification, Context: "catalog", Name: "LateOrder", Anchor: application.AnchorMissing},
		},
	}
}

func TestPlainContextScopedDomainBytes(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.ContextReport{Context: application.ArchitecturalContext{
		Scope:     "m/event.go",
		Paths:     []application.PathBinding{{Path: "m/event.go", Modules: []string{"m"}}},
		Languages: []string{"go"},
		RuleCount: 1,
		Modules:   []application.ModulePolicy{{Name: "m", Paths: []string{"m/**"}, External: "allow", Stdlib: "allow"}},
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
		"module m",
		"  paths: m/**",
		"",
		"project domain (domain.arclint.yaml): 1 of 2 contexts, 1 of 2 entities, 1 of 3 value objects, 3 of 5 invariants, 1 of 1 specification anchor into this scope; --full shows the whole model",
		"  context catalog:",
		"    entities: Event [aggregate]",
		"    value objects: Price",
		"    invariants:",
		"      A published Event never changes. (owner: Event, id: published-frozen) event/event.go:90",
		"      An Event has one Venue. (owner: Event) unanchorable",
		"      A Price is never negative. (owner: Price) missing",
		"    specifications:",
		"      LateOrder missing",
		"  relation: catalog -[conformist]-> billing",
		"  unanchored contracts: 1 unanchorable, 2 missing",
		"    unanchorable: 1 invariant owned by Event (context catalog)",
		"      owner Event is an aggregate and the invariant has no id, so no method is named to carry it",
		"    missing: 1 invariant owned by Price (context catalog)",
		"      no constructor declared for Price",
		"    missing: specification LateOrder (context catalog)",
		"      no SatisfiedBy method declared on LateOrder",
		"    an unanchorable contract needs its recording changed before any source can carry it",
		"    an invariants Rule on the owning Module reports each missing contract as a Violation",
		"",
	}, "\n")
	if buf.String() != want {
		t.Fatalf("bytes =\n%s\nwant\n%s", buf.String(), want)
	}
}

func TestPlainContextEmptyScopeAndWholeModelHeadlines(t *testing.T) {
	empty := &application.DomainKnowledge{
		Source:  "domain.arclint.yaml",
		Counts:  vocab.Counts{Contexts: 2, Entities: 2, Aggregates: 1, ValueObjects: 3, Invariants: 5},
		Scoped:  true,
		Located: true,
	}
	var buf bytes.Buffer
	if err := New().Render(&buf, cli.ContextReport{Context: application.ArchitecturalContext{Scope: "elsewhere.go", Domain: empty}}); err != nil {
		t.Fatal(err)
	}
	want := "project domain (domain.arclint.yaml): nothing recorded anchors into this scope; --full shows the whole model (2 contexts, 2 entities (1 aggregate), 3 value objects, 5 invariants, 0 assertions, 0 specifications, 0 events)\n"
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
	if !strings.Contains(buf.String(), "project domain (domain.arclint.yaml): 2 contexts, 2 entities (1 aggregate), 3 value objects, 5 invariants, 0 assertions, 1 specification, 0 events\n") {
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
			Invariants:   []application.DomainInvariantRef{{Statement: "A Price is never negative.", Owner: "Price"}},
		}},
	}
	var buf bytes.Buffer
	if err := New().Render(&buf, cli.ContextReport{Context: application.ArchitecturalContext{Scope: "repository", Domain: dk}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "      A Price is never negative. (owner: Price)\n") {
		t.Fatalf("an unlocated contract must print bare: %q", buf.String())
	}
	if strings.Contains(buf.String(), "unanchored") || strings.Contains(buf.String(), "missing") {
		t.Fatalf("an unlocated projection must not report anchors: %q", buf.String())
	}
}

func TestPlainDomainOverviewSourcePhrases(t *testing.T) {
	lang, err := vocab.NewUbiquitousLanguage([]vocab.BoundedContext{{
		Name: "catalog",
		Entities: []vocab.Entity{{
			Definition: vocab.Definition{Name: "Event", Definition: "A show."},
			Aggregate:  true,
		}},
		ValueObjects: []vocab.Definition{{Name: "Price", Definition: "Whole cents."}},
		Invariants: []vocab.Invariant{
			{Statement: "A published Event never changes.", Owner: "Event", ID: "published-frozen"},
			{Statement: "An Event has one Venue.", Owner: "Event"},
			{Statement: "A Price is never negative.", Owner: "Price"},
		},
	}}, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	matrix := scopedKnowledge()
	var buf bytes.Buffer
	err = New().Render(&buf, cli.DomainOverviewReport{Overview: application.DomainOverview{
		Found:    true,
		Source:   "domain.arclint.yaml",
		Counts:   lang.Counts(),
		Language: lang,
		Matrix:   matrix,
	}})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"    source: event/event.go:90\n",
		"    source: unanchorable (owner Event is an aggregate and the invariant has no id, so no method is named to carry it)\n",
		"    source: missing\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("overview lacks %q:\n%s", want, out)
		}
	}
}
