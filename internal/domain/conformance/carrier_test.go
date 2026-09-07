package conformance_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// parsed is the facts of one parsed file: its declarations and nothing
// else.
func parsed(lang rule.Language, pkg string, decls []conformance.Declaration) conformance.LanguageFacts {
	return conformance.LanguageFacts{
		Language:              lang,
		Package:               pkg,
		DeclarationsAvailable: true,
		Declarations:          decls,
	}
}

// contextOf records one context with the given aggregates, value
// objects, and specifications, every term carrying a definition.
func contextOf(name string, aggregates []vocab.Aggregate, values []vocab.ValueObject, specs []vocab.Specification) vocab.BoundedContext {
	return vocab.BoundedContext{
		Name:           name,
		Definition:     "the " + name + " context",
		Aggregates:     aggregates,
		ValueObjects:   values,
		Specifications: specs,
	}
}

func valueObject(name string) vocab.ValueObject {
	return vocab.ValueObject{Name: name, Definition: "the " + name + " value"}
}

func aggregateOf(name, identity string, invariants []string, assertions []vocab.Assertion) vocab.Aggregate {
	agg := vocab.Aggregate{Name: name, Definition: "the " + name + " aggregate", Identity: identity, Assertions: assertions}
	for _, key := range invariants {
		agg.Invariants = append(agg.Invariants, vocab.Invariant{Key: key, Statement: strings.ReplaceAll(key, "-", " ")})
	}
	return agg
}

