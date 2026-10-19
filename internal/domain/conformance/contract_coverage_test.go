package conformance_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

func TestContractCoverageSeparatesOwnersAndReportsLimits(t *testing.T) {
	ctx := vocab.BoundedContext{Name: "catalog", Definition: "d", Services: []vocab.DomainService{
		{Name: "Selection", Definition: "d", Assertions: []vocab.Assertion{{Key: "accompanying-included", On: "Select", Statement: "Accompanying commands are included.", Line: 8}}},
		{Name: "Removal", Definition: "d", Assertions: []vocab.Assertion{{Key: "complete", On: "Remove", Statement: "Every selected item is removed.", Line: 12}}},
	}}
	knowledge := oneContext(t, ctx)
	r := builtInRule(t, "assertion/checked-by-its-operation")
	facts := resolvedGo("catalog", []conformance.Declaration{
		{Kind: "struct", Name: "Selection", StartLine: 1},
		{Kind: "method", Owner: "Selection", Name: "Select", StartLine: 2},
		{Kind: "method", Owner: "Selection", Name: "AssertAccompanyingIncluded", StartLine: 3},
		{Kind: "struct", Name: "Removal", StartLine: 5},
		{Kind: "method", Owner: "Removal", Name: "Remove", StartLine: 6},
	}, conformance.Call{Callee: "AssertAccompanyingIncluded", Enclosing: "Select", EnclosingOwner: "Selection", Line: 2, Target: declared("catalog.go", "Selection", "AssertAccompanyingIncluded")})
	a := runOne(t, r, nil, observed("catalog.go"), map[string]conformance.LanguageFacts{"catalog.go": facts}, knowledge)
	byOwner := map[string]conformance.ContractCoverage{}
	for _, c := range a.Contracts() {
		byOwner[c.Owner()] = c
	}
	if byOwner["Selection"].Status() != "structurally_checked" || byOwner["Removal"].Status() != "violates" {
		t.Fatalf("coverage borrowed another owner's result: %#v", byOwner)
	}
	for owner, c := range byOwner {
		if c.Path() != testRecording {
			t.Errorf("contract %s is anchored in %q, want the project's domain file %q", owner, c.Path(), testRecording)
		}
	}
	checks := byOwner["Selection"].Checks()
	if len(checks) != 1 || !strings.Contains(strings.Join(checks[0].Limitations(), " "), "check bodies") {
		t.Fatal("check-body limitation lost")
	}
	// Defensive copies cannot rewrite a reported check.
	checks[0] = conformance.Evaluation{}
	if byOwner["Selection"].Checks()[0].Rule().IsZero() {
		t.Fatal("mutable coverage checks")
	}
	labelCalls := 0
	relabeled, err := a.RelabelViolations(func(conformance.Violation) (conformance.Status, string, bool) {
		labelCalls++
		return conformance.StatusBaselined, "adopted", true
	})
	if err != nil {
		t.Fatal(err)
	}
	if labelCalls != len(a.Violations()) {
		t.Fatal("coverage consumed baseline occurrences twice")
	}
	for _, c := range relabeled.Contracts() {
		for _, e := range c.Checks() {
			for _, v := range e.Violations() {
				if v.Status() != conformance.StatusBaselined {
					t.Fatal("coverage did not retain baseline status")
				}
			}
		}
	}
}

func TestContractCoverageReportsUnperformedAndUnavailableChecks(t *testing.T) {
	ctx := vocab.BoundedContext{Name: "catalog", Definition: "Content selection.", Line: 4,
		Services: []vocab.DomainService{{Name: "Selection", Definition: "Select includes each item's accompanying commands.", Line: 8,
			Assertions: []vocab.Assertion{{Key: "accompanying-included", On: "Select", Statement: "Accompanying commands are included.", Line: 9}}}}}
	knowledge := oneContext(t, ctx)
	r := builtInRule(t, "assertion/checked-by-its-operation")
	d, err := rule.NewDisablement("not adopted")
	if err != nil {
		t.Fatal(err)
	}
	decls := resolvedGo("catalog", []conformance.Declaration{{Kind: "struct", Name: "Selection", StartLine: 1}})
	noCalls := decls
	noCalls.CallsAvailable, noCalls.ResolvedCallsAvailable = false, false
	for _, tc := range []struct {
		name  string
		rules []rule.Rule
		facts conformance.LanguageFacts
		want  string
	}{
		{"unselected", nil, decls, "unchecked"},
		{"disabled", []rule.Rule{r.Disable(d)}, decls, "unchecked"},
		{"calls unavailable", []rule.Rule{r}, noCalls, "unsupported"},
		{"missing owner", []rule.Rule{r}, resolvedGo("catalog", nil), "undetermined"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obs, err := conformance.NewObservations(observed("catalog.go"), map[string]conformance.LanguageFacts{"catalog.go": tc.facts})
			if err != nil {
				t.Fatal(err)
			}
			a, err := conformance.Run(conformance.Request{Rules: tc.rules, Knowledge: knowledge, Observations: obs})
			if err != nil {
				t.Fatal(err)
			}
			cs := a.Contracts()
			if len(cs) != 1 || cs[0].Status() != tc.want {
				t.Fatalf("coverage = %#v, want %s", cs, tc.want)
			}
			if tc.want == "unchecked" && len(cs[0].Unperformed()) == 0 {
				t.Fatal("missing reason")
			}
			if tc.name == "calls unavailable" && len(a.Violations()) > 0 {
				t.Fatal("unavailable calls interpreted as missing calls")
			}
		})
	}
}

