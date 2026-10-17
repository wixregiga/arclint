package conformance_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// words is a text of n words.
func words(n int) string { return strings.TrimSpace(strings.Repeat("word ", n)) }

func wordLimitRule(t *testing.T) rule.Rule {
	t.Helper()
	rules, err := rule.BuiltIn()
	if err != nil {
		t.Fatalf("BuiltIn: %v", err)
	}
	for _, r := range rules {
		if r.ID().Qualified() == rule.WordLimitRuleID {
			return r
		}
	}
	t.Fatalf("BuiltIn composes no %s", rule.WordLimitRuleID)
	return rule.Rule{}
}

// overLimitLanguage records one text of every limited kind one word
// over its limit, and one text of every kind exactly at it.
func overLimitLanguage(t *testing.T) vocab.UbiquitousLanguage {
	t.Helper()
	lang, err := newRecordedLanguage("skills", words(101), []vocab.BoundedContext{
		{
			Name: "skill", Definition: words(61), Line: 10,
			Aggregates: []vocab.Aggregate{{
				Name: "Package", Definition: words(41), Identity: "PackageID", Line: 20,
				Entities:   []vocab.Entity{{Name: "Item", Definition: words(41), Line: 25}},
				Invariants: []vocab.Invariant{{Key: "one-source", Statement: words(41), Line: 30}},
				Assertions: []vocab.Assertion{{Key: "source-pinned", On: "Install", Statement: words(41), Line: 32}},
			}},
			ValueObjects: []vocab.ValueObject{
				{Name: "Client", Definition: words(385), Line: 113, Invariants: []vocab.Invariant{{Key: "known-product", Statement: words(40), Line: 150}}},
				{Name: "SourceRef", Definition: words(40), Line: 160},
			},
			Events:         []vocab.DomainEvent{{Name: "Installed", Definition: words(41), Line: 170}},
			Services:       []vocab.DomainService{{Name: "Uninstall", Definition: words(41), Line: 180, Assertions: []vocab.Assertion{{Key: "removes-everything", On: "Remove", Statement: words(41), Line: 185}}}},
			Specifications: []vocab.Specification{{Name: "Installable", Definition: words(41), Line: 190}},
		},
		{Name: "catalog", Definition: words(60), Line: 200},
	}, nil)
	if err != nil {
		t.Fatalf("newRecordedLanguage: %v", err)
	}
	lang.DescriptionLine = 3
	return lang
}

func TestWordLimitReportsEveryOverLimitTextAtItsLineInTheRecording(t *testing.T) {
	a, err := conformance.Run(conformance.Request{
		Rules:     []rule.Rule{wordLimitRule(t)},
		Knowledge: overLimitLanguage(t),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := map[string]string{
		"docs/model.arclint.yaml:3":   `project "skills": the description has 101 words, and a description holds at most 100`,
		"docs/model.arclint.yaml:10":  `context "skill": the definition has 61 words, and a definition holds at most 60`,
		"docs/model.arclint.yaml:20":  `aggregate "Package" in context "skill": the definition has 41 words, and a definition holds at most 40`,
		"docs/model.arclint.yaml:25":  `entity "Item" of "Package" in context "skill": the definition has 41 words, and a definition holds at most 40`,
		"docs/model.arclint.yaml:30":  `invariant "one-source" of "Package" in context "skill": the statement has 41 words, and a statement holds at most 40`,
		"docs/model.arclint.yaml:32":  `assertion "source-pinned" of "Package" in context "skill": the statement has 41 words, and a statement holds at most 40`,
		"docs/model.arclint.yaml:113": `value object "Client" in context "skill": the definition has 385 words, and a definition holds at most 40`,
		"docs/model.arclint.yaml:170": `domain event "Installed" in context "skill": the definition has 41 words, and a definition holds at most 40`,
		"docs/model.arclint.yaml:180": `domain service "Uninstall" in context "skill": the definition has 41 words, and a definition holds at most 40`,
		"docs/model.arclint.yaml:185": `assertion "removes-everything" of "Uninstall" in context "skill": the statement has 41 words, and a statement holds at most 40`,
		"docs/model.arclint.yaml:190": `specification "Installable" in context "skill": the definition has 41 words, and a definition holds at most 40`,
	}
	got := map[string]string{}
	for _, v := range a.Violations() {
		got[fmt.Sprintf("%s:%d", v.Path(), v.Line())] = v.Message()
		if v.Severity() != rule.SeverityError || !v.FailsGate() {
			t.Errorf("%s: severity %s, want an error that fails the gate", v.Message(), v.Severity())
		}
		if v.Subject().String() != "file:"+testRecording {
			t.Errorf("%s: subject %s, want the recording's file", v.Message(), v.Subject())
		}
		for _, step := range []string{"arclint domain define invariant", "arclint domain define assertion", "put promises that are not domain rules in tests"} {
			if !strings.Contains(v.Remediation(), step) {
				t.Errorf("%s: remediation %q lacks %q", v.Message(), v.Remediation(), step)
			}
		}
	}
	for at, message := range want {
		if got[at] != message {
			t.Errorf("%s: message %q, want %q", at, got[at], message)
		}
	}
	if len(got) != len(want) {
		t.Errorf("violations = %d, want %d; texts at their limit are within it:\n%v", len(got), len(want), got)
	}
}

func TestWordLimitConformsWhenEveryTextIsWithinItsLimit(t *testing.T) {
	lang, err := newRecordedLanguage("skills", words(100), []vocab.BoundedContext{
		{Name: "skill", Definition: words(60), Line: 2, ValueObjects: []vocab.ValueObject{{Name: "Client", Definition: words(40), Line: 4}}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	a, err := conformance.Run(conformance.Request{Rules: []rule.Rule{wordLimitRule(t)}, Knowledge: lang})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := outcomesOf(a)[rule.WordLimitRuleID+"|file:"+testRecording]; got != conformance.OutcomeConforms {
		t.Errorf("outcome %s, want conforms", got)
	}
}

func TestWordLimitIsNotApplicableToAnExcludedRecording(t *testing.T) {
	glob, err := rule.NewGlob(testRecording)
	if err != nil {
		t.Fatal(err)
	}
	exclusion, err := rule.NewExclusion([]rule.Glob{glob}, nil, "the recording is rewritten next sprint")
	if err != nil {
		t.Fatal(err)
	}
	excluded, err := wordLimitRule(t).Exclude(exclusion)
	if err != nil {
		t.Fatal(err)
	}
	a, err := conformance.Run(conformance.Request{Rules: []rule.Rule{excluded}, Knowledge: overLimitLanguage(t)})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(a.Violations()) != 0 {
		t.Errorf("an excluded recording yields no violation; got %d", len(a.Violations()))
	}
	if got := outcomesOf(a)[rule.WordLimitRuleID+"|file:"+testRecording]; got != conformance.OutcomeNotApplicable {
		t.Errorf("outcome %s, want not applicable", got)
	}
}

func TestWordLimitJudgesNothingWithoutARecording(t *testing.T) {
	lang, err := vocab.NewUbiquitousLanguage("skills", words(101), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	a, err := conformance.Run(conformance.Request{Rules: []rule.Rule{wordLimitRule(t)}, Knowledge: lang})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(a.Evaluations()) != 0 || len(a.Violations()) != 0 {
		t.Errorf("a language read from no file is not judged; evaluations %d violations %d", len(a.Evaluations()), len(a.Violations()))
	}
}
