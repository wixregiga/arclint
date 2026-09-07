package yamlvocab_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	yamlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/yaml"
)

// ticketingYAML is a complete recorded domain in the file's shape:
// two contexts, an aggregate with a member entity, invariants under
// both owner kinds, an assertion, an event raised by the aggregate, a
// service, a specification, an open question, and one relation.
const ticketingYAML = `# yaml-language-server: $schema=.arclint/schemas/domain.arclint.schema.json
version: 1
project: ticketing
description: Selling tickets.
contexts:
  sales:
    definition: Selling seats for events.
    aggregates:
      Order:
        definition: A customer's purchase of seats for one event.
        identity: OrderID
        aliases: [booking]
        entities:
          OrderLine:
            definition: One seat on the order.
            identity: LineNumber
        invariants:
          total-is-sum-of-lines: The order total equals the sum of its lines.
        assertions:
          lines-priced:
            on: Confirm
            statement: Every line carries a price once the order is confirmed.
        repository: OrderRepository
    value_objects:
      Money:
        definition: An amount in one currency.
        invariants:
          never-negative: Money is never negative.
    events:
      OrderConfirmed:
        definition: The order was confirmed.
        raised_by: Order
    services:
      Pricing:
        definition: Prices an order against the event's tiers.
    specifications:
      PreferredCustomer:
        definition: A customer entitled to early access.
    questions:
      refund-window: How long after purchase may an order be refunded?
  catalog:
    definition: The events on sale and their seating.
    aggregates:
      Event:
        definition: A performance with a seating plan.
        identity: EventID
relations:
  - from: catalog
    to: sales
    kind: customer_supplier
    description: Sales reads seating from the catalog.
`

// ticketing is ticketingYAML as the domain records it, lines aside.
func ticketing(t *testing.T) vocab.UbiquitousLanguage {
	t.Helper()
	contexts := []vocab.BoundedContext{
		{
			Name:       "sales",
			Definition: "Selling seats for events.",
			Aggregates: []vocab.Aggregate{{
				Name:       "Order",
				Definition: "A customer's purchase of seats for one event.",
				Identity:   "OrderID",
				Aliases:    []string{"booking"},
				Entities: []vocab.Entity{{
					Name:       "OrderLine",
					Definition: "One seat on the order.",
					Identity:   "LineNumber",
				}},
				Invariants: []vocab.Invariant{
					{Key: "total-is-sum-of-lines", Statement: "The order total equals the sum of its lines."},
				},
				Assertions: []vocab.Assertion{
					{Key: "lines-priced", On: "Confirm", Statement: "Every line carries a price once the order is confirmed."},
				},
				Repository: "OrderRepository",
			}},
			ValueObjects: []vocab.ValueObject{{
				Name:       "Money",
				Definition: "An amount in one currency.",
				Invariants: []vocab.Invariant{{Key: "never-negative", Statement: "Money is never negative."}},
			}},
			Events:         []vocab.DomainEvent{{Name: "OrderConfirmed", Definition: "The order was confirmed.", RaisedBy: "Order"}},
			Services:       []vocab.DomainService{{Name: "Pricing", Definition: "Prices an order against the event's tiers."}},
			Specifications: []vocab.Specification{{Name: "PreferredCustomer", Definition: "A customer entitled to early access."}},
			Questions:      []vocab.Question{{Key: "refund-window", Text: "How long after purchase may an order be refunded?"}},
		},
		{
			Name:       "catalog",
			Definition: "The events on sale and their seating.",
			Aggregates: []vocab.Aggregate{{
				Name:       "Event",
				Definition: "A performance with a seating plan.",
				Identity:   "EventID",
			}},
		},
	}
	relations := []vocab.ContextRelation{
		{From: "catalog", To: "sales", Kind: vocab.RelationCustomerSupplier, Description: "Sales reads seating from the catalog."},
	}
	lang, err := vocab.NewUbiquitousLanguage("ticketing", "Selling tickets.", contexts, relations)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	return lang
}

