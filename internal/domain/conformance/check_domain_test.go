package conformance_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// domainScenario is a miniature of the boxoffice proving ground: three
// recorded contexts, two of them with Zones named for them and one
// with no Zone at all, a non-context Zone the ordering model reaches
// out to, one aggregate that enforces its invariant at every door but
// one, a setter on each root, a recorded value object, event, and
// specification the code never declares, and an import that runs
// against the recorded conformist relation.
func domainScenario(t *testing.T) conformance.Request {
	t.Helper()
	zones := []rule.Zone{
		mustZone(t, "catalog", "internal/event/**"),
		mustZone(t, "ordering", "internal/order/**"),
		mustZone(t, "app", "internal/app/**"),
	}
	files := []conformance.ObservedFile{
		{Path: "internal/app/app.go"},
		{Path: "internal/event/README.md"},
		{Path: "internal/event/event.go"},
		{Path: "internal/event/id.go"},
		{Path: "internal/event/repository.go"},
		{Path: "internal/order/order.go"},
	}
	goFacts := func(pkg string, imports []conformance.Import, decls []conformance.Declaration, calls []conformance.Call) conformance.LanguageFacts {
		return conformance.LanguageFacts{
			Language:              rule.LanguageGo,
			Package:               pkg,
			ImportsAvailable:      true,
			DeclarationsAvailable: true,
			CallsAvailable:        true,
			Imports:               imports,
			Declarations:          decls,
			Calls:                 calls,
		}
	}
	facts := map[string]conformance.LanguageFacts{
		"internal/app/app.go": goFacts("app", []conformance.Import{
			{Path: "example.com/mod/internal/event", Line: 3, Class: conformance.ImportInternal, TargetDir: "internal/event"},
		}, nil, nil),
		"internal/event/event.go": goFacts("event", []conformance.Import{
			{Path: "errors", Line: 4, Class: conformance.ImportStdlib},
			{Path: "example.com/mod/internal/order", Line: 5, Class: conformance.ImportInternal, TargetDir: "internal/order"},
		}, []conformance.Declaration{
			{Kind: "struct", Name: "Price", Exported: true, StartLine: 8},
			{Kind: "func", Name: "NewPrice", Exported: true, StartLine: 12, Results: []string{"Price", "error"}},
			{Kind: "struct", Name: "Event", Exported: true, StartLine: 20},
			{Kind: "func", Name: "New", Exported: true, StartLine: 30, Results: []string{"*Event", "error"}},
			{Kind: "method", Name: "EnsurePublishedOnly", Owner: "Event", Exported: true, StartLine: 40, Results: []string{"error"}},
			{Kind: "method", Name: "AssertTiersPriced", Owner: "Event", Exported: true, StartLine: 45, Results: []string{"error"}},
			{Kind: "method", Name: "Publish", Owner: "Event", Exported: true, StartLine: 50, Results: []string{"error"}},
			{Kind: "method", Name: "SetName", Owner: "Event", Exported: true, StartLine: 60},
			{Kind: "method", Name: "Cancel", Owner: "Event", Exported: true, StartLine: 65, Results: []string{"error"}},
			{Kind: "method", Name: "Name", Owner: "Event", Exported: true, StartLine: 70, Results: []string{"string"}},
		}, []conformance.Call{
			{Callee: "EnsurePublishedOnly", Line: 33, Enclosing: "New"},
			{Callee: "EnsurePublishedOnly", Line: 52, Enclosing: "Publish"},
			{Callee: "AssertTiersPriced", Line: 53, Enclosing: "Publish"},
			{Callee: "New", Line: 66, Enclosing: "Cancel"},
		}),
		"internal/event/id.go": goFacts("event", nil, []conformance.Declaration{
			{Kind: "type", Name: "EventID", Exported: true, StartLine: 3},
		}, nil),
		"internal/event/repository.go": goFacts("event", nil, []conformance.Declaration{
			{Kind: "interface", Name: "EventRepository", Exported: true, StartLine: 5},
			{Kind: "method", Name: "Load", Owner: "EventRepository", Exported: true, StartLine: 6, Results: []string{"*Event", "error"}},
		}, nil),
		"internal/order/order.go": goFacts("order", []conformance.Import{
			{Path: "example.com/mod/internal/event", Line: 4, Class: conformance.ImportInternal, TargetDir: "internal/event"},
			{Path: "example.com/mod/internal/app", Line: 5, Class: conformance.ImportInternal, TargetDir: "internal/app"},
		}, []conformance.Declaration{
			{Kind: "type", Name: "OrderID", Exported: true, StartLine: 7},
			{Kind: "struct", Name: "Order", Exported: true, StartLine: 10},
			{Kind: "method", Name: "SetStatus", Owner: "Order", Exported: true, StartLine: 20},
		}, nil),
	}
	obs, err := conformance.NewObservations(files, facts)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	knowledge, err := vocab.NewUbiquitousLanguage("boxoffice", "tickets for events", []vocab.BoundedContext{
		{
			Name: "catalog", Definition: "what is on sale", Line: 2,
			Aggregates: []vocab.Aggregate{{
				Name: "Event", Definition: "one performance on sale", Identity: "EventID", Line: 3,
				Invariants: []vocab.Invariant{{Key: "published-only", Statement: "tickets sell for a published event only", Line: 5}},
				Assertions: []vocab.Assertion{{Key: "tiers-priced", On: "Publish", Statement: "every tier carries a price", Line: 8}},
				Repository: "EventRepository",
			}},
			ValueObjects: []vocab.ValueObject{{
				Name: "Price", Definition: "an amount in whole cents", Line: 12,
				Invariants: []vocab.Invariant{{Key: "whole-cents", Statement: "never negative, never fractional", Line: 13}},
			}},
			Specifications: []vocab.Specification{{Name: "Sellable", Definition: "an event tickets can be sold for", Line: 15}},
		},
		{
			Name: "ordering", Definition: "buying tickets", Line: 18,
			Aggregates:   []vocab.Aggregate{{Name: "Order", Definition: "one purchase", Identity: "OrderID", Line: 20}},
			ValueObjects: []vocab.ValueObject{{Name: "OrderLine", Definition: "tickets of one tier in an order", Line: 25}},
			Events:       []vocab.DomainEvent{{Name: "OrderPlaced", Definition: "an order was placed", RaisedBy: "Order", Line: 28}},
		},
		{Name: "billing", Definition: "collecting money", Line: 30},
	}, []vocab.ContextRelation{
		{From: "catalog", To: "ordering", Kind: vocab.RelationConformist, Line: 33},
	})
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	rules, err := rule.BuiltIn()
	if err != nil {
		t.Fatalf("BuiltIn: %v", err)
	}
	return conformance.Request{
		Rules:          rules,
		Zones:          zones,
		Observations:   obs,
		UnknownImports: rule.UnknownImportsWarn,
		Knowledge:      knowledge,
	}
}

// outcomesOf keys every evaluation of the assessment by rule and
// context: "rule|subject" to outcome.
func outcomesOf(a conformance.Assessment) map[string]conformance.Outcome {
	out := map[string]conformance.Outcome{}
	for _, e := range a.Evaluations() {
		out[e.Rule().Qualified()+"|"+e.Subject().String()] = e.Outcome()
	}
	return out
}

