package vocab

import (
	"fmt"
	"strings"
)

// TextKind is one kind of recorded text a WordLimit bounds.
type TextKind string

// The kinds of recorded text a WordLimit bounds. A question's text and
// a relation's description carry no limit.
const (
	// TextDefinition is the definition of an aggregate, a member entity,
	// a value object, a domain event, a domain service, or a
	// specification.
	TextDefinition TextKind = "definition"
	// TextContextDefinition is the definition of a bounded context.
	TextContextDefinition TextKind = "context definition"
	// TextStatement is the statement of an invariant or an assertion.
	TextStatement TextKind = "statement"
	// TextProjectDescription is the project's description.
	TextProjectDescription TextKind = "project description"
)

// WordLimit is the most words one kind of recorded text may hold. A
// definition says what a word means and a statement states one rule;
// the rules a definition would carry are recorded as invariants and
// assertions, where arclint checks them, and promises that are not
// domain rules belong in tests. WordLimitOf is the only door, so a
// WordLimit is always one of the published limits.
type WordLimit struct {
	kind  TextKind
	words int
}

// wordLimits are the published limits, in the order a listing names
// them.
var wordLimits = []WordLimit{
	{kind: TextDefinition, words: 40},
	{kind: TextContextDefinition, words: 60},
	{kind: TextStatement, words: 40},
	{kind: TextProjectDescription, words: 100},
}

// WordLimits lists every published WordLimit.
func WordLimits() []WordLimit {
	return append([]WordLimit(nil), wordLimits...)
}

// WordLimitOf returns the WordLimit of one kind of recorded text; false
// for a kind no limit bounds.
func WordLimitOf(kind TextKind) (WordLimit, bool) {
	for _, w := range wordLimits {
		if w.kind == kind {
			return w, true
		}
	}
	return WordLimit{}, false
}

// Kind returns the kind of recorded text the limit bounds.
func (w WordLimit) Kind() TextKind { return w.kind }

// Words returns the most words the text may hold.
func (w WordLimit) Words() int { return w.words }

// Admits reports whether text holds no more words than the limit.
func (w WordLimit) Admits(text string) bool { return CountWords(text) <= w.words }

// CountWords counts the words of a recorded text: the runs of
// characters between spaces and line breaks. Reflowing a text, as
// writing it to a DomainRecording does, never changes its count.
func CountWords(text string) int { return len(strings.Fields(text)) }

// WordLimitsStatement states every published limit in one sentence,
// the proposition of the check that reports an over-limit text.
func WordLimitsStatement() string {
	definition, _ := WordLimitOf(TextDefinition)
	context, _ := WordLimitOf(TextContextDefinition)
	statement, _ := WordLimitOf(TextStatement)
	description, _ := WordLimitOf(TextProjectDescription)
	return fmt.Sprintf("Every recorded text stays within its word limit: %d words for the definition of an aggregate, entity, value object, domain event, domain service, or specification; %d for a bounded context's definition; %d for an invariant's or assertion's statement; %d for the project description",
		definition.words, context.words, statement.words, description.words)
}

// OverlongText is one recorded text that holds more words than its
// WordLimit: what holds it, how many words it has, and where it is
// written down, 0 when it was not read from a file.
type OverlongText struct {
	// Subject names what holds the text, as a reader finds it:
	// value object "Client" of context "skill".
	Subject string
	Context string
	Words   int
	Limit   WordLimit
	Line    int
}

// Problem says what is over the limit, with the count and the limit.
func (o OverlongText) Problem() string {
	return fmt.Sprintf("%s: the %s has %d words, and a %s holds at most %d",
		o.Subject, o.Limit.noun(), o.Words, o.Limit.noun(), o.Limit.words)
}

// Remedy says how to bring the text within its limit.
func (o OverlongText) Remedy() string {
	const rules = "record each rule it states with arclint domain define invariant or arclint domain define assertion; put promises that are not domain rules in tests"
	switch o.Limit.kind {
	case TextStatement:
		return "Keep the one rule this statement names; record each further rule with arclint domain define invariant or arclint domain define assertion; put promises that are not domain rules in tests"
	case TextProjectDescription:
		return "Keep what the project is about; " + rules
	default:
		return "Keep what the word means; " + rules
	}
}

// Error reads the OverlongText as the refusal of a write: the problem
// and the remedy in one message.
func (o OverlongText) Error() string {
	return "word limit: " + o.Problem() + ". " + o.Remedy() + "."
}

// noun is how a message names the kind of text: the property that
// holds it.
func (w WordLimit) noun() string {
	switch w.kind {
	case TextStatement:
		return propStatement
	case TextProjectDescription:
		return propDescription
	default:
		return propDefinition
	}
}