// withoutLines is the language with every Line zeroed, so a parsed
// language compares equal to one built in memory.
func withoutLines(l vocab.UbiquitousLanguage) vocab.UbiquitousLanguage {
	for i := range l.Contexts {
		c := &l.Contexts[i]
		c.Line = 0
		for j := range c.Aggregates {
			a := &c.Aggregates[j]
			a.Line = 0
			for k := range a.Entities {
				a.Entities[k].Line = 0
			}
			for k := range a.Invariants {
				a.Invariants[k].Line = 0
			}
			for k := range a.Assertions {
				a.Assertions[k].Line = 0
			}
		}
		for j := range c.ValueObjects {
			c.ValueObjects[j].Line = 0
			for k := range c.ValueObjects[j].Invariants {
				c.ValueObjects[j].Invariants[k].Line = 0
			}
		}
		for j := range c.Events {
			c.Events[j].Line = 0
		}
		for j := range c.Services {
			c.Services[j].Line = 0
		}
		for j := range c.Specifications {
			c.Specifications[j].Line = 0
		}
		for j := range c.Questions {
			c.Questions[j].Line = 0
		}
	}
	for i := range l.Relations {
		l.Relations[i].Line = 0
	}
	return l
}

// repository binds a fresh repository over a temp dir holding content,
// or over an empty dir when content is "".
func repository(t *testing.T, content string) (yamlvocab.Repository, string) {
	t.Helper()
	dir := t.TempDir()
	if content != "" {
		if err := os.WriteFile(filepath.Join(dir, vocab.UbiquitousLanguageFileName), []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	repo, err := yamlvocab.NewRepository(dir)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	return repo, dir
}

func load(t *testing.T, content string) vocab.UbiquitousLanguage {
	t.Helper()
	repo, _ := repository(t, content)
	lang, found, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatalf("RecordedLanguage: %v", err)
	}
	if !found {
		t.Fatal("found = false for a written file")
	}
	return lang
}

func loadError(t *testing.T, content string) string {
	t.Helper()
	repo, _ := repository(t, content)
	_, found, err := repo.RecordedLanguage()
	if err == nil {
		t.Fatalf("RecordedLanguage accepted:\n%s", content)
	}
	if !found {
		t.Error("found = false for a written file that failed to load")
	}
	return err.Error()
}

func TestNewRepositoryBindsTheDomainFile(t *testing.T) {
	if _, err := yamlvocab.NewRepository(""); err == nil {
		t.Error("NewRepository(\"\") accepted an empty root")
	}
	repo, dir := repository(t, "")
	if got, want := repo.Path(), filepath.Join(dir, vocab.UbiquitousLanguageFileName); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestLoadMissingFile(t *testing.T) {
	repo, _ := repository(t, "")
	lang, found, err := repo.RecordedLanguage()
	if err != nil || found || !lang.Empty() {
		t.Fatalf("missing file: lang=%+v found=%v err=%v", lang, found, err)
	}
}

func TestLoadReadsTheWholeLanguage(t *testing.T) {
	got := withoutLines(load(t, ticketingYAML))
	want := ticketing(t)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("loaded language differs from the recorded one\n got: %+v\nwant: %+v", got, want)
	}
}

func TestLoadCarriesLines(t *testing.T) {
	lang := load(t, ticketingYAML)
	sales := lang.Contexts[0]
	order := sales.Aggregates[0]
	lines := map[string]int{
		"context sales":            sales.Line,
		"aggregate Order":          order.Line,
		"entity OrderLine":         order.Entities[0].Line,
		"invariant total-is-sum":   order.Invariants[0].Line,
		"assertion lines-priced":   order.Assertions[0].Line,
		"value object Money":       sales.ValueObjects[0].Line,
		"invariant never-negative": sales.ValueObjects[0].Invariants[0].Line,
		"event OrderConfirmed":     sales.Events[0].Line,
		"service Pricing":          sales.Services[0].Line,
		"spec PreferredCustomer":   sales.Specifications[0].Line,
		"question refund-window":   sales.Questions[0].Line,
		"context catalog":          lang.Contexts[1].Line,
		"relation":                 lang.Relations[0].Line,
	}
	want := map[string]int{
		"context sales": 6, "aggregate Order": 9, "entity OrderLine": 14, "invariant total-is-sum": 18,
		"assertion lines-priced": 20, "value object Money": 25, "invariant never-negative": 28,
		"event OrderConfirmed": 30, "service Pricing": 34, "spec PreferredCustomer": 37,
		"question refund-window": 40, "context catalog": 41, "relation": 48,
	}
	for what, line := range want {
		if lines[what] != line {
			t.Errorf("%s: line %d, want %d", what, lines[what], line)
		}
	}
}

func TestLoadRejectsMalformedYAML(t *testing.T) {
	msg := loadError(t, "version: 1\nproject: [\n")
	if !strings.Contains(msg, vocab.UbiquitousLanguageFileName) {
		t.Errorf("error does not name the file: %s", msg)
	}
}

func TestLoadRejectsTheWrongVersion(t *testing.T) {
	cases := []struct{ name, content, want string }{
		{"version 2", "version: 2\nproject: x\ncontexts: {}\n", `line 1: unsupported version "2" (this arclint accepts version 1)`},
		{"missing", "project: x\ncontexts: {}\n", "version is missing (this arclint accepts version 1)"},
		{"text", "version: one\nproject: x\ncontexts: {}\n", `unsupported version "one"`},
		{"empty file", "# nothing recorded yet\n", "the file is empty"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if msg := loadError(t, c.content); !strings.Contains(msg, c.want) {
				t.Errorf("error %q lacks %q", msg, c.want)
			}
		})
	}
}