// carriersOf locates the recorded contexts in the given files under
// the given Zones; without Zones the whole repository is every
// context's scope.
func carriersOf(t *testing.T, contexts []vocab.BoundedContext, zones []rule.Zone, files map[string]conformance.LanguageFacts) conformance.Carriers {
	t.Helper()
	observed := make([]conformance.ObservedFile, 0, len(files))
	for p := range files {
		observed = append(observed, conformance.ObservedFile{Path: p})
	}
	sort.Slice(observed, func(i, j int) bool { return observed[i].Path < observed[j].Path })
	obs, err := conformance.NewObservations(observed, files)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	knowledge, err := vocab.NewUbiquitousLanguage("p", "", contexts, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	c, err := conformance.NewCarriers(obs, knowledge, zones)
	if err != nil {
		t.Fatalf("NewCarriers: %v", err)
	}
	return c
}

// oneFileCarriers indexes one parsed file under one context recording
// one value object and one specification of the same name, so a term
// is looked up whichever concept the test reads it as.
func oneFileCarriers(t *testing.T, path string, lang rule.Language, pkg, term string, decls []conformance.Declaration) conformance.Carriers {
	t.Helper()
	ctx := contextOf("catalog", nil, []vocab.ValueObject{valueObject(term)}, nil)
	return carriersOf(t, []vocab.BoundedContext{ctx}, nil, map[string]conformance.LanguageFacts{path: parsed(lang, pkg, decls)})
}

// A value object's invariants are enforced at its doors, whichever the
// language spells: any function of the unit returning the type in Go
// (New, NewPrice, ParseSeverity, FromCents), constructor and a static
// factory starting with a constructor word in TypeScript, __init__ and
// a classmethod factory in Python. A function returning another type or
// a collection of the type, a method that returns the type under another
// name, a constructor of a type nobody declares, and an empty file carry
// nothing.
func TestCarriersConstructorRecognisesEveryDoor(t *testing.T) {
	tests := []struct {
		name  string
		lang  rule.Language
		path  string
		decls []conformance.Declaration
		term  string
		want  conformance.Carrier
		found bool
	}{
		{
			name: "go NewType",
			lang: rule.LanguageGo,
			path: "price.go",
			decls: []conformance.Declaration{
				{Kind: "struct", Name: "Price", Exported: true, StartLine: 1},
				{Kind: "func", Name: "NewPrice", Results: []string{"Price", "error"}, StartLine: 3},
			},
			term:  "Price",
			want:  conformance.Carrier{Path: "price.go", Line: 3},
			found: true,
		},
		{
			name: "go ParseType",
			lang: rule.LanguageGo,
			path: "severity.go",
			decls: []conformance.Declaration{
				{Kind: "type", Name: "Severity", Exported: true, StartLine: 1},
				{Kind: "func", Name: "ParseSeverity", Results: []string{"Severity", "error"}, StartLine: 5},
			},
			term:  "Severity",
			want:  conformance.Carrier{Path: "severity.go", Line: 5},
			found: true,
		},
		{
			name: "go New returning pointer",
			lang: rule.LanguageGo,
			path: "event.go",
			decls: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1},
				{Kind: "func", Name: "New", Results: []string{"*Event", "error"}, StartLine: 11},
			},
			term:  "Event",
			want:  conformance.Carrier{Path: "event.go", Line: 11},
			found: true,
		},
		{
			name: "go FromCents",
			lang: rule.LanguageGo,
			path: "money.go",
			decls: []conformance.Declaration{
				{Kind: "struct", Name: "Money", Exported: true, StartLine: 1},
				{Kind: "func", Name: "FromCents", Results: []string{"Money", "error"}, StartLine: 8},
			},
			term:  "Money",
			want:  conformance.Carrier{Path: "money.go", Line: 8},
			found: true,
		},
		{
			name: "go New returning another type",
			lang: rule.LanguageGo,
			path: "event.go",
			decls: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1},
				{Kind: "func", Name: "New", Results: []string{"*Venue", "error"}, StartLine: 11},
			},
			term: "Event",
		},
		{
			name: "go function returning a slice of the type",
			lang: rule.LanguageGo,
			path: "event.go",
			decls: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1},
				{Kind: "func", Name: "Sorted", Results: []string{"[]Event"}, StartLine: 11},
			},
			term: "Event",
		},
		{
			name: "typescript constructor",
			lang: rule.LanguageTypeScript,
			path: "event.ts",
			decls: []conformance.Declaration{
				{Kind: "class", Name: "Event", Exported: true, StartLine: 1},
				{Kind: "method", Name: "constructor", Owner: "Event", StartLine: 2},
			},
			term:  "Event",
			want:  conformance.Carrier{Path: "event.ts", Line: 2},
			found: true,
		},
		{
			name: "typescript static from",
			lang: rule.LanguageTypeScript,
			path: "money.ts",
			decls: []conformance.Declaration{
				{Kind: "class", Name: "Money", Exported: true, StartLine: 1},
				{Kind: "method", Name: "fromCents", Owner: "Money", Results: []string{"Money"}, StartLine: 6},
			},
			term:  "Money",
			want:  conformance.Carrier{Path: "money.ts", Line: 6},
			found: true,
		},
		{
			name: "typescript method returning the type under another name",
			lang: rule.LanguageTypeScript,
			path: "money.ts",
			decls: []conformance.Declaration{
				{Kind: "class", Name: "Money", Exported: true, StartLine: 1},
				{Kind: "method", Name: "clone", Owner: "Money", Results: []string{"Money"}, StartLine: 6},
			},
			term: "Money",
		},
		{
			name: "python init",
			lang: rule.LanguagePython,
			path: "event.py",
			decls: []conformance.Declaration{
				{Kind: "class", Name: "Event", StartLine: 1},
				{Kind: "method", Name: "__init__", Owner: "Event", StartLine: 5},
			},
			term:  "Event",
			want:  conformance.Carrier{Path: "event.py", Line: 5},
			found: true,
		},
		{
			name: "python classmethod parse",
			lang: rule.LanguagePython,
			path: "money.py",
			decls: []conformance.Declaration{
				{Kind: "class", Name: "Money", StartLine: 1},
				{Kind: "method", Name: "parse", Owner: "Money", Results: []string{"\"Money\""}, StartLine: 9},
			},
			term:  "Money",
			want:  conformance.Carrier{Path: "money.py", Line: 9},
			found: true,
		},
		{
			name: "constructor without its type",
			lang: rule.LanguageGo,
			path: "price.go",
			decls: []conformance.Declaration{
				{Kind: "func", Name: "NewPrice", Results: []string{"Price", "error"}, StartLine: 3},
			},
			term: "Price",
		},
		{
			name: "nothing declared",
			lang: rule.LanguageGo,
			path: "empty.go",
			term: "Price",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, found := oneFileCarriers(t, tc.path, tc.lang, "", tc.term, tc.decls).Constructor("catalog", tc.term)
			if got != tc.want || found != tc.found {
				t.Fatalf("Constructor = %+v, %v, want %+v, %v", got, found, tc.want, tc.found)
			}
		})
	}
}

