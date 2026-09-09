package rule_test

import (
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

func TestConstraintScopeCompatibilityAtConstruction(t *testing.T) {
	snake, err := rule.NewCaseSpec("snake_case")
	if err != nil {
		t.Fatal(err)
	}
	glob := mustGlob(t, "**/*.go")
	zones := mustZoneScope(t, "domain")
	repository := mustRepoScope(t)
	zonesWithFiles, err := rule.ZoneScope(zones.Zones(), glob)
	if err != nil {
		t.Fatal(err)
	}
	repositoryWithFiles, err := rule.RepositoryScope(glob)
	if err != nil {
		t.Fatal(err)
	}
	scopes := []struct {
		name  string
		value rule.Scope
	}{
		{"zero", rule.Scope{}}, {"zones", zones}, {"zones with files", zonesWithFiles},
		{"repository", repository}, {"repository with files", repositoryWithFiles},
	}
	for _, tc := range []struct {
		constraint rule.Constraint
		id         string
		accepts    [5]bool
	}{
		{rule.ConsumesConstraint{Internal: emptyAllowList(t)}, "imports", [5]bool{false, true, false, false, false}},
		{rule.StructureConstraint{Require: []rule.Glob{glob}}, "structure", [5]bool{false, true, false, false, false}},
		{rule.NamingConstraint{Case: snake}, "naming", [5]bool{false, true, true, false, false}},
		{rule.LayersConstraint{Layers: []rule.ZoneName{"app", "domain"}}, "layers", [5]bool{false, false, false, true, false}},
		{rule.ProtectedConstraint{Zone: "domain"}, "protected", [5]bool{false, false, false, true, false}},
		{rule.IndependenceConstraint{Folders: []rule.Glob{glob}}, "independent", [5]bool{false, false, false, true, false}},
		{rule.AcyclicConstraint{}, "acyclic", [5]bool{false, false, false, true, false}},
		{rule.DomainConstraint{Invariant: "assertion/checked-by-its-operation"}, "assertion/checked-by-its-operation", [5]bool{false, false, false, true, false}},
		{rule.ContentConstraint{Forbid: "TODO"}, "content", [5]bool{false, true, true, true, true}},
		{rule.ExtensionConstraint{Uses: "probe"}, "extension", [5]bool{false, true, true, true, true}},
	} {
		for i, scope := range scopes {
			t.Run(tc.id+"/"+scope.name, func(t *testing.T) {
				want := tc.accepts[i]
				if got := tc.constraint.AcceptsScope(scope.value); got != want {
					t.Fatalf("AcceptsScope = %v, want %v", got, want)
				}
				r, err := rule.New(rule.Spec{ID: tc.id, Constraint: tc.constraint, Scope: scope.value})
				if (err == nil) != want {
					t.Fatalf("construction error = %v, want accepted %v", err, want)
				}
				if !want {
					return
				}
				if err := r.Validate(); err != nil {
					t.Fatal(err)
				}
				exclusion, err := rule.NewExclusion([]rule.Glob{glob}, nil, "adopt existing code")
				if err != nil {
					t.Fatal(err)
				}
				narrowed, err := r.Exclude(exclusion)
				if err != nil || narrowed.Validate() != nil {
					t.Fatalf("exclusion invalidated compatible scope: %v", err)
				}
				if len(r.Scope().Exclusions()) != 0 {
					t.Fatal("exclusion changed the original Rule")
				}
			})
		}
	}
}

func TestZeroRuleCannotPassScopeInvariant(t *testing.T) {
	if err := (rule.Rule{}).EnsureConstraintAcceptsScope(); err == nil {
		t.Fatal("zero Rule passed compatibility invariant")
	}
}
