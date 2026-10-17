package vocab_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// The librarian's distillation rules cite the meta-model's bibliography
// by key, so the same check that judges the meta-model judges them.
func TestDistillationRulesCiteTheBibliography(t *testing.T) {
	if err := vocab.DDD().ValidateDistillationRules(vocab.DistillationRules()); err != nil {
		t.Fatal(err)
	}
	r, ok := vocab.DistillationRuleByID("identity-regardless-of-history")
	if !ok || len(r.Sources) == 0 {
		t.Fatal("identity-regardless-of-history no longer cites the Reference by key")
	}
	if strings.Contains(r.Rule, "(Evans") {
		t.Errorf("the rule still carries its source as free text: %q", r.Rule)
	}
}

func TestValidateDistillationRulesRejects(t *testing.T) {
	model := vocab.DDD()
	cases := []struct {
		name  string
		rule  vocab.DistillationRule
		wants string
	}{
		{"unlisted work", vocab.DistillationRule{ID: "a-rule", Rule: "A rule.", Sources: []vocab.Citation{{Work: "somebody"}}}, `cites work "somebody"`},
		{"unmarked tier 3", vocab.DistillationRule{ID: "a-rule", Rule: "A rule.", Sources: []vocab.Citation{{Work: "evans2015", Page: "11"}, {Work: "bogard-vertical-slice"}}}, `without marking the citation illustrative`},
		{"only illustrated", vocab.DistillationRule{ID: "a-rule", Rule: "A rule.", Sources: []vocab.Citation{{Work: "bogard-vertical-slice", Illustrative: true}}}, `must cite at least one tier 1 or tier 2 work`},
		{"convention off the text", vocab.DistillationRule{ID: "a-rule", Rule: "A rule.", Conventions: []string{"never"}}, `convention label "never" does not quote the entry's text`},
		{"id not kebab", vocab.DistillationRule{ID: "A_Rule", Rule: "A rule."}, `id "A_Rule" is not lowercase kebab case`},
	}
	for _, c := range cases {
		err := model.ValidateDistillationRules([]vocab.DistillationRule{c.rule})
		if err == nil || !strings.Contains(err.Error(), c.wants) {
			t.Errorf("%s: got %v, want an error mentioning %q", c.name, err, c.wants)
		}
	}
	twice := vocab.DistillationRule{ID: "a-rule", Rule: "A rule."}
	if err := model.ValidateDistillationRules([]vocab.DistillationRule{twice, twice}); err == nil || !strings.Contains(err.Error(), `"a-rule" recorded twice`) {
		t.Errorf("duplicate id: got %v", err)
	}
}