// A door is looked for in the unit holding the type: the directory of
// the file declaring it. A function of another directory returning the
// type is not the type's door.
func TestCarriersConstructorIsInTheUnitOfTheType(t *testing.T) {
	ctx := contextOf("catalog", nil, []vocab.ValueObject{valueObject("Price")}, nil)
	files := map[string]conformance.LanguageFacts{
		"internal/event/price.go": parsed(rule.LanguageGo, "event", []conformance.Declaration{
			{Kind: "struct", Name: "Price", Exported: true, StartLine: 1},
		}),
		"internal/event/doors.go": parsed(rule.LanguageGo, "event", []conformance.Declaration{
			{Kind: "func", Name: "NewPrice", Results: []string{"Price", "error"}, StartLine: 14},
		}),
		"internal/fixtures/factory.go": parsed(rule.LanguageGo, "fixtures", []conformance.Declaration{
			{Kind: "func", Name: "NewPrice", Results: []string{"Price"}, StartLine: 3},
		}),
	}
	c := carriersOf(t, []vocab.BoundedContext{ctx}, nil, files)
	if got, found := c.Constructor("catalog", "Price"); !found || got != (conformance.Carrier{Path: "internal/event/doors.go", Line: 14}) {
		t.Fatalf("Constructor(Price) = %+v, %v, want the door beside the type", got, found)
	}
	delete(files, "internal/event/doors.go")
	c = carriersOf(t, []vocab.BoundedContext{ctx}, nil, files)
	if got, found := c.Constructor("catalog", "Price"); found {
		t.Fatalf("Constructor(Price) = %+v from another directory, want missing", got)
	}
}

// In Go the package name completes a declared type, so rule.ID spells
// RuleID and ParseID is the constructor of RuleID; the root of the Rule
// aggregate enforces its invariant under the ensure word. The same
// declarations under another package spell nothing of the kind, and
// TypeScript names are never package-completed.
func TestCarriersCompleteAGoNameWithItsPackage(t *testing.T) {
	ruleContext := contextOf("rule",
		[]vocab.Aggregate{aggregateOf("Rule", "RuleID", []string{"unique-within-the-ruleset"}, nil)},
		nil, nil)
	idDecls := []conformance.Declaration{
		{Kind: "type", Name: "ID", Exported: true, StartLine: 4},
		{Kind: "func", Name: "ParseID", Results: []string{"ID", "error"}, StartLine: 9},
	}
	ruleDecls := []conformance.Declaration{
		{Kind: "struct", Name: "Rule", Exported: true, StartLine: 6},
		{Kind: "method", Name: "EnsureUniqueWithinTheRuleset", Owner: "Rule", Exported: true, StartLine: 20, Results: []string{"error"}},
	}
	c := carriersOf(t, []vocab.BoundedContext{ruleContext}, nil, map[string]conformance.LanguageFacts{
		"internal/rule/id.go":   parsed(rule.LanguageGo, "rule", idDecls),
		"internal/rule/rule.go": parsed(rule.LanguageGo, "rule", ruleDecls),
	})
	if files := c.TypeFiles("rule", "RuleID"); len(files) != 1 || files[0] != "internal/rule/id.go" {
		t.Fatalf("TypeFiles(RuleID) = %v, want the file declaring rule.ID", files)
	}
	if files := c.TypeFiles("rule", "Rule"); len(files) != 1 || files[0] != "internal/rule/rule.go" {
		t.Fatalf("TypeFiles(Rule) = %v, want the file declaring the root", files)
	}
	if got, found := c.Constructor("rule", "RuleID"); !found || got != (conformance.Carrier{Path: "internal/rule/id.go", Line: 9}) {
		t.Fatalf("Constructor(RuleID) = %+v, %v, want ParseID at line 9", got, found)
	}
	got, found, err := c.Invariant("rule", "Rule", "unique-within-the-ruleset")
	if err != nil || !found || got != (conformance.Carrier{Path: "internal/rule/rule.go", Line: 20}) {
		t.Fatalf("Invariant(Rule) = %+v, %v, %v, want the method at line 20", got, found, err)
	}
	other := carriersOf(t, []vocab.BoundedContext{ruleContext}, nil, map[string]conformance.LanguageFacts{
		"internal/order/id.go": parsed(rule.LanguageGo, "order", idDecls),
	})
	if files := other.TypeFiles("rule", "RuleID"); len(files) != 0 {
		t.Fatalf("TypeFiles(RuleID) under package order = %v, want none", files)
	}
	ts := carriersOf(t, []vocab.BoundedContext{ruleContext}, nil, map[string]conformance.LanguageFacts{
		"rule/id.ts": parsed(rule.LanguageTypeScript, "rule", []conformance.Declaration{{Kind: "class", Name: "ID", Exported: true, StartLine: 1}}),
	})
	if files := ts.TypeFiles("rule", "RuleID"); len(files) != 0 {
		t.Fatalf("TypeFiles(RuleID) in TypeScript = %v, want none: only Go names are package-completed", files)
	}
}