// overlong returns the OverlongText of one recorded text, and false
// when the text is within its limit.
func overlong(kind TextKind, subject, context, text string, line int) (OverlongText, bool) {
	limit, ok := WordLimitOf(kind)
	if !ok {
		return OverlongText{}, false
	}
	words := CountWords(text)
	if words <= limit.words {
		return OverlongText{}, false
	}
	return OverlongText{Subject: subject, Context: context, Words: words, Limit: limit, Line: line}, true
}

// OverlongTexts lists every recorded text that holds more words than
// its WordLimit, in file order: the project description, then each
// bounded context's definition followed by the texts recorded in it.
func (l UbiquitousLanguage) OverlongTexts() []OverlongText {
	var out []OverlongText
	add := func(kind TextKind, subject, context, text string, line int) {
		if o, ok := overlong(kind, subject, context, text, line); ok {
			out = append(out, o)
		}
	}
	add(TextProjectDescription, fmt.Sprintf("project %q", l.Project), "", l.Description, l.DescriptionLine)
	for _, c := range l.Contexts {
		add(TextContextDefinition, fmt.Sprintf("context %q", c.Name), c.Name, c.Definition, c.Line)
		for _, a := range c.Aggregates {
			add(TextDefinition, entrySubject(ConceptAggregate, a.Name, "", c.Name), c.Name, a.Definition, a.Line)
			for _, e := range a.Entities {
				add(TextDefinition, entrySubject(ConceptEntity, e.Name, a.Name, c.Name), c.Name, e.Definition, e.Line)
			}
			for _, inv := range a.Invariants {
				add(TextStatement, entrySubject(ConceptInvariant, inv.Key, a.Name, c.Name), c.Name, inv.Statement, inv.Line)
			}
			for _, as := range a.Assertions {
				add(TextStatement, entrySubject(ConceptAssertion, as.Key, a.Name, c.Name), c.Name, as.Statement, as.Line)
			}
		}
		for _, v := range c.ValueObjects {
			add(TextDefinition, entrySubject(ConceptValueObject, v.Name, "", c.Name), c.Name, v.Definition, v.Line)
			for _, inv := range v.Invariants {
				add(TextStatement, entrySubject(ConceptInvariant, inv.Key, v.Name, c.Name), c.Name, inv.Statement, inv.Line)
			}
		}
		for _, e := range c.Events {
			add(TextDefinition, entrySubject(ConceptDomainEvent, e.Name, "", c.Name), c.Name, e.Definition, e.Line)
		}
		for _, s := range c.Services {
			add(TextDefinition, entrySubject(ConceptDomainService, s.Name, "", c.Name), c.Name, s.Definition, s.Line)
			for _, as := range s.Assertions {
				add(TextStatement, entrySubject(ConceptAssertion, as.Key, s.Name, c.Name), c.Name, as.Statement, as.Line)
			}
		}
		for _, s := range c.Specifications {
			add(TextDefinition, entrySubject(ConceptSpecification, s.Name, "", c.Name), c.Name, s.Definition, s.Line)
		}
	}
	return out
}

// entrySubject names a recorded entry for a message: its concept, its
// name or key, the owner it is recorded under when nested, and its
// context.
func entrySubject(c Concept, name, owner, context string) string {
	subject := fmt.Sprintf("%s %q", strings.ReplaceAll(string(c), "_", " "), name)
	if owner != "" {
		subject += fmt.Sprintf(" of %q", owner)
	}
	return subject + fmt.Sprintf(" in context %q", context)
}

// changeWithinWordLimit refuses a Define whose change sets a definition
// or a statement over the WordLimit of its kind. Only the text the
// change sets is judged; a text already recorded over its limit is
// reported by arclint check and refused only when it is written again.
func changeWithinWordLimit(c Concept, at Locator, owner string, ch Change) error {
	if ch.SetDefinition {
		kind := TextDefinition
		subject := entrySubject(c, at.Name, owner, at.Context)
		if c == ConceptBoundedContext {
			kind, subject = TextContextDefinition, fmt.Sprintf("context %q", at.Name)
		}
		if o, over := overlong(kind, subject, at.Context, ch.Definition, 0); over {
			return o
		}
	}
	if ch.SetStatement {
		if o, over := overlong(TextStatement, entrySubject(c, at.Name, owner, at.Context), at.Context, ch.Statement, 0); over {
			return o
		}
	}
	return nil
}

// Describe returns the language with its project description replaced.
// A description over its WordLimit is refused; the receiver is
// untouched.
func (l UbiquitousLanguage) Describe(description string) (UbiquitousLanguage, error) {
	description = strings.TrimSpace(description)
	if o, over := overlong(TextProjectDescription, fmt.Sprintf("project %q", l.Project), "", description, 0); over {
		return UbiquitousLanguage{}, o
	}
	next, err := NewUbiquitousLanguage(l.Project, description, l.Contexts, l.Relations)
	if err != nil {
		return UbiquitousLanguage{}, err
	}
	next.Recording = l.Recording
	next.DescriptionLine = l.DescriptionLine
	return next, nil
}
