package cmlvocab

import (
	"os"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	"github.com/wixregiga/arclint/internal/infrastructure/vocab/cml/syntax"
)

func TestRecordRepositoryCollisions(t *testing.T) {
	for _, authored := range []bool{false, true} {
		t.Run(map[bool]string{false: "modeled", true: "authored"}[authored], func(t *testing.T) {
			source := `Domain Project
BoundedContext one {
  domainVisionStatement "First context."
  Aggregate First {
    "First root." Entity First {
      hint = "foreign=kept"
      aggregateRoot FirstId id key
      Repository CommonRepository
    }
  }
}`
			if authored {
				source = strings.Replace(source, "Domain Project", `Domain Project { Subdomain External { "Foreign entity." Entity Outside { aggregateRoot Repository AuthoredRepository { find(); } } } }`, 1)
			}
			repo, err := NewRepository(t.TempDir(), recording(t))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(repo.Path(), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			lang, _, err := repo.RecordedLanguage()
			if err != nil {
				t.Fatal(err)
			}
			if authored {
				lang.Contexts[0].Aggregates[0].Repository = "AuthoredRepository"
			} else {
				lang.Contexts = append(lang.Contexts, vocab.BoundedContext{Name: "two", Definition: "Second context.", Aggregates: []vocab.Aggregate{{Name: "Second", Definition: "Second root.", Identity: "SecondId", Repository: "CommonRepository"}}})
			}
			assertRecorded(t, repo, lang)
			data, err := os.ReadFile(repo.Path())
			if err != nil {
				t.Fatal(err)
			}
			assertRoundTrip(t, lang, data)
			if !strings.Contains(string(data), "foreign=kept,arclint.repository=") {
				t.Fatalf("collision must migrate the native repository to a hint while keeping foreign hints:\n%s", data)
			}
			file, err := syntax.Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			natives := 0
			visitRepositories(file, nil, func(repo *syntax.Repository) {
				natives++
				if !authored || repo.Name != "AuthoredRepository" || len(repo.Operations) != 1 {
					t.Fatalf("unexpected or modified authored repository: %#v", repo)
				}
			})
			if authored && natives != 1 {
				t.Fatal("authored Repository was removed")
			}
			before := string(data)
			if err := repo.Record(lang); err != nil {
				t.Fatal(err)
			}
			data, err = os.ReadFile(repo.Path())
			if err != nil || string(data) != before {
				t.Fatalf("no-op Record changed the file: %v", err)
			}
			validateContextMapper(t, data)
		})
	}
}

func TestRecordRepositoryRenamePreservesBody(t *testing.T) {
	for _, name := range []string{"OrderStoreRepository", "OrderStore", "Order Repository"} {
		t.Run(name, func(t *testing.T) {
			source := `Domain Shop
BoundedContext sales {
 domainVisionStatement "Selling."
 Aggregate Order {
  "An order." Entity Order {
   aggregateRoot OrderId id key
   Repository OrderRepository { // authored operation
    @Order find(@OrderId id);
   }
  }
 }
 Module sales { "An order identity." ValueObject OrderId }
}`
			repo, err := NewRepository(t.TempDir(), recording(t))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(repo.Path(), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			lang, _, err := repo.RecordedLanguage()
			if err != nil {
				t.Fatal(err)
			}
			lang.Contexts[0].Aggregates[0].Repository = name
			err = repo.Record(lang)
			if name != "OrderStoreRepository" {
				if err == nil || !strings.Contains(err.Error(), "OrderRepository") || !strings.Contains(err.Error(), ":7:") || !strings.Contains(err.Error(), "authored body") {
					t.Fatalf("missing positioned refusal: %v", err)
				}
				data, readErr := os.ReadFile(repo.Path())
				if readErr != nil || string(data) != source {
					t.Fatalf("refused edit wrote file: %v", readErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(repo.Path())
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), "@Order find(@OrderId id)") || !strings.Contains(string(data), "authored operation") {
				t.Fatal("authored repository body lost")
			}
			validateContextMapper(t, data)
			lang.Contexts[0].Aggregates[0].Repository = "OrderRepository"
			assertRecorded(t, repo, lang)
		})
	}
}

func TestRecordRepositoryRefusalPreservesEveryBodyKind(t *testing.T) {
	for _, body := range []string{"hint = \"\"", "gap", "nogap", "subscribe to orders", "inject @Worker", "// Authored repository comment\n", "find();"} {
		t.Run(body, func(t *testing.T) {
			source := `Domain Shop
BoundedContext sales {
 domainVisionStatement "Selling."
 "A worker." Service Worker
 Aggregate Order { "An order." Entity Order { aggregateRoot OrderId id key Repository OrderRepository { ` + body + ` } } }
}`
			repo := syncFixtureRepository(t, source)
			lang := readSyncLanguage(t, repo)
			lang.Contexts[0].Aggregates[0].Repository = "OrderStore"
			if err := repo.Record(lang); err == nil || !strings.Contains(err.Error(), "authored body") {
				t.Fatalf("missing body refusal: %v", err)
			}
			if readSyncFile(t, repo) != source {
				t.Fatal("refusal changed file")
			}
		})
	}
}

func TestRecordEmptyRepositoryUsesHintAndKeepsLeadingComment(t *testing.T) {
	source := `Domain Shop
BoundedContext sales {
 domainVisionStatement "Selling."
 Aggregate Order { "An order." Entity Order {
 aggregateRoot OrderId id key
 // Storage responsibility.
 Repository OrderRepository {}
 } }
}`
	repo := syncFixtureRepository(t, source)
	lang := readSyncLanguage(t, repo)
	lang.Contexts[0].Aggregates[0].Repository = "OrderStore"
	assertRecorded(t, repo, lang)
	if !strings.Contains(readSyncFile(t, repo), "Storage responsibility.") || !strings.Contains(readSyncFile(t, repo), "arclint.repository=OrderStore") {
		t.Fatal("empty repository migration lost comment or name")
	}
}