// The root method enforcing an invariant is ensure followed by the key,
// and the one checking an assertion is assert followed by the key, each
// in the method case of the root's language: PascalCase in Go, camelCase
// in TypeScript, snake_case in Python, an initialism keeping its
// capitals. A spelling from another language, the key without its word,
// and a root nobody declares carry nothing.
func TestCarriersRootMethodsSpellTheKeyPerLanguage(t *testing.T) {
	event := aggregateOf("Event", "EventID",
		[]string{"published-frozen", "published-frozen-by-id"},
		[]vocab.Assertion{{Key: "tiers-priced", On: "publish", Statement: "every tier carries a price"}})
	ctx := contextOf("catalog", []vocab.Aggregate{event}, nil, nil)
	tests := []struct {
		name          string
		lang          rule.Language
		path          string
		decls         []conformance.Declaration
		key           string
		wantInvariant conformance.Carrier
		wantAssertion conformance.Carrier
	}{
		{
			name: "go pascal",
			lang: rule.LanguageGo,
			path: "event.go",
			decls: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1},
				{Kind: "method", Name: "EnsurePublishedFrozen", Owner: "Event", Exported: true, StartLine: 90},
				{Kind: "method", Name: "AssertTiersPriced", Owner: "Event", Exported: true, StartLine: 95},
			},
			wantInvariant: conformance.Carrier{Path: "event.go", Line: 90},
			wantAssertion: conformance.Carrier{Path: "event.go", Line: 95},
		},
		{
			name: "typescript camel",
			lang: rule.LanguageTypeScript,
			path: "event.ts",
			decls: []conformance.Declaration{
				{Kind: "class", Name: "Event", Exported: true, StartLine: 1},
				{Kind: "method", Name: "ensurePublishedFrozen", Owner: "Event", Exported: true, StartLine: 4},
				{Kind: "method", Name: "assertTiersPriced", Owner: "Event", Exported: true, StartLine: 6},
			},
			wantInvariant: conformance.Carrier{Path: "event.ts", Line: 4},
			wantAssertion: conformance.Carrier{Path: "event.ts", Line: 6},
		},
		{
			name: "python snake",
			lang: rule.LanguagePython,
			path: "event.py",
			decls: []conformance.Declaration{
				{Kind: "class", Name: "Event", StartLine: 1},
				{Kind: "method", Name: "ensure_published_frozen", Owner: "Event", StartLine: 7},
				{Kind: "method", Name: "assert_tiers_priced", Owner: "Event", StartLine: 9},
			},
			wantInvariant: conformance.Carrier{Path: "event.py", Line: 7},
			wantAssertion: conformance.Carrier{Path: "event.py", Line: 9},
		},
		{
			name: "go initialism keeps its capitals",
			lang: rule.LanguageGo,
			path: "id.go",
			decls: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1},
				{Kind: "method", Name: "EnsurePublishedFrozenByID", Owner: "Event", Exported: true, StartLine: 12},
			},
			key:           "published-frozen-by-id",
			wantInvariant: conformance.Carrier{Path: "id.go", Line: 12},
		},
		{
			name: "wrong case for the language",
			lang: rule.LanguageGo,
			path: "event.go",
			decls: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1},
				{Kind: "method", Name: "ensurePublishedFrozen", Owner: "Event", StartLine: 4},
				{Kind: "method", Name: "assertTiersPriced", Owner: "Event", StartLine: 6},
			},
		},
		{
			name: "key without its word",
			lang: rule.LanguageGo,
			path: "event.go",
			decls: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1},
				{Kind: "method", Name: "PublishedFrozen", Owner: "Event", Exported: true, StartLine: 4},
				{Kind: "method", Name: "TiersPriced", Owner: "Event", Exported: true, StartLine: 6},
			},
		},
		{
			name: "root undeclared",
			lang: rule.LanguageGo,
			path: "event.go",
			decls: []conformance.Declaration{
				{Kind: "method", Name: "EnsurePublishedFrozen", Owner: "Event", Exported: true, StartLine: 90},
				{Kind: "method", Name: "AssertTiersPriced", Owner: "Event", Exported: true, StartLine: 95},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key := tc.key
			if key == "" {
				key = "published-frozen"
			}
			c := carriersOf(t, []vocab.BoundedContext{ctx}, nil, map[string]conformance.LanguageFacts{tc.path: parsed(tc.lang, "", tc.decls)})
			got, found, err := c.Invariant("catalog", "Event", key)
			if err != nil {
				t.Fatalf("Invariant: %v", err)
			}
			if got != tc.wantInvariant || found != (tc.wantInvariant != conformance.Carrier{}) {
				t.Fatalf("Invariant = %+v, %v, want %+v", got, found, tc.wantInvariant)
			}
			got, found, err = c.Assertion("catalog", "Event", "tiers-priced")
			if err != nil {
				t.Fatalf("Assertion: %v", err)
			}
			if got != tc.wantAssertion || found != (tc.wantAssertion != conformance.Carrier{}) {
				t.Fatalf("Assertion = %+v, %v, want %+v", got, found, tc.wantAssertion)
			}
		})
	}
}

