package conformance_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

func invariantsModule(t *testing.T) rule.Module {
	t.Helper()
	return mustModule(t, "domain", "internal/domain/**")
}

func invariantsRule(t *testing.T, closed bool) rule.Rule {
	t.Helper()
	return mustRule(t, rule.Spec{
		ID:            "t:domain/contracts-visible",
		Type:          rule.TypeInvariants,
		Params:        rule.InvariantsParams{Closed: closed},
		Applicability: moduleScope(t, "domain"),
	})
}

func catalogLang(t *testing.T, extra ...func(*vocab.BoundedContext)) vocab.UbiquitousLanguage {
	t.Helper()
	ctx := vocab.BoundedContext{
		Name: "catalog",
		Entities: []vocab.Entity{
			{Definition: vocab.Definition{Name: "Event"}, Aggregate: true},
		},
		ValueObjects: []vocab.Definition{
			{Name: "Price", Definition: "Whole cents."},
		},
		Invariants: []vocab.Invariant{
			{Statement: "A published Event never changes.", Owner: "Event", ID: "published-frozen"},
			{Statement: "A Price is never negative.", Owner: "Price"},
		},
	}
	for _, fn := range extra {
		fn(&ctx)
	}
	lang, err := vocab.NewUbiquitousLanguage([]vocab.BoundedContext{ctx}, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	return lang
}

func runInvariants(t *testing.T, closed bool, lang vocab.UbiquitousLanguage, files []conformance.ObservedFile, facts map[string]conformance.LanguageFacts) conformance.Assessment {
	t.Helper()
	obs, err := conformance.NewObservations(files, facts)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	a, err := conformance.Run(conformance.Request{
		Rules:        []rule.Rule{invariantsRule(t, closed)},
		Modules:      []rule.Module{invariantsModule(t)},
		Observations: obs,
		Knowledge:    lang,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return a
}

func messagesOf(a conformance.Assessment) []string {
	var out []string
	for _, v := range a.Violations() {
		out = append(out, v.Message())
	}
	return out
}

func hasMessage(msgs []string, substr string) bool {
	for _, m := range msgs {
		if strings.Contains(m, substr) {
			return true
		}
	}
	return false
}

func TestInvariantsEmptyCalledMethodConforms(t *testing.T) {
	lang := catalogLang(t)
	files := []conformance.ObservedFile{{Path: "internal/domain/event.go"}}
	facts := map[string]conformance.LanguageFacts{
		"internal/domain/event.go": {
			Language:              rule.LanguageGo,
			DeclarationsAvailable: true,
			CallsAvailable:        true,
			Declarations: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1, EndLine: 4},
				{Kind: "type", Name: "Price", Exported: true, StartLine: 6, EndLine: 6},
				{Kind: "func", Name: "New", Exported: true, StartLine: 8, EndLine: 12, Results: []string{"Event", "error"}},
				{Kind: "func", Name: "NewPrice", Exported: true, StartLine: 14, EndLine: 18, Results: []string{"Price", "error"}},
				{Kind: "method", Name: "PublishedFrozen", Owner: "Event", Exported: true, StartLine: 20, EndLine: 22, Results: []string{"error"}},
				{Kind: "method", Name: "Tell", Owner: "Event", Exported: true, StartLine: 24, EndLine: 28, Results: []string{"error"}},
			},
			Calls: []conformance.Call{
				{Callee: "PublishedFrozen", Line: 10, Enclosing: "New"},
				{Callee: "PublishedFrozen", Line: 26, Enclosing: "Tell"},
			},
		},
	}
	a := runInvariants(t, false, lang, files, facts)
	if len(a.Violations()) != 0 {
		t.Fatalf("expected conforms, got %v", messagesOf(a))
	}
}

func TestInvariantsMissingMethod(t *testing.T) {
	lang := catalogLang(t)
	files := []conformance.ObservedFile{{Path: "internal/domain/event.go"}}
	facts := map[string]conformance.LanguageFacts{
		"internal/domain/event.go": {
			Language:              rule.LanguageGo,
			DeclarationsAvailable: true,
			CallsAvailable:        true,
			Declarations: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1, EndLine: 4},
				{Kind: "type", Name: "Price", Exported: true, StartLine: 6, EndLine: 6},
				{Kind: "func", Name: "New", Exported: true, StartLine: 8, EndLine: 12, Results: []string{"Event", "error"}},
				{Kind: "func", Name: "NewPrice", Exported: true, StartLine: 14, EndLine: 18, Results: []string{"Price", "error"}},
			},
		},
	}
	a := runInvariants(t, false, lang, files, facts)
	if !hasMessage(messagesOf(a), "missing method PublishedFrozen") {
		t.Fatalf("expected missing method, got %v", messagesOf(a))
	}
}

