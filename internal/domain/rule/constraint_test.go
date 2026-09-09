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
		constraint rule.Constraint
		id         string
		scope      rule.Scope
	}{
		{rule.ConsumesConstraint{Internal: emptyAllowList(t)}, "test/imports", mustZoneScope(t, "domain")},
		{rule.StructureConstraint{Require: []rule.Glob{glob}}, "test/structure", mustZoneScope(t, "domain")},
		{rule.NamingConstraint{Case: snake}, "test/naming", mustZoneScope(t, "domain")},
		{rule.LayersConstraint{Layers: []rule.ZoneName{"app", "domain"}}, "test/layers", mustRepoScope(t)},
		{rule.ProtectedConstraint{Zone: "domain"}, "test/protected", mustRepoScope(t)},
		{rule.IndependenceConstraint{Folders: []rule.Glob{glob}}, "test/independent", mustRepoScope(t)},
		{rule.AcyclicConstraint{}, "test/acyclic", mustRepoScope(t)},
		{rule.DomainConstraint{Invariant: "assertion/checked-by-its-operation"}, "assertion/checked-by-its-operation", mustRepoScope(t)},
		{rule.ContentConstraint{Forbid: "TODO"}, "test/content", mustRepoScope(t)},
		{rule.ExtensionConstraint{Uses: "test/check"}, "test/extension", mustRepoScope(t)},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			c := tc.constraint
			r, err := rule.New(rule.Spec{ID: tc.id, Constraint: c, Scope: tc.scope})
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
			legacy, err := rule.New(rule.Spec{ID: tc.id, Type: c.Kind(), Params: c, Scope: tc.scope})
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
			if _, err := rule.New(rule.Spec{ID: "test/invalid", Constraint: c, Scope: mustRepoScope(t)}); err == nil {
				t.Fatal("invalid constraint became a Rule")
			}
		})
	}
	c := rule.ContentConstraint{Forbid: "TODO"}
	for _, spec := range []rule.Spec{
		{ID: "test/missing", Scope: mustRepoScope(t)},
		{ID: "test/mixed-type", Constraint: c, Type: rule.TypeContent, Scope: mustRepoScope(t)},
		{ID: "test/mixed-params", Constraint: c, Params: c, Scope: mustRepoScope(t)},
		{ID: "test/scope", Constraint: rule.ConsumesConstraint{Internal: emptyAllowList(t)}, Scope: mustRepoScope(t)},
	} {
		if _, err := rule.New(spec); err == nil {
			t.Errorf("accepted invalid spec %s", spec.ID)
		}
	}
}
