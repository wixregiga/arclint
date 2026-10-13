package cmlvocab

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	"github.com/wixregiga/arclint/internal/infrastructure/vocab/cml/syntax"
)

func TestRecordDocumentedServiceBlock(t *testing.T) {
	for _, body := range []string{
		`Module sales { "An id." ValueObject OrderId }
/** A worker. */ Service Worker { hint = "foreign=yes" void work(); }`,
		`Aggregate Order { "An order." Entity Order { aggregateRoot OrderId id key } }
/** A worker. */ Service Worker { hint = "foreign=yes" void work(); }`,
		`/** A worker. */ Service Worker { hint = "foreign=yes" void work(); }
Module sales { "An id." ValueObject OrderId }`,
		`Module sales { "An id." ValueObject OrderId
/** A worker. */ Service Worker { hint = "foreign=yes" void work(); } }`,
	} {
		t.Run(body, func(t *testing.T) {
			source := "Domain Shop\nBoundedContext sales {\n domainVisionStatement \"Selling.\"\n" + body + "\n}"
			repo := syncFixtureRepository(t, source)
			lang := readSyncLanguage(t, repo)
			assertRecorded(t, repo, lang)
			if readSyncFile(t, repo) != source {
				t.Fatal("unchanged Record moved authored services")
			}
			foreign := foreignServicePlacement(t, []byte(source))
			lang.Contexts[0].Services = append(lang.Contexts[0].Services, vocab.DomainService{Name: "AddedService", Definition: "An added service."})
			assertRecorded(t, repo, lang)
			data := []byte(readSyncFile(t, repo))
			file, err := syntax.Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			bc := file.BoundedContexts[0]
			if len(bc.DomainServices) != 2 || bc.DomainServices[0].Name != "Worker" || bc.DomainServices[1].Name != "AddedService" {
				t.Fatalf("wrong direct services block: %s", data)
			}
			last := bc.DomainServices[1].Pos.Offset
			for _, m := range bc.Modules {
				if m.Pos.Offset < last {
					t.Fatal("services follow Module")
				}
			}
			for _, a := range bc.Aggregates {
				if a.Pos.Offset < last {
					t.Fatal("services follow Aggregate")
				}
			}
			if !strings.Contains(string(data), "A worker.") || !strings.Contains(string(data), "foreign=yes") || len(bc.DomainServices[0].Operations) != 1 || bc.DomainServices[0].Operations[0].Name != "work" {
				t.Fatalf("authored service changed: %s", data)
			}
			if !bytes.Equal(foreign, foreignServicePlacement(t, data)) {
				t.Fatal("service placement changed foreign bodies")
			}
			assertRecorded(t, repo, lang)
			if !bytes.Equal(data, []byte(readSyncFile(t, repo))) {
				t.Fatal("unchanged second Record wrote bytes")
			}
		})
	}
}

func TestRecordServiceDefinitionMovesWholeBlock(t *testing.T) {
	source := `Domain Shop
BoundedContext sales {
 domainVisionStatement "Selling."
 Module sales { "An id." ValueObject OrderId }
 /** First worker. */ Service First { void first(); }
 /** Second worker. */ Service Second { void second(); }
}`
	repo := syncFixtureRepository(t, source)
	lang := readSyncLanguage(t, repo)
	lang.Contexts[0].Services[1].Definition = "Changed second worker."
	assertRecorded(t, repo, lang)
	data := []byte(readSyncFile(t, repo))
	file, err := syntax.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	bc := file.BoundedContexts[0]
	if len(bc.DomainServices) != 2 || bc.DomainServices[0].Name != "First" || bc.DomainServices[1].Name != "Second" || bc.DomainServices[1].Pos.Offset > bc.Modules[0].Pos.Offset {
		t.Fatalf("whole block was not moved in order: %s", data)
	}
}

// Only the modeled service placement is ignored. Context ownership, each complete
// service body, every comment, and all other foreign grammar remain comparable.
func foreignServicePlacement(t *testing.T, data []byte) []byte {
	t.Helper()
	var file map[string]any
	if err := json.Unmarshal(foreignProjection(t, data), &file); err != nil {
		t.Fatal(err)
	}
	contexts, _ := file["BoundedContexts"].([]any)
	for _, value := range contexts {
		bc := value.(map[string]any)
		var services []any
		var collect func(map[string]any, string)
		collect = func(owner map[string]any, field string) {
			if items, ok := owner[field].([]any); ok {
				services = append(services, items...)
			}
			owner[field] = nil
			for _, key := range []string{"Modules", "Aggregates"} {
				if containers, ok := owner[key].([]any); ok {
					for _, item := range containers {
						collect(item.(map[string]any), "Services")
					}
				}
			}
		}
		collect(bc, "DomainServices")
		sort.SliceStable(services, func(i, j int) bool {
			a, _ := json.Marshal(services[i])
			b, _ := json.Marshal(services[j])
			return bytes.Compare(a, b) < 0
		})
		bc["DomainServices"] = services
	}
	result, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return result
}