func TestInvariantsMissingCall(t *testing.T) {
	lang := catalogLang(t)
	files := []conformance.ObservedFile{{Path: "internal/domain/event.go"}}
	facts := map[string]conformance.LanguageFacts{
		"internal/domain/event.go": {
			Language:              rule.LanguageGo,
			DeclarationsAvailable: true,
			CallsAvailable:        true,
			Declarations: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1, EndLine: 4},
				{Kind: "type", Name: "Price", Exported: true, StartLine: 6, EndLine: 6},
				{Kind: "func", Name: "New", Exported: true, StartLine: 8, EndLine: 12, Results: []string{"Event", "error"}},
				{Kind: "func", Name: "NewPrice", Exported: true, StartLine: 14, EndLine: 18, Results: []string{"Price", "error"}},
				{Kind: "method", Name: "PublishedFrozen", Owner: "Event", Exported: true, StartLine: 20, EndLine: 22, Results: []string{"error"}},
				{Kind: "method", Name: "Tell", Owner: "Event", Exported: true, StartLine: 24, EndLine: 28, Results: []string{"error"}},
			},
			Calls: []conformance.Call{
				{Callee: "PublishedFrozen", Line: 10, Enclosing: "New"},
			},
		},
	}
	a := runInvariants(t, false, lang, files, facts)
	if !hasMessage(messagesOf(a), "command Tell does not call PublishedFrozen") {
		t.Fatalf("expected missing call, got %v", messagesOf(a))
	}
}

func TestInvariantsAssertionOnMiss(t *testing.T) {
	lang := catalogLang(t, func(c *vocab.BoundedContext) {
		c.Assertions = []vocab.Assertion{{
			Statement: "Every TicketTier has a Price before publish.",
			Owner:     "Event",
			ID:        "tiers-priced",
			On:        "Publish",
		}}
	})
	files := []conformance.ObservedFile{{Path: "internal/domain/event.go"}}
	facts := map[string]conformance.LanguageFacts{
		"internal/domain/event.go": {
			Language:              rule.LanguageGo,
			DeclarationsAvailable: true,
			CallsAvailable:        true,
			Declarations: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1, EndLine: 4},
				{Kind: "type", Name: "Price", Exported: true, StartLine: 6, EndLine: 6},
				{Kind: "func", Name: "New", Exported: true, StartLine: 8, EndLine: 12, Results: []string{"Event", "error"}},
				{Kind: "func", Name: "NewPrice", Exported: true, StartLine: 14, EndLine: 18, Results: []string{"Price", "error"}},
				{Kind: "method", Name: "PublishedFrozen", Owner: "Event", Exported: true, StartLine: 20, EndLine: 22, Results: []string{"error"}},
				{Kind: "method", Name: "TiersPriced", Owner: "Event", Exported: true, StartLine: 24, EndLine: 26, Results: []string{"error"}},
			},
			Calls: []conformance.Call{
				{Callee: "PublishedFrozen", Line: 10, Enclosing: "New"},
			},
		},
	}
	a := runInvariants(t, false, lang, files, facts)
	if !hasMessage(messagesOf(a), "missing operation Publish") {
		t.Fatalf("expected assertion on miss, got %v", messagesOf(a))
	}
}

