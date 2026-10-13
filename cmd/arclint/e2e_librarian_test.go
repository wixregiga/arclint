package main

import (
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// The boxoffice language is real input. This test records it from scratch
// using only the commands the librarian uses, then reads it through each
// production adapter and compares every modeled field.
func TestLibrarianCommandsRecordBoxofficeLanguageInBothFormats(t *testing.T) {
	lang := boxofficeLanguage(t)
	for _, format := range []string{"yaml", "cml"} {
		t.Run(format, func(t *testing.T) {
			work := t.TempDir()
			write(t, work, "rules.arclint.yaml", "runtime: [go]\ndomain:\n  file: language."+format+"\nzones:\n  src:\n    paths: [\"src/**\"]\n")
			mustRunDomain(t, work, "agents", "skill")
			mustRunDomain(t, work, "domain", "init", "--project", lang.Project)
			mustRunDomain(t, work, "domain", "define", "description", "--description", lang.Description)
			for _, ctx := range lang.Contexts {
				mustRunDomain(t, work, "domain", "define", "bounded_context", ctx.Name, "--definition", ctx.Definition)
				define := func(concept, name string, flags ...string) {
					t.Helper()
					args := append([]string{"domain", "define", concept, name, "--context", ctx.Name}, flags...)
					mustRunDomain(t, work, args...)
				}
				invariants := func(owner string, entries []vocab.Invariant) {
					for _, entry := range entries {
						define("invariant", entry.Key, "--owner", owner, "--statement", entry.Statement)
					}
				}
				assertions := func(owner string, entries []vocab.Assertion) {
					for _, entry := range entries {
						define("assertion", entry.Key, "--owner", owner, "--on", entry.On, "--statement", entry.Statement)
					}
				}
				for _, aggregate := range ctx.Aggregates {
					flags := []string{"--definition", aggregate.Definition, "--identity", aggregate.Identity}
					flags = append(flags, aliasFlags(aggregate.Aliases)...)
					if aggregate.Repository != "" {
						flags = append(flags, "--repository", aggregate.Repository)
					}
					if aggregate.Factory != "" {
						flags = append(flags, "--factory", aggregate.Factory)
					}
					define("aggregate", aggregate.Name, flags...)
					for _, entity := range aggregate.Entities {
						flags := []string{"--owner", aggregate.Name, "--definition", entity.Definition}
						if entity.Identity != "" {
							flags = append(flags, "--identity", entity.Identity)
						}
						define("entity", entity.Name, append(flags, aliasFlags(entity.Aliases)...)...)
					}
					invariants(aggregate.Name, aggregate.Invariants)
					assertions(aggregate.Name, aggregate.Assertions)
				}
				for _, value := range ctx.ValueObjects {
					flags := append([]string{"--definition", value.Definition}, aliasFlags(value.Aliases)...)
					define("value_object", value.Name, flags...)
					invariants(value.Name, value.Invariants)
				}
				for _, event := range ctx.Events {
					define("domain_event", event.Name, "--definition", event.Definition, "--raised-by", event.RaisedBy)
				}
				for _, service := range ctx.Services {
					define("domain_service", service.Name, "--definition", service.Definition)
					assertions(service.Name, service.Assertions)
				}
				for _, specification := range ctx.Specifications {
					define("specification", specification.Name, "--definition", specification.Definition)
				}
				for _, question := range ctx.Questions {
					define("question", question.Key, "--text", question.Text)
				}
			}
			for _, relation := range lang.Relations {
				mustRunDomain(t, work, "domain", "define", "relation", "--from", relation.From, "--to", relation.To, "--kind", string(relation.Kind), "--description", relation.Description)
			}
			assertRecordedLanguage(t, definitionRepository(t, work, format), lang)
			t.Logf("recorded %s: %+v; read-back Difference empty", lang.Project, lang.Counts())
		})
	}
}

func aliasFlags(aliases []string) []string {
	var flags []string
	for _, alias := range aliases {
		flags = append(flags, "--alias", alias)
	}
	return flags
}
