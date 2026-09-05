package application

import (
	"strings"
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
	got, found := locateSpecification(idx, "HighValueOrder")
	if !found || got != "order/spec.go:34" {
		t.Fatalf("locateSpecification = %q, %v, want order/spec.go:34, true", got, found)
	}
}

func TestLocateSpecificationTypeWithoutSatisfiedByIsMissing(t *testing.T) {
	idx := declsOn(t, "order/spec.go", rule.LanguageGo, []conformance.Declaration{
		{Kind: "struct", Name: "HighValueOrder", Exported: true, StartLine: 10, EndLine: 12},
	})
	got, found := locateSpecification(idx, "HighValueOrder")
	if found || got != "" {
		t.Fatalf("locateSpecification = %q, %v, want empty, false", got, found)
	}
}

func TestLocateSpecificationAbsentIsMissing(t *testing.T) {
	got, found := locateSpecification(declsOn(t, "order/spec.go", rule.LanguageGo, nil), "HighValueOrder")
	if found || got != "" {
		t.Fatalf("locateSpecification = %q, %v, want empty, false", got, found)
	}
}

func TestLocateInvariantValueIntegrityUsesConstructor(t *testing.T) {
	idx := declsOn(t, "event/event.go", rule.LanguageGo, []conformance.Declaration{
		{Kind: "func", Name: "NewPrice", Results: []string{"Price", "error"}, StartLine: 8, EndLine: 14},
	})
	src, anchor, reason := locateInvariant(idx, catalogContext(t), DomainInvariantRef{Owner: "Price"})
	if src != "event/event.go:8" || anchor != AnchorFound || reason != "" {
		t.Fatalf("locateInvariant Price = %q, %s, %q; want event/event.go:8, found, no reason", src, anchor, reason)
	}
}

func TestLocateInvariantValueIntegrityWithoutConstructorIsMissing(t *testing.T) {
	src, anchor, reason := locateInvariant(nil, catalogContext(t), DomainInvariantRef{Owner: "Price"})
	if src != "" || anchor != AnchorMissing || reason != "" {
		t.Fatalf("locateInvariant Price = %q, %s, %q; want empty, missing, no reason", src, anchor, reason)
	}
}

func TestLocateInvariantClusterUsesNamedMethod(t *testing.T) {
	idx := declsOn(t, "event/event.go", rule.LanguageGo, []conformance.Declaration{
		{Kind: "method", Name: "PublishedFrozen", Owner: "Event", Exported: true, StartLine: 90, EndLine: 96},
	})
	src, anchor, reason := locateInvariant(idx, catalogContext(t), DomainInvariantRef{Owner: "Event", ID: "published-frozen"})
	if src != "event/event.go:90" || anchor != AnchorFound || reason != "" {
		t.Fatalf("locateInvariant Event = %q, %s, %q; want event/event.go:90, found, no reason", src, anchor, reason)
	}
}

func TestLocateInvariantClusterWithoutMethodIsMissing(t *testing.T) {
	src, anchor, reason := locateInvariant(nil, catalogContext(t), DomainInvariantRef{Owner: "Event", ID: "published-frozen"})
	if src != "" || anchor != AnchorMissing || reason != "" {
		t.Fatalf("locateInvariant Event = %q, %s, %q; want empty, missing, no reason", src, anchor, reason)
	}
}

// Each recorded shape that names no declaration is unanchorable, and
// the reason names the shape: an aggregate without an id, a value
// object with one, an entity that is no aggregate, and an owner the
// context never recorded.
func TestLocateInvariantUnanchorableShapes(t *testing.T) {
	ctx := catalogContext(t)
	idx := declsOn(t, "event/event.go", rule.LanguageGo, []conformance.Declaration{
		{Kind: "func", Name: "NewPrice", Results: []string{"Price", "error"}, StartLine: 8},
		{Kind: "method", Name: "PublishedFrozen", Owner: "Event", Exported: true, StartLine: 90},
		{Kind: "method", Name: "Named", Owner: "Venue", Exported: true, StartLine: 120},
	})
	tests := []struct {
		name   string
		inv    DomainInvariantRef
		reason string
	}{
		{"aggregate without id", DomainInvariantRef{Owner: "Event"}, "owner Event is an aggregate and the invariant has no id"},
		{"value object with id", DomainInvariantRef{Owner: "Price", ID: "positive"}, "owner Price is a value object and the invariant carries id positive"},
		{"entity that is no aggregate", DomainInvariantRef{Owner: "Venue", ID: "named"}, "owner Venue is an entity that is not an aggregate"},
		{"unrecorded owner", DomainInvariantRef{Owner: "Nobody", ID: "x"}, "owner Nobody is not a recorded entity or value object of context catalog"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src, anchor, reason := locateInvariant(idx, ctx, tc.inv)
			if src != "" || anchor != AnchorUnanchorable {
				t.Fatalf("locateInvariant = %q, %s; want empty source, unanchorable", src, anchor)
			}
			if !strings.Contains(reason, tc.reason) {
				t.Fatalf("reason = %q, want it to contain %q", reason, tc.reason)
			}
		})
	}
}