// A Rule that excludes the project's domain file judges nothing recorded
// in it, so the coverage of that Rule's contract is not applicable. The
// file excluded is the DomainRecording, not a file named by default.
func TestContractCoverageHonorsAnExclusionOfTheRecordingFile(t *testing.T) {
	ctx := vocab.BoundedContext{Name: "catalog", Definition: "d", Services: []vocab.DomainService{
		{Name: "Selection", Definition: "d", Assertions: []vocab.Assertion{{Key: "accompanying-included", On: "Select", Statement: "Accompanying commands are included.", Line: 8}}},
	}}
	knowledge := oneContext(t, ctx)
	facts := resolvedGo("catalog", []conformance.Declaration{
		{Kind: "struct", Name: "Selection", StartLine: 1},
		{Kind: "method", Owner: "Selection", Name: "Select", StartLine: 2},
	}, conformance.Call{Callee: "Other", Enclosing: "Select", EnclosingOwner: "Selection", Line: 2, Target: declared("catalog.go", "", "Other")})
	for _, tc := range []struct {
		exclude string
		want    string
	}{
		{testRecording, "not_applicable"},
		{vocab.DefaultDomainRecordingPath, "violates"},
	} {
		glob, err := rule.NewGlob(tc.exclude)
		if err != nil {
			t.Fatal(err)
		}
		exclusion, err := rule.NewExclusion([]rule.Glob{glob}, nil, "the recorded file is not judged")
		if err != nil {
			t.Fatal(err)
		}
		r, err := builtInRule(t, "assertion/checked-by-its-operation").Exclude(exclusion)
		if err != nil {
			t.Fatal(err)
		}
		a := runOne(t, r, nil, observed("catalog.go"), map[string]conformance.LanguageFacts{"catalog.go": facts}, knowledge)
		cs := a.Contracts()
		if len(cs) != 1 || cs[0].Status() != tc.want {
			t.Errorf("excluding %s: coverage = %#v, want %s", tc.exclude, cs, tc.want)
		}
	}
}

// A check that locates the declaration carrying a contract hands it to
// the coverage, so a reader learns where the code enforces the contract
// beside how the check judged it; a contract nothing carries has none.
func TestContractCoverageCarriesTheDeclarationACheckLocated(t *testing.T) {
	ctx := vocab.BoundedContext{Name: "catalog", Definition: "d",
		Aggregates: []vocab.Aggregate{{Name: "Product", Definition: "d", Identity: "ProductID",
			Invariants: []vocab.Invariant{
				{Key: "price-set", Statement: "A Product always has a price.", Line: 5},
				{Key: "name-set", Statement: "A Product always has a name.", Line: 6},
			}}},
		ValueObjects: []vocab.ValueObject{{Name: "Sku", Definition: "d",
			Invariants: []vocab.Invariant{{Key: "well-formed", Statement: "A Sku is never empty.", Line: 9}}}},
		Services: []vocab.DomainService{{Name: "Reservation", Definition: "d",
			Assertions: []vocab.Assertion{{Key: "in-stock", On: "Reserve", Statement: "Only stocked Products are reserved.", Line: 12}}}},
	}
	knowledge := oneContext(t, ctx)
	facts := resolvedGo("catalog", []conformance.Declaration{
		{Kind: "struct", Name: "Product", StartLine: 1},
		{Kind: "method", Owner: "Product", Name: "EnsurePriceSet", StartLine: 3},
		{Kind: "struct", Name: "Sku", StartLine: 10},
		{Kind: "func", Name: "NewSku", StartLine: 11, Results: []string{"Sku", "error"}},
		{Kind: "struct", Name: "Reservation", StartLine: 20},
		{Kind: "method", Owner: "Reservation", Name: "Reserve", StartLine: 21},
		{Kind: "method", Owner: "Reservation", Name: "AssertInStock", StartLine: 24},
	}, conformance.Call{Callee: "AssertInStock", Enclosing: "Reserve", EnclosingOwner: "Reservation", Line: 22, Target: declared("catalog.go", "Reservation", "AssertInStock")})
	obs, err := conformance.NewObservations(observed("catalog.go"), map[string]conformance.LanguageFacts{"catalog.go": facts})
	if err != nil {
		t.Fatal(err)
	}
	var rules []rule.Rule
	for _, id := range conformance.ContractRuleIDs() {
		rules = append(rules, builtInRule(t, id))
	}
	a, err := conformance.Run(conformance.Request{Rules: rules, Knowledge: knowledge, Observations: obs})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"price-set": "catalog.go:3", "name-set": "", "well-formed": "catalog.go:11", "in-stock": "catalog.go:24"}
	for _, c := range a.Contracts() {
		got := ""
		if at, located := c.Carrier(); located {
			got = fmt.Sprintf("%s:%d", at.Path, at.Line)
		}
		if got != want[c.Key()] {
			t.Errorf("%s of %s: carrier %q, want %q", c.Key(), c.Owner(), got, want[c.Key()])
		}
		delete(want, c.Key())
	}
	if len(want) > 0 {
		t.Errorf("contracts without coverage: %v", want)
	}
}

