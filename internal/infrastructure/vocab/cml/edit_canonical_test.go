package cmlvocab

// Regression tests for the defects the edit property tests found. Each one is
// the smallest edit that reproduces a failure, run against a real recording
// (the boxoffice proving ground) or a small hand-written model, and states the
// exact bytes the edit must leave.

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	"github.com/wixregiga/arclint/internal/infrastructure/vocab/cml/syntax"
)

const committedBoxoffice = "../../../../testing/boxoffice/domain.arclint.cml"

func boxofficeCopy(t *testing.T) (Repository, []byte) {
	t.Helper()
	data, err := os.ReadFile(committedBoxoffice)
	if err != nil {
		t.Fatal(err)
	}
	return syncFixtureRepository(t, string(data)), data
}

func recordEdit(t *testing.T, repo Repository, concept vocab.Concept, at vocab.Locator, change vocab.Change) string {
	t.Helper()
	next, _, err := readSyncLanguage(t, repo).Define(concept, at, change)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Record(next); err != nil {
		t.Fatal(err)
	}
	return readSyncFile(t, repo)
}

func recordRemoval(t *testing.T, repo Repository, concept vocab.Concept, at vocab.Locator) string {
	t.Helper()
	next, _, err := readSyncLanguage(t, repo).Remove(concept, at)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Record(next); err != nil {
		t.Fatal(err)
	}
	return readSyncFile(t, repo)
}