func TestInvariantsSpecificationWithoutSatisfaction(t *testing.T) {
	lang := catalogLang(t, func(c *vocab.BoundedContext) {
		c.Specifications = []vocab.Specification{{
			Name:       "HighValueOrder",
			Definition: "An order the house treats as high value.",
		}}
	})
	files := []conformance.ObservedFile{{Path: "internal/domain/event.go"}}
	facts := map[string]conformance.LanguageFacts{
		"internal/domain/event.go": {
			Language:              rule.LanguageGo,
			DeclarationsAvailable: true,
			CallsAvailable:        true,
			Declarations: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1, EndLine: 4},
				{Kind: "type", Name: "Price", Exported: true, StartLine: 6, EndLine: 6},
				{Kind: "struct", Name: "HighValueOrder", Exported: true, StartLine: 30, EndLine: 32},
				{Kind: "func", Name: "New", Exported: true, StartLine: 8, EndLine: 12, Results: []string{"Event", "error"}},
				{Kind: "func", Name: "NewPrice", Exported: true, StartLine: 14, EndLine: 18, Results: []string{"Price", "error"}},
				{Kind: "method", Name: "PublishedFrozen", Owner: "Event", Exported: true, StartLine: 20, EndLine: 22, Results: []string{"error"}},
			},
			Calls: []conformance.Call{
				{Callee: "PublishedFrozen", Line: 10, Enclosing: "New"},
			},
		},
	}
	a := runInvariants(t, false, lang, files, facts)
	if !hasMessage(messagesOf(a), "missing satisfaction method") {
		t.Fatalf("expected spec without satisfaction, got %v", messagesOf(a))
	}
}

func TestInvariantsClosedTrueExtraFail(t *testing.T) {
	lang := catalogLang(t)
	files := []conformance.ObservedFile{{Path: "internal/domain/event.go"}}
	facts := map[string]conformance.LanguageFacts{
		"internal/domain/event.go": {
			Language:              rule.LanguageGo,
			DeclarationsAvailable: true,
			CallsAvailable:        true,
			Declarations: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1, EndLine: 4},
				{Kind: "type", Name: "Price", Exported: true, StartLine: 6, EndLine: 6},
				{Kind: "func", Name: "New", Exported: true, StartLine: 8, EndLine: 12, Results: []string{"Event", "error"}},
				{Kind: "func", Name: "NewPrice", Exported: true, StartLine: 14, EndLine: 18, Results: []string{"Price", "error"}},
				{Kind: "func", Name: "Extra", Exported: true, StartLine: 19, EndLine: 21, Results: []string{"error"}},
				{Kind: "method", Name: "PublishedFrozen", Owner: "Event", Exported: true, StartLine: 22, EndLine: 24, Results: []string{"error"}},
			},
			Calls: []conformance.Call{
				{Callee: "PublishedFrozen", Line: 10, Enclosing: "New"},
			},
		},
	}
	a := runInvariants(t, true, lang, files, facts)
	if !hasMessage(messagesOf(a), "extra Extra does not call PublishedFrozen") {
		t.Fatalf("expected closed extra fail, got %v", messagesOf(a))
	}
}

func TestInvariantsClosedFalseChildConstructorAllowed(t *testing.T) {
	lang := catalogLang(t)
	files := []conformance.ObservedFile{{Path: "internal/domain/event.go"}}
	facts := map[string]conformance.LanguageFacts{
		"internal/domain/event.go": {
			Language:              rule.LanguageGo,
			DeclarationsAvailable: true,
			CallsAvailable:        true,
			Declarations: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1, EndLine: 4},
				{Kind: "type", Name: "Price", Exported: true, StartLine: 6, EndLine: 6},
				{Kind: "func", Name: "New", Exported: true, StartLine: 8, EndLine: 12, Results: []string{"Event", "error"}},
				{Kind: "func", Name: "NewPrice", Exported: true, StartLine: 14, EndLine: 18, Results: []string{"Price", "error"}},
				{Kind: "method", Name: "PublishedFrozen", Owner: "Event", Exported: true, StartLine: 20, EndLine: 22, Results: []string{"error"}},
			},
			Calls: []conformance.Call{
				{Callee: "PublishedFrozen", Line: 10, Enclosing: "New"},
				{Callee: "NewPrice", Line: 9, Enclosing: "New"},
			},
		},
	}
	a := runInvariants(t, false, lang, files, facts)
	if hasMessage(messagesOf(a), "NewPrice") {
		t.Fatalf("child constructor should be allowed when closed is off, got %v", messagesOf(a))
	}
	if len(a.Violations()) != 0 {
		t.Fatalf("expected conforms, got %v", messagesOf(a))
	}
}