// anchorsOf keys every violation by rule: "path:line" per rule.
func anchorsOf(a conformance.Assessment) map[string][]string {
	out := map[string][]string{}
	for _, v := range a.Violations() {
		id := v.Rule().Qualified()
		out[id] = append(out[id], fmt.Sprintf("%s:%d", v.Path(), v.Line()))
	}
	for id := range out {
		sort.Strings(out[id])
	}
	return out
}

func TestBuiltInRulesJudgeEveryContextOfTheRecordedDomain(t *testing.T) {
	req := domainScenario(t)
	a, err := conformance.Run(req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	outcomes := outcomesOf(a)
	anchors := anchorsOf(a)

	domainRules := 0
	for _, r := range req.Rules {
		if r.Type() != rule.TypeDomain {
			t.Errorf("rule %s: a built-in rule is a domain rule, not %s", r.ID(), r.Type())
			continue
		}
		domainRules++
		for _, ctx := range []string{"catalog", "ordering", "billing"} {
			if _, ok := outcomes[r.ID().Qualified()+"|context:"+ctx]; !ok {
				t.Errorf("rule %s: no evaluation for context %s", r.ID(), ctx)
			}
		}
	}
	if domainRules != 20 {
		t.Fatalf("built-in domain rules = %d, want 20", domainRules)
	}
	if got := len(a.AppliedRules()); got != 20 {
		t.Errorf("applied rules = %d, want 20", got)
	}

	want := map[string][]string{
		"ubiquitous_language/terms-declared-in-code":            {"domain.arclint.yaml:25"},
		"context_relation/imports-follow-influence":             {"internal/event/event.go:5"},
		"domain_isolation/model-imports-nothing-outside-itself": {"internal/order/order.go:5"},
		"invariant/enforced-at-every-mutation":                  {"internal/event/event.go:65"},
		"specification/satisfaction-method":                     {"domain.arclint.yaml:15"},
		"aggregate/protects-an-invariant":                       {"domain.arclint.yaml:20"},
		"aggregate/commands-named-for-behavior":                 {"internal/event/event.go:60", "internal/order/order.go:20"},
		"domain_event/declared":                                 {"domain.arclint.yaml:28"},
	}
	for id, wantAnchors := range want {
		if got := anchors[id]; strings.Join(got, ",") != strings.Join(wantAnchors, ",") {
			t.Errorf("rule %s: violations at %v, want %v", id, got, wantAnchors)
		}
	}
	for id, got := range anchors {
		if _, expected := want[id]; !expected {
			t.Errorf("rule %s: unexpected violations at %v", id, got)
		}
	}

	wantOutcomes := map[string]conformance.Outcome{
		"bounded_context/isolated|context:catalog":                              conformance.OutcomeConforms,
		"bounded_context/isolated|context:ordering":                             conformance.OutcomeConforms,
		"bounded_context/isolated|context:billing":                              conformance.OutcomeConforms,
		"aggregate/protects-an-invariant|context:billing":                       conformance.OutcomeConforms,
		"aggregate/root-declared|context:catalog":                               conformance.OutcomeConforms,
		"aggregate/root-declared|context:billing":                               conformance.OutcomeConforms,
		"aggregate/invariants-enforced-by-root|context:catalog":                 conformance.OutcomeConforms,
		"assertion/checked-by-its-operation|context:catalog":                    conformance.OutcomeConforms,
		"repository/declared|context:catalog":                                   conformance.OutcomeConforms,
		"value_object/constructed-through-one-door|context:catalog":             conformance.OutcomeConforms,
		"invariant/enforced-at-every-mutation|context:ordering":                 conformance.OutcomeConforms,
		"ubiquitous_language/terms-declared-in-code|context:catalog":            conformance.OutcomeConforms,
		"domain_isolation/model-imports-nothing-outside-itself|context:catalog": conformance.OutcomeConforms,
		"context_relation/imports-follow-influence|context:ordering":            conformance.OutcomeConforms,
		"aggregate/protects-an-invariant|context:ordering":                      conformance.OutcomeViolates,
		"aggregate/references-by-identity|context:catalog":                      conformance.OutcomeConforms,
		"bounded_context/code-held-by-one-context|context:catalog":              conformance.OutcomeConforms,
	}
	for key, wantOutcome := range wantOutcomes {
		if got := outcomes[key]; got != wantOutcome {
			t.Errorf("%s: outcome %s, want %s", key, got, wantOutcome)
		}
	}
}

func TestDomainViolationsSpeakTheRecordedLanguage(t *testing.T) {
	a, err := conformance.Run(domainScenario(t))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	messages := map[string]string{}
	for _, v := range a.Violations() {
		messages[v.Rule().Qualified()+"@"+v.Path()] = v.Message()
	}
	want := map[string]string{
		"invariant/enforced-at-every-mutation@internal/event/event.go":                  "aggregate Event: command Cancel does not call EnsurePublishedOnly, so invariant published-only is not enforced when it completes",
		"context_relation/imports-follow-influence@internal/event/event.go":             "context catalog imports context ordering (example.com/mod/internal/order) against the recorded conformist relation: catalog is upstream of ordering, and only the downstream imports the upstream",
		"domain_isolation/model-imports-nothing-outside-itself@internal/order/order.go": "context ordering imports example.com/mod/internal/app, code of Zone(s) [\"app\"] that no context holds; dependencies run toward the model and never out of it",
		"ubiquitous_language/terms-declared-in-code@domain.arclint.yaml":                "value object OrderLine of context ordering names no type declaration in Zone \"ordering\"",
		"aggregate/commands-named-for-behavior@internal/order/order.go":                 "aggregate root Order declares setter SetStatus; a change of state is a command named for what it does",
		"aggregate/protects-an-invariant@domain.arclint.yaml":                           "aggregate Order of context ordering records no invariant; a boundary drawn around nothing that must stay consistent is not yet justified",
		"specification/satisfaction-method@domain.arclint.yaml":                         "specification Sellable of context catalog names no type declaration in Zone \"catalog\"",
	}
	for key, wantMessage := range want {
		if got := messages[key]; got != wantMessage {
			t.Errorf("%s:\n got %q\nwant %q", key, got, wantMessage)
		}
	}
}

// builtInRule finds one built-in Rule by id.
func builtInRule(t *testing.T, id string) rule.Rule {
	t.Helper()
	rules, err := rule.BuiltIn()
	if err != nil {
		t.Fatalf("BuiltIn: %v", err)
	}
	for _, r := range rules {
		if r.ID().Qualified() == id {
			return r
		}
	}
	t.Fatalf("no built-in rule %s", id)
	return rule.Rule{}
}

// oneContext records one context with the given aggregates and value
// objects.
func oneContext(t *testing.T, ctx vocab.BoundedContext) vocab.UbiquitousLanguage {
	t.Helper()
	knowledge, err := vocab.NewUbiquitousLanguage("p", "", []vocab.BoundedContext{ctx}, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	return knowledge
}

func runOne(t *testing.T, r rule.Rule, zones []rule.Zone, files []conformance.ObservedFile, facts map[string]conformance.LanguageFacts, knowledge vocab.UbiquitousLanguage) conformance.Assessment {
	t.Helper()
	obs, err := conformance.NewObservations(files, facts)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	a, err := conformance.Run(conformance.Request{
		Rules:          []rule.Rule{r},
		Zones:          zones,
		Observations:   obs,
		UnknownImports: rule.UnknownImportsWarn,
		Knowledge:      knowledge,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return a
}

// observed lists files by path.
func observed(paths ...string) []conformance.ObservedFile {
	out := make([]conformance.ObservedFile, 0, len(paths))
	for _, p := range paths {
		out = append(out, conformance.ObservedFile{Path: p})
	}
	return out
}

// goDecls is one parsed Go file of declarations and calls.
func goDecls(pkg string, decls []conformance.Declaration, calls ...conformance.Call) conformance.LanguageFacts {
	return conformance.LanguageFacts{Language: rule.LanguageGo, Package: pkg, DeclarationsAvailable: true, CallsAvailable: true, Declarations: decls, Calls: calls}
}

func TestDomainRuleIsUnsupportedWhenNoFileYieldsItsFacts(t *testing.T) {
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "catalog", Definition: "d", Line: 1,
		Aggregates: []vocab.Aggregate{{Name: "Event", Definition: "d", Identity: "EventID", Line: 2}},
	})
	zones := []rule.Zone{mustZone(t, "catalog", "internal/event/**")}
	files := observed("internal/event/event.rs", "internal/event/notes.md")
	facts := map[string]conformance.LanguageFacts{}

	a := runOne(t, builtInRule(t, "aggregate/root-declared"), zones, files, facts, knowledge)
	if got := outcomesOf(a)["aggregate/root-declared|context:catalog"]; got != conformance.OutcomeUnsupported {
		t.Errorf("declarations rule over facts-less code: outcome %s, want %s", got, conformance.OutcomeUnsupported)
	}

	// A rule reading no fact of the code judges the record alone.
	a = runOne(t, builtInRule(t, "aggregate/protects-an-invariant"), zones, files, facts, knowledge)
	if got := outcomesOf(a)["aggregate/protects-an-invariant|context:catalog"]; got != conformance.OutcomeViolates {
		t.Errorf("record-only rule over facts-less code: outcome %s, want %s", got, conformance.OutcomeViolates)
	}
}

// An aggregate's root is the one type spelling its name, found by
// declaration wherever it lives; nothing in the domain file names a
// path. Two such types leave nothing to choose between and are a
// finding that says where each is; an interface of the name yields to
// the struct; a Zone spelled with the context's name, in any case,
// narrows the search.
func TestAggregateRootIsLocatedByDeclaration(t *testing.T) {
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "ticket_catalog", Definition: "d", Line: 1,
		Aggregates: []vocab.Aggregate{{Name: "TicketTier", Definition: "d", Identity: "TierID", Line: 2}},
	})
	r := builtInRule(t, "aggregate/root-declared")
	tier := func(kind string) []conformance.Declaration {
		return []conformance.Declaration{{Kind: kind, Name: "TicketTier", Exported: true, StartLine: 3}}
	}
	cases := []struct {
		name        string
		zones       []rule.Zone
		facts       map[string]conformance.LanguageFacts
		wantAnchors []string
		wantMessage string
		wantRemedy  string
	}{
		{
			name:  "python class anywhere",
			facts: map[string]conformance.LanguageFacts{"src/domain/tiers/ticket_tier.py": {Language: rule.LanguagePython, DeclarationsAvailable: true, Declarations: tier("class")}},
		},
		{
			name:  "typescript class anywhere",
			facts: map[string]conformance.LanguageFacts{"src/tiers.ts": {Language: rule.LanguageTypeScript, DeclarationsAvailable: true, Declarations: tier("class")}},
		},
		{
			name:        "nothing spells it",
			facts:       map[string]conformance.LanguageFacts{"src/tiers.ts": {Language: rule.LanguageTypeScript, DeclarationsAvailable: true, Declarations: []conformance.Declaration{{Kind: "class", Name: "Tier", StartLine: 3}}}},
			wantAnchors: []string{"domain.arclint.yaml:2"},
			wantMessage: "aggregate TicketTier of context ticket_catalog has no root: no type TicketTier is declared in the repository",
			wantRemedy:  "declare the root type TicketTier (a struct in Go, a class in TypeScript or Python)",
		},
		{
			name: "two structs with nothing to choose between",
			facts: map[string]conformance.LanguageFacts{
				"internal/a/tier.go": goDecls("a", tier("struct")),
				"internal/b/tier.go": goDecls("b", tier("struct")),
			},
			wantAnchors: []string{"domain.arclint.yaml:2"},
			wantMessage: "aggregate TicketTier of context ticket_catalog has 2 candidate roots in the repository (internal/a/tier.go:3, internal/b/tier.go:3), and nothing tells which is the model's",
			wantRemedy:  "declare a Zone \"ticket_catalog\" over the context's code in rules.arclint.yaml, or exclude the twin under scan.exclude",
		},
		{
			name: "an interface yields to the struct",
			facts: map[string]conformance.LanguageFacts{
				"internal/a/tier.go":  goDecls("a", tier("struct")),
				"internal/b/ports.go": goDecls("b", tier("interface")),
			},
		},
		{
			name:  "a Zone spelled with the context's name narrows the search",
			zones: []rule.Zone{mustZone(t, "ticket-catalog", "internal/a/**")},
			facts: map[string]conformance.LanguageFacts{
				"internal/a/tier.go": goDecls("a", tier("struct")),
				"internal/b/tier.go": goDecls("b", tier("struct")),
			},
		},
		{
			name:  "the Zone is where the root must be",
			zones: []rule.Zone{mustZone(t, "ticket_catalog", "internal/a/**")},
			facts: map[string]conformance.LanguageFacts{
				"internal/a/pricing.go": goDecls("a", []conformance.Declaration{{Kind: "struct", Name: "Pricing", StartLine: 3}}),
				"internal/b/tier.go":    goDecls("b", tier("struct")),
			},
			wantAnchors: []string{"domain.arclint.yaml:2"},
			wantMessage: "aggregate TicketTier of context ticket_catalog has no root: no type TicketTier is declared in Zone \"ticket_catalog\"",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var paths []string
			for p := range tc.facts {
				paths = append(paths, p)
			}
			a := runOne(t, r, tc.zones, observed(paths...), tc.facts, knowledge)
			got := anchorsOf(a)[r.ID().Qualified()]
			if strings.Join(got, ",") != strings.Join(tc.wantAnchors, ",") {
				t.Fatalf("violations at %v, want %v", got, tc.wantAnchors)
			}
			if tc.wantMessage == "" {
				return
			}
			v := a.Violations()[0]
			if v.Message() != tc.wantMessage {
				t.Errorf("message:\n got %q\nwant %q", v.Message(), tc.wantMessage)
			}
			if tc.wantRemedy != "" && v.Remediation() != tc.wantRemedy {
				t.Errorf("remediation:\n got %q\nwant %q", v.Remediation(), tc.wantRemedy)
			}
		})
	}
}