func TestLocateNamedMethodMissingIsMissing(t *testing.T) {
	got, found := locateNamedMethod(declsOn(t, "event/event.go", rule.LanguageGo, nil), "Event", "published-frozen")
	if found || got != "" {
		t.Fatalf("locateNamedMethod = %q, %v, want empty, false", got, found)
	}
}

func TestLocateNamedMethodUsesLanguageCase(t *testing.T) {
	idx := declsOn(t, "event.ts", rule.LanguageTypeScript, []conformance.Declaration{
		{Kind: "method", Name: "publishedFrozen", Owner: "Event", Exported: true, StartLine: 4, EndLine: 6},
	})
	got, found := locateNamedMethod(idx, "Event", "published-frozen")
	if !found || got != "event.ts:4" {
		t.Fatalf("locateNamedMethod ts = %q, %v, want event.ts:4, true", got, found)
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
		found bool
	}{
		{
			name: "go NewType",
			lang: rule.LanguageGo,
			path: "price.go",
			decls: []conformance.Declaration{
				{Kind: "func", Name: "NewPrice", Results: []string{"Price", "error"}, StartLine: 3},
			},
			typ:   "Price",
			want:  "price.go:3",
			found: true,
		},
		{
			name: "go New returning pointer",
			lang: rule.LanguageGo,
			path: "event.go",
			decls: []conformance.Declaration{
				{Kind: "func", Name: "New", Results: []string{"*Event", "error"}, StartLine: 11},
			},
			typ:   "Event",
			want:  "event.go:11",
			found: true,
		},
		{
			name: "typescript constructor",
			lang: rule.LanguageTypeScript,
			path: "event.ts",
			decls: []conformance.Declaration{
				{Kind: "method", Name: "constructor", Owner: "Event", StartLine: 2},
			},
			typ:   "Event",
			want:  "event.ts:2",
			found: true,
		},
		{
			name: "python init",
			lang: rule.LanguagePython,
			path: "event.py",
			decls: []conformance.Declaration{
				{Kind: "method", Name: "__init__", Owner: "Event", StartLine: 5},
			},
			typ:   "Event",
			want:  "event.py:5",
			found: true,
		},
		{
			name: "missing",
			lang: rule.LanguageGo,
			path: "empty.go",
			typ:  "Price",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, found := locateConstructor(declsOn(t, tc.path, tc.lang, tc.decls), tc.typ)
			if got != tc.want || found != tc.found {
				t.Fatalf("locateConstructor = %q, %v, want %q, %v", got, found, tc.want, tc.found)
			}
		})
	}
}

func TestTypeDeclarationPathsListsEachFileOnce(t *testing.T) {
	obs, err := conformance.NewObservations(
		[]conformance.ObservedFile{{Path: "a/event.go"}, {Path: "b/event.ts"}, {Path: "c/other.go"}},
		map[string]conformance.LanguageFacts{
			"a/event.go": {
				Language:              rule.LanguageGo,
				DeclarationsAvailable: true,
				Declarations: []conformance.Declaration{
					{Kind: "struct", Name: "Event", StartLine: 3},
					{Kind: "interface", Name: "Event", StartLine: 9},
					{Kind: "func", Name: "Event", StartLine: 20},
				},
			},
			"b/event.ts": {
				Language:              rule.LanguageTypeScript,
				DeclarationsAvailable: true,
				Declarations:          []conformance.Declaration{{Kind: "class", Name: "Event", StartLine: 1}},
			},
			"c/other.go": {
				Language:              rule.LanguageGo,
				DeclarationsAvailable: true,
				Declarations:          []conformance.Declaration{{Kind: "struct", Name: "Price", StartLine: 1}},
			},
		},
	)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	got := typeDeclarationPaths(indexDeclarations(obs), "Event")
	if len(got) != 2 || got[0] != "a/event.go" || got[1] != "b/event.ts" {
		t.Fatalf("typeDeclarationPaths = %v, want [a/event.go b/event.ts]", got)
	}
	if got := typeDeclarationPaths(indexDeclarations(obs), "Ghost"); len(got) != 0 {
		t.Fatalf("typeDeclarationPaths Ghost = %v, want none", got)
	}
}

