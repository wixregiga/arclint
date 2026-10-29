package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/vocab"
	cmlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/cml"
	yamlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/yaml"
)

func definitionRepository(t *testing.T, root, format string) vocab.Repository {
	t.Helper()
	recording, err := vocab.NewDomainRecording("language." + format)
	if err != nil {
		t.Fatal(err)
	}
	if format == "cml" {
		repo, err := cmlvocab.NewRepository(root, recording)
		if err != nil {
			t.Fatal(err)
		}
		return repo
	}
	repo, err := yamlvocab.NewRepository(root, recording)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func assertRecordedLanguage(t *testing.T, repo vocab.Repository, want vocab.UbiquitousLanguage) {
	t.Helper()
	got, found, err := repo.RecordedLanguage()
	if err != nil || !found {
		t.Fatalf("read back: found=%v err=%v", found, err)
	}
	if difference := want.Difference(got); difference != "" {
		t.Fatalf("read back differs: %s", difference)
	}
}

func TestDomainRelationAndProjectThroughRecordingRepositories(t *testing.T) {
	for _, format := range []string{"yaml", "cml"} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			repo := definitionRepository(t, dir, format)
			lang, err := vocab.NewUbiquitousLanguage("shop", "", []vocab.BoundedContext{
				{Name: "catalog", Definition: "What can be purchased."},
				{Name: "ordering", Definition: "Taking and fulfilling orders."},
			}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := repo.Record(lang); err != nil {
				t.Fatal(err)
			}
			define, err := application.NewDefineDomainDefinition(repo, dir)
			if err != nil {
				t.Fatal(err)
			}
			remove, err := application.NewRemoveDomainDefinition(repo)
			if err != nil {
				t.Fatal(err)
			}
			for _, kind := range vocab.RelationKinds() {
				req := application.DefineRelationRequest{From: "catalog", To: "ordering", SetKind: true, Kind: string(kind), SetDescription: true, Description: "Ordering uses the catalog language."}
				if _, err := define.ExecuteRelation(req); err != nil {
					t.Fatalf("define %s: %v", kind, err)
				}
				lang.Relations = []vocab.ContextRelation{{From: req.From, To: req.To, Kind: kind, Description: req.Description}}
				assertRecordedLanguage(t, repo, lang)
				out, err := define.ExecuteRelation(req)
				if err != nil || out.Outcome != vocab.OutcomeUnchanged {
					t.Fatalf("repeat: %+v %v", out, err)
				}
			}
			if _, err := define.ExecuteRelation(application.DefineRelationRequest{From: "ordering", To: "catalog", SetDescription: true}); err != nil {
				t.Fatal(err)
			}
			lang.Relations[0].From, lang.Relations[0].To, lang.Relations[0].Description = "ordering", "catalog", ""
			assertRecordedLanguage(t, repo, lang)
			for _, description := range []string{"A shop's catalog and purchases.", ""} {
				req := application.DefineProjectRequest{SetDescription: true, Description: description}
				if _, err := define.ExecuteProject(req); err != nil {
					t.Fatal(err)
				}
				lang.Description = description
				assertRecordedLanguage(t, repo, lang)
				out, err := define.ExecuteProject(req)
				if err != nil || out.Outcome != vocab.OutcomeUnchanged {
					t.Fatalf("repeat description: %+v %v", out, err)
				}
			}
			for _, name := range []string{"boutique", "shop"} {
				out, err := define.ExecuteProject(application.DefineProjectRequest{SetName: true, Name: name})
				if err != nil || out.Outcome != vocab.OutcomeUpdated || out.Name != name {
					t.Fatalf("rename to %s: %+v %v", name, out, err)
				}
				lang.Project = name
				assertRecordedLanguage(t, repo, lang)
			}
			for _, invalid := range []struct {
				from, to, kind, rule string
			}{
				{"catalog", "catalog", "conformist", "context_map/one-relation-per-pair"},
				{"catalog", "missing", "conformist", "context_map/relations-name-recorded-contexts"},
				{"", "catalog", "conformist", "context_map/relations-name-recorded-contexts"},
				{"catalog", "ordering", "unknown", "context_relation/published-kind"},
			} {
				_, err := define.ExecuteRelation(application.DefineRelationRequest{From: invalid.from, To: invalid.to, SetKind: true, Kind: invalid.kind})
				if !errors.Is(err, application.ErrDomainUsage) || !strings.Contains(err.Error(), invalid.rule) {
					t.Fatalf("invalid relation: %v, want usage naming %s", err, invalid.rule)
				}
				assertRecordedLanguage(t, repo, lang)
			}
			if _, err := remove.ExecuteRelation("catalog", "ordering"); err != nil {
				t.Fatal(err)
			}
			lang.Relations = nil
			assertRecordedLanguage(t, repo, lang)
			if _, err := remove.ExecuteRelation("catalog", "ordering"); !errors.Is(err, vocab.ErrDefinitionNotFound) {
				t.Fatalf("missing relation: %v", err)
			}
		})
	}
}

func TestDomainRelationEditsPreserveBoxofficeLanguage(t *testing.T) {
	recording, err := vocab.NewDomainRecording("domain.arclint.cml")
	if err != nil {
		t.Fatal(err)
	}
	source, err := cmlvocab.NewRepository(repoRoot(t)+"/testing/boxoffice", recording)
	if err != nil {
		t.Fatal(err)
	}
	lang, found, err := source.RecordedLanguage()
	if err != nil || !found || len(lang.Relations) == 0 {
		t.Fatalf("boxoffice recording: found=%v err=%v", found, err)
	}
	for _, format := range []string{"yaml", "cml"} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			repo := definitionRepository(t, dir, format)
			if err := repo.Record(lang); err != nil {
				t.Fatal(err)
			}
			define, err := application.NewDefineDomainDefinition(repo, dir)
			if err != nil {
				t.Fatal(err)
			}
			remove, err := application.NewRemoveDomainDefinition(repo)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := define.ExecuteProject(application.DefineProjectRequest{SetDescription: true, Description: lang.Description}); err != nil {
				t.Fatal(err)
			}
			relation := lang.Relations[len(lang.Relations)-1]
			if _, err := remove.ExecuteRelation(relation.From, relation.To); err != nil {
				t.Fatal(err)
			}
			if _, err := define.ExecuteRelation(application.DefineRelationRequest{
				From: relation.From, To: relation.To, SetKind: true, Kind: string(relation.Kind),
				SetDescription: true, Description: relation.Description,
			}); err != nil {
				t.Fatal(err)
			}
			assertRecordedLanguage(t, repo, lang)
		})
	}
}
