package vocab_test

import (
	"slices"
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

// A distillation rule that repeats a meta-model statement labelled as
// ArcLint's convention carries the same label, so the skill never
// presents it unlabelled.
func TestDistillationRulesLabelTheMetaModelsConventions(t *testing.T) {
	var conventions []string
	for _, b := range vocab.DDD().Blocks {
		conventions = append(conventions, b.Definition.Conventions...)
		for _, inv := range b.Invariants {
			conventions = append(conventions, inv.Conventions...)
		}
		for _, g := range b.Guidance {
			conventions = append(conventions, g.Conventions...)
		}
	}
	for _, r := range vocab.DistillationRules() {
		for _, c := range conventions {
			if strings.Contains(r.Rule, c) && !slices.Contains(r.Conventions, c) {
				t.Errorf("distillation rule %s repeats %q without labelling it as ArcLint's convention", r.ID, c)
			}
		}
		// The method naming shapes are ArcLint's convention however a
		// rule words them.
		for _, shape := range []string{"assert followed by the key", "ensure followed by the key"} {
			if strings.Contains(r.Rule, shape) && !slices.ContainsFunc(r.Conventions, func(c string) bool { return strings.Contains(c, shape) }) {
				t.Errorf("distillation rule %s states %q without labelling it as ArcLint's convention", r.ID, shape)
			}
		}
	}
}

func TestValidateDistillationRulesRejects(t *testing.T) {
	model := vocab.DDD()
	// A tier 3 work defined here, so the tier 3 check stays covered
	// whatever the bibliography lists.
	model.Works = append(model.Works, vocab.Work{
		Key: "a-blog", Title: "A Blog Post", Authors: []string{"A Practitioner"}, Year: 2020,
		URL: "https://example.com/post", Tier: vocab.TierPractitioner, Verification: vocab.VerifiedFetched,
	})
	cases := []struct {
		name  string
		rule  vocab.DistillationRule
		wants string
	}{
		{"unlisted work", vocab.DistillationRule{ID: "a-rule", Rule: "A rule.", Sources: []vocab.Citation{{Work: "somebody"}}}, `cites work "somebody"`},
		{"unmarked tier 3", vocab.DistillationRule{ID: "a-rule", Rule: "A rule.", Sources: []vocab.Citation{{Work: "evans2015", Page: "11"}, {Work: "a-blog"}}}, `without marking the citation illustrative`},
		{"only illustrated", vocab.DistillationRule{ID: "a-rule", Rule: "A rule.", Sources: []vocab.Citation{{Work: "a-blog", Illustrative: true}}}, `must cite at least one tier 1 or tier 2 work`},
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