// Two Zones spelling one context's name in different cases leave
// nothing to choose between; the run fails saying so rather than
// picking one.
func TestTwoZonesSpellingOneContextIsAnError(t *testing.T) {
	knowledge := oneContext(t, vocab.BoundedContext{Name: "ticket_catalog", Definition: "d", Line: 1})
	obs, err := conformance.NewObservations(observed("internal/a/tier.go"), nil)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	_, err = conformance.Run(conformance.Request{
		Rules:          []rule.Rule{builtInRule(t, "aggregate/root-declared")},
		Zones:          []rule.Zone{mustZone(t, "ticket-catalog", "internal/a/**"), mustZone(t, "ticketcatalog", "internal/b/**")},
		Observations:   obs,
		UnknownImports: rule.UnknownImportsWarn,
		Knowledge:      knowledge,
	})
	if err == nil || !strings.Contains(err.Error(), `context ticket_catalog: Zones ["ticket-catalog", "ticketcatalog"] all spell its name`) {
		t.Fatalf("Run = %v, want the two Zones named", err)
	}
}

func TestRepositoryMustBeAnInterfaceInGo(t *testing.T) {
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "catalog", Definition: "d", Line: 1,
		Aggregates: []vocab.Aggregate{{Name: "Event", Definition: "d", Identity: "EventID", Repository: "EventRepository", Line: 2}},
	})
	zones := []rule.Zone{mustZone(t, "catalog", "internal/event/**")}
	files := observed("internal/event/event.go", "internal/event/repository.go")
	facts := map[string]conformance.LanguageFacts{
		"internal/event/event.go": goDecls("event", []conformance.Declaration{
			{Kind: "struct", Name: "Event", Exported: true, StartLine: 3},
		}),
		"internal/event/repository.go": goDecls("event", []conformance.Declaration{
			{Kind: "struct", Name: "EventRepository", Exported: true, StartLine: 4},
		}),
	}
	a := runOne(t, builtInRule(t, "repository/declared"), zones, files, facts, knowledge)
	got := anchorsOf(a)["repository/declared"]
	if strings.Join(got, ",") != "internal/event/repository.go:4" {
		t.Errorf("violations at %v, want the struct declaration", got)
	}
	vs := a.Violations()
	if len(vs) != 1 || !strings.Contains(vs[0].Message(), "declared as a struct") {
		t.Errorf("message = %q, want the declaration kind named", vs[0].Message())
	}
}