func canonicalCML(t *testing.T, text string) string {
	t.Helper()
	file, err := syntax.Parse([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	data, err := syntax.FormatChecked(file)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func requireBytes(t *testing.T, got, want, doing string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s:\n%s\nwant:\n%s", doing, got, want)
	}
}

// Clearing the aliases that gave a bare value object its braces takes the
// braces away: the file is what it was before the aliases.
func TestClearedAliasesTakeTheirBracesAway(t *testing.T) {
	repo, original := boxofficeCopy(t)
	at := vocab.Locator{Context: "catalog", Name: "TicketTier"}
	withAlias := recordEdit(t, repo, vocab.ConceptValueObject, at, vocab.Change{SetAliases: true, Aliases: []string{"Tier"}})
	if !strings.Contains(withAlias, `hint = "arclint.alias=Tier"`) {
		t.Fatalf("the alias was not written:\n%s", withAlias)
	}
	requireBytes(t, recordEdit(t, repo, vocab.ConceptValueObject, at, vocab.Change{SetAliases: true}), string(original), "clearing the aliases left a trace")
}

// Clearing an aggregate's repository removes the hint that carried it, not
// just the value: no hint = "" is left on the root.
func TestClearedRepositoryLeavesNoEmptyHint(t *testing.T) {
	repo, original := boxofficeCopy(t)
	at := vocab.Locator{Context: "catalog", Name: "Event"}
	withRepository := recordEdit(t, repo, vocab.ConceptAggregate, at, vocab.Change{SetRepository: true, Repository: "event store"})
	if !strings.Contains(withRepository, `arclint.repository=event store`) {
		t.Fatalf("the repository was not written:\n%s", withRepository)
	}
	cleared := recordEdit(t, repo, vocab.ConceptAggregate, at, vocab.Change{SetRepository: true})
	if strings.Contains(cleared, `hint = ""`) {
		t.Fatalf("an empty hint was left behind:\n%s", cleared)
	}
	requireBytes(t, cleared, string(original), "clearing the repository left a trace")
}

// The Module a context's value objects live in appears with the first of them
// and goes with the last, so adding one and removing it again is no change.
// A Module the author left empty is not the edit's to remove.
func TestModuleComesAndGoesWithItsMembers(t *testing.T) {
	t.Parallel()
	repo, _ := boxofficeCopy(t)
	withContext := recordEdit(t, repo, vocab.ConceptBoundedContext, vocab.Locator{Name: "billing"}, definitionChange("Billing."))
	value := vocab.Locator{Context: "billing", Name: "Invoice"}
	withValue := recordEdit(t, repo, vocab.ConceptValueObject, value, definitionChange("What a customer is asked to pay."))
	if !strings.Contains(withValue, "Module billing {") {
		t.Fatalf("the value object was not written into a Module:\n%s", withValue)
	}
	requireBytes(t, recordRemoval(t, repo, vocab.ConceptValueObject, value), withContext, "the Module stayed after its last member went")

	authored := strings.Replace(withContext, "domainVisionStatement = \"Billing.\"\n", "domainVisionStatement = \"Billing.\"\n  Module billing {}\n", 1)
	if authored == withContext {
		t.Fatal("the authored Module was not placed")
	}
	if err := os.WriteFile(repo.Path(), []byte(authored), 0o600); err != nil {
		t.Fatal(err)
	}
	edited := recordEdit(t, repo, vocab.ConceptBoundedContext, vocab.Locator{Name: "billing"}, definitionChange("Billing, changed."))
	if !strings.Contains(edited, "Module billing {}") {
		t.Fatalf("an authored empty Module was removed:\n%s", edited)
	}
}

func definitionChange(text string) vocab.Change {
	return vocab.Change{SetDefinition: true, Definition: text}
}

// A relation that changes kind is written in the canonical notation of its
// new kind, whatever notation the old kind was parsed in; changing the kind
// back gives the file back.
func TestKindChangeWritesTheCanonicalNotation(t *testing.T) {
	repo, original := boxofficeCopy(t)
	relation := func(kind vocab.RelationKind) string {
		next, err := mutate(readSyncLanguage(t, repo), func(l *vocab.UbiquitousLanguage) error {
			for i := range l.Relations {
				if l.Relations[i].From == "catalog" && l.Relations[i].To == "ordering" {
					l.Relations[i].Kind = kind
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.Record(next); err != nil {
			t.Fatal(err)
		}
		return readSyncFile(t, repo)
	}
	if partnership := relation(vocab.RelationPartnership); !strings.Contains(partnership, "catalog [P]<->[P] ordering") {
		t.Fatalf("the partnership is not in canonical notation:\n%s", partnership)
	}
	requireBytes(t, relation(vocab.RelationConformist), string(original), "the conformist relation did not come back as it was")
}

const interleavedModule = `Domain Shop {
  domainVisionStatement = "A shop."
}

ContextMap {
  contains sales
}

BoundedContext sales {
  domainVisionStatement = "Selling."
  Module sales {
    "First value."
    ValueObject Alpha

    "First event."
    DomainEvent E1

    "Second value."
    ValueObject Gamma

    "Second event."
    DomainEvent E2
  }
}
`

// A declaration added in the middle of its kind is written beside the one
// before it. The declarations of the kind after it stay where they are, and so
// does everything of another kind written between them.
func TestInsertedDeclarationsShiftNothingElse(t *testing.T) {
	repo := syncFixtureRepository(t, canonicalCML(t, interleavedModule))
	before := readSyncLanguage(t, repo)
	inserted, err := mutate(before, func(l *vocab.UbiquitousLanguage) error {
		c := &l.Contexts[0]
		c.ValueObjects = []vocab.ValueObject{c.ValueObjects[0], {Name: "Beta", Definition: "Between value."}, c.ValueObjects[1]}
		c.Events = []vocab.DomainEvent{c.Events[0], {Name: "E15", Definition: "Between event."}, c.Events[1]}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Record(inserted); err != nil {
		t.Fatal(err)
	}
	want := canonicalCML(t, strings.Replace(strings.Replace(interleavedModule,
		"    \"First event.\"", "    \"Between value.\"\n    ValueObject Beta\n\n    \"First event.\"", 1),
		"    \"Second value.\"", "    \"Between event.\"\n    DomainEvent E15\n\n    \"Second value.\"", 1))
	requireBytes(t, readSyncFile(t, repo), want, "an insertion moved a declaration of another place")
}

const aggregateWithOperations = `Domain Shop {
  domainVisionStatement = "A shop."
}

ContextMap {
  contains sales
}

BoundedContext sales {
  domainVisionStatement = "Selling."
  Aggregate Order {
    "An order."
    Entity Order {
      aggregateRoot
      OrderID id key
      "Placed means priced."
      def void assertPlacedPriced() hint = "arclint.on=Place";
      def void Place();
      def void Cancel();
    }
  }

  Module sales {
    "The order's identity."
    ValueObject OrderID
  }
}
`

// An invariant added to a root that already has an assertion is written before
// it; the assertion does not move past the operations written after it.
func TestAddedInvariantLeavesTheAssertionWhereItStands(t *testing.T) {
	repo := syncFixtureRepository(t, canonicalCML(t, aggregateWithOperations))
	recordEdit(t, repo, vocab.ConceptInvariant, vocab.Locator{Context: "sales", Owner: "Order", Name: "priced-once"}, vocab.Change{SetStatement: true, Statement: "An order is priced once."})
	want := canonicalCML(t, strings.Replace(aggregateWithOperations,
		"      \"Placed means priced.\"", "      \"An order is priced once.\"\n      def void ensurePricedOnce();\n      \"Placed means priced.\"", 1))
	requireBytes(t, readSyncFile(t, repo), want, "the new invariant displaced the assertion")
}

// An alias added to a root that already carries a repository hint is written
// where a fresh recording writes it, ahead of the repository.
func TestHintEntriesAreWrittenInTheOrderOfAFreshRecording(t *testing.T) {
	repo, _ := boxofficeCopy(t)
	at := vocab.Locator{Context: "catalog", Name: "Event"}
	recordEdit(t, repo, vocab.ConceptAggregate, at, vocab.Change{SetRepository: true, Repository: "event store"})
	both := recordEdit(t, repo, vocab.ConceptAggregate, at, vocab.Change{SetAliases: true, Aliases: []string{"Show"}})
	if !strings.Contains(both, `hint = "arclint.alias=Show,arclint.repository=event store"`) {
		t.Fatalf("the hint entries are not in the order a fresh recording writes them:\n%s", both)
	}
	fresh, err := Render(readSyncLanguage(t, repo))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(fresh, []byte(`hint = "arclint.alias=Show,arclint.repository=event store"`)) {
		t.Fatalf("a fresh recording writes them differently:\n%s", fresh)
	}
}