// A key that no case can spell is an error the caller sees, never a
// silent miss, and never a bare Ensure or Assert that the prefix alone
// would spell.
func TestCarriersRootMethodsRefuseAKeyNoCaseSpells(t *testing.T) {
	ctx := contextOf("catalog", []vocab.Aggregate{aggregateOf("Event", "EventID", nil, nil)}, nil, nil)
	c := carriersOf(t, []vocab.BoundedContext{ctx}, nil, map[string]conformance.LanguageFacts{
		"event.go": parsed(rule.LanguageGo, "", []conformance.Declaration{
			{Kind: "struct", Name: "Event", Exported: true, StartLine: 1},
			{Kind: "method", Name: "Ensure", Owner: "Event", Exported: true, StartLine: 3},
			{Kind: "method", Name: "Assert", Owner: "Event", Exported: true, StartLine: 5},
		}),
	})
	if _, found, err := c.Invariant("catalog", "Event", ""); err == nil || found {
		t.Fatalf("Invariant with an empty key = found %v, err %v; want an error", found, err)
	}
	if _, found, err := c.Assertion("catalog", "Event", "-"); err == nil || found {
		t.Fatalf("Assertion with a key of no letters = found %v, err %v; want an error", found, err)
	}
}

// A specification is carried by its satisfaction method in any of the
// three spellings; a type without one, or no type at all, carries none.
func TestCarriersSatisfactionFindsTheSatisfactionMethod(t *testing.T) {
	ctx := contextOf("ordering", nil, nil, []vocab.Specification{{Name: "HighValueOrder", Definition: "an order worth a call"}})
	build := func(decls []conformance.Declaration) conformance.Carriers {
		return carriersOf(t, []vocab.BoundedContext{ctx}, nil, map[string]conformance.LanguageFacts{
			"order/spec.go": parsed(rule.LanguageGo, "", decls),
		})
	}
	for _, name := range []string{"SatisfiedBy", "satisfiedBy", "satisfied_by"} {
		c := build([]conformance.Declaration{
			{Kind: "struct", Name: "HighValueOrder", Exported: true, StartLine: 10},
			{Kind: "method", Name: name, Owner: "HighValueOrder", StartLine: 34},
		})
		if got, found := c.Satisfaction("ordering", "HighValueOrder"); !found || got != (conformance.Carrier{Path: "order/spec.go", Line: 34}) {
			t.Fatalf("Satisfaction via %s = %+v, %v, want line 34", name, got, found)
		}
	}
	bare := build([]conformance.Declaration{{Kind: "struct", Name: "HighValueOrder", Exported: true, StartLine: 10}})
	if got, found := bare.Satisfaction("ordering", "HighValueOrder"); found || got != (conformance.Carrier{}) {
		t.Fatalf("Satisfaction without a method = %+v, %v, want missing", got, found)
	}
	if got, found := build(nil).Satisfaction("ordering", "HighValueOrder"); found || got != (conformance.Carrier{}) {
		t.Fatalf("Satisfaction of an undeclared type = %+v, %v, want missing", got, found)
	}
}