// A constructor or command of one root that takes or returns another
// aggregate's root is a finding: through the package it imports in Go,
// bare inside one package or a module it imports in TypeScript, never
// through the other's identity, and never across packages that do not
// import each other.
func TestReferencesByIdentity(t *testing.T) {
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "ordering", Definition: "d", Line: 1,
		Aggregates: []vocab.Aggregate{
			{Name: "Order", Definition: "d", Identity: "OrderID", Line: 2},
			{Name: "Refund", Definition: "d", Identity: "RefundID", Line: 3},
		},
	})
	r := builtInRule(t, "aggregate/references-by-identity")
	goWithImports := func(pkg string, imports []conformance.Import, decls []conformance.Declaration) conformance.LanguageFacts {
		f := goDecls(pkg, decls)
		f.ImportsAvailable = true
		f.Imports = imports
		return f
	}
	orderImport := conformance.Import{Path: "example.com/mod/internal/order", Line: 3, Class: conformance.ImportInternal, TargetDir: "internal/order"}
	orderRoot := goWithImports("order", nil, []conformance.Declaration{{Kind: "struct", Name: "Order", Exported: true, StartLine: 5}})

	t.Run("go root through its package", func(t *testing.T) {
		facts := map[string]conformance.LanguageFacts{
			"internal/order/order.go": orderRoot,
			"internal/refund/refund.go": goWithImports("refund", []conformance.Import{orderImport}, []conformance.Declaration{
				{Kind: "struct", Name: "Refund", Exported: true, StartLine: 6},
				{Kind: "func", Name: "New", Exported: true, StartLine: 10, Params: []conformance.DeclarationParam{{Name: "o", Type: "*order.Order"}}, Results: []string{"*Refund", "error"}},
				{Kind: "method", Name: "Attach", Owner: "Refund", Exported: true, StartLine: 20, Params: []conformance.DeclarationParam{{Name: "id", Type: "order.ID"}}, Results: []string{"error"}},
				{Kind: "method", Name: "Settle", Owner: "Refund", Exported: true, StartLine: 30, Results: []string{"*order.Order", "error"}},
			}),
		}
		a := runOne(t, r, nil, observed("internal/order/order.go", "internal/refund/refund.go"), facts, knowledge)
		if got := anchorsOf(a)[r.ID().Qualified()]; strings.Join(got, ",") != "internal/refund/refund.go:10,internal/refund/refund.go:30" {
			t.Fatalf("violations at %v, want New and Settle, not Attach", got)
		}
		vs := a.Violations()
		if vs[0].Message() != "aggregate Refund: constructor New takes *order.Order, the root of aggregate Order; what one aggregate needs of another it receives as an identity or a value" {
			t.Errorf("message = %q", vs[0].Message())
		}
		if vs[0].Remediation() != "pass Order's identity (OrderID) or a value into Refund instead of its root" {
			t.Errorf("remediation = %q", vs[0].Remediation())
		}
		if vs[1].Message() != "aggregate Refund: command Settle returns *order.Order, the root of aggregate Order; what one aggregate needs of another it receives as an identity or a value" {
			t.Errorf("message = %q", vs[1].Message())
		}
	})

	t.Run("go root bare in one package", func(t *testing.T) {
		facts := map[string]conformance.LanguageFacts{
			"internal/ordering/order.go": goWithImports("ordering", nil, []conformance.Declaration{{Kind: "struct", Name: "Order", Exported: true, StartLine: 5}}),
			"internal/ordering/refund.go": goWithImports("ordering", nil, []conformance.Declaration{
				{Kind: "struct", Name: "Refund", Exported: true, StartLine: 6},
				{Kind: "func", Name: "NewRefund", Exported: true, StartLine: 10, Params: []conformance.DeclarationParam{{Name: "o", Type: "Order"}}, Results: []string{"Refund", "error"}},
				{Kind: "func", Name: "RefundFor", Exported: true, StartLine: 15, Params: []conformance.DeclarationParam{{Name: "id", Type: "OrderID"}}, Results: []string{"Refund", "error"}},
			}),
		}
		a := runOne(t, r, nil, observed("internal/ordering/order.go", "internal/ordering/refund.go"), facts, knowledge)
		if got := anchorsOf(a)[r.ID().Qualified()]; strings.Join(got, ",") != "internal/ordering/refund.go:10" {
			t.Fatalf("violations at %v, want NewRefund alone: OrderID is the identity", got)
		}
	})

	t.Run("go packages that do not import each other", func(t *testing.T) {
		facts := map[string]conformance.LanguageFacts{
			"internal/order/order.go": orderRoot,
			"internal/refund/refund.go": goWithImports("refund", nil, []conformance.Declaration{
				{Kind: "struct", Name: "Refund", Exported: true, StartLine: 6},
				{Kind: "struct", Name: "Order", Exported: true, StartLine: 8},
				{Kind: "func", Name: "New", Exported: true, StartLine: 10, Params: []conformance.DeclarationParam{{Name: "o", Type: "Order"}}, Results: []string{"*Refund", "error"}},
			}),
		}
		// Two structs spell Order now, so Order itself is unlocated and
		// Refund's bare Order is its own package's type.
		a := runOne(t, r, nil, observed("internal/order/order.go", "internal/refund/refund.go"), facts, knowledge)
		if got := anchorsOf(a)[r.ID().Qualified()]; len(got) != 0 {
			t.Fatalf("violations at %v, want none", got)
		}
	})

	t.Run("typescript root through an imported module", func(t *testing.T) {
		facts := map[string]conformance.LanguageFacts{
			"src/order/order.ts": {Language: rule.LanguageTypeScript, DeclarationsAvailable: true, ImportsAvailable: true, Declarations: []conformance.Declaration{
				{Kind: "class", Name: "Order", Exported: true, StartLine: 1},
			}},
			"src/refund/refund.ts": {
				Language: rule.LanguageTypeScript, DeclarationsAvailable: true, ImportsAvailable: true,
				Imports: []conformance.Import{{Path: "../order/order", Line: 1, Class: conformance.ImportInternal, TargetFile: "src/order/order.ts"}},
				Declarations: []conformance.Declaration{
					{Kind: "class", Name: "Refund", Exported: true, StartLine: 3},
					{Kind: "method", Name: "create", Owner: "Refund", Exported: true, StartLine: 5, Params: []conformance.DeclarationParam{{Name: "order", Type: "Order"}}, Results: []string{"Refund"}},
					{Kind: "method", Name: "settle", Owner: "Refund", Exported: true, StartLine: 12, Params: []conformance.DeclarationParam{{Name: "order", Type: "Order"}}, Results: []string{"Result<void, RefundError>"}},
				},
			},
		}
		a := runOne(t, r, nil, observed("src/order/order.ts", "src/refund/refund.ts"), facts, knowledge)
		if got := anchorsOf(a)[r.ID().Qualified()]; strings.Join(got, ",") != "src/refund/refund.ts:12,src/refund/refund.ts:5" {
			t.Fatalf("violations at %v, want create and settle", got)
		}
	})
}