// Every block rejects a key it does not record, naming the line, the
// block, and what the block does record.
func TestLoadRejectsUnknownKeys(t *testing.T) {
	cases := []struct{ name, content, want string }{
		{
			"document", "version: 1\nproject: x\nextra: true\ncontexts: {}\n",
			`line 3: the domain file: unknown key "extra"; it records version, project, description, contexts, relations`,
		},
		{
			"context", "version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\n    entities: {}\n",
			`line 6: context "sales": unknown key "entities"; it records definition, aggregates, value_objects, events, services, specifications, questions`,
		},
		{
			"context zones", "version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\n    zones: [sales_core]\n",
			`line 6: context "sales": unknown key "zones"; it records definition, aggregates, value_objects, events, services, specifications, questions`,
		},
		{
			"aggregate", "version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\n    aggregates:\n      Order:\n        definition: d\n        identity: OrderID\n        owner: x\n",
			`line 10: context "sales": aggregate "Order": unknown key "owner"; it records definition, identity, aliases, entities, invariants, assertions, repository, factory`,
		},
		{
			"entity", "version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\n    aggregates:\n      Order:\n        definition: d\n        identity: OrderID\n        entities:\n          Line:\n            definition: d\n            invariants: {}\n",
			`line 13: context "sales": aggregate "Order": entity "Line": unknown key "invariants"; it records definition, identity, aliases`,
		},
		{
			"value object", "version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\n    value_objects:\n      Money:\n        definition: d\n        identity: MoneyID\n",
			`line 9: context "sales": value object "Money": unknown key "identity"; it records definition, aliases, invariants`,
		},
		{
			"assertion", "version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\n    aggregates:\n      Order:\n        definition: d\n        identity: OrderID\n        assertions:\n          priced:\n            on: Confirm\n            statement: s\n            owner: Order\n",
			`line 14: context "sales": aggregate "Order": assertion "priced": unknown key "owner"; it records on, statement`,
		},
		{
			"event", "version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\n    events:\n      Placed:\n        definition: d\n        aliases: [x]\n",
			`line 9: context "sales": event "Placed": unknown key "aliases"; it records definition, raised_by`,
		},
		{
			"service", "version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\n    services:\n      Pricing:\n        definition: d\n        raised_by: x\n",
			`line 9: context "sales": service "Pricing": unknown key "raised_by"; it records definition`,
		},
		{
			"specification", "version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\n    specifications:\n      Preferred:\n        definition: d\n        aliases: [x]\n",
			`line 9: context "sales": specification "Preferred": unknown key "aliases"; it records definition`,
		},
		{
			"relation", "version: 1\nproject: x\ncontexts:\n  a:\n    definition: d\n  b:\n    definition: d\nrelations:\n  - from: a\n    to: b\n    kind: conformist\n    name: x\n",
			`line 12: relation: unknown key "name"; it records from, to, kind, description`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if msg := loadError(t, c.content); !strings.Contains(msg, c.want) {
				t.Errorf("error %q lacks %q", msg, c.want)
			}
		})
	}
}

