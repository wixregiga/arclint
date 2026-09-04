package rule_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

func recordedLanguage() vocab.UbiquitousLanguage {
	return vocab.UbiquitousLanguage{Contexts: []vocab.BoundedContext{
		{
			Name: "ordering",
			Entities: []vocab.Entity{
				{Definition: vocab.Definition{Name: "Order"}, Aggregate: true},
				{Definition: vocab.Definition{Name: "Order Line"}},
			},
			ValueObjects: []vocab.Definition{{Name: "Money"}},
			Events:       []vocab.Definition{{Name: "OrderPlaced"}},
			Invariants: []vocab.Invariant{
				{Statement: "Lines never change.", Owner: "Order", ID: "lines-frozen"},
				{Statement: "Money is never negative.", Owner: "Money"},
			},
			Assertions: []vocab.Assertion{
				{Statement: "Priced before place.", Owner: "Order", ID: "lines-priced", On: "Place"},
			},
			Specifications: []vocab.Specification{
				{Name: "PreferredCustomer", Definition: "A named predicate."},
			},
		},
		{
			Name:     "billing",
			Entities: []vocab.Entity{{Definition: vocab.Definition{Name: "Invoice"}, Aggregate: true}},
		},
	}}
}

func TestExpansionResolvesPerSourceTerm(t *testing.T) {
	e, err := rule.NewExpansion("domain.aggregates",
		[]string{"internal/domain/{name:flatcase}/root.go"},
		[]string{"internal/domain/{name:flatcase}/util.go"})
	if err != nil {
		t.Fatalf("NewExpansion: %v", err)
	}
	params, err := e.Resolve(recordedLanguage())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	wantRequire := []string{"internal/domain/order/root.go", "internal/domain/invoice/root.go"}
	if len(params.Require) != len(wantRequire) {
		t.Fatalf("require = %v, want %v", params.Require, wantRequire)
	}
	for i, g := range params.Require {
		if g.String() != wantRequire[i] {
			t.Errorf("require[%d] = %q, want %q", i, g, wantRequire[i])
		}
	}
	if len(params.Forbid) != 2 || params.Forbid[0].String() != "internal/domain/order/util.go" {
		t.Errorf("forbid = %v", params.Forbid)
	}
}

func TestExpansionSourcesSelectTheRecordedCollections(t *testing.T) {
	lang := recordedLanguage()
	counts := map[string]int{
		"domain.aggregates":     2, // Order, Invoice
		"domain.entities":       3, // + Order Line
		"domain.value_objects":  1, // Money
		"domain.events":         1, // OrderPlaced
		"domain.contexts":       2, // ordering, billing
		"domain.invariants":     2, // lines-frozen, Money
		"domain.assertions":     1, // lines-priced
		"domain.specifications": 1, // PreferredCustomer
	}
	for source, want := range counts {
		e, err := rule.NewExpansion(source, []string{"{name:flatcase}/present.go"}, nil)
		if err != nil {
			t.Fatalf("NewExpansion(%s): %v", source, err)
		}
		params, err := e.Resolve(lang)
		if err != nil {
			t.Fatalf("Resolve(%s): %v", source, err)
		}
		if len(params.Require) != want {
			t.Errorf("source %s resolved %d globs, want %d (%v)", source, len(params.Require), want, params.Require)
		}
	}
}

func TestExpansionCollapsesPlaceholderFreeGlobs(t *testing.T) {
	e, err := rule.NewExpansion("domain.aggregates",
		[]string{"internal/domain/README.md"}, nil)
	if err != nil {
		t.Fatalf("NewExpansion: %v", err)
	}
	params, err := e.Resolve(recordedLanguage())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(params.Require) != 1 {
		t.Errorf("placeholder-free glob duplicated per term: %v", params.Require)
	}
}

func TestExpansionEmptyVocabularyResolvesEmpty(t *testing.T) {
	e, err := rule.NewExpansion("domain.aggregates",
		[]string{"internal/domain/{name:flatcase}/root.go"}, nil)
	if err != nil {
		t.Fatalf("NewExpansion: %v", err)
	}
	params, err := e.Resolve(vocab.UbiquitousLanguage{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(params.Require)+len(params.Forbid) != 0 {
		t.Errorf("empty vocabulary resolved %v", params)
	}
}

func TestNewExpansionRejectsInvalidDeclarations(t *testing.T) {
	cases := map[string]struct {
		source  string
		require []string
	}{
		"unknown source":       {"domain.everything", []string{"x/{name:flatcase}.go"}},
		"unknown case":         {"domain.aggregates", []string{"x/{name:SCREAMING}.go"}},
		"stray brace":          {"domain.aggregates", []string{"x/{aggregate}.go"}},
		"no globs":             {"domain.aggregates", nil},
		"invalid after subst":  {"domain.aggregates", []string{"//{name:flatcase}"}},
		"nested brace garbage": {"domain.aggregates", []string{"x/{name:flatcase}}.go"}},
		"glob listed twice":    {"domain.aggregates", []string{"x/{name:flatcase}.go", "x/{name:flatcase}.go"}},
	}
	for name, c := range cases {
		if _, err := rule.NewExpansion(c.source, c.require, nil); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func expandedRule(t *testing.T, lang vocab.UbiquitousLanguage) rule.Rule {
	t.Helper()
	e, err := rule.NewExpansion("domain.aggregates",
		[]string{"internal/domain/{name:flatcase}/root.go"}, nil)
	if err != nil {
		t.Fatalf("NewExpansion: %v", err)
	}
	params, err := e.Resolve(lang)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	name, err := rule.NewModuleName("domain")
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	scope, err := rule.ModuleApplicability([]rule.ModuleName{name})
	if err != nil {
		t.Fatalf("applicability: %v", err)
	}
	r, err := rule.New(rule.Spec{
		ID:            "test/p:domain/aggregate-skeleton",
		Type:          rule.TypeStructure,
		Params:        params,
		Applicability: scope,
		Expansion:     &e,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return r
}

func TestExpandedRuleIsOneRuleWithQuantifiedClaim(t *testing.T) {
	r := expandedRule(t, recordedLanguage())
	if _, ok := r.Expansion(); !ok {
		t.Fatalf("expansion not carried")
	}
	claim := r.Claim().Statement()
	if !strings.Contains(claim, "derived from each recorded domain.aggregates") {
		t.Errorf("claim %q does not state its derivation", claim)
	}
	if err := r.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestExpandedRuleOverEmptyVocabularyExistsAndSaysSo(t *testing.T) {
	r := expandedRule(t, vocab.UbiquitousLanguage{})
	claim := r.Claim().Statement()
	if !strings.Contains(claim, "none recorded yet") {
		t.Errorf("claim %q does not state the empty derivation", claim)
	}
	if err := r.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestReexpandKeepsIdentityAndRederivesParams(t *testing.T) {
	r := expandedRule(t, vocab.UbiquitousLanguage{})
	re, err := r.Reexpand(recordedLanguage())
	if err != nil {
		t.Fatalf("Reexpand: %v", err)
	}
	if re.ID().Qualified() != r.ID().Qualified() {
		t.Errorf("identity changed: %s", re.ID())
	}
	params, ok := re.Params().(rule.StructureParams)
	if !ok || len(params.Require) != 2 {
		t.Errorf("re-derived params = %#v", re.Params())
	}
	if !strings.Contains(re.Claim().Statement(), "derived from each recorded") {
		t.Errorf("re-derived claim = %q", re.Claim().Statement())
	}
}