func TestContextIsolationRequiresARecordedRelation(t *testing.T) {
	knowledge, err := vocab.NewUbiquitousLanguage("p", "", []vocab.BoundedContext{
		{Name: "catalog", Definition: "d", Line: 1},
		{Name: "ordering", Definition: "d", Line: 2},
	}, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	zones := []rule.Zone{mustZone(t, "catalog", "internal/event/**"), mustZone(t, "ordering", "internal/order/**")}
	files := observed("internal/event/event.go", "internal/order/order.go")
	facts := map[string]conformance.LanguageFacts{
		"internal/event/event.go": {Language: rule.LanguageGo, ImportsAvailable: true, DeclarationsAvailable: true},
		"internal/order/order.go": {Language: rule.LanguageGo, ImportsAvailable: true, DeclarationsAvailable: true, Imports: []conformance.Import{
			{Path: "example.com/mod/internal/event", Line: 4, Class: conformance.ImportInternal, TargetDir: "internal/event"},
		}},
	}
	a := runOne(t, builtInRule(t, "bounded_context/isolated"), zones, files, facts, knowledge)
	got := anchorsOf(a)["bounded_context/isolated"]
	if strings.Join(got, ",") != "internal/order/order.go:4" {
		t.Errorf("violations at %v, want the unrecorded import", got)
	}
	vs := a.Violations()
	if len(vs) != 1 || !strings.Contains(vs[0].Message(), "records no relation between them") {
		t.Errorf("message = %v", vs)
	}
}

// Without a Zone named for it, a context's code is what declares its
// terms: the packages of its roots and the files of its other terms.
// An import from that code into another context's code is judged; a
// file declaring nothing recorded is nobody's code.
func TestContextCodeIsLocatedFromItsTermsWithoutAZone(t *testing.T) {
	knowledge, err := vocab.NewUbiquitousLanguage("p", "", []vocab.BoundedContext{
		{Name: "catalog", Definition: "d", Line: 1, Aggregates: []vocab.Aggregate{{Name: "Event", Definition: "d", Identity: "EventID", Line: 2}}},
		{Name: "ordering", Definition: "d", Line: 3, ValueObjects: []vocab.ValueObject{{Name: "OrderLine", Definition: "d", Line: 4}}},
	}, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	eventImport := conformance.Import{Path: "example.com/mod/internal/event", Line: 4, Class: conformance.ImportInternal, TargetDir: "internal/event"}
	files := observed("internal/event/event.go", "internal/event/notes.go", "internal/order/line.go", "internal/glue/glue.go")
	facts := map[string]conformance.LanguageFacts{
		"internal/event/event.go": goDecls("event", []conformance.Declaration{{Kind: "struct", Name: "Event", Exported: true, StartLine: 3}}),
		"internal/event/notes.go": goDecls("event", nil),
		"internal/order/line.go": {
			Language: rule.LanguageGo, Package: "order", DeclarationsAvailable: true, ImportsAvailable: true,
			Declarations: []conformance.Declaration{{Kind: "struct", Name: "OrderLine", Exported: true, StartLine: 6}},
			Imports:      []conformance.Import{eventImport},
		},
		"internal/glue/glue.go": {Language: rule.LanguageGo, Package: "glue", ImportsAvailable: true, Imports: []conformance.Import{eventImport}},
	}
	r := builtInRule(t, "bounded_context/isolated")
	a := runOne(t, r, nil, files, facts, knowledge)
	if got := anchorsOf(a)[r.ID().Qualified()]; strings.Join(got, ",") != "internal/order/line.go:4" {
		t.Errorf("violations at %v, want the import from OrderLine's file alone", got)
	}
}

func TestValueObjectWithInvariantNeedsAConstructor(t *testing.T) {
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "catalog", Definition: "d", Line: 1,
		ValueObjects: []vocab.ValueObject{
			{Name: "Price", Definition: "d", Line: 2, Invariants: []vocab.Invariant{{Key: "whole-cents", Statement: "s", Line: 3}}},
			{Name: "Label", Definition: "d", Line: 4},
		},
	})
	zones := []rule.Zone{mustZone(t, "catalog", "src/**")}
	files := observed("src/price.ts", "src/label.ts")
	facts := map[string]conformance.LanguageFacts{
		"src/price.ts": {Language: rule.LanguageTypeScript, DeclarationsAvailable: true, Declarations: []conformance.Declaration{
			{Kind: "class", Name: "Price", Exported: true, StartLine: 1},
			{Kind: "method", Name: "setCents", Owner: "Price", Exported: true, StartLine: 5},
		}},
		"src/label.ts": {Language: rule.LanguageTypeScript, DeclarationsAvailable: true, Declarations: []conformance.Declaration{
			{Kind: "class", Name: "Label", Exported: true, StartLine: 1},
		}},
	}
	a := runOne(t, builtInRule(t, "value_object/constructed-through-one-door"), zones, files, facts, knowledge)
	if got := anchorsOf(a)["value_object/constructed-through-one-door"]; strings.Join(got, ",") != "src/price.ts:1" {
		t.Errorf("constructor violations at %v, want Price alone (Label records no invariant)", got)
	}
	a = runOne(t, builtInRule(t, "value_object/no-setters"), zones, files, facts, knowledge)
	if got := anchorsOf(a)["value_object/no-setters"]; strings.Join(got, ",") != "src/price.ts:5" {
		t.Errorf("setter violations at %v, want setCents", got)
	}

	facts["src/price.ts"] = conformance.LanguageFacts{Language: rule.LanguageTypeScript, DeclarationsAvailable: true, Declarations: []conformance.Declaration{
		{Kind: "class", Name: "Price", Exported: true, StartLine: 1},
		{Kind: "method", Name: "constructor", Owner: "Price", StartLine: 2},
		{Kind: "method", Name: "settle", Owner: "Price", Exported: true, StartLine: 5},
	}}
	a = runOne(t, builtInRule(t, "value_object/constructed-through-one-door"), zones, files, facts, knowledge)
	if got := anchorsOf(a)["value_object/constructed-through-one-door"]; len(got) != 0 {
		t.Errorf("a class constructor is a door: violations at %v", got)
	}
	a = runOne(t, builtInRule(t, "value_object/no-setters"), zones, files, facts, knowledge)
	if got := anchorsOf(a)["value_object/no-setters"]; len(got) != 0 {
		t.Errorf("settle is not a setter: violations at %v", got)
	}
}

