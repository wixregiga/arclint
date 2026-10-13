package main

import (
	"os"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

func TestDomainRelationAndDescriptionThroughBinary(t *testing.T) {
	for _, format := range []string{"yaml", "cml"} {
		t.Run(format, func(t *testing.T) {
			work := t.TempDir()
			write(t, work, "rules.arclint.yaml", "runtime: [go]\ndomain:\n  file: language."+format+"\nzones:\n  catalog:\n    paths: [\"catalog/**\"]\n  ordering:\n    paths: [\"ordering/**\"]\n")
			run := func(wantCode int, want string, args ...string) {
				t.Helper()
				stdout, stderr, code := runBin(t, work, os.Environ(), args...)
				if code != wantCode || !strings.Contains(stdout+stderr, want) {
					t.Fatalf("%v: exit=%d want=%d\n%s\n%s", args, code, wantCode, stdout, stderr)
				}
			}
			run(0, "Defined", "domain", "define", "bounded_context", "catalog", "--definition", "What can be purchased.")
			run(0, "Defined", "domain", "define", "bounded_context", "ordering", "--definition", "Taking and fulfilling orders.")
			run(0, "Updated description", "domain", "define", "description", "--description", "A shop's catalog and purchases.")
			run(0, "Defined relation", "domain", "define", "relation", "--from", "catalog", "--to", "ordering", "--kind", "conformist", "--description", "Ordering uses the catalog language.")
			lang := vocab.UbiquitousLanguage{Project: "", Description: "A shop's catalog and purchases.", Contexts: []vocab.BoundedContext{{Name: "catalog", Definition: "What can be purchased."}, {Name: "ordering", Definition: "Taking and fulfilling orders."}}, Relations: []vocab.ContextRelation{{From: "catalog", To: "ordering", Kind: vocab.RelationConformist, Description: "Ordering uses the catalog language."}}}
			repo := definitionRepository(t, work, format)
			got, found, err := repo.RecordedLanguage()
			if err != nil || !found {
				t.Fatalf("read back: %v %v", found, err)
			}
			lang.Project = got.Project
			assertRecordedLanguage(t, repo, lang)
			run(0, "Unchanged relation", "domain", "define", "relation", "--from", "catalog", "--to", "ordering", "--kind", "conformist")
			run(0, `"description": ""`, "domain", "define", "relation", "--from", "catalog", "--to", "ordering", "--description", "", "--format", "json")
			lang.Relations[0].Description = ""
			assertRecordedLanguage(t, repo, lang)
			run(2, "context_relation/published-kind", "domain", "define", "relation", "--from", "catalog", "--to", "ordering", "--kind", "unknown")
			run(2, "context_map/one-relation-per-pair", "domain", "define", "relation", "--from", "catalog", "--to", "catalog", "--kind", "conformist")
			run(2, "context_map/relations-name-recorded-contexts", "domain", "define", "relation", "--from", "catalog", "--to", "missing", "--kind", "conformist")
			run(2, "does not apply", "domain", "define", "description", "--description", "x", "--definition", "wrong")
			run(2, "requires --description", "domain", "define", "description")
			run(2, "does not apply", "domain", "define", "value_object", "Money", "--description", "wrong")
			run(0, "relation", "__complete", "domain", "define", "rel")
			run(0, "description", "__complete", "domain", "define", "desc")
			run(0, "relation", "__complete", "domain", "remove", "rel")
			run(0, "conformist", "__complete", "domain", "define", "relation", "--kind", "conf")
			run(0, "catalog", "__complete", "domain", "define", "relation", "--from", "cat")
			run(0, "ordering", "__complete", "domain", "remove", "relation", "--to", "ord")
			run(0, "Removed relation", "domain", "remove", "relation", "--from", "ordering", "--to", "catalog")
			lang.Relations = nil
			assertRecordedLanguage(t, repo, lang)
			run(1, "domain definition not found", "domain", "remove", "relation", "--from", "catalog", "--to", "ordering")
		})
	}
}
