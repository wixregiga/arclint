package conformance_test

import (
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
	facts := goDecls("catalog", []conformance.Declaration{
		{Kind: "struct", Name: "Selection", StartLine: 1},
		{Kind: "method", Owner: "Selection", Name: "Select", StartLine: 2},
		{Kind: "method", Owner: "Selection", Name: "AssertAccompanyingIncluded", StartLine: 3},
		{Kind: "struct", Name: "Removal", StartLine: 5},
		{Kind: "method", Owner: "Removal", Name: "Remove", StartLine: 6},
	}, conformance.Call{Callee: "AssertAccompanyingIncluded", Enclosing: "Select", Line: 2})
	a := runOne(t, r, nil, observed("catalog.go"), map[string]conformance.LanguageFacts{"catalog.go": facts}, knowledge)
	byOwner := map[string]conformance.ContractCoverage{}
	for _, c := range a.Contracts() {
		byOwner[c.Owner()] = c
	}
	if byOwner["Selection"].Status() != "structurally_checked" || byOwner["Removal"].Status() != "violates" {
		t.Fatalf("coverage borrowed another owner's result: %#v", byOwner)
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
	decls := goDecls("catalog", []conformance.Declaration{{Kind: "struct", Name: "Selection", StartLine: 1}})
	noCalls := decls
	noCalls.CallsAvailable = false
	for _, tc := range []struct {
		name  string
		rules []rule.Rule
		facts conformance.LanguageFacts
		want  string
	}{
		{"unselected", nil, decls, "unchecked"},
		{"disabled", []rule.Rule{r.Disable(d)}, decls, "unchecked"},
		{"calls unavailable", []rule.Rule{r}, noCalls, "unsupported"},
		{"missing owner", []rule.Rule{r}, goDecls("catalog", nil), "undetermined"},
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
