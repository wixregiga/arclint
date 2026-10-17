package vocab_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// words is a text of n words, spread over lines the way a recording
// reflows long text.
func words(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = fmt.Sprintf("w%d", i)
		if i%9 == 8 {
			parts[i] += "\n"
		}
	}
	return strings.Join(parts, " ")
}

// remedySteps are the steps every refusal tells the writer to take.
var remedySteps = []string{
	"record each", "arclint domain define invariant", "arclint domain define assertion",
	"put promises that are not domain rules in tests",
}

func TestDefineRefusesTextOverItsWordLimitAndAcceptsItsLimit(t *testing.T) {
	cases := []struct {
		name    string
		concept vocab.Concept
		at      vocab.Locator
		change  func(text string) vocab.Change
		limit   int
		subject string
		keep    string
	}{
		{"bounded context", vocab.ConceptBoundedContext, vocab.Locator{Name: "billing"}, def, 60, `context "billing"`, "Keep what the word means"},
		{"aggregate", vocab.ConceptAggregate, vocab.Locator{Context: "catalog", Name: "Venue"}, func(text string) vocab.Change {
			return vocab.Change{SetDefinition: true, Definition: text, SetIdentity: true, Identity: "VenueID"}
		}, 40, `aggregate "Venue" in context "catalog"`, "Keep what the word means"},
		{"aggregate root", vocab.ConceptAggregateRoot, vocab.Locator{Context: "sales", Name: "Order"}, def, 40, `aggregate "Order" in context "sales"`, "Keep what the word means"},
		{"entity", vocab.ConceptEntity, vocab.Locator{Context: "sales", Owner: "Order", Name: "Seat"}, def, 40, `entity "Seat" of "Order" in context "sales"`, "Keep what the word means"},
		{"value object", vocab.ConceptValueObject, vocab.Locator{Context: "sales", Name: "Money"}, def, 40, `value object "Money" in context "sales"`, "Keep what the word means"},
		{"domain event", vocab.ConceptDomainEvent, vocab.Locator{Context: "sales", Name: "OrderPlaced"}, def, 40, `domain event "OrderPlaced" in context "sales"`, "Keep what the word means"},
		{"domain service", vocab.ConceptDomainService, vocab.Locator{Context: "sales", Name: "Pricing"}, def, 40, `domain service "Pricing" in context "sales"`, "Keep what the word means"},
		{"specification", vocab.ConceptSpecification, vocab.Locator{Context: "sales", Name: "PreferredCustomer"}, def, 40, `specification "PreferredCustomer" in context "sales"`, "Keep what the word means"},
		{"invariant", vocab.ConceptInvariant, vocab.Locator{Context: "sales", Owner: "Money", Name: "whole-cents"}, func(text string) vocab.Change {
			return vocab.Change{SetStatement: true, Statement: text}
		}, 40, `invariant "whole-cents" of "Money" in context "sales"`, "Keep the one rule this statement names"},
		{"invariant found by key", vocab.ConceptInvariant, vocab.Locator{Context: "sales", Name: "total-is-sum-of-lines"}, func(text string) vocab.Change {
			return vocab.Change{SetStatement: true, Statement: text}
		}, 40, `invariant "total-is-sum-of-lines" of "Order" in context "sales"`, "Keep the one rule this statement names"},
		{"assertion", vocab.ConceptAssertion, vocab.Locator{Context: "sales", Owner: "Pricing", Name: "tiers-on-sale"}, func(text string) vocab.Change {
			return vocab.Change{SetOn: true, On: "Price", SetStatement: true, Statement: text}
		}, 40, `assertion "tiers-on-sale" of "Pricing" in context "sales"`, "Keep the one rule this statement names"},
		{"business rule", vocab.ConceptBusinessRule, vocab.Locator{Context: "sales", Owner: "Order", Name: "lines-priced"}, func(text string) vocab.Change {
			return vocab.Change{SetOn: true, On: "Confirm", SetStatement: true, Statement: text}
		}, 40, `assertion "lines-priced" of "Order" in context "sales"`, "Keep the one rule this statement names"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lang := mustTicketing(t)
			if _, _, err := lang.Define(tc.concept, tc.at, tc.change(words(tc.limit))); err != nil {
				t.Fatalf("%d words: %v", tc.limit, err)
			}
			_, _, err := lang.Define(tc.concept, tc.at, tc.change(words(tc.limit+1)))
			var over vocab.OverlongText
			if !errors.As(err, &over) {
				t.Fatalf("%d words: err = %v, want an OverlongText", tc.limit+1, err)
			}
			if over.Words != tc.limit+1 || over.Limit.Words() != tc.limit {
				t.Errorf("count %d limit %d, want %d and %d", over.Words, over.Limit.Words(), tc.limit+1, tc.limit)
			}
			msg := err.Error()
			if !strings.Contains(msg, tc.subject+": the ") || !strings.Contains(msg, fmt.Sprintf("has %d words", tc.limit+1)) || !strings.Contains(msg, fmt.Sprintf("at most %d", tc.limit)) {
				t.Errorf("message does not name the entry, the count and the limit: %s", msg)
			}
			for _, step := range append([]string{tc.keep}, remedySteps...) {
				if !strings.Contains(msg, step) {
					t.Errorf("message lacks %q: %s", step, msg)
				}
			}
			if errors.Is(err, vocab.ErrChangeIncomplete) || errors.Is(err, vocab.ErrChangeMisapplied) {
				t.Errorf("an over-limit text is refused by the language, not a malformed request: %v", err)
			}
		})
	}
}

