package application_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// selectionFixture extends the shared fixture with a second rule so
// selection has something to narrow: the naming rule t:m/snake plus a
// structure rule t:m/keep requiring m/ok.go.
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
	for _, id := range []string{"t:m/keep", "t:m/keep2"} {
		r, err := rule.New(rule.Spec{
			ID:            id,
			Type:          rule.TypeStructure,
			Params:        rule.StructureParams{Require: []rule.Glob{keep}},
			Applicability: scope,
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
		// t:m/keep is a prefix of t:m/keep2, so this also proves an
		// exact id wins completely over prefix expansion.
		{"only exact id", []string{"t:m/keep"}, nil, []string{"t:m/keep"}},
		{"only prefix", []string{"t:m/"}, nil, []string{"t:m/keep", "t:m/keep2", "t:m/snake"}},
		{"only pattern", []string{"t:m/*"}, nil, []string{"t:m/keep", "t:m/keep2", "t:m/snake"}},
		{"exclude one", nil, []string{"t:m/snake"}, []string{"t:m/keep", "t:m/keep2"}},
		{"only pattern minus exact exclude", []string{"t:m/*"}, []string{"t:m/keep"}, []string{"t:m/keep2", "t:m/snake"}},
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
	exact, err := list.Select("t:m/keep")
	if err != nil || len(exact) != 1 || exact[0].ID != "t:m/keep" {
		t.Errorf("exact select = %v, %v", exact, err)
	}
	prefix, err := list.Select("t:m/")
	if err != nil || len(prefix) != 3 {
		t.Errorf("prefix select = %v, %v", prefix, err)
	}
	pattern, err := list.Select("t:m/s*")
	if err != nil || len(pattern) != 1 || pattern[0].ID != "t:m/snake" {
		t.Errorf("pattern select = %v, %v", pattern, err)
	}
	if _, err := list.Select("t:x/*"); err == nil ||
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
		{"only matches nothing", []string{"t:m/nope"}, nil, "matches no configured rule"},
		{"exclude matches nothing", nil, []string{"t:other/*"}, "matches no configured rule"},
		{"selection cancels to empty", []string{"t:m/keep"}, []string{"t:m/keep"}, "leaves no rule"},
		{"malformed pattern", []string{"t:m/["}, nil, "rule selector"},
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