func TestLoadRejectsAKeyWrittenTwice(t *testing.T) {
	content := "version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\n  sales:\n    definition: again\n"
	if msg := loadError(t, content); !strings.Contains(msg, `line 6: the domain file: contexts: "sales" is written twice (also line 4)`) {
		t.Errorf("error %q", msg)
	}
	content = "version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\n    aggregates:\n      Order:\n        definition: d\n        identity: OrderID\n        invariants:\n          a: s\n          a: t\n"
	if msg := loadError(t, content); !strings.Contains(msg, `line 12: context "sales": aggregate "Order": invariants: "a" is written twice (also line 11)`) {
		t.Errorf("error %q", msg)
	}
}

// A value of the wrong shape is named by its block and property; the
// old list shape of contexts is the case adopters hit first.
func TestLoadRejectsTheWrongShape(t *testing.T) {
	cases := []struct{ name, content, want string }{
		{"contexts as a list", "version: 1\nproject: x\ncontexts:\n  - name: sales\n", "line 4: the domain file: contexts is not a mapping"},
		{"definition as a mapping", "version: 1\nproject: x\ncontexts:\n  sales:\n    definition:\n      text: d\n", `line 5: context "sales": definition is not text`},
		{"aliases as text", "version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\n    aggregates:\n      Order:\n        definition: d\n        identity: OrderID\n        aliases: booking\n", `line 10: context "sales": aggregate "Order": aliases is not a list`},
		{"alias not text", "version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\n    aggregates:\n      Order:\n        definition: d\n        identity: OrderID\n        aliases:\n          - a: b\n", `line 11: context "sales": aggregate "Order": aliases 1 is not text`},
		{"relations as a mapping", "version: 1\nproject: x\ncontexts: {}\nrelations:\n  a: b\n", "line 4: the domain file: relations is not a list"},
		{"relation as text", "version: 1\nproject: x\ncontexts: {}\nrelations:\n  - a-to-b\n", "line 5: relation is not a mapping"},
		{"invariant as a mapping", "version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\n    value_objects:\n      Money:\n        definition: d\n        invariants:\n          positive:\n            statement: s\n", `line 10: context "sales": value object "Money": invariant "positive" is not text`},
		{"question as a mapping", "version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\n    questions:\n      open:\n        text: t\n", `line 7: context "sales": question "open" is not text`},
		{"document as a list", "- version: 1\n", "line 1: the domain file is not a mapping"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if msg := loadError(t, c.content); !strings.Contains(msg, c.want) {
				t.Errorf("error %q lacks %q", msg, c.want)
			}
		})
	}
}

