package rule_test

import (
	"reflect"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

func TestRuleConstructionWithConcreteConstraints(t *testing.T) {
	snake, err := rule.NewCaseSpec("snake_case")
	if err != nil {
		t.Fatal(err)
	}
	glob, err := rule.NewGlob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		constraint    rule.Constraint
		id            string
		applicability rule.Applicability
	}{
		{rule.ConsumesConstraint{Internal: emptyAllowList(t)}, "test/imports", mustZoneApplicability(t, "domain")},
		{rule.StructureConstraint{Require: []rule.Glob{glob}}, "test/structure", mustZoneApplicability(t, "domain")},
		{rule.NamingConstraint{Case: snake}, "test/naming", mustZoneApplicability(t, "domain")},
		{rule.LayersConstraint{Layers: []rule.ZoneName{"app", "domain"}}, "test/layers", mustRepoApplicability(t)},
		{rule.ProtectedConstraint{Zone: "domain"}, "test/protected", mustRepoApplicability(t)},
		{rule.IndependenceConstraint{Folders: []rule.Glob{glob}}, "test/independent", mustRepoApplicability(t)},
		{rule.AcyclicConstraint{}, "test/acyclic", mustRepoApplicability(t)},
		{rule.DomainConstraint{Invariant: "assertion/checked-by-its-operation"}, "assertion/checked-by-its-operation", mustRepoApplicability(t)},
		{rule.ContentConstraint{Forbid: "TODO"}, "test/content", mustRepoApplicability(t)},
		{rule.ExtensionConstraint{Uses: "test/check"}, "test/extension", mustRepoApplicability(t)},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			c := tc.constraint
			r, err := rule.New(rule.Spec{ID: tc.id, Constraint: c, Applicability: tc.applicability})
			if err != nil {
				t.Fatal(err)
			}
			if err := r.Validate(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(r.Constraint(), c) || !reflect.DeepEqual(r.Params(), c) || r.Type() != c.Kind() {
				t.Fatalf("constraint was not preserved: %+v", r)
			}
			if c.Proposition() == "" {
				t.Fatal("missing configured proposition")
			}
			legacy, err := rule.New(rule.Spec{ID: tc.id, Type: c.Kind(), Params: c, Applicability: tc.applicability})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(r, legacy) {
				t.Fatal("legacy and constraint construction differ")
			}
		})
	}
}

func TestConstraintConstructionRejectsInvalidValues(t *testing.T) {
	for _, c := range []rule.Constraint{
		rule.ConsumesConstraint{}, rule.StructureConstraint{}, rule.NamingConstraint{},
		rule.LayersConstraint{}, rule.ProtectedConstraint{}, rule.IndependenceConstraint{},
		rule.AcyclicConstraint{Zones: []rule.ZoneName{"domain", "domain"}},
		rule.DomainConstraint{Invariant: "unknown"}, rule.ContentConstraint{Forbid: "("},
		rule.ExtensionConstraint{},
	} {
		t.Run(string(c.Kind()), func(t *testing.T) {
			if err := c.Validate(); err == nil {
				t.Fatal("invalid constraint accepted")
			}
			if _, err := rule.New(rule.Spec{ID: "test/invalid", Constraint: c, Applicability: mustRepoApplicability(t)}); err == nil {
				t.Fatal("invalid constraint became a Rule")
			}
		})
	}
	c := rule.ContentConstraint{Forbid: "TODO"}
	for _, spec := range []rule.Spec{
		{ID: "test/missing", Applicability: mustRepoApplicability(t)},
		{ID: "test/mixed-type", Constraint: c, Type: rule.TypeContent, Applicability: mustRepoApplicability(t)},
		{ID: "test/mixed-params", Constraint: c, Params: c, Applicability: mustRepoApplicability(t)},
		{ID: "test/scope", Constraint: rule.ConsumesConstraint{Internal: emptyAllowList(t)}, Applicability: mustRepoApplicability(t)},
	} {
		if _, err := rule.New(spec); err == nil {
			t.Errorf("accepted invalid spec %s", spec.ID)
		}
	}
}
