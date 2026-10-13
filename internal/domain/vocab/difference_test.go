package vocab

import (
	"strings"
	"testing"
)

func differenceLanguage(t *testing.T, line int, aliases []string, definition string) UbiquitousLanguage {
	t.Helper()
	recording, err := NewDomainRecording("domain.arclint.cml")
	if err != nil {
		t.Fatal(err)
	}
	lang, err := NewUbiquitousLanguage("boxoffice", "Shows on sale.", []BoundedContext{{
		Name:       "ordering",
		Definition: definition,
		Line:       line,
		Aggregates: []Aggregate{{
			Name: "Order", Definition: "The deal as struck.", Identity: "OrderID", Aliases: aliases, Line: line + 1,
			Invariants: []Invariant{{Key: "lines-frozen", Statement: "A placed Order's lines never change.", Line: line + 2}},
		}},
		ValueObjects: []ValueObject{{Name: "Refund", Definition: "Tickets given back.", Line: line + 3}},
		Questions:    []Question{{Key: "order-placed-event", Text: "Is OrderPlaced an event?", Line: line + 4}},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	lang.Recording = recording
	return lang
}

func TestDifferenceIgnoresWhereTheLanguageIsWritten(t *testing.T) {
	a := differenceLanguage(t, 3, nil, "The deal as struck.")
	b := differenceLanguage(t, 40, []string{}, "The deal as struck.")
	b.Recording = mustDefaultRecording(t)
	if d := a.Difference(b); d != "" {
		t.Fatalf("Difference = %q, want none: lines, the DomainRecording and an empty alias list are not part of the language", d)
	}
}

func TestDifferenceNamesTheFirstEntryThatDiffers(t *testing.T) {
	a := differenceLanguage(t, 3, nil, "The deal as struck.")
	cases := map[string]struct {
		change func(*UbiquitousLanguage)
		want   string
	}{
		"definition": {func(l *UbiquitousLanguage) { l.Contexts[0].Definition = "Something else." }, `bounded context "ordering": the definition differs`},
		"alias":      {func(l *UbiquitousLanguage) { l.Contexts[0].Aggregates[0].Aliases = []string{"Purchase"} }, `aggregates: "Order" differs`},
		"invariant":  {func(l *UbiquitousLanguage) { l.Contexts[0].Aggregates[0].Invariants[0].Statement = "Changed." }, `aggregates: "Order" differs`},
		"question":   {func(l *UbiquitousLanguage) { l.Contexts[0].Questions = nil }, "1 questions against 0"},
		"project":    {func(l *UbiquitousLanguage) { l.Project = "other" }, `the project is "boxoffice" in one and "other" in the other`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			b := differenceLanguage(t, 3, nil, "The deal as struck.")
			c.change(&b)
			d := a.Difference(b)
			if !strings.Contains(d, c.want) {
				t.Fatalf("Difference = %q, want it to say %q", d, c.want)
			}
		})
	}
}

func TestDifferenceCountsOrder(t *testing.T) {
	a := differenceLanguage(t, 3, nil, "The deal as struck.")
	a.Contexts[0].ValueObjects = append(a.Contexts[0].ValueObjects, ValueObject{Name: "Attendee", Definition: "The person seats are promised to."})
	b := differenceLanguage(t, 3, nil, "The deal as struck.")
	b.Contexts[0].ValueObjects = append([]ValueObject{{Name: "Attendee", Definition: "The person seats are promised to."}}, b.Contexts[0].ValueObjects...)
	if d := a.Difference(b); !strings.Contains(d, `value objects: "Refund" stands where "Attendee" does`) {
		t.Fatalf("Difference = %q, want the reordered value objects named", d)
	}
}

func mustDefaultRecording(t *testing.T) DomainRecording {
	t.Helper()
	recording, err := DefaultDomainRecording()
	if err != nil {
		t.Fatal(err)
	}
	return recording
}