// Any function of the unit returning the type is a door, whatever its
// name; a type no function returns has none, and the finding says what
// counts.
func TestAnyFunctionReturningTheTypeIsADoor(t *testing.T) {
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "distribution", Definition: "d", Line: 1,
		ValueObjects: []vocab.ValueObject{
			{Name: "Digest", Definition: "d", Line: 2, Invariants: []vocab.Invariant{{Key: "hex-encoded", Statement: "s", Line: 3}}},
			{Name: "Version", Definition: "d", Line: 4, Invariants: []vocab.Invariant{{Key: "semver", Statement: "s", Line: 5}}},
		},
	})
	files := observed("internal/distribution/digest.go", "internal/distribution/version.go")
	facts := map[string]conformance.LanguageFacts{
		"internal/distribution/digest.go": goDecls("distribution", []conformance.Declaration{
			{Kind: "struct", Name: "Digest", Exported: true, StartLine: 3},
			{Kind: "func", Name: "Sum", Exported: true, StartLine: 8, Results: []string{"Digest", "error"}},
		}),
		"internal/distribution/version.go": goDecls("distribution", []conformance.Declaration{
			{Kind: "struct", Name: "Version", Exported: true, StartLine: 3},
			{Kind: "func", Name: "Compare", Exported: true, StartLine: 8, Params: []conformance.DeclarationParam{{Name: "a", Type: "Version"}, {Name: "b", Type: "Version"}}, Results: []string{"int"}},
			{Kind: "func", Name: "Sorted", Exported: true, StartLine: 14, Results: []string{"[]Version"}},
		}),
	}
	r := builtInRule(t, "value_object/constructed-through-one-door")
	a := runOne(t, r, nil, files, facts, knowledge)
	if got := anchorsOf(a)[r.ID().Qualified()]; strings.Join(got, ",") != "internal/distribution/version.go:3" {
		t.Errorf("violations at %v, want Version alone: Sum returns a Digest, and a slice of Versions is not a door", got)
	}
	vs := a.Violations()
	if len(vs) != 1 || vs[0].Message() != "value object Version records invariant semver and declares no constructor; a value that violates it can be built" {
		t.Errorf("message = %v", vs)
	}
	if !strings.Contains(vs[0].Remediation(), "declare a constructor for Version (a function returning it, constructor or __init__, or a factory method on it named create, from, of, parse, new, or build)") {
		t.Errorf("remediation = %q", vs[0].Remediation())
	}
}

// The root method enforcing an invariant is ensure followed by the
// key in the language's method case; the finding for a missing one
// spells the name it expected, and the doors are judged only once the
// method exists.
func TestInvariantMethodsAreSpelledPerLanguage(t *testing.T) {
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "capacity", Definition: "d", Line: 1,
		Aggregates: []vocab.Aggregate{{
			Name: "Capacity", Definition: "d", Identity: "CapacityID", Line: 2,
			Invariants: []vocab.Invariant{{Key: "seat-budget", Statement: "s", Line: 3}},
		}},
	})
	files := observed("src/capacity/capacity.py")
	facts := map[string]conformance.LanguageFacts{
		"src/capacity/capacity.py": {Language: rule.LanguagePython, DeclarationsAvailable: true, CallsAvailable: true, Declarations: []conformance.Declaration{
			{Kind: "class", Name: "Capacity", Exported: true, StartLine: 1},
			{Kind: "method", Name: "__init__", Owner: "Capacity", StartLine: 2},
			{Kind: "method", Name: "ensure_seat_budget", Owner: "Capacity", Exported: true, StartLine: 6},
		}, Calls: []conformance.Call{{Callee: "ensure_seat_budget", Line: 4, Enclosing: "__init__"}}},
	}
	for _, id := range []string{"aggregate/invariants-enforced-by-root", "invariant/enforced-at-every-mutation"} {
		a := runOne(t, builtInRule(t, id), nil, files, facts, knowledge)
		if got := outcomesOf(a)[id+"|context:capacity"]; got != conformance.OutcomeConforms {
			t.Errorf("%s: outcome %s, want conforms; violations %v", id, got, anchorsOf(a)[id])
		}
	}

	facts["src/capacity/capacity.py"] = conformance.LanguageFacts{Language: rule.LanguagePython, DeclarationsAvailable: true, CallsAvailable: true, Declarations: []conformance.Declaration{
		{Kind: "class", Name: "Capacity", Exported: true, StartLine: 1},
		{Kind: "method", Name: "seat_budget", Owner: "Capacity", Exported: true, StartLine: 6},
	}}
	a := runOne(t, builtInRule(t, "aggregate/invariants-enforced-by-root"), nil, files, facts, knowledge)
	vs := a.Violations()
	if len(vs) != 1 || vs[0].Message() != "aggregate Capacity: invariant seat-budget is not enforced by a method of the root; expected ensure_seat_budget (python)" {
		t.Errorf("missing method message = %v", vs)
	}
	a = runOne(t, builtInRule(t, "invariant/enforced-at-every-mutation"), nil, files, facts, knowledge)
	if got := anchorsOf(a)["invariant/enforced-at-every-mutation"]; len(got) != 0 {
		t.Errorf("an undeclared method is the root's finding, not every door's: violations at %v", got)
	}
}