// Domain invariants speak through the adapter with the file and the
// line in front of them.
func TestLoadNamesTheFileAndLineOnDomainErrors(t *testing.T) {
	repo, _ := repository(t, "version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\n    aggregates:\n      Order:\n        definition: d\n")
	_, _, err := repo.RecordedLanguage()
	if err == nil {
		t.Fatal("accepted an aggregate without identity")
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, repo.Path()+": aggregate/identity-recorded: line 7: ") {
		t.Errorf("error = %q", msg)
	}
	if _, err := yamlvocab.Parse([]byte("version: 1\nproject: x\ncontexts:\n  sales: {}\n")); err == nil ||
		!strings.HasPrefix(err.Error(), vocab.UbiquitousLanguageFileName+": ubiquitous_language/terms-carry-definitions: line 4: ") {
		t.Errorf("Parse error = %v", err)
	}
	if _, err := (yamlvocab.Parser{}).ParseUbiquitousLanguage([]byte("version: 1\n")); err == nil {
		t.Error("Parser accepted a file without project and contexts")
	}
}

func TestLoadResolvesAnchors(t *testing.T) {
	content := "version: 1\nproject: x\ncontexts:\n  sales:\n    definition: &d Selling seats.\n  catalog:\n    definition: *d\n"
	lang := load(t, content)
	if got := lang.Contexts[1].Definition; got != "Selling seats." {
		t.Errorf("aliased definition = %q", got)
	}
}

func TestLoadAcceptsEmptySections(t *testing.T) {
	content := "version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\n    aggregates: {}\n    value_objects: {}\nrelations: []\n"
	lang := load(t, content)
	if c := lang.Counts(); c.Contexts != 1 || c.Aggregates != 0 || c.Relations != 0 {
		t.Errorf("counts = %+v", c)
	}
}

// A key written without a value is an error, as it is under the schema
// in an editor: the section is recorded or the key is left out.
func TestLoadRejectsAKeyWithoutAValue(t *testing.T) {
	cases := []struct{ name, content, want string }{
		{
			"section",
			"version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\n    aggregates:\n",
			`line 6: context "sales": aggregates has no value`,
		},
		{
			"text",
			"version: 1\nproject: x\ndescription:\ncontexts:\n  sales:\n    definition: d\n",
			"line 3: the domain file: description has no value",
		},
		{
			"list",
			"version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\n    aggregates:\n      Order:\n        definition: d\n        identity: OrderID\n        aliases:\n",
			`line 10: context "sales": aggregate "Order": aliases has no value`,
		},
		{
			"relations",
			"version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\nrelations:\n",
			"line 6: the domain file: relations has no value",
		},
		{
			"question",
			"version: 1\nproject: x\ncontexts:\n  sales:\n    definition: d\n    questions:\n      open:\n",
			`line 7: context "sales": question "open" has no value`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := loadError(t, c.content); !strings.Contains(got, c.want) {
				t.Errorf("error = %q, want it to contain %q", got, c.want)
			}
		})
	}
}

// The proving ground's recorded domain is the real oracle: it loads,
// and it round-trips through a fresh Record byte-for-content.
func TestLoadsAndRoundTripsTheBoxofficeDomain(t *testing.T) {
	path := filepath.Join(repoRoot(t), "testing", "boxoffice", vocab.UbiquitousLanguageFileName)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	lang, err := yamlvocab.Parse(content)
	if err != nil {
		t.Fatalf("Parse boxoffice: %v", err)
	}
	counts := lang.Counts()
	if counts.Contexts != 3 || counts.Aggregates != 3 || counts.ValueObjects != 6 || counts.Invariants != 16 ||
		counts.Assertions != 1 || counts.Events != 0 || counts.Questions != 5 || counts.Relations != 3 {
		t.Errorf("boxoffice counts = %+v", counts)
	}
	repo, _ := repository(t, "")
	if err := repo.Record(lang); err != nil {
		t.Fatalf("Record: %v", err)
	}
	again, _, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reflect.DeepEqual(withoutLines(again), withoutLines(lang)) {
		t.Errorf("boxoffice domain changed across Record\n got: %+v\nwant: %+v", withoutLines(again), withoutLines(lang))
	}
}