// TypeFiles names each file once however many declarations of the term
// it holds, in path order, only for type kinds, and lists every
// candidate when nothing tells which is the model's. A term recorded as
// an aggregate is located as a root, so an interface of the same name
// yields to the struct and the class; a context nobody recorded has no
// files.
func TestCarriersTypeFilesListEveryCandidateOnce(t *testing.T) {
	files := map[string]conformance.LanguageFacts{
		"a/event.go": parsed(rule.LanguageGo, "", []conformance.Declaration{
			{Kind: "struct", Name: "Event", StartLine: 3},
			{Kind: "interface", Name: "Event", StartLine: 9},
			{Kind: "func", Name: "Event", StartLine: 20},
		}),
		"b/event.ts": parsed(rule.LanguageTypeScript, "", []conformance.Declaration{{Kind: "class", Name: "Event", StartLine: 1}}),
		"c/other.go": parsed(rule.LanguageGo, "", []conformance.Declaration{
			{Kind: "struct", Name: "Price", StartLine: 1},
			{Kind: "func", Name: "Event", StartLine: 4},
		}),
	}
	asValue := carriersOf(t, []vocab.BoundedContext{contextOf("catalog", nil, []vocab.ValueObject{valueObject("Event")}, nil)}, nil, files)
	if got := asValue.TypeFiles("catalog", "Event"); len(got) != 2 || got[0] != "a/event.go" || got[1] != "b/event.ts" {
		t.Fatalf("TypeFiles(Event) as a value object = %v, want [a/event.go b/event.ts]", got)
	}
	asRoot := carriersOf(t, []vocab.BoundedContext{contextOf("catalog", []vocab.Aggregate{aggregateOf("Event", "EventID", nil, nil)}, nil, nil)}, nil, files)
	if got := asRoot.TypeFiles("catalog", "Event"); len(got) != 2 || got[0] != "a/event.go" || got[1] != "b/event.ts" {
		t.Fatalf("TypeFiles(Event) as an aggregate = %v, want both candidate roots", got)
	}
	if _, found, err := asRoot.Invariant("catalog", "Event", "published-frozen"); found || err != nil {
		t.Fatalf("Invariant on an ambiguous root = found %v, err %v; want missing", found, err)
	}
	if got := asValue.TypeFiles("catalog", "Ghost"); len(got) != 0 {
		t.Fatalf("TypeFiles(Ghost) = %v, want none", got)
	}
	if got := asValue.TypeFiles("ordering", "Event"); len(got) != 0 {
		t.Fatalf("TypeFiles under an unrecorded context = %v, want none", got)
	}
}

