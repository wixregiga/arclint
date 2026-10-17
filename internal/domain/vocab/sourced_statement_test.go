package vocab_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// The guidance arclint repeats to agents cites the meta-model's
// bibliography by key, so the same check that judges the meta-model
// judges it, and every reader sees each work's tier and whether its
// page was read.
func TestSourcedStatementsCiteTheBibliography(t *testing.T) {
	if err := vocab.DDD().ValidateSourcedStatements(vocab.SourcedStatements()); err != nil {
		t.Fatal(err)
	}
	for _, s := range vocab.SourcedStatements() {
		if !strings.HasSuffix(s.String(), ", read)).") && !strings.HasSuffix(s.String(), ", unverified page)).") {
			t.Errorf("statement does not end with its last source's standing: %q", s.String())
		}
	}
}

func TestValidateSourcedStatementsRejects(t *testing.T) {
	model := vocab.DDD()
	cases := []struct {
		name      string
		statement vocab.SourcedStatement
		wants     string
	}{
		{"no text", vocab.SourcedStatement{Sources: []vocab.Citation{{Work: "evans2015", Page: "10"}}}, `text must be non-empty`},
		{"no source", vocab.SourcedStatement{Text: "A claim."}, `must cite at least one work`},
		{"unlisted work", vocab.SourcedStatement{Text: "A claim.", Sources: []vocab.Citation{{Work: "somebody"}}}, `cites work "somebody"`},
		{"unmarked tier 3", vocab.SourcedStatement{Text: "A claim.", Sources: []vocab.Citation{{Work: "evans2015", Page: "10"}, {Work: "bogard-vertical-slice"}}}, `without marking the citation illustrative`},
		{"only illustrated", vocab.SourcedStatement{Text: "A claim.", Sources: []vocab.Citation{{Work: "bogard-vertical-slice", Illustrative: true}}}, `must cite at least one tier 1 or tier 2 work`},
	}
	for _, c := range cases {
		err := model.ValidateSourcedStatements([]vocab.SourcedStatement{c.statement})
		if err == nil || !strings.Contains(err.Error(), c.wants) {
			t.Errorf("%s: got %v, want an error mentioning %q", c.name, err, c.wants)
		}
	}
}

// Consecutive citations of one work name it once, with every page, and
// end with its standing once.
func TestInlineNamesEachWorkOnce(t *testing.T) {
	got := vocab.DDD().Inline([]vocab.Citation{
		{Work: "evans2015", Page: "10", Section: "Layered Architecture"},
		{Work: "evans2015", Page: "21", Section: "Side-Effect-Free Functions"},
		{Work: "vernon2013", Chapter: 10, Page: "359"},
	})
	want := "Eric Evans, Domain-Driven Design Reference: Definitions and Pattern Summaries (2015), p. 10, Layered Architecture, p. 21, Side-Effect-Free Functions (tier 1, read); " +
		"Vaughn Vernon, Implementing Domain-Driven Design (2013), ch. 10, p. 359 (tier 1, unverified page)"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// freeTextSource is a parenthesis that opens on an author's name, the
// way a source written as free text begins.
var freeTextSource = regexp.MustCompile(`\((Evans|Vernon|Fowler|Stafford|Khorikov|Meyer|Wlaschin)[ ,']`)

// No text arclint gives an agent carries a source as free text, which no
// bibliography check would cover.
func TestAgentFacingTextCitesNoFreeTextSource(t *testing.T) {
	var texts []string
	texts = append(texts, vocab.SkillProtocolRules()...)
	for _, r := range vocab.DistillationRules() {
		texts = append(texts, r.Rule, r.Example)
	}
	for _, s := range vocab.SourcedStatements() {
		texts = append(texts, s.Text)
	}
	texts = append(texts, vocab.SkillAgentWorkflow, vocab.SkillIntro, vocab.SkillReference)
	for _, text := range texts {
		if loc := freeTextSource.FindStringIndex(text); loc != nil {
			t.Errorf("free-text source %q in: %s", text[loc[0]:min(len(text), loc[0]+60)], text)
		}
	}
}