func TestInvariantsMissingValueConstructor(t *testing.T) {
	lang := catalogLang(t)
	files := []conformance.ObservedFile{{Path: "internal/domain/event.go"}}
	facts := map[string]conformance.LanguageFacts{
		"internal/domain/event.go": {
			Language:              rule.LanguageGo,
			DeclarationsAvailable: true,
			CallsAvailable:        true,
			Declarations: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1, EndLine: 4},
				{Kind: "type", Name: "Price", Exported: true, StartLine: 6, EndLine: 6},
				{Kind: "func", Name: "New", Exported: true, StartLine: 8, EndLine: 12, Results: []string{"Event", "error"}},
				{Kind: "method", Name: "PublishedFrozen", Owner: "Event", Exported: true, StartLine: 20, EndLine: 22, Results: []string{"error"}},
			},
			Calls: []conformance.Call{
				{Callee: "PublishedFrozen", Line: 10, Enclosing: "New"},
			},
		},
	}
	a := runInvariants(t, false, lang, files, facts)
	if !hasMessage(messagesOf(a), "missing constructor for Price") {
		t.Fatalf("expected missing Price constructor, got %v", messagesOf(a))
	}
}

func TestInvariantsConstructorDoesNotCallClusterMethod(t *testing.T) {
	lang := catalogLang(t)
	files := []conformance.ObservedFile{{Path: "internal/domain/event.go"}}
	facts := map[string]conformance.LanguageFacts{
		"internal/domain/event.go": {
			Language:              rule.LanguageGo,
			DeclarationsAvailable: true,
			CallsAvailable:        true,
			Declarations: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1, EndLine: 4},
				{Kind: "type", Name: "Price", Exported: true, StartLine: 6, EndLine: 6},
				{Kind: "func", Name: "New", Exported: true, StartLine: 8, EndLine: 12, Results: []string{"Event", "error"}},
				{Kind: "func", Name: "NewPrice", Exported: true, StartLine: 14, EndLine: 18, Results: []string{"Price", "error"}},
				{Kind: "method", Name: "PublishedFrozen", Owner: "Event", Exported: true, StartLine: 20, EndLine: 22, Results: []string{"error"}},
			},
		},
	}
	a := runInvariants(t, false, lang, files, facts)
	if !hasMessage(messagesOf(a), "constructor New does not call PublishedFrozen") {
		t.Fatalf("expected constructor missing cluster call, got %v", messagesOf(a))
	}
}

func TestInvariantsAssertionMethodMissingWhenOperationExists(t *testing.T) {
	lang := catalogLang(t, func(c *vocab.BoundedContext) {
		c.Assertions = []vocab.Assertion{{
			Statement: "Every TicketTier has a Price before publish.",
			Owner:     "Event",
			ID:        "tiers-priced",
			On:        "Publish",
		}}
	})
	files := []conformance.ObservedFile{{Path: "internal/domain/event.go"}}
	facts := map[string]conformance.LanguageFacts{
		"internal/domain/event.go": {
			Language:              rule.LanguageGo,
			DeclarationsAvailable: true,
			CallsAvailable:        true,
			Declarations: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1, EndLine: 4},
				{Kind: "type", Name: "Price", Exported: true, StartLine: 6, EndLine: 6},
				{Kind: "func", Name: "New", Exported: true, StartLine: 8, EndLine: 12, Results: []string{"Event", "error"}},
				{Kind: "func", Name: "NewPrice", Exported: true, StartLine: 14, EndLine: 18, Results: []string{"Price", "error"}},
				{Kind: "method", Name: "PublishedFrozen", Owner: "Event", Exported: true, StartLine: 20, EndLine: 22, Results: []string{"error"}},
				{Kind: "method", Name: "Publish", Owner: "Event", Exported: true, StartLine: 40, EndLine: 48, Results: []string{"error"}},
			},
			Calls: []conformance.Call{
				{Callee: "PublishedFrozen", Line: 10, Enclosing: "New"},
			},
		},
	}
	a := runInvariants(t, false, lang, files, facts)
	if !hasMessage(messagesOf(a), "missing method TiersPriced") {
		t.Fatalf("expected missing assertion method, got %v", messagesOf(a))
	}
}