func TestRecordWritesAFreshFile(t *testing.T) {
	repo, _ := repository(t, "")
	lang := ticketing(t)
	if err := repo.Record(lang); err != nil {
		t.Fatalf("Record: %v", err)
	}
	written, err := os.ReadFile(repo.Path())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	text := string(written)
	if !strings.HasPrefix(text, "# yaml-language-server: $schema="+vocab.SchemaID+"\nversion: 1\nproject: ticketing\ndescription: Selling tickets.\ncontexts:\n  sales:\n    definition: Selling seats for events.\n    aggregates:\n      Order:\n") {
		t.Errorf("fresh file starts:\n%s", text)
	}
	for _, want := range []string{
		"        aliases: [booking]\n",
		"        entities:\n          OrderLine:\n            definition: One seat on the order.\n            identity: LineNumber\n",
		"        invariants:\n          total-is-sum-of-lines: The order total equals the sum of its lines.\n",
		"        assertions:\n          lines-priced:\n            on: Confirm\n            statement: Every line carries a price once the order is confirmed.\n",
		"        repository: OrderRepository\n",
		"    questions:\n      refund-window: How long after purchase may an order be refunded?\n",
		"relations:\n  - from: catalog\n    to: sales\n    kind: customer_supplier\n    description: Sales reads seating from the catalog.\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("fresh file lacks:\n%s\nfile:\n%s", want, text)
		}
	}
	again, _, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reflect.DeepEqual(withoutLines(again), lang) {
		t.Errorf("language changed across Record\n got: %+v\nwant: %+v", withoutLines(again), lang)
	}
}