func TestDefineJudgesOnlyTheTextItSets(t *testing.T) {
	contexts, relations := ticketing()
	contexts[0].ValueObjects[0].Definition = words(385)
	lang, err := vocab.NewUbiquitousLanguage("boxoffice", "Selling tickets.", contexts, relations)
	if err != nil {
		t.Fatalf("the loader admits a recorded text over its limit: %v", err)
	}
	if _, _, err := lang.Define(vocab.ConceptValueObject, vocab.Locator{Context: "sales", Name: "Money"}, vocab.Change{SetAliases: true, Aliases: []string{"Amount"}}); err != nil {
		t.Errorf("a change that leaves the over-limit definition as it is: %v", err)
	}
	if _, _, err := lang.Define(vocab.ConceptInvariant, vocab.Locator{Context: "sales", Owner: "Money", Name: "whole-cents"}, vocab.Change{SetStatement: true, Statement: "Money counts whole cents."}); err != nil {
		t.Errorf("an invariant under an over-limit owner: %v", err)
	}
	if _, _, err := lang.Define(vocab.ConceptValueObject, vocab.Locator{Context: "sales", Name: "Money"}, def(words(41))); err == nil {
		t.Error("rewriting the definition still over its limit must be refused")
	}
	if _, _, err := lang.Define(vocab.ConceptQuestion, vocab.Locator{Context: "sales", Name: "currency"}, vocab.Change{SetText: true, Text: words(200)}); err != nil {
		t.Errorf("a question's text carries no limit: %v", err)
	}
}

func TestDescribeRefusesADescriptionOverOneHundredWords(t *testing.T) {
	lang := mustTicketing(t)
	next, err := lang.Describe(words(100))
	if err != nil {
		t.Fatalf("100 words: %v", err)
	}
	if vocab.CountWords(next.Description) != 100 || lang.Description != "Selling tickets." {
		t.Errorf("Describe must return the new description and leave its receiver untouched")
	}
	_, err = lang.Describe(words(101))
	var over vocab.OverlongText
	if !errors.As(err, &over) {
		t.Fatalf("101 words: err = %v, want an OverlongText", err)
	}
	msg := err.Error()
	for _, want := range append([]string{`project "boxoffice": the description has 101 words, and a description holds at most 100`, "Keep what the project is about"}, remedySteps...) {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q: %s", want, msg)
		}
	}
}

func TestCountWordsIgnoresHowTheTextIsFlowed(t *testing.T) {
	flowed := "A customer's purchase\n  of seats\tfor one event. "
	if vocab.CountWords(flowed) != vocab.CountWords("A customer's purchase of seats for one event.") || vocab.CountWords(flowed) != 8 {
		t.Errorf("CountWords(%q) = %d, want 8", flowed, vocab.CountWords(flowed))
	}
	if vocab.CountWords("") != 0 || vocab.CountWords(" \n ") != 0 {
		t.Error("an empty text has no words")
	}
}

func TestWordLimitsArePublishedOncePerKind(t *testing.T) {
	want := map[vocab.TextKind]int{
		vocab.TextDefinition:         40,
		vocab.TextContextDefinition:  60,
		vocab.TextStatement:          40,
		vocab.TextProjectDescription: 100,
	}
	limits := vocab.WordLimits()
	if len(limits) != len(want) {
		t.Fatalf("WordLimits = %v", limits)
	}
	for _, l := range limits {
		if want[l.Kind()] != l.Words() {
			t.Errorf("%s: %d words, want %d", l.Kind(), l.Words(), want[l.Kind()])
		}
		if got, ok := vocab.WordLimitOf(l.Kind()); !ok || got != l {
			t.Errorf("WordLimitOf(%s) = %v, %v", l.Kind(), got, ok)
		}
		if !l.Admits(words(l.Words())) || l.Admits(words(l.Words()+1)) {
			t.Errorf("%s admits up to %d words", l.Kind(), l.Words())
		}
	}
	if _, ok := vocab.WordLimitOf("question"); ok {
		t.Error("a question's text carries no limit")
	}
	statement := vocab.WordLimitsStatement()
	for _, n := range []string{"40 words", "60 for a bounded context", "40 for an invariant", "100 for the project description"} {
		if !strings.Contains(statement, n) {
			t.Errorf("WordLimitsStatement lacks %q: %s", n, statement)
		}
	}
}

func TestOverlongTextsListsEveryOverLimitTextInFileOrder(t *testing.T) {
	contexts, relations := ticketing()
	contexts[0].Definition = words(61)
	contexts[0].Aggregates[0].Entities[0].Definition = words(41)
	contexts[0].Aggregates[0].Assertions[0].Statement = words(41)
	contexts[0].ValueObjects[0].Invariants[0].Statement = words(40)
	contexts[0].Services[0].Definition = words(41)
	contexts[0].Questions[0].Text = words(500)
	contexts[1].Aggregates[0].Definition = words(41)
	lang, err := vocab.NewUbiquitousLanguage("boxoffice", words(101), contexts, relations)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, o := range lang.OverlongTexts() {
		got = append(got, fmt.Sprintf("%s %d/%d", o.Subject, o.Words, o.Limit.Words()))
	}
	want := []string{
		`project "boxoffice" 101/100`,
		`context "sales" 61/60`,
		`entity "OrderLine" of "Order" in context "sales" 41/40`,
		`assertion "lines-priced" of "Order" in context "sales" 41/40`,
		`domain service "Pricing" in context "sales" 41/40`,
		`aggregate "Event" in context "catalog" 41/40`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("OverlongTexts =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
