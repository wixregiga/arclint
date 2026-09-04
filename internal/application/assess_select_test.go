package application_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// selectionFixture extends the shared fixture with two Rules the
// Pattern t/shared distributes, so selection has something to narrow:
// the local naming rule t/p:m/snake plus the structure rules
// t/shared:m/keep and t/shared:m/keep2 requiring m/ok.go.
func selectionFixture(t *testing.T) (rule.Configured, conformance.Observations) {
	t.Helper()
	cfg, obs := fixture(t, "m/ok.go")
	keep, err := rule.NewGlob("m/ok.go")
	if err != nil {
		t.Fatalf("NewGlob: %v", err)
	}
	scope, err := rule.ModuleApplicability([]rule.ModuleName{"m"})
	if err != nil {
		t.Fatalf("ModuleApplicability: %v", err)
	}
	shared, err := rule.ParsePatternReference("t/shared@1.2.0")
	if err != nil {
		t.Fatalf("ParsePatternReference: %v", err)
	}
	for _, id := range []string{"t/shared:m/keep", "t/shared:m/keep2"} {
		r, err := rule.New(rule.Spec{
			ID:            id,
			Type:          rule.TypeStructure,
			Params:        rule.StructureParams{Require: []rule.Glob{keep}},
			Applicability: scope,
			Provenance:    &shared,
		})
		if err != nil {
			t.Fatalf("rule.New: %v", err)
		}
		cfg.Rules = append(cfg.Rules, r)
	}
	return cfg, obs
}

func TestAssessConformanceSelectsRules(t *testing.T) {
	cfg, obs := selectionFixture(t)

	cases := []struct {
		name    string
		only    []string
		exclude []string
		applied []string
	}{
		// t/shared:m/keep is a prefix of t/shared:m/keep2, so this also proves an
		// exact id wins completely over prefix expansion.
		{"only exact id", []string{"t/shared:m/keep"}, nil, []string{"t/shared:m/keep"}},
		{"only prefix", []string{"t/shared:m/"}, nil, []string{"t/shared:m/keep", "t/shared:m/keep2"}},
		{"only namespace prefix", []string{"t/"}, nil, []string{"t/p:m/snake", "t/shared:m/keep", "t/shared:m/keep2"}},
		{"only pattern", []string{"t/shared:m/*"}, nil, []string{"t/shared:m/keep", "t/shared:m/keep2"}},
		{"exclude one", nil, []string{"t/p:m/snake"}, []string{"t/shared:m/keep", "t/shared:m/keep2"}},
		{"only prefix minus exact exclude", []string{"t/"}, []string{"t/shared:m/keep"}, []string{"t/p:m/snake", "t/shared:m/keep2"}},
		// Provenance spellings select what one Pattern distributed,
		// exactly or at whatever version is extended.
		{"only exact provenance", []string{"t/shared@1.2.0"}, nil, []string{"t/shared:m/keep", "t/shared:m/keep2"}},
		{"only provenance name", []string{"t/shared"}, nil, []string{"t/shared:m/keep", "t/shared:m/keep2"}},
		{"exclude provenance", nil, []string{"t/shared"}, []string{"t/p:m/snake"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc := newAssess(t, cfg, obs, &fakeBaselineSource{})
			a, err := uc.Execute(application.AssessConformanceRequest{
				SkipBaseline: true, Only: tc.only, Exclude: tc.exclude,
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			got := a.AppliedRules()
			if len(got) != len(tc.applied) {
				t.Fatalf("applied = %v, want %v", got, tc.applied)
			}
			for i, id := range tc.applied {
				if got[i] != id {
					t.Errorf("applied = %v, want %v", got, tc.applied)
				}
			}
		})
	}
}

func TestListRulesSelect(t *testing.T) {
	cfg, _ := selectionFixture(t)
	list, err := application.NewListRules(fakeRepository{cfg})
	if err != nil {
		t.Fatalf("NewListRules: %v", err)
	}
	exact, err := list.Select("t/shared:m/keep")
	if err != nil || len(exact) != 1 || exact[0].ID != "t/shared:m/keep" {
		t.Errorf("exact select = %v, %v", exact, err)
	}
	prefix, err := list.Select("t/")
	if err != nil || len(prefix) != 3 {
		t.Errorf("prefix select = %v, %v", prefix, err)
	}
	pattern, err := list.Select("t/p:m/s*")
	if err != nil || len(pattern) != 1 || pattern[0].ID != "t/p:m/snake" {
		t.Errorf("pattern select = %v, %v", pattern, err)
	}
	shared, err := list.Select("t/shared")
	if err != nil || len(shared) != 2 || shared[0].Provenance != "t/shared@1.2.0" {
		t.Errorf("provenance select = %v, %v", shared, err)
	}
	if _, err := list.Select("t/shared:x/*"); err == nil ||
		!strings.Contains(err.Error(), "matches no configured rule") {
		t.Errorf("unmatched selector err = %v", err)
	}
}

func TestAssessConformanceSelectionFailsLoudly(t *testing.T) {
	cfg, obs := selectionFixture(t)

	cases := []struct {
		name    string
		only    []string
		exclude []string
		want    string
	}{
		{"only matches nothing", []string{"t/shared:m/nope"}, nil, "matches no configured rule"},
		{"other version of the pattern", []string{"t/shared@9.0.0"}, nil, "matches no configured rule"},
		{"exclude matches nothing", nil, []string{"t/shared:other/*"}, "matches no configured rule"},
		{"selection cancels to empty", []string{"t/shared:m/keep"}, []string{"t/shared:m/keep"}, "leaves no rule"},
		{"malformed pattern", []string{"t/shared:m/["}, nil, "rule selector"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc := newAssess(t, cfg, obs, &fakeBaselineSource{})
			_, err := uc.Execute(application.AssessConformanceRequest{
				SkipBaseline: true, Only: tc.only, Exclude: tc.exclude,
			})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