func TestRecordKeepsContextsWhenEmpty(t *testing.T) {
	repo, _ := repository(t, "")
	lang, err := vocab.NewUbiquitousLanguage("bare", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Record(lang); err != nil {
		t.Fatalf("Record: %v", err)
	}
	written, _ := os.ReadFile(repo.Path())
	if got := string(written); !strings.HasSuffix(got, "version: 1\nproject: bare\ncontexts: {}\n") {
		t.Errorf("fresh empty file:\n%s", got)
	}
	if _, _, err := repo.RecordedLanguage(); err != nil {
		t.Errorf("reload: %v", err)
	}
}

// Long prose is written folded and filled to the line width, and reads
// back as the same text.
func TestRecordFoldsLongProse(t *testing.T) {
	repo, _ := repository(t, "")
	definition := "Selling seats for events, one order at a time, with the price captured as it was struck and never recomputed afterwards, whatever the catalog later says."
	lang, err := vocab.NewUbiquitousLanguage("ticketing", "", []vocab.BoundedContext{{Name: "sales", Definition: definition}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Record(lang); err != nil {
		t.Fatalf("Record: %v", err)
	}
	written, _ := os.ReadFile(repo.Path())
	text := string(written)
	if !strings.Contains(text, "    definition: >-\n      Selling seats") {
		t.Errorf("long prose is not folded:\n%s", text)
	}
	for _, line := range strings.Split(text, "\n") {
		if len(line) > 80 && !strings.HasPrefix(line, "# yaml-language-server") {
			t.Errorf("line wider than 80 columns: %q", line)
		}
	}
	again, _, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := again.Contexts[0].Definition; got != definition {
		t.Errorf("definition read back as %q", got)
	}
}

// A file holding only comments is written fresh, as a missing one is.
func TestRecordTreatsACommentOnlyFileAsFresh(t *testing.T) {
	repo, _ := repository(t, "# nothing recorded yet\n")
	if err := repo.Record(ticketing(t)); err != nil {
		t.Fatalf("Record: %v", err)
	}
	written, _ := os.ReadFile(repo.Path())
	if !strings.HasPrefix(string(written), "# yaml-language-server: $schema=") {
		t.Errorf("fresh file:\n%s", written)
	}
	if _, _, err := repo.RecordedLanguage(); err != nil {
		t.Errorf("reload: %v", err)
	}
}

func TestFreshModelineUsesLocalSchemaWhenPresent(t *testing.T) {
	repo, dir := repository(t, "")
	local := filepath.Join(dir, filepath.FromSlash(vocab.SchemaPath))
	if err := os.MkdirAll(filepath.Dir(local), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := repo.Record(ticketing(t)); err != nil {
		t.Fatalf("Record: %v", err)
	}
	written, _ := os.ReadFile(repo.Path())
	if !strings.HasPrefix(string(written), "# yaml-language-server: $schema="+vocab.SchemaPath+"\n") {
		t.Errorf("modeline:\n%s", string(written)[:80])
	}
}

// authored is a hand-written file with comments, folded prose, a flow
// list, and an order of its own; every untouched part must survive an
// edit byte for byte.
const authored = `# yaml-language-server: $schema=.arclint/schemas/domain.arclint.schema.json
# The box office, as the team speaks it.
version: 1
project: ticketing

contexts:
  sales: # where the money is
    definition: >-
      Selling seats for events, one order at a time, with the price
      captured as struck.
    aggregates:
      Order:
        # The heart of sales.
        definition: A customer's purchase of seats for one event.
        identity: OrderID
        invariants:
          total-is-sum-of-lines: The order total equals the sum of its lines. # never recomputed
    value_objects:
      Money:
        definition: An amount in one currency.
      Seat:
        definition: One seat in the room.
    questions:
      refund-window: How long after purchase may an order be refunded?
  catalog:
    definition: The events on sale and their seating.
    aggregates:
      Event:
        definition: A performance with a seating plan.
        identity: EventID
relations:
  - from: catalog
    to: sales
    kind: customer_supplier # sales asks, catalog answers
`

func TestRecordPreservesAuthoringAroundAnEdit(t *testing.T) {
	repo, _ := repository(t, authored)
	lang, _, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	sales := &lang.Contexts[0]
	sales.Aggregates[0].Definition = "The deal as struck."
	sales.Aggregates[0].Invariants = append(sales.Aggregates[0].Invariants, vocab.Invariant{Key: "lines-frozen", Statement: "Placed lines never change."})
	sales.Aggregates[0].Assertions = []vocab.Assertion{{Key: "lines-priced", On: "Confirm", Statement: "Every line carries a price."}}
	sales.ValueObjects = sales.ValueObjects[:1]
	sales.Events = []vocab.DomainEvent{{Name: "OrderPlaced", Definition: "A deal was struck.", RaisedBy: "Order"}}
	sales.Questions = nil
	lang.Relations[0].Kind = vocab.RelationConformist
	edited, err := vocab.NewUbiquitousLanguage(lang.Project, "", lang.Contexts, lang.Relations)
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if err := repo.Record(edited); err != nil {
		t.Fatalf("Record: %v", err)
	}
	written, _ := os.ReadFile(repo.Path())
	text := string(written)
	for _, want := range []string{
		"# The box office, as the team speaks it.\nversion: 1\nproject: ticketing\n",
		// Folded prose keeps its style; its hand wrapping is refilled.
		"  sales: # where the money is\n    definition: >-\n      Selling seats for events, one order at a time, with the price captured as\n      struck.\n",
		"      Order:\n        # The heart of sales.\n        definition: The deal as struck.\n        identity: OrderID\n",
		"          total-is-sum-of-lines: The order total equals the sum of its lines. # never recomputed\n          lines-frozen: Placed lines never change.\n",
		"        assertions:\n          lines-priced:\n            on: Confirm\n            statement: Every line carries a price.\n    value_objects:\n      Money:\n        definition: An amount in one currency.\n    events:\n      OrderPlaced:\n        definition: A deal was struck.\n        raised_by: Order\n  catalog:\n",
		"    kind: conformist # sales asks, catalog answers\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("edited file lacks:\n%s\nfile:\n%s", want, text)
		}
	}
	for _, gone := range []string{"Seat", "refund-window", "questions:"} {
		if strings.Contains(text, gone) {
			t.Errorf("edited file still holds %q:\n%s", gone, text)
		}
	}
	again, _, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reflect.DeepEqual(withoutLines(again), withoutLines(edited)) {
		t.Errorf("language changed across Record\n got: %+v\nwant: %+v", withoutLines(again), withoutLines(edited))
	}
}

func TestRecordDropsAContextAndItsRelation(t *testing.T) {
	repo, _ := repository(t, authored)
	lang, _, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	only, err := vocab.NewUbiquitousLanguage(lang.Project, lang.Description, lang.Contexts[:1], nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Record(only); err != nil {
		t.Fatalf("Record: %v", err)
	}
	written, _ := os.ReadFile(repo.Path())
	text := string(written)
	if strings.Contains(text, "catalog") || strings.Contains(text, "relations:") {
		t.Errorf("dropped context or relation survived:\n%s", text)
	}
	if !strings.Contains(text, "  sales: # where the money is\n") {
		t.Errorf("kept context lost its comment:\n%s", text)
	}
}

// A section written in another shape (a list where a mapping belongs)
// is replaced rather than edited; the file records the language again.
func TestRecordReplacesASectionOfTheWrongShape(t *testing.T) {
	repo, _ := repository(t, "version: 1\nproject: ticketing\ncontexts:\n  sales:\n    definition: d\n    aggregates: []\n")
	lang := ticketing(t)
	if err := repo.Record(lang); err != nil {
		t.Fatalf("Record: %v", err)
	}
	again, _, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reflect.DeepEqual(withoutLines(again), lang) {
		t.Errorf("language changed across Record\n got: %+v\nwant: %+v", withoutLines(again), lang)
	}
}

func TestRecordRefusesAFileThatIsNotAMapping(t *testing.T) {
	repo, _ := repository(t, "- version: 1\n")
	err := repo.Record(ticketing(t))
	if err == nil || !strings.Contains(err.Error(), "not a mapping") {
		t.Errorf("Record over a list: %v", err)
	}
}

func TestRecordAddsMissingVersionAndProject(t *testing.T) {
	repo, _ := repository(t, "contexts:\n  sales:\n    definition: d\n")
	lang, err := vocab.NewUbiquitousLanguage("ticketing", "", []vocab.BoundedContext{{Name: "sales", Definition: "d"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Record(lang); err != nil {
		t.Fatalf("Record: %v", err)
	}
	written, _ := os.ReadFile(repo.Path())
	if got := string(written); got != "version: 1\nproject: ticketing\ncontexts:\n  sales:\n    definition: d\n" {
		t.Errorf("file:\n%s", got)
	}
}

func TestRecordLeavesNoTempFiles(t *testing.T) {
	repo, dir := repository(t, "")
	if err := repo.Record(ticketing(t)); err != nil {
		t.Fatalf("Record: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != vocab.UbiquitousLanguageFileName {
			t.Errorf("unexpected file %q after Record", e.Name())
		}
	}
	info, err := os.Stat(repo.Path())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o, want 600", info.Mode().Perm())
	}
}

// A path that exists but cannot be read as a file is an error, not a
// missing file.
func TestRecordReportsUnreadableFiles(t *testing.T) {
	repo, _ := repository(t, "")
	if err := os.Mkdir(repo.Path(), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := repo.Record(ticketing(t)); err == nil {
		t.Error("Record over a directory succeeded")
	}
	if _, _, err := repo.RecordedLanguage(); err == nil {
		t.Error("RecordedLanguage over a directory succeeded")
	}
}
