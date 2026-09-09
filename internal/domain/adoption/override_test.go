package adoption_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/adoption"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

func overrideTarget(t *testing.T, provenance bool) rule.Rule {
	t.Helper()
	ref, err := rule.ParsePatternReference("acme/checks@1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	scope, err := rule.RepositoryScope()
	if err != nil {
		t.Fatal(err)
	}
	spec := rule.Spec{ID: "acme/checks:no-cycles", Constraint: rule.AcyclicConstraint{}, Scope: scope}
	if provenance {
		spec.Provenance = &ref
	}
	r, err := rule.New(spec)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestOverrideAppliesAdoptionAndPreservesRule(t *testing.T) {
	original := overrideTarget(t, true)
	paths, err := rule.NewGlobs([]string{"generated/**"})
	if err != nil {
		t.Fatal(err)
	}
	exclusion, err := rule.NewExclusion(paths, nil, "generated")
	if err != nil {
		t.Fatal(err)
	}
	suppression, err := rule.NewSuppression(paths, "debt")
	if err != nil {
		t.Fatal(err)
	}
	disablement, err := rule.NewDisablement("migration")
	if err != nil {
		t.Fatal(err)
	}
	override, err := adoption.NewOverride(original.ID(), rule.SeverityWarning, &exclusion, &suppression, &disablement)
	if err != nil {
		t.Fatal(err)
	}
	// Caller-owned inputs cannot change the constructed decision.
	exclusion = rule.Exclusion{}
	suppression = rule.Suppression{}
	disablement = rule.Disablement{}
	adopted, err := override.Apply(original)
	if err != nil {
		t.Fatal(err)
	}
	if !override.Target().Equals(original.ID()) || !adopted.ID().Equals(original.ID()) {
		t.Fatal("override changed the Rule identity")
	}
	if !reflect.DeepEqual(adopted.Constraint(), original.Constraint()) || adopted.Claim() != original.Claim() {
		t.Fatal("override changed the Rule proposition")
	}
	originalRef, _ := original.Provenance()
	adoptedRef, ok := adopted.Provenance()
	if !ok || adoptedRef != originalRef {
		t.Fatal("override lost provenance")
	}
	if adopted.Severity() != rule.SeverityWarning || !adopted.Disabled() {
		t.Fatal("severity or disablement not applied")
	}
	if adopted.AppliesToFile("generated/a.go", nil) {
		t.Fatal("exclusion not applied")
	}
	if reason, ok := adopted.SuppressionFor("generated/a.go"); !ok || reason != "debt" {
		t.Fatal("suppression not applied")
	}
	if !original.AppliesToFile("generated/a.go", nil) || original.Disabled() || len(original.Suppressions()) != 0 || original.Severity() != rule.SeverityError {
		t.Fatal("override mutated the original Rule")
	}
}

func TestOverrideRejectsInvalidValues(t *testing.T) {
	target := overrideTarget(t, true).ID()
	local, err := rule.NewID("local/no-cycles")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		target      rule.ID
		severity    rule.Severity
		exclusion   *rule.Exclusion
		suppression *rule.Suppression
		disablement *rule.Disablement
		want        string
	}{
		{name: "missing target", severity: rule.SeverityWarning, want: "missing target"},
		{name: "local target", target: local, severity: rule.SeverityWarning, want: "neither"},
		{name: "no decision", target: target, want: "changes something"},
		{name: "invalid severity", target: target, severity: "fatal", want: "severity"},
		{name: "invalid exclusion", target: target, exclusion: &rule.Exclusion{}, want: "exclusion"},
		{name: "invalid suppression", target: target, suppression: &rule.Suppression{}, want: "suppression"},
		{name: "invalid disablement", target: target, disablement: &rule.Disablement{}, want: "disablement"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := adoption.NewOverride(tc.target, tc.severity, tc.exclusion, tc.suppression, tc.disablement)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
}

func TestOverrideProtectsItsTarget(t *testing.T) {
	target := overrideTarget(t, true)
	override, err := adoption.NewOverride(target.ID(), rule.SeverityInfo, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	builtins, err := rule.BuiltIn()
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []rule.Rule{rule.Rule{}, builtins[0], overrideTarget(t, false)} {
		if _, err := override.Apply(candidate); err == nil {
			t.Fatalf("accepted ineligible target %s", candidate.ID())
		}
	}
	if _, err := (adoption.Override{}).Apply(target); err == nil {
		t.Fatal("accepted zero Override")
	}
}

func TestOverrideAdoptsBuiltInAndKeepsUnspecifiedSeverity(t *testing.T) {
	builtins, err := rule.BuiltIn()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range builtins {
		disablement, err := rule.NewDisablement("migration")
		if err != nil {
			t.Fatal(err)
		}
		override, err := adoption.NewOverride(target.ID(), "", nil, nil, &disablement)
		if err != nil {
			t.Fatal(err)
		}
		adopted, err := override.Apply(target)
		if err != nil {
			t.Fatalf("%s: %v", target.ID(), err)
		}
		if !adopted.BuiltIn() || !adopted.Disabled() || adopted.Severity() != target.Severity() || adopted.Claim() != target.Claim() || !reflect.DeepEqual(adopted.Constraint(), target.Constraint()) {
			t.Fatalf("%s: built-in changed beyond requested adoption", target.ID())
		}
	}
}