// A term is looked for inside the located aggregate units first, where
// the context's code is settled, so a value object declared beside its
// root is the model's even when a helper package spells the same name.
func TestCarriersPreferTheAggregateUnit(t *testing.T) {
	ctx := contextOf("ordering",
		[]vocab.Aggregate{aggregateOf("Order", "OrderID", nil, nil)},
		[]vocab.ValueObject{valueObject("Money")}, nil)
	c := carriersOf(t, []vocab.BoundedContext{ctx}, nil, map[string]conformance.LanguageFacts{
		"internal/order/order.go": parsed(rule.LanguageGo, "order", []conformance.Declaration{{Kind: "struct", Name: "Order", Exported: true, StartLine: 5}}),
		"internal/order/money.go": parsed(rule.LanguageGo, "order", []conformance.Declaration{
			{Kind: "struct", Name: "Money", Exported: true, StartLine: 3},
			{Kind: "func", Name: "NewMoney", Results: []string{"Money", "error"}, StartLine: 8},
		}),
		"internal/shared/money.go": parsed(rule.LanguageGo, "shared", []conformance.Declaration{
			{Kind: "struct", Name: "Money", Exported: true, StartLine: 1},
			{Kind: "func", Name: "NewMoney", Results: []string{"Money", "error"}, StartLine: 4},
		}),
	})
	if got := c.TypeFiles("ordering", "Money"); len(got) != 1 || got[0] != "internal/order/money.go" {
		t.Fatalf("TypeFiles(Money) = %v, want the declaration beside the root", got)
	}
	if got, found := c.Constructor("ordering", "Money"); !found || got != (conformance.Carrier{Path: "internal/order/money.go", Line: 8}) {
		t.Fatalf("Constructor(Money) = %+v, %v, want NewMoney beside the root", got, found)
	}
}

// A Zone named for a context narrows where its terms are looked for,
// so two contexts recording one name each find their own declaration;
// a context with no Zone named for it is looked for everywhere.
func TestCarriersNarrowAContextToTheZoneNamedForIt(t *testing.T) {
	money := []vocab.ValueObject{valueObject("Money")}
	contexts := []vocab.BoundedContext{
		contextOf("catalog", nil, money, nil),
		contextOf("ordering", nil, money, nil),
		contextOf("billing", nil, money, nil),
	}
	zones := []rule.Zone{
		mustZone(t, "catalog", "internal/event/**"),
		mustZone(t, "ordering", "internal/order/**"),
	}
	c := carriersOf(t, contexts, zones, map[string]conformance.LanguageFacts{
		"internal/event/money.go": parsed(rule.LanguageGo, "event", []conformance.Declaration{{Kind: "struct", Name: "Money", Exported: true, StartLine: 3}}),
		"internal/order/money.go": parsed(rule.LanguageGo, "order", []conformance.Declaration{{Kind: "struct", Name: "Money", Exported: true, StartLine: 7}}),
	})
	if got := c.TypeFiles("catalog", "Money"); len(got) != 1 || got[0] != "internal/event/money.go" {
		t.Fatalf("TypeFiles(catalog, Money) = %v, want the catalog Zone's declaration", got)
	}
	if got := c.TypeFiles("ordering", "Money"); len(got) != 1 || got[0] != "internal/order/money.go" {
		t.Fatalf("TypeFiles(ordering, Money) = %v, want the ordering Zone's declaration", got)
	}
	if got := c.TypeFiles("billing", "Money"); len(got) != 2 {
		t.Fatalf("TypeFiles(billing, Money) = %v, want both declarations of the repository", got)
	}
}