// A contract whose owner no declaration spells is undetermined, and the
// check says so rather than leaving the reader to guess.
func TestContractCoverageSaysWhyItIsUndetermined(t *testing.T) {
	ctx := vocab.BoundedContext{Name: "catalog", Definition: "d", Services: []vocab.DomainService{
		{Name: "Reservation", Definition: "d", Assertions: []vocab.Assertion{{Key: "in-stock", On: "Reserve", Statement: "Only stocked Products are reserved.", Line: 8}}},
	}}
	a := runOne(t, builtInRule(t, "assertion/checked-by-its-operation"), nil, observed("catalog.go"),
		map[string]conformance.LanguageFacts{"catalog.go": resolvedGo("catalog", nil)}, oneContext(t, ctx))
	cs := a.Contracts()
	if len(cs) != 1 || cs[0].Status() != "undetermined" {
		t.Fatalf("coverage = %#v", cs)
	}
	if _, located := cs[0].Carrier(); located {
		t.Error("an undeclared owner carries nothing")
	}
	reasons := cs[0].Checks()[0].Unresolved()
	if len(reasons) != 1 || reasons[0] != "no single declaration of Reservation is located in the repository" {
		t.Errorf("reasons = %q", reasons)
	}
}

func TestContractCoverageRefusesACarrierWithoutALocation(t *testing.T) {
	for _, carrier := range []conformance.Carrier{{Path: "", Line: 3}, {Path: "catalog.go", Line: 0}} {
		_, err := conformance.NewContractCoverage(conformance.ContractCoverageSpec{
			Context: "catalog", Owner: "Sku", Key: "well-formed", Statement: "A Sku is never empty.",
			Kind: vocab.ConceptInvariant, Path: testRecording, Line: 4, Carrier: &carrier,
		})
		if err == nil {
			t.Errorf("carrier %+v accepted", carrier)
		}
	}
}

// An Exclusion that removes every analyzed file of the context's code
// leaves the contract not applicable to the Rule; the language of the
// files that remain does not make it unsupported.
func TestContractCoverageIsNotApplicableWhenAnExclusionRemovesTheCode(t *testing.T) {
	ctx := vocab.BoundedContext{Name: "catalog", Definition: "d", Services: []vocab.DomainService{
		{Name: "Selection", Definition: "d", Assertions: []vocab.Assertion{{Key: "accompanying-included", On: "Select", Statement: "Accompanying commands are included.", Line: 8}}},
	}}
	facts := resolvedGo("catalog", []conformance.Declaration{
		{Kind: "struct", Name: "Selection", StartLine: 1},
		{Kind: "method", Owner: "Selection", Name: "Select", StartLine: 2},
	})
	glob, err := rule.NewGlob("catalog/catalog.go")
	if err != nil {
		t.Fatal(err)
	}
	exclusion, err := rule.NewExclusion([]rule.Glob{glob}, nil, "generated")
	if err != nil {
		t.Fatal(err)
	}
	r, err := builtInRule(t, "assertion/checked-by-its-operation").Exclude(exclusion)
	if err != nil {
		t.Fatal(err)
	}
	zones := []rule.Zone{mustZone(t, "catalog", "catalog/**")}
	a := runOne(t, r, zones, observed("catalog/catalog.go", "catalog/README.md"),
		map[string]conformance.LanguageFacts{"catalog/catalog.go": facts}, oneContext(t, ctx))
	if cs := a.Contracts(); len(cs) != 1 || cs[0].Status() != "not_applicable" {
		t.Errorf("coverage = %#v, want not_applicable", cs)
	}
}
