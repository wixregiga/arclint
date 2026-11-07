package conformance_test

import (
	"slices"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// attributionOver runs the rules over the files and attributes the
// assessment to the recorded language, the way Studio's evidence does.
func attributionOver(t *testing.T, rules []rule.Rule, zones []rule.Zone, facts map[string]conformance.LanguageFacts, knowledge vocab.UbiquitousLanguage, parseFailures ...string) (conformance.EvidenceAttribution, conformance.Assessment) {
	t.Helper()
	paths := make([]string, 0, len(facts))
	for p := range facts {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	obs, err := conformance.NewObservations(observed(paths...), facts)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	a, err := conformance.Run(conformance.Request{Rules: rules, Zones: zones, Observations: obs, UnknownImports: rule.UnknownImportsWarn, Knowledge: knowledge})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	inspection, err := conformance.NewInspectionFacts(zones, obs, rule.FactDeclarations)
	if err != nil {
		t.Fatal(err)
	}
	carriers, err := conformance.NewCarriers(inspection, knowledge)
	if err != nil {
		t.Fatalf("NewCarriers: %v", err)
	}
	attribution, err := conformance.NewEvidenceAttribution(conformance.EvidenceAttributionSpec{Knowledge: knowledge, Assessment: a, Carriers: carriers, ParseFailures: parseFailures})
	if err != nil {
		t.Fatalf("NewEvidenceAttribution: %v", err)
	}
	return attribution, a
}

func evidenceNamed(t *testing.T, a conformance.EvidenceAttribution, concept vocab.Concept, name string) conformance.EntryEvidence {
	t.Helper()
	for _, e := range a.Entries() {
		if e.Entry().Concept == concept && e.Entry().Name == name {
			return e
		}
	}
	t.Fatalf("no evidence for %s %s", concept, name)
	return conformance.EntryEvidence{}
}

func basesOf(e conformance.EntryEvidence, ruleID string) []conformance.EvidenceBasis {
	var out []conformance.EvidenceBasis
	for _, f := range e.Findings() {
		if f.Violation.Rule().Qualified() == ruleID {
			out = append(out, f.Bases...)
		}
	}
	return out
}

// eventCode is one Go file declaring the Event root with a setter, the
// Price value object, and nothing for Order.
func eventCode() map[string]conformance.LanguageFacts {
	return map[string]conformance.LanguageFacts{
		"internal/catalog/event.go": goDecls("catalog", []conformance.Declaration{
			{Kind: "struct", Name: "Event", Exported: true, StartLine: 5, EndLine: 9},
			{Kind: "method", Name: "SetTitle", Owner: "Event", Exported: true, StartLine: 12, EndLine: 14},
			{Kind: "struct", Name: "Price", Exported: true, StartLine: 20, EndLine: 22},
		}),
	}
}

func catalogLanguage(t *testing.T) vocab.UbiquitousLanguage {
	t.Helper()
	return oneContext(t, vocab.BoundedContext{
		Name: "catalog", Definition: "d", Line: 4,
		Aggregates: []vocab.Aggregate{
			{Name: "Event", Definition: "d", Identity: "EventID", Line: 6},
			{Name: "Order", Definition: "d", Identity: "OrderID", Line: 12},
		},
		ValueObjects: []vocab.ValueObject{{Name: "Price", Definition: "d", Line: 20}},
	})
}

// Every finding is tied to an entry through what its structure shows:
// a finding anchored at an entry's recorded line goes to that entry,
// and one in a file declaring the entry goes to it through that file,
// and to the bounded context it judges. Nothing is matched from the
// message text.
func TestEvidenceAttributionTiesFindingsThroughTheirStructure(t *testing.T) {
	zones := []rule.Zone{mustZone(t, "catalog", "internal/catalog/**")}
	rules := []rule.Rule{builtInRule(t, "aggregate/root-declared"), builtInRule(t, "aggregate/commands-named-for-behavior")}
	attribution, a := attributionOver(t, rules, zones, eventCode(), catalogLanguage(t))

	event := evidenceNamed(t, attribution, vocab.ConceptAggregate, "Event")
	if event.Declarations() != conformance.DeclarationLocated || len(event.Locations()) != 1 || event.Locations()[0] != (conformance.Carrier{Path: "internal/catalog/event.go", Line: 5}) {
		t.Fatalf("Event declarations: %s %v, want located at internal/catalog/event.go:5", event.Declarations(), event.Locations())
	}
	if got := basesOf(event, "aggregate/commands-named-for-behavior"); !slices.Equal(got, []conformance.EvidenceBasis{conformance.BasisDeclaringFile}) {
		t.Errorf("setter finding on Event: bases %v, want declaring_file", got)
	}
	price := evidenceNamed(t, attribution, vocab.ConceptValueObject, "Price")
	if got := basesOf(price, "aggregate/commands-named-for-behavior"); !slices.Equal(got, []conformance.EvidenceBasis{conformance.BasisDeclaringFile}) {
		t.Errorf("a file declaring Price holds the setter finding too: bases %v, want declaring_file", got)
	}
	catalog := evidenceNamed(t, attribution, vocab.ConceptBoundedContext, "catalog")
	if got := basesOf(catalog, "aggregate/commands-named-for-behavior"); !slices.Equal(got, []conformance.EvidenceBasis{conformance.BasisBoundedContext}) {
		t.Errorf("setter finding on the context: bases %v, want bounded_context", got)
	}

	order := evidenceNamed(t, attribution, vocab.ConceptAggregate, "Order")
	if order.Declarations() != conformance.DeclarationMissing {
		t.Errorf("Order declarations: %s, want missing", order.Declarations())
	}
	if got := basesOf(order, "aggregate/root-declared"); !slices.Equal(got, []conformance.EvidenceBasis{conformance.BasisRecordedLine}) {
		t.Errorf("root-declared finding for Order: bases %v, want recorded_line", got)
	}
	if got := basesOf(event, "aggregate/root-declared"); len(got) != 0 {
		t.Errorf("Order's finding must not reach Event: %v", got)
	}
	if order.Reading() != conformance.ReadingFindings || order.ActiveFindings() != 1 {
		t.Errorf("Order reading %s with %d active, want findings with 1", order.Reading(), order.ActiveFindings())
	}
	if len(attribution.Unattributed()) != 0 {
		t.Errorf("unattributed %v, want none", attribution.Unattributed())
	}
	if len(a.Violations()) != 2 {
		t.Fatalf("the check found %d violations, want 2", len(a.Violations()))
	}
}

// A finding no entry's structure ties to stays in the attribution as
// unattributed: the project description has no entry.
func TestEvidenceAttributionAccountsForEveryFinding(t *testing.T) {
	attribution, a := attributionOver(t, []rule.Rule{wordLimitRule(t)}, nil, nil, overLimitLanguage(t))
	total := len(attribution.Unattributed())
	held := map[string]bool{}
	for _, e := range attribution.Entries() {
		for _, f := range e.Findings() {
			held[f.Violation.Fingerprint()] = true
		}
	}
	total += len(held)
	if total != len(a.Violations()) {
		t.Fatalf("attributed %d and unattributed %d of %d findings", len(held), len(attribution.Unattributed()), len(a.Violations()))
	}
	if len(attribution.Unattributed()) != 1 || attribution.Unattributed()[0].Line() != 3 {
		t.Errorf("unattributed %v, want the project description's finding at line 3", attribution.Unattributed())
	}
	pkg := evidenceNamed(t, attribution, vocab.ConceptAggregate, "Package")
	if got := basesOf(pkg, rule.WordLimitRuleID); !slices.Equal(got, []conformance.EvidenceBasis{conformance.BasisRecordedLine}) {
		t.Errorf("Package's definition finding: bases %v, want recorded_line", got)
	}
	inv := evidenceNamed(t, attribution, vocab.ConceptInvariant, "one-source")
	if inv.ActiveFindings() != 1 {
		t.Errorf("invariant one-source: %d active findings, want its own statement's", inv.ActiveFindings())
	}
}

// Evidence without an attributed finding reads as partial whenever the
// assessment could not look everywhere: a file that failed to parse,
// code ArcLint did not read, or a contract no check decided.
func TestEntryEvidencePartialNeverReadsClean(t *testing.T) {
	zones := []rule.Zone{mustZone(t, "catalog", "internal/catalog/**")}
	rules := []rule.Rule{builtInRule(t, "aggregate/root-declared")}
	clean, _ := attributionOver(t, rules, zones, eventCode(), catalogLanguage(t))
	if got := evidenceNamed(t, clean, vocab.ConceptValueObject, "Price").Reading(); got != conformance.ReadingNoAttributedFindings {
		t.Fatalf("Price over a complete assessment reads %s, want no_attributed_findings", got)
	}

	failed, _ := attributionOver(t, rules, zones, eventCode(), catalogLanguage(t), "internal/catalog/broken.go")
	for _, e := range failed.Entries() {
		if e.Reading() == conformance.ReadingNoAttributedFindings {
			t.Errorf("%s %s reads without findings while a file failed to parse", e.Entry().Concept, e.Entry().Name)
		}
	}
	if failed.Complete() || !slices.Equal(failed.ParseFailures(), []string{"internal/catalog/broken.go"}) {
		t.Errorf("complete %v, parse failures %v", failed.Complete(), failed.ParseFailures())
	}

	unread, _ := attributionOver(t, rules, []rule.Zone{mustZone(t, "catalog", "internal/catalog/**")}, map[string]conformance.LanguageFacts{}, catalogLanguage(t))
	price := evidenceNamed(t, unread, vocab.ConceptValueObject, "Price")
	if price.Declarations() != conformance.DeclarationNotObserved || price.Reading() != conformance.ReadingPartial {
		t.Errorf("Price without readable code: %s, %s; want not_observed and partial", price.Declarations(), price.Reading())
	}

	undecided, err := conformance.NewEntryEvidence(conformance.EntryEvidenceSpec{
		Entry:        conformance.EntryAddress{Concept: vocab.ConceptInvariant, Context: "catalog", Owner: "Event", Name: "one-open"},
		Declarations: conformance.DeclarationNotApplicable,
		Contracts:    []conformance.ContractEvidence{{Kind: vocab.ConceptInvariant, Owner: "Event", Key: "one-open", CodeRead: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if undecided.Reading() != conformance.ReadingPartial || undecided.Partial()[0].Kind != conformance.PartialContractUndecided {
		t.Errorf("a contract no check covered reads %s with %v, want partial for the contract", undecided.Reading(), undecided.Partial())
	}
}

// A basis the finding's structure does not show is refused: the wrong
// recorded line, a file that does not declare the entry, a context
// subject on an entry that is not the context.
func TestEntryEvidenceRefusesABasisTheFindingDoesNotShow(t *testing.T) {
	zones := []rule.Zone{mustZone(t, "catalog", "internal/catalog/**")}
	_, a := attributionOver(t, []rule.Rule{builtInRule(t, "aggregate/root-declared"), builtInRule(t, "aggregate/commands-named-for-behavior")}, zones, eventCode(), catalogLanguage(t))
	var recorded, setter conformance.Violation
	for _, v := range a.Violations() {
		switch v.Rule().Qualified() {
		case "aggregate/root-declared":
			recorded = v
		case "aggregate/commands-named-for-behavior":
			setter = v
		}
	}
	price := conformance.EntryAddress{Concept: vocab.ConceptValueObject, Context: "catalog", Name: "Price"}
	cases := map[string]conformance.EntryEvidenceSpec{
		"recorded line of another entry": {
			Entry: price, Recording: recorded.Path(), RecordedLine: 20, Declarations: conformance.DeclarationMissing,
			Findings: []conformance.AttributedFinding{{Violation: recorded, Bases: []conformance.EvidenceBasis{conformance.BasisRecordedLine}}},
		},
		"file that does not declare it": {
			Entry: price, Declarations: conformance.DeclarationLocated, Locations: []conformance.Carrier{{Path: "internal/catalog/price.go", Line: 3}},
			Findings: []conformance.AttributedFinding{{Violation: setter, Bases: []conformance.EvidenceBasis{conformance.BasisDeclaringFile}}},
		},
		"context subject on a value object": {
			Entry: price, Declarations: conformance.DeclarationMissing,
			Findings: []conformance.AttributedFinding{{Violation: setter, Bases: []conformance.EvidenceBasis{conformance.BasisBoundedContext}}},
		},
		"contract it does not record": {
			Entry: price, Declarations: conformance.DeclarationMissing,
			Findings: []conformance.AttributedFinding{{Violation: setter, Bases: []conformance.EvidenceBasis{conformance.BasisContract}}},
		},
		"no basis": {
			Entry: price, Declarations: conformance.DeclarationMissing,
			Findings: []conformance.AttributedFinding{{Violation: setter}},
		},
		"located without a location": {Entry: price, Declarations: conformance.DeclarationLocated},
	}
	for name, spec := range cases {
		if _, err := conformance.NewEntryEvidence(spec); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// A contract whose coverage reads violates because one check found a
// violation, while another of its checks was not performed, is not
// decided: the evidence reads partial, never free of findings, when the
// violation itself is baselined.
func TestEntryEvidencePartialWhenAContractCheckWasNotPerformed(t *testing.T) {
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "catalog", Definition: "d", Line: 4,
		Aggregates: []vocab.Aggregate{{Name: "Event", Definition: "d", Identity: "EventID", Line: 6,
			Invariants: []vocab.Invariant{{Key: "one-open", Statement: "s", Line: 8}}}},
	})
	zones := []rule.Zone{mustZone(t, "catalog", "internal/catalog/**")}
	_, a := attributionOver(t, []rule.Rule{builtInRule(t, "aggregate/invariants-enforced-by-root")}, zones, eventCode(), knowledge)
	var check conformance.Evaluation
	for _, e := range a.Evaluations() {
		if e.Rule().Qualified() == "aggregate/invariants-enforced-by-root" && e.Outcome() == conformance.OutcomeViolates {
			check = e
		}
	}
	if check.Rule().IsZero() {
		t.Fatal("the root without EnsureOneOpen did not violate invariants-enforced-by-root")
	}
	coverage, err := conformance.NewContractCoverage(conformance.ContractCoverageSpec{
		Context: "catalog", Owner: "Event", Key: "one-open", Statement: "s", Kind: vocab.ConceptInvariant,
		Path: "docs/model.arclint.yaml", Line: 8, Checks: []conformance.Evaluation{check},
		Unperformed: []string{"invariant/enforced-at-every-mutation: the rule is disabled"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if coverage.Status() != string(conformance.OutcomeViolates) {
		t.Fatalf("coverage status %s, want violates", coverage.Status())
	}
	e, err := conformance.NewEntryEvidence(conformance.EntryEvidenceSpec{
		Entry:        conformance.EntryAddress{Concept: vocab.ConceptInvariant, Context: "catalog", Owner: "Event", Name: "one-open"},
		Declarations: conformance.DeclarationNotApplicable,
		Contracts:    []conformance.ContractEvidence{{Kind: vocab.ConceptInvariant, Owner: "Event", Key: "one-open", CodeRead: true, Coverage: coverage, Covered: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if e.Reading() != conformance.ReadingPartial {
		t.Fatalf("reading %s with partial %v, want partial", e.Reading(), e.Partial())
	}
	if p := e.Partial(); len(p) != 1 || p[0].Kind != conformance.PartialContractUndecided || p[0].Outcome != "incomplete" {
		t.Errorf("partial %v, want the contract undecided as incomplete", p)
	}
}