// Two Zones spelling one context's name in different cases leave
// nothing to choose between; that is an error, never a silent pick.
func TestCarriersRefuseTwoZonesSpellingOneContext(t *testing.T) {
	obs, err := conformance.NewObservations([]conformance.ObservedFile{{Path: "internal/event/money.go"}}, nil)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	knowledge, err := vocab.NewUbiquitousLanguage("p", "", []vocab.BoundedContext{contextOf("event_catalog", nil, nil, nil)}, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	zones := []rule.Zone{
		mustZone(t, "event-catalog", "internal/event/**"),
		mustZone(t, "eventcatalog", "internal/catalog/**"),
	}
	_, err = conformance.NewCarriers(obs, knowledge, zones)
	if err == nil || !strings.Contains(err.Error(), "event_catalog") {
		t.Fatalf("NewCarriers = %v, want an error naming the context", err)
	}
}

// A file that failed to parse, or yielded no facts, carries nothing; the
// files around it still do.
func TestCarriersSkipUnusableFacts(t *testing.T) {
	ctx := contextOf("catalog", nil, []vocab.ValueObject{valueObject("Price"), valueObject("Event"), valueObject("Venue")}, nil)
	c := carriersOf(t, []vocab.BoundedContext{ctx}, nil, map[string]conformance.LanguageFacts{
		"ok.go": parsed(rule.LanguageGo, "", []conformance.Declaration{{Kind: "struct", Name: "Price", Exported: true, StartLine: 1}}),
		"bad.go": {
			Language:              rule.LanguageGo,
			DeclarationsAvailable: true,
			ParseFailure:          "parse: boom",
			Declarations:          []conformance.Declaration{{Kind: "struct", Name: "Event", Exported: true, StartLine: 1}},
		},
		"empty.go": {
			Language:              rule.LanguageGo,
			DeclarationsAvailable: false,
			Declarations:          []conformance.Declaration{{Kind: "struct", Name: "Venue", Exported: true, StartLine: 1}},
		},
	})
	if got := c.TypeFiles("catalog", "Price"); len(got) != 1 || got[0] != "ok.go" {
		t.Fatalf("TypeFiles(Price) = %v, want [ok.go]", got)
	}
	for _, term := range []string{"Event", "Venue"} {
		if got := c.TypeFiles("catalog", term); len(got) != 0 {
			t.Fatalf("TypeFiles(%s) = %v, want none from an unusable file", term, got)
		}
	}
}

// Carriers built from nothing, including the zero value, answer missing
// everywhere rather than failing.
func TestCarriersZeroValueCarriesNothing(t *testing.T) {
	var c conformance.Carriers
	if got := c.TypeFiles("catalog", "Event"); len(got) != 0 {
		t.Fatalf("TypeFiles = %v", got)
	}
	if _, found := c.Constructor("catalog", "Event"); found {
		t.Fatalf("Constructor found on the zero value")
	}
	if _, found, err := c.Invariant("catalog", "Event", "published-frozen"); found || err != nil {
		t.Fatalf("Invariant = %v, %v on the zero value", found, err)
	}
	if _, found, err := c.Assertion("catalog", "Event", "tiers-priced"); found || err != nil {
		t.Fatalf("Assertion = %v, %v on the zero value", found, err)
	}
	if _, found := c.Satisfaction("catalog", "Event"); found {
		t.Fatalf("Satisfaction found on the zero value")
	}
}

// MethodName spells a contract key the way each language spells a
// method, the ensure and assert words included, and the language nobody
// named spells it as Go does.
func TestMethodNameSpellsPerLanguage(t *testing.T) {
	cases := []struct {
		lang rule.Language
		key  string
		want string
	}{
		{rule.LanguageGo, conformance.EnsureKey("published-frozen"), "EnsurePublishedFrozen"},
		{rule.LanguageTypeScript, conformance.EnsureKey("published-frozen"), "ensurePublishedFrozen"},
		{rule.LanguagePython, conformance.EnsureKey("published-frozen"), "ensure_published_frozen"},
		{"", conformance.EnsureKey("published-frozen"), "EnsurePublishedFrozen"},
		{rule.LanguageGo, conformance.AssertKey("tiers-priced"), "AssertTiersPriced"},
		{rule.LanguageTypeScript, conformance.AssertKey("tiers-priced"), "assertTiersPriced"},
		{rule.LanguagePython, conformance.AssertKey("tiers-priced"), "assert_tiers_priced"},
	}
	for _, c := range cases {
		got, err := conformance.MethodName(c.key, c.lang)
		if err != nil {
			t.Fatalf("MethodName(%q, %q): %v", c.key, c.lang, err)
		}
		if got != c.want {
			t.Errorf("MethodName(%q, %q) = %q, want %q", c.key, c.lang, got, c.want)
		}
	}
}