func TestOwnerKind(t *testing.T) {
	ctx := catalogContext(t)
	if agg, vo := ownerKind(ctx, "Event"); !agg || vo {
		t.Fatalf("Event: aggregate=%v valueObject=%v", agg, vo)
	}
	if agg, vo := ownerKind(ctx, "Venue"); agg || vo {
		t.Fatalf("Venue: aggregate=%v valueObject=%v", agg, vo)
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

func TestLocateDomainContractsFillsSourcesAndAnchors(t *testing.T) {
	lang := catalogLanguage(t)
	dk := domainKnowledgeOf(lang)
	if dk.Located {
		t.Fatalf("projection reports Located before any observation")
	}
	locateDomainContracts(dk, lang, indexDeclarations(catalogObservations(t)))
	if !dk.Located {
		t.Fatalf("projection does not report Located after locating")
	}
	inv := dk.Contexts[0].Invariants
	if inv[0].Source != "event/event.go:90" || inv[0].Anchor != AnchorFound {
		t.Fatalf("cluster = %+v", inv[0])
	}
	if inv[1].Source != "event/event.go:8" || inv[1].Anchor != AnchorFound {
		t.Fatalf("value integrity = %+v", inv[1])
	}
	if inv[2].Source != "" || inv[2].Anchor != AnchorUnanchorable || inv[2].Reason == "" {
		t.Fatalf("aggregate without id = %+v, want unanchorable with a reason", inv[2])
	}
	if inv[3].Source != "" || inv[3].Anchor != AnchorMissing || inv[3].Reason != "" {
		t.Fatalf("value object without constructor = %+v, want missing without a reason", inv[3])
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
	locateDomainContracts(nil, vocab.UbiquitousLanguage{}, nil)
}

// The unanchored listing puts every unanchorable contract before every
// missing one and never lists a found contract; a projection that was
// never located lists nothing, since nothing was looked for.
func TestUnanchoredContractsOrdersUnanchorableFirst(t *testing.T) {
	lang := catalogLanguage(t)
	dk := domainKnowledgeOf(lang)
	if got := unanchoredContracts(dk); got != nil {
		t.Fatalf("unanchored before locating = %+v, want none", got)
	}
	locateDomainContracts(dk, lang, indexDeclarations(catalogObservations(t)))
	got := unanchoredContracts(dk)
	if len(got) != 3 {
		t.Fatalf("unanchored = %+v, want 3", got)
	}
	if got[0].Kind != "invariant" || got[0].Owner != "Event" || got[0].Anchor != AnchorUnanchorable || got[0].Reason == "" {
		t.Fatalf("first = %+v, want the unanchorable Event invariant with its reason", got[0])
	}
	if got[1].Kind != "invariant" || got[1].Owner != "Discount" || got[1].Anchor != AnchorMissing {
		t.Fatalf("second = %+v, want the missing Discount invariant", got[1])
	}
	if got[2].Kind != "specification" || got[2].Name != "LateOrder" || got[2].Anchor != AnchorMissing {
		t.Fatalf("third = %+v, want the missing LateOrder specification", got[2])
	}
	for _, c := range got {
		if c.Context != "catalog" {
			t.Fatalf("contract %+v names context %q, want catalog", c, c.Context)
		}
	}
}

func catalogContext(t *testing.T) vocab.BoundedContext {
	t.Helper()
	return vocab.BoundedContext{
		Name: "catalog",
		Entities: []vocab.Entity{
			{Definition: vocab.Definition{Name: "Event", Definition: "A show."}, Aggregate: true},
			{Definition: vocab.Definition{Name: "Venue", Definition: "A hall."}},
		},
		ValueObjects: []vocab.Definition{{Name: "Price", Definition: "Whole cents."}},
	}
}

// catalogLanguage records one context whose contracts cover every
// anchor outcome: a found cluster invariant, a found value integrity,
// an unanchorable aggregate invariant without an id, a missing value
// integrity, a found assertion, and one found and one missing
// specification.
func catalogLanguage(t *testing.T) vocab.UbiquitousLanguage {
	t.Helper()
	lang, err := vocab.NewUbiquitousLanguage([]vocab.BoundedContext{{
		Name: "catalog",
		Entities: []vocab.Entity{{
			Definition: vocab.Definition{Name: "Event", Definition: "A show."},
			Aggregate:  true,
		}},
		ValueObjects: []vocab.Definition{
			{Name: "Price", Definition: "Whole cents."},
			{Name: "Discount", Definition: "A percentage off."},
		},
		Invariants: []vocab.Invariant{
			{Statement: "A published Event never changes.", Owner: "Event", ID: "published-frozen"},
			{Statement: "A Price is never negative.", Owner: "Price"},
			{Statement: "An Event has at most one Venue.", Owner: "Event"},
			{Statement: "A Discount never exceeds the whole.", Owner: "Discount"},
		},
		Assertions: []vocab.Assertion{
			{Statement: "Tiers are priced.", Owner: "Event", ID: "tiers-priced", On: "Publish"},
		},
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
					{Kind: "method", Name: "PublishedFrozen", Owner: "Event", Exported: true, StartLine: 90},
					{Kind: "method", Name: "TiersPriced", Owner: "Event", Exported: true, StartLine: 120},
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
