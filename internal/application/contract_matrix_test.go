package application

import (
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

func TestLocateSpecificationFindsSatisfiedBy(t *testing.T) {
	idx := declsOn(t, "order/spec.go", rule.LanguageGo, []conformance.Declaration{
		{Kind: "struct", Name: "HighValueOrder", Exported: true, StartLine: 10, EndLine: 12},
		{Kind: "method", Name: "SatisfiedBy", Owner: "HighValueOrder", Exported: true, StartLine: 34, EndLine: 36},
	})
	got := locateSpecification(idx, "HighValueOrder")
	if got != "order/spec.go:34" {
		t.Fatalf("locateSpecification = %q, want order/spec.go:34", got)
	}
}

func TestLocateSpecificationTypeWithoutSatisfiedByIsMissing(t *testing.T) {
	idx := declsOn(t, "order/spec.go", rule.LanguageGo, []conformance.Declaration{
		{Kind: "struct", Name: "HighValueOrder", Exported: true, StartLine: 10, EndLine: 12},
	})
	got := locateSpecification(idx, "HighValueOrder")
	if got != sourceMissing {
		t.Fatalf("locateSpecification = %q, want %q", got, sourceMissing)
	}
}

func TestLocateSpecificationAbsentIsMissing(t *testing.T) {
	got := locateSpecification(declsOn(t, "order/spec.go", rule.LanguageGo, nil), "HighValueOrder")
	if got != sourceMissing {
		t.Fatalf("locateSpecification = %q, want %q", got, sourceMissing)
	}
}

func TestLocateInvariantValueIntegrityUsesConstructor(t *testing.T) {
	idx := declsOn(t, "event/event.go", rule.LanguageGo, []conformance.Declaration{
		{Kind: "func", Name: "NewPrice", Results: []string{"Price", "error"}, StartLine: 8, EndLine: 14},
	})
	got := locateInvariant(idx, catalogContext(t), DomainInvariantRef{Owner: "Price"})
	if got != "event/event.go:8" {
		t.Fatalf("locateInvariant Price = %q, want event/event.go:8", got)
	}
}

func TestLocateInvariantClusterUsesNamedMethod(t *testing.T) {
	idx := declsOn(t, "event/event.go", rule.LanguageGo, []conformance.Declaration{
		{Kind: "method", Name: "PublishedFrozen", Owner: "Event", Exported: true, StartLine: 90, EndLine: 96},
	})
	got := locateInvariant(idx, catalogContext(t), DomainInvariantRef{Owner: "Event", ID: "published-frozen"})
	if got != "event/event.go:90" {
		t.Fatalf("locateInvariant Event = %q, want event/event.go:90", got)
	}
}

func TestLocateInvariantUnknownOwnerIsBlank(t *testing.T) {
	got := locateInvariant(nil, catalogContext(t), DomainInvariantRef{Owner: "Nobody", ID: "x"})
	if got != "" {
		t.Fatalf("locateInvariant unknown = %q, want empty", got)
	}
}

func TestLocateAssertionUsesNamedMethod(t *testing.T) {
	idx := declsOn(t, "event/event.go", rule.LanguageGo, []conformance.Declaration{
		{Kind: "method", Name: "TiersPriced", Owner: "Event", Exported: true, StartLine: 120, EndLine: 128},
	})
	got := locateAssertion(idx, DomainAssertionRef{Owner: "Event", ID: "tiers-priced"})
	if got != "event/event.go:120" {
		t.Fatalf("locateAssertion = %q, want event/event.go:120", got)
	}
}

func TestLocateNamedMethodMissingIsMissing(t *testing.T) {
	got := locateNamedMethod(declsOn(t, "event/event.go", rule.LanguageGo, nil), "Event", "published-frozen")
	if got != sourceMissing {
		t.Fatalf("locateNamedMethod = %q, want %q", got, sourceMissing)
	}
}

func TestLocateNamedMethodUsesLanguageCase(t *testing.T) {
	idx := declsOn(t, "event.ts", rule.LanguageTypeScript, []conformance.Declaration{
		{Kind: "method", Name: "publishedFrozen", Owner: "Event", Exported: true, StartLine: 4, EndLine: 6},
	})
	got := locateNamedMethod(idx, "Event", "published-frozen")
	if got != "event.ts:4" {
		t.Fatalf("locateNamedMethod ts = %q, want event.ts:4", got)
	}
}

func TestLocateConstructorVariants(t *testing.T) {
	tests := []struct {
		name  string
		lang  rule.Language
		path  string
		decls []conformance.Declaration
		typ   string
		want  string
	}{
		{
			name: "go NewType",
			lang: rule.LanguageGo,
			path: "price.go",
			decls: []conformance.Declaration{
				{Kind: "func", Name: "NewPrice", Results: []string{"Price", "error"}, StartLine: 3},
			},
			typ:  "Price",
			want: "price.go:3",
		},
		{
			name: "go New returning pointer",
			lang: rule.LanguageGo,
			path: "event.go",
			decls: []conformance.Declaration{
				{Kind: "func", Name: "New", Results: []string{"*Event", "error"}, StartLine: 11},
			},
			typ:  "Event",
			want: "event.go:11",
		},
		{
			name: "typescript constructor",
			lang: rule.LanguageTypeScript,
			path: "event.ts",
			decls: []conformance.Declaration{
				{Kind: "method", Name: "constructor", Owner: "Event", StartLine: 2},
			},
			typ:  "Event",
			want: "event.ts:2",
		},
		{
			name: "python init",
			lang: rule.LanguagePython,
			path: "event.py",
			decls: []conformance.Declaration{
				{Kind: "method", Name: "__init__", Owner: "Event", StartLine: 5},
			},
			typ:  "Event",
			want: "event.py:5",
		},
		{
			name: "missing",
			lang: rule.LanguageGo,
			path: "empty.go",
			typ:  "Price",
			want: sourceMissing,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := locateConstructor(declsOn(t, tc.path, tc.lang, tc.decls), tc.typ)
			if got != tc.want {
				t.Fatalf("locateConstructor = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestOwnerKind(t *testing.T) {
	ctx := catalogContext(t)
	if agg, vo := ownerKind(ctx, "Event"); !agg || vo {
		t.Fatalf("Event: aggregate=%v valueObject=%v", agg, vo)
	}
	if agg, vo := ownerKind(ctx, "Price"); agg || !vo {
		t.Fatalf("Price: aggregate=%v valueObject=%v", agg, vo)
	}
	if agg, vo := ownerKind(ctx, "Ghost"); agg || vo {
		t.Fatalf("Ghost: aggregate=%v valueObject=%v", agg, vo)
	}
}

func TestMethodCase(t *testing.T) {
	if got := methodCase(rule.LanguageGo); got != "PascalCase" {
		t.Fatalf("go = %q", got)
	}
	if got := methodCase(rule.LanguageTypeScript); got != "camelCase" {
		t.Fatalf("ts = %q", got)
	}
	if got := methodCase(rule.LanguagePython); got != "snake_case" {
		t.Fatalf("py = %q", got)
	}
	if got := methodCase(""); got != "PascalCase" {
		t.Fatalf("default = %q", got)
	}
}

func TestIndexDeclarationsSkipsUnusableFacts(t *testing.T) {
	obs, err := conformance.NewObservations(
		[]conformance.ObservedFile{{Path: "ok.go"}, {Path: "bad.go"}, {Path: "empty.go"}},
		map[string]conformance.LanguageFacts{
			"ok.go": {
				Language:              rule.LanguageGo,
				DeclarationsAvailable: true,
				Declarations:          []conformance.Declaration{{Kind: "func", Name: "New", StartLine: 1}},
			},
			"bad.go": {
				Language:              rule.LanguageGo,
				DeclarationsAvailable: true,
				ParseFailure:          "parse: boom",
			},
			"empty.go": {
				Language:              rule.LanguageGo,
				DeclarationsAvailable: false,
			},
		},
	)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	idx := indexDeclarations(obs)
	if len(idx) != 1 || idx[0].path != "ok.go" {
		t.Fatalf("indexDeclarations = %+v, want one hit on ok.go", idx)
	}
}

func TestLocateDomainContractsFillsSources(t *testing.T) {
	lang, err := vocab.NewUbiquitousLanguage([]vocab.BoundedContext{{
		Name: "catalog",
		Entities: []vocab.Entity{{
			Definition: vocab.Definition{Name: "Event", Definition: "A show."},
			Aggregate:  true,
		}},
		ValueObjects: []vocab.Definition{{Name: "Price", Definition: "Whole cents."}},
		Invariants: []vocab.Invariant{
			{Statement: "A published Event never changes.", Owner: "Event", ID: "published-frozen"},
			{Statement: "A Price is never negative.", Owner: "Price"},
		},
		Assertions: []vocab.Assertion{
			{Statement: "Tiers are priced.", Owner: "Event", ID: "tiers-priced", On: "Publish"},
		},
		Specifications: []vocab.Specification{
			{Name: "HighValueOrder", Definition: "Orders above a threshold."},
		},
	}}, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	obs, err := conformance.NewObservations(
		[]conformance.ObservedFile{{Path: "event/event.go"}, {Path: "order/spec.go"}},
		map[string]conformance.LanguageFacts{
			"event/event.go": {
				Language:              rule.LanguageGo,
				DeclarationsAvailable: true,
				Declarations: []conformance.Declaration{
					{Kind: "func", Name: "NewPrice", Results: []string{"Price", "error"}, StartLine: 8},
					{Kind: "method", Name: "PublishedFrozen", Owner: "Event", Exported: true, StartLine: 90},
					{Kind: "method", Name: "TiersPriced", Owner: "Event", Exported: true, StartLine: 120},
				},
			},
			"order/spec.go": {
				Language:              rule.LanguageGo,
				DeclarationsAvailable: true,
				Declarations: []conformance.Declaration{
					{Kind: "method", Name: "SatisfiedBy", Owner: "HighValueOrder", Exported: true, StartLine: 34},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	dk := domainKnowledgeOf(lang)
	locateDomainContracts(dk, lang, obs)
	if dk.Contexts[0].Invariants[0].Source != "event/event.go:90" {
		t.Fatalf("cluster source = %q", dk.Contexts[0].Invariants[0].Source)
	}
	if dk.Contexts[0].Invariants[1].Source != "event/event.go:8" {
		t.Fatalf("value integrity source = %q", dk.Contexts[0].Invariants[1].Source)
	}
	if dk.Contexts[0].Assertions[0].Source != "event/event.go:120" {
		t.Fatalf("assertion source = %q", dk.Contexts[0].Assertions[0].Source)
	}
	if dk.Contexts[0].Specifications[0].Source != "order/spec.go:34" {
		t.Fatalf("specification source = %q", dk.Contexts[0].Specifications[0].Source)
	}
}

func TestLocateDomainContractsNilIsSafe(t *testing.T) {
	locateDomainContracts(nil, vocab.UbiquitousLanguage{}, conformance.Observations{})
}

func catalogContext(t *testing.T) vocab.BoundedContext {
	t.Helper()
	return vocab.BoundedContext{
		Name: "catalog",
		Entities: []vocab.Entity{{
			Definition: vocab.Definition{Name: "Event", Definition: "A show."},
			Aggregate:  true,
		}},
		ValueObjects: []vocab.Definition{{Name: "Price", Definition: "Whole cents."}},
	}
}

func declsOn(t *testing.T, path string, lang rule.Language, decls []conformance.Declaration) []declHit {
	t.Helper()
	obs, err := conformance.NewObservations(
		[]conformance.ObservedFile{{Path: path}},
		map[string]conformance.LanguageFacts{
			path: {
				Language:              lang,
				DeclarationsAvailable: true,
				Declarations:          decls,
			},
		},
	)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	return indexDeclarations(obs)
}
