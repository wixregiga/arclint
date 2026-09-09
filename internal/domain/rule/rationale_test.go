package rule_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

func TestRationaleRequiresAnAuthoredExplanation(t *testing.T) {
	for _, blank := range []string{"", " \t\n"} {
		if _, err := rule.NewRationale(blank); err == nil {
			t.Errorf("accepted blank explanation %q", blank)
		}
	}
	r, err := rule.NewRationale("  Keep dependencies reviewable. \n")
	if err != nil || r.IsZero() || r.String() != "Keep dependencies reviewable." {
		t.Fatalf("rationale = %v, %v", r, err)
	}
}

func TestRuleSeparatesRationaleFromProposition(t *testing.T) {
	scope, err := rule.RepositoryScope()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, rationale, want string
		invalid               bool
	}{
		{name: "absent"},
		{name: "authored", rationale: "  Enable independent changes.  ", want: "Enable independent changes."},
		{name: "blank canonical", rationale: " \t", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := rule.New(rule.Spec{ID: "acyclic", Constraint: rule.AcyclicConstraint{}, Scope: scope, Rationale: tc.rationale})
			if tc.invalid {
				if err == nil {
					t.Fatal("accepted invalid rationale")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if r.Rationale().String() != tc.want || r.Rationale().IsZero() != (tc.want == "") {
				t.Fatalf("rationale = %q, want %q", r.Rationale(), tc.want)
			}
			if r.Proposition() == "" || r.Proposition() == tc.want {
				t.Fatalf("proposition = %q", r.Proposition())
			}

		})
	}
}

func TestReexpandPreservesAuthoredRationale(t *testing.T) {
	empty := expandedRule(t, vocab.UbiquitousLanguage{})
	expansion, _ := empty.Expansion()
	r, err := rule.New(rule.Spec{ID: empty.ID().Qualified(), Constraint: empty.Constraint(), Scope: empty.Scope(), Expansion: &expansion, Rationale: "Give each aggregate an identifiable home."})
	if err != nil {
		t.Fatal(err)
	}
	re, err := r.Reexpand(recordedLanguage())
	if err != nil {
		t.Fatal(err)
	}
	if re.Rationale() != r.Rationale() || re.Rationale().IsZero() {
		t.Fatal("re-expansion replaced the authored rationale")
	}
	if re.ID() != r.ID() || re.Proposition() == r.Proposition() || !strings.Contains(r.Proposition(), "none recorded yet") {
		t.Fatal("re-expansion must preserve identity and the original value while updating the proposition")
	}
}

func TestBuiltInPropositionsDoNotInventRationale(t *testing.T) {
	rules, err := rule.BuiltIn()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rules {
		inv, ok := vocab.DDD().Invariant(r.ID().Local())
		if !ok || r.Proposition() != inv.Statement || !r.Rationale().IsZero() {
			t.Errorf("built-in %s: proposition %q, rationale %q", r.ID(), r.Proposition(), r.Rationale())
		}
	}
}
