package yamlvocab_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	yamlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/yaml"
)

// linedModel exercises what real vocabulary files do to line numbers:
// folded and literal multi-line definitions push later entries down,
// one term records its name after its definition, and a second context
// follows the first.
const linedModel = `version: 1
contexts:
  - name: catalog
    entities:
      - name: Event
        definition: >-
          One show an Organizer puts on sale: its title, when and
          where it happens, and its TicketTiers. An Event stays a
          draft until its Organizer publishes it.
        aggregate: true
      - name: Organizer
        definition: The person whose page it is.
    value_objects:
      - name: TicketTier
        definition: |
          One named offer within an Event, like general or front-row.
          A TicketTier without a Price is not on sale.
      - definition: What one ticket of a TicketTier costs.
        name: Price
    invariants:
      - statement: >-
          An Event sells tickets only while it is published; a draft
          Event is invisible to ordering and capacity alike.
        owner: Event
      - statement: Every TicketTier carries a Price.
        owner: TicketTier
    events:
      - name: EventPublished
        definition: An Organizer put an Event on sale.
  - name: ordering
    entities:
      - name: Order
        definition: The deal as struck.
relations:
  - from: catalog
    to: ordering
    kind: customer_supplier
`

// aliasedModel records one term once and names it again from a second
// context, so an entry can appear on a line other than the one it is
// written on.
const aliasedModel = `version: 1
contexts:
  - name: catalog
    entities:
      - &show
        name: Event
        definition: One show on sale.
  - name: archive
    entities:
      - *show
`

// TestParseRecordsEntryLines proves every recorded entry carries the
// line it is written on, so a finding about the entry can point at it.
func TestParseRecordsEntryLines(t *testing.T) {
	lang, err := yamlvocab.Parse([]byte(linedModel))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(lang.Contexts) != 2 || len(lang.Relations) != 1 {
		t.Fatalf("parsed %d contexts and %d relations, want 2 and 1",
			len(lang.Contexts), len(lang.Relations))
	}
	catalog, ordering := lang.Contexts[0], lang.Contexts[1]

	for _, tc := range []struct {
		what  string
		got   int
		entry string
	}{
		{"context catalog", catalog.Line, "- name: catalog"},
		{"entity Event", catalog.Entities[0].Line, "- name: Event"},
		{"entity Organizer", catalog.Entities[1].Line, "- name: Organizer"},
		{"value object TicketTier", catalog.ValueObjects[0].Line, "- name: TicketTier"},
		{"value object Price", catalog.ValueObjects[1].Line, "name: Price"},
		{"invariant on Event", catalog.Invariants[0].Line, "- statement: >-"},
		{"invariant on TicketTier", catalog.Invariants[1].Line, "- statement: Every TicketTier carries a Price."},
		{"event EventPublished", catalog.Events[0].Line, "- name: EventPublished"},
		{"context ordering", ordering.Line, "- name: ordering"},
		{"entity Order", ordering.Entities[0].Line, "- name: Order"},
		{"relation catalog -> ordering", lang.Relations[0].Line, "- from: catalog"},
	} {
		if want := lineOf(t, linedModel, tc.entry); tc.got != want {
			t.Errorf("%s: line = %d, want %d (%q)", tc.what, tc.got, want, tc.entry)
		}
	}
}

// TestParseRecordsLinesThroughAliases proves a term named again by
// alias anchors where the reader finds it in that context's list, not
// where the term was first written.
func TestParseRecordsLinesThroughAliases(t *testing.T) {
	lang, err := yamlvocab.Parse([]byte(aliasedModel))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(lang.Contexts) != 2 {
		t.Fatalf("parsed %d contexts, want 2", len(lang.Contexts))
	}
	written := lineOf(t, aliasedModel, "name: Event")
	if got := lang.Contexts[0].Entities[0].Line; got != written {
		t.Errorf("catalog Event: line = %d, want %d", got, written)
	}
	named := lineOf(t, aliasedModel, "- *show")
	if got := lang.Contexts[1].Entities[0].Line; got != named {
		t.Errorf("archive Event: line = %d, want %d", got, named)
	}
}

// TestRecordedLanguageCarriesEntryLines proves the repository file path
// carries the same lines the fixture-bytes path does.
func TestRecordedLanguageCarriesEntryLines(t *testing.T) {
	dir := t.TempDir()
	writeModel(t, dir, linedModel)
	lang, found, err := newRepo(t, dir).RecordedLanguage()
	if err != nil {
		t.Fatalf("RecordedLanguage: %v", err)
	}
	if !found {
		t.Fatal("found = false for a written file")
	}
	inv, ok := lang.FindInvariant("catalog", lang.Contexts[0].Invariants[0].Statement)
	if !ok {
		t.Fatal("FindInvariant missed the recorded invariant")
	}
	if want := lineOf(t, linedModel, "- statement: >-"); inv.Line != want {
		t.Errorf("invariant line = %d, want %d", inv.Line, want)
	}
	if want := lineOf(t, linedModel, "- name: Event"); lang.Contexts[0].Entities[0].Line != want {
		t.Errorf("entity line = %d, want %d", lang.Contexts[0].Entities[0].Line, want)
	}
}

// TestDefinedEntryCarriesNoLine proves a term the project has not
// written down yet reports no line rather than a borrowed one.
func TestDefinedEntryCarriesNoLine(t *testing.T) {
	lang, err := yamlvocab.Parse([]byte(linedModel))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, _, err := lang.Define(vocab.ConceptValueObject, "catalog", "SeatMap",
		vocab.Change{SetDefinition: true, DefinitionText: "Where the seats are."})
	if err != nil {
		t.Fatalf("Define: %v", err)
	}
	def, ok := out.Find(vocab.ConceptValueObject, "catalog", "SeatMap")
	if !ok {
		t.Fatal("Find missed the defined value object")
	}
	if def.Line != 0 {
		t.Errorf("line = %d, want 0 for a term that is not written down yet", def.Line)
	}
	if want := lineOf(t, linedModel, "- name: TicketTier"); out.Contexts[0].ValueObjects[0].Line != want {
		t.Errorf("recorded neighbour line = %d, want %d", out.Contexts[0].ValueObjects[0].Line, want)
	}
}

// lineOf returns the one-based line of the fixture line whose content
// is entry, so expectations name the entry instead of an offset.
func lineOf(t *testing.T, model, entry string) int {
	t.Helper()
	found := 0
	for i, line := range strings.Split(model, "\n") {
		if strings.TrimSpace(line) != entry {
			continue
		}
		if found != 0 {
			t.Fatalf("fixture line %q is not unique", entry)
		}
		found = i + 1
	}
	if found == 0 {
		t.Fatalf("fixture records no line %q", entry)
	}
	return found
}