func TestInvariantsAssertionOperationDoesNotCallMethod(t *testing.T) {
	lang := catalogLang(t, func(c *vocab.BoundedContext) {
		c.Assertions = []vocab.Assertion{{
			Statement: "Every TicketTier has a Price before publish.",
			Owner:     "Event",
			ID:        "tiers-priced",
			On:        "Publish",
		}}
	})
	files := []conformance.ObservedFile{{Path: "internal/domain/event.go"}}
	facts := map[string]conformance.LanguageFacts{
		"internal/domain/event.go": {
			Language:              rule.LanguageGo,
			DeclarationsAvailable: true,
			CallsAvailable:        true,
			Declarations: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1, EndLine: 4},
				{Kind: "type", Name: "Price", Exported: true, StartLine: 6, EndLine: 6},
				{Kind: "func", Name: "New", Exported: true, StartLine: 8, EndLine: 12, Results: []string{"Event", "error"}},
				{Kind: "func", Name: "NewPrice", Exported: true, StartLine: 14, EndLine: 18, Results: []string{"Price", "error"}},
				{Kind: "method", Name: "PublishedFrozen", Owner: "Event", Exported: true, StartLine: 20, EndLine: 22, Results: []string{"error"}},
				{Kind: "method", Name: "TiersPriced", Owner: "Event", Exported: true, StartLine: 24, EndLine: 26, Results: []string{"error"}},
				{Kind: "method", Name: "Publish", Owner: "Event", Exported: true, StartLine: 40, EndLine: 48, Results: []string{"error"}},
			},
			Calls: []conformance.Call{
				{Callee: "PublishedFrozen", Line: 10, Enclosing: "New"},
			},
		},
	}
	a := runInvariants(t, false, lang, files, facts)
	if !hasMessage(messagesOf(a), "Publish does not call TiersPriced") {
		t.Fatalf("expected assertion call miss, got %v", messagesOf(a))
	}
}

func TestInvariantsSpecificationTypeMissing(t *testing.T) {
	lang := catalogLang(t, func(c *vocab.BoundedContext) {
		c.Specifications = []vocab.Specification{{
			Name:       "HighValueOrder",
			Definition: "An order the house treats as high value.",
		}}
	})
	files := []conformance.ObservedFile{{Path: "internal/domain/event.go"}}
	facts := map[string]conformance.LanguageFacts{
		"internal/domain/event.go": {
			Language:              rule.LanguageGo,
			DeclarationsAvailable: true,
			CallsAvailable:        true,
			Declarations: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1, EndLine: 4},
				{Kind: "type", Name: "Price", Exported: true, StartLine: 6, EndLine: 6},
				{Kind: "func", Name: "New", Exported: true, StartLine: 8, EndLine: 12, Results: []string{"Event", "error"}},
				{Kind: "func", Name: "NewPrice", Exported: true, StartLine: 14, EndLine: 18, Results: []string{"Price", "error"}},
				{Kind: "method", Name: "PublishedFrozen", Owner: "Event", Exported: true, StartLine: 20, EndLine: 22, Results: []string{"error"}},
			},
			Calls: []conformance.Call{
				{Callee: "PublishedFrozen", Line: 10, Enclosing: "New"},
			},
		},
	}
	a := runInvariants(t, false, lang, files, facts)
	if !hasMessage(messagesOf(a), "missing type HighValueOrder") {
		t.Fatalf("expected missing spec type, got %v", messagesOf(a))
	}
}