// A root with its ensure method but no constructor is a finding at the
// root: nothing enforces the invariant when one is built.
func TestInvariantNeedsADoorToBeEnforcedAt(t *testing.T) {
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "capacity", Definition: "d", Line: 1,
		Aggregates: []vocab.Aggregate{{
			Name: "Capacity", Definition: "d", Identity: "CapacityID", Line: 2,
			Invariants: []vocab.Invariant{{Key: "seat-budget", Statement: "s", Line: 3}},
		}},
	})
	files := observed("src/capacity/capacity.go")
	facts := map[string]conformance.LanguageFacts{
		"src/capacity/capacity.go": goDecls("capacity", []conformance.Declaration{
			{Kind: "struct", Name: "Capacity", Exported: true, StartLine: 1},
			{Kind: "method", Name: "EnsureSeatBudget", Owner: "Capacity", Exported: true, Results: []string{"error"}, StartLine: 6},
		}),
	}
	r := builtInRule(t, "invariant/enforced-at-every-mutation")
	a := runOne(t, r, nil, files, facts, knowledge)
	vs := a.Violations()
	if len(vs) != 1 || vs[0].Message() != "aggregate Capacity declares no constructor (a function returning it, constructor or __init__, or a factory method on it named create, from, of, parse, new, or build), so invariant seat-budget is not enforced when a Capacity is built" {
		t.Errorf("violations = %v", vs)
	}
}

// A Go initialism keeps its capitals: the method and the call spelled
// EnsureUniqueID both carry the key unique-id, and a command spelled
// that way is a contract method, not a command left uncovered.
func TestGoInitialismsSpellAKey(t *testing.T) {
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "capacity", Definition: "d", Line: 1,
		Aggregates: []vocab.Aggregate{{
			Name: "Capacity", Definition: "d", Identity: "CapacityID", Line: 2,
			Invariants: []vocab.Invariant{{Key: "unique-id", Statement: "s", Line: 3}},
		}},
	})
	files := observed("src/capacity/capacity.go")
	facts := map[string]conformance.LanguageFacts{
		"src/capacity/capacity.go": goDecls("capacity", []conformance.Declaration{
			{Kind: "struct", Name: "Capacity", Exported: true, StartLine: 1},
			{Kind: "func", Name: "New", Results: []string{"*Capacity", "error"}, StartLine: 2},
			{Kind: "method", Name: "EnsureUniqueID", Owner: "Capacity", Exported: true, Results: []string{"error"}, StartLine: 6},
			{Kind: "method", Name: "Reserve", Owner: "Capacity", Exported: true, Results: []string{"error"}, StartLine: 10},
		},
			conformance.Call{Callee: "EnsureUniqueID", Line: 4, Enclosing: "New"},
			conformance.Call{Callee: "EnsureUniqueID", Line: 12, Enclosing: "Reserve"},
		),
	}
	for _, id := range []string{"aggregate/invariants-enforced-by-root", "invariant/enforced-at-every-mutation"} {
		a := runOne(t, builtInRule(t, id), nil, files, facts, knowledge)
		if got := outcomesOf(a)[id+"|context:capacity"]; got != conformance.OutcomeConforms {
			t.Errorf("%s: outcome %s, want conforms; violations %v", id, got, anchorsOf(a)[id])
		}
	}
}

// An assertion is checked by assert followed by its key, called from
// the operation it constrains; the findings name the operation the root
// lacks, the checking method it lacks, and the call the operation
// lacks.
func TestAssertionIsCheckedByItsOperation(t *testing.T) {
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "catalog", Definition: "d", Line: 1,
		Aggregates: []vocab.Aggregate{{
			Name: "Event", Definition: "d", Identity: "EventID", Line: 2,
			Assertions: []vocab.Assertion{{Key: "tiers-priced", On: "Publish", Statement: "s", Line: 4}},
		}},
	})
	r := builtInRule(t, "assertion/checked-by-its-operation")
	files := observed("internal/event/event.go")
	run := func(decls []conformance.Declaration, calls ...conformance.Call) []string {
		a := runOne(t, r, nil, files, map[string]conformance.LanguageFacts{"internal/event/event.go": goDecls("event", decls, calls...)}, knowledge)
		var out []string
		for _, v := range a.Violations() {
			out = append(out, v.Message())
		}
		return out
	}
	root := conformance.Declaration{Kind: "struct", Name: "Event", Exported: true, StartLine: 3}
	publish := conformance.Declaration{Kind: "method", Name: "Publish", Owner: "Event", Exported: true, StartLine: 10, Results: []string{"error"}}
	check := conformance.Declaration{Kind: "method", Name: "AssertTiersPriced", Owner: "Event", Exported: true, StartLine: 20, Results: []string{"error"}}

	got := run([]conformance.Declaration{root})
	want := []string{
		"aggregate Event: assertion tiers-priced constrains operation Publish, which the root does not declare",
		"aggregate Event: assertion tiers-priced names no checking method on the root; expected AssertTiersPriced (go)",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("bare root:\n got %q\nwant %q", got, want)
	}
	got = run([]conformance.Declaration{root, publish, check})
	if len(got) != 1 || got[0] != "aggregate Event: operation Publish does not call AssertTiersPriced, so assertion tiers-priced is not checked when it completes" {
		t.Errorf("uncalled check: %q", got)
	}
	if got = run([]conformance.Declaration{root, publish, check}, conformance.Call{Callee: "AssertTiersPriced", Line: 12, Enclosing: "Publish"}); len(got) != 0 {
		t.Errorf("checked operation: %q", got)
	}
}