func TestInvariantsSpecificationSatisfiedByInAnotherFile(t *testing.T) {
	lang := catalogLang(t, func(c *vocab.BoundedContext) {
		c.Specifications = []vocab.Specification{{
			Name:       "HighValueOrder",
			Definition: "An order the house treats as high value.",
		}}
	})
	files := []conformance.ObservedFile{
		{Path: "internal/domain/spec.go"},
		{Path: "internal/domain/spec_satisfied.go"},
		{Path: "internal/domain/event.go"},
	}
	facts := map[string]conformance.LanguageFacts{
		"internal/domain/spec.go": {
			Language:              rule.LanguageGo,
			DeclarationsAvailable: true,
			CallsAvailable:        true,
			Declarations: []conformance.Declaration{
				{Kind: "struct", Name: "HighValueOrder", Exported: true, StartLine: 3, EndLine: 6},
			},
		},
		"internal/domain/spec_satisfied.go": {
			Language:              rule.LanguageGo,
			DeclarationsAvailable: true,
			CallsAvailable:        true,
			Declarations: []conformance.Declaration{
				{Kind: "method", Name: "SatisfiedBy", Owner: "HighValueOrder", Exported: true, StartLine: 5, EndLine: 8},
			},
		},
		"internal/domain/event.go": {
			Language:              rule.LanguageGo,
			DeclarationsAvailable: true,
			CallsAvailable:        true,
			Declarations: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1, EndLine: 4},
				{Kind: "type", Name: "Price", Exported: true, StartLine: 6, EndLine: 6},
				{Kind: "func", Name: "New", Exported: true, StartLine: 8, EndLine: 12, Results: []string{"Event", "error"}},
				{Kind: "func", Name: "NewPrice", Exported: true, StartLine: 14, EndLine: 18, Results: []string{"Price", "error"}},
				{Kind: "method", Name: "PublishedFrozen", Owner: "Event", Exported: true, StartLine: 20, EndLine: 22, Results: []string{"error"}},
			},
			Calls: []conformance.Call{
				{Callee: "PublishedFrozen", Line: 10, Enclosing: "New"},
			},
		},
	}
	a := runInvariants(t, false, lang, files, facts)
	if len(a.Violations()) != 0 {
		t.Fatalf("SatisfiedBy in another file of the same module must count, got %v", messagesOf(a))
	}
}

func TestInvariantsSpecificationSatisfiedByConforms(t *testing.T) {
	lang := catalogLang(t, func(c *vocab.BoundedContext) {
		c.Specifications = []vocab.Specification{{
			Name:       "HighValueOrder",
			Definition: "An order the house treats as high value.",
		}}
	})
	files := []conformance.ObservedFile{{Path: "internal/domain/event.go"}}
	facts := map[string]conformance.LanguageFacts{
		"internal/domain/event.go": {
			Language:              rule.LanguageGo,
			DeclarationsAvailable: true,
			CallsAvailable:        true,
			Declarations: []conformance.Declaration{
				{Kind: "struct", Name: "Event", Exported: true, StartLine: 1, EndLine: 4},
				{Kind: "type", Name: "Price", Exported: true, StartLine: 6, EndLine: 6},
				{Kind: "struct", Name: "HighValueOrder", Exported: true, StartLine: 30, EndLine: 32},
				{Kind: "method", Name: "SatisfiedBy", Owner: "HighValueOrder", Exported: true, StartLine: 34, EndLine: 36},
				{Kind: "func", Name: "New", Exported: true, StartLine: 8, EndLine: 12, Results: []string{"Event", "error"}},
				{Kind: "func", Name: "NewPrice", Exported: true, StartLine: 14, EndLine: 18, Results: []string{"Price", "error"}},
				{Kind: "method", Name: "PublishedFrozen", Owner: "Event", Exported: true, StartLine: 20, EndLine: 22, Results: []string{"error"}},
			},
			Calls: []conformance.Call{
				{Callee: "PublishedFrozen", Line: 10, Enclosing: "New"},
			},
		},
	}
	a := runInvariants(t, false, lang, files, facts)
	if len(a.Violations()) != 0 {
		t.Fatalf("expected spec with SatisfiedBy to conform, got %v", messagesOf(a))
	}
}