// Two contexts holding one file is a finding for each unless they
// record a shared_kernel. A context with a Zone named for it holds
// every file of the Zone; one without holds the files of its terms.
func TestTwoContextsHoldingOneCodeIsNotABoundary(t *testing.T) {
	contexts := []vocab.BoundedContext{
		{Name: "rule", Definition: "d", Line: 1},
		{Name: "adoption", Definition: "d", Line: 2, ValueObjects: []vocab.ValueObject{{Name: "Binding", Definition: "d", Line: 3}}},
	}
	zones := []rule.Zone{mustZone(t, "rule", "internal/rule/**")}
	files := observed("internal/rule/rule.go", "internal/rule/binding.go")
	facts := map[string]conformance.LanguageFacts{
		"internal/rule/rule.go":    goDecls("rule", nil),
		"internal/rule/binding.go": goDecls("rule", []conformance.Declaration{{Kind: "struct", Name: "Binding", Exported: true, StartLine: 4}}),
	}
	r := builtInRule(t, "bounded_context/code-held-by-one-context")

	knowledge, err := vocab.NewUbiquitousLanguage("p", "", contexts, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	a := runOne(t, r, zones, files, facts, knowledge)
	if got := anchorsOf(a)[r.ID().Qualified()]; strings.Join(got, ",") != "domain.arclint.yaml:1,domain.arclint.yaml:2" {
		t.Errorf("violations at %v, want one per context at its record", got)
	}
	vs := a.Violations()
	if len(vs) != 2 || vs[0].Message() != "context rule holds 1 file(s) that context adoption holds too (internal/rule/binding.go), and the context map records no shared_kernel between them; a boundary two models straddle is not a boundary" {
		t.Errorf("message = %v", vs)
	}
	if len(vs) == 2 && vs[1].Remediation() != "narrow adoption and rule with a Zone named for each in rules.arclint.yaml, or record a shared_kernel relation between them under relations in domain.arclint.yaml" {
		t.Errorf("remediation = %q", vs[1].Remediation())
	}

	knowledge, err = vocab.NewUbiquitousLanguage("p", "", contexts, []vocab.ContextRelation{
		{From: "rule", To: "adoption", Kind: vocab.RelationSharedKernel, Line: 4},
	})
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	a = runOne(t, r, zones, files, facts, knowledge)
	if got := anchorsOf(a)[r.ID().Qualified()]; len(got) != 0 {
		t.Errorf("a recorded shared_kernel permits the shared code: violations at %v", got)
	}
}

func TestModelImportsOutsideJudgeTheImportedFile(t *testing.T) {
	// One Zone spans the whole tree and a context-named Zone sits inside
	// it: an import of the context's own sibling file lands in both
	// Zones and is held by the context, while an import of a file only
	// the spanning Zone holds is code no context holds.
	knowledge := oneContext(t, vocab.BoundedContext{Name: "ordering", Definition: "d", Line: 1})
	zones := []rule.Zone{mustZone(t, "source", "internal/**"), mustZone(t, "ordering", "internal/order/**")}
	files := observed("internal/order/order.go", "internal/order/line.go", "internal/clock/clock.go")
	facts := map[string]conformance.LanguageFacts{
		"internal/order/order.go": {Language: rule.LanguageGo, ImportsAvailable: true, DeclarationsAvailable: true, Imports: []conformance.Import{
			{Path: "example.com/mod/internal/order/line", Line: 3, Class: conformance.ImportInternal, TargetFile: "internal/order/line.go"},
			{Path: "example.com/mod/internal/clock", Line: 4, Class: conformance.ImportInternal, TargetDir: "internal/clock"},
		}},
		"internal/order/line.go":  {Language: rule.LanguageGo, ImportsAvailable: true, DeclarationsAvailable: true},
		"internal/clock/clock.go": {Language: rule.LanguageGo, ImportsAvailable: true, DeclarationsAvailable: true},
	}
	r := builtInRule(t, "domain_isolation/model-imports-nothing-outside-itself")
	a := runOne(t, r, zones, files, facts, knowledge)
	if got := anchorsOf(a)[r.ID().Qualified()]; strings.Join(got, ",") != "internal/order/order.go:4" {
		t.Errorf("violations at %v, want the clock import alone", got)
	}
	vs := a.Violations()
	if len(vs) != 1 || vs[0].Message() != `context ordering imports example.com/mod/internal/clock, code of Zone(s) ["source"] that no context holds; dependencies run toward the model and never out of it` {
		t.Errorf("message = %v", vs)
	}
	if len(vs) == 1 && vs[0].Remediation() != "invert the dependency so that example.com/mod/internal/clock depends on the model, or make it the model's code by recording the terms it declares in domain.arclint.yaml" {
		t.Errorf("remediation = %q", vs[0].Remediation())
	}

	// Without a declared Zone there is no outside to judge.
	a = runOne(t, r, nil, files, facts, knowledge)
	if got := outcomesOf(a)[r.ID().Qualified()+"|context:ordering"]; got != conformance.OutcomeNotApplicable {
		t.Errorf("without Zones: outcome %s, want %s", got, conformance.OutcomeNotApplicable)
	}
}

func TestGoPackageCompletesADeclaredName(t *testing.T) {
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "ordering", Definition: "d", Line: 1,
		Aggregates:   []vocab.Aggregate{{Name: "Order", Definition: "d", Identity: "OrderID", Line: 2}},
		ValueObjects: []vocab.ValueObject{{Name: "OrderLine", Definition: "d", Line: 3}, {Name: "Money", Definition: "d", Line: 4}},
	})
	zones := []rule.Zone{mustZone(t, "ordering", "internal/order/**")}
	files := observed("internal/order/order.go", "internal/order/id.go", "internal/order/line.go")
	facts := map[string]conformance.LanguageFacts{
		"internal/order/order.go": goDecls("order", []conformance.Declaration{{Kind: "struct", Name: "Order", Exported: true, StartLine: 3}}),
		"internal/order/id.go":    goDecls("order", []conformance.Declaration{{Kind: "type", Name: "ID", Exported: true, StartLine: 3}}),
		"internal/order/line.go": goDecls("order", []conformance.Declaration{
			{Kind: "struct", Name: "Line", Exported: true, StartLine: 3},
			{Kind: "type", Name: "money", StartLine: 9},
		}),
	}
	r := builtInRule(t, "ubiquitous_language/terms-declared-in-code")
	a := runOne(t, r, zones, files, facts, knowledge)
	got := anchorsOf(a)[r.ID().Qualified()]
	if strings.Join(got, ",") != "domain.arclint.yaml:4" {
		t.Errorf("violations at %v, want Money alone: order.ID spells OrderID and order.Line spells OrderLine, and lowercase money is not type case", got)
	}
	vs := a.Violations()
	if len(vs) != 1 || vs[0].Message() != `value object Money of context ordering names no type declaration in Zone "ordering"` {
		t.Errorf("message = %v", vs)
	}
}

// A term two declarations spell with nothing to choose between them is
// a finding that names both, not a silent pick; a declaration beside a
// located root settles it.
func TestTermDeclaredTwiceIsAFinding(t *testing.T) {
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "ordering", Definition: "d", Line: 1,
		Aggregates:   []vocab.Aggregate{{Name: "Order", Definition: "d", Identity: "OrderID", Line: 2}},
		ValueObjects: []vocab.ValueObject{{Name: "Money", Definition: "d", Line: 3}},
	})
	r := builtInRule(t, "ubiquitous_language/terms-declared-in-code")
	money := []conformance.Declaration{{Kind: "struct", Name: "Money", Exported: true, StartLine: 3}}
	files := observed("internal/order/order.go", "internal/billing/money.go", "internal/shared/money.go")
	facts := map[string]conformance.LanguageFacts{
		"internal/order/order.go": goDecls("order", []conformance.Declaration{
			{Kind: "struct", Name: "Order", Exported: true, StartLine: 3},
			{Kind: "type", Name: "ID", Exported: true, StartLine: 8},
		}),
		"internal/billing/money.go": goDecls("billing", money),
		"internal/shared/money.go":  goDecls("shared", money),
	}
	a := runOne(t, r, nil, files, facts, knowledge)
	vs := a.Violations()
	if len(vs) != 1 || vs[0].Message() != "value object Money of context ordering is declared 2 times in the repository (internal/billing/money.go:3, internal/shared/money.go:3), and nothing tells which is the model's" {
		t.Errorf("violations = %v", vs)
	}
	if len(vs) == 1 && vs[0].Remediation() != `declare a Zone "ordering" over the context's code in rules.arclint.yaml, or exclude the twin under scan.exclude` {
		t.Errorf("remediation = %q", vs[0].Remediation())
	}

	files = append(files, conformance.ObservedFile{Path: "internal/order/money.go"})
	facts["internal/order/money.go"] = goDecls("order", money)
	a = runOne(t, r, nil, files, facts, knowledge)
	if got := anchorsOf(a)[r.ID().Qualified()]; len(got) != 0 {
		t.Errorf("the declaration beside the root is the model's: violations at %v", got)
	}
}
