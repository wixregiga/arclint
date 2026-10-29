package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// The boxoffice language is real input. This test records it from scratch
// using only the commands the librarian uses, then reads it through each
// production adapter and compares every modeled field. A text the
// recording holds over its word limit is refused by define, naming the
// entry, and is recorded cut to its limit; the refused entries are
// exactly the ones check reports.
func TestLibrarianCommandsRecordBoxofficeLanguageInBothFormats(t *testing.T) {
	lang := boxofficeLanguage(t)
	for _, format := range []string{"yaml", "cml"} {
		t.Run(format, func(t *testing.T) {
			work := t.TempDir()
			write(t, work, "rules.arclint.yaml", "runtime: [go]\ndomain:\n  file: language."+format+"\nzones:\n  src:\n    paths: [\"src/**\"]\n")
			mustRunDomain(t, work, "agents", "skill")
			mustRunDomain(t, work, "domain", "init", "--project", lang.Project)
			var refused []string
			// within runs one define of a text: a text over the limit of its
			// kind is refused first, then recorded cut to its limit.
			within := func(kind vocab.TextKind, subject, text string, args func(text string) []string) {
				t.Helper()
				if cut, over := cutToLimit(kind, text); over {
					_, stderr, code := runBin(t, work, os.Environ(), args(text)...)
					if code != 1 || !strings.Contains(stderr, "word limit: "+subject+": ") {
						t.Fatalf("%s over its limit: exit %d, want a refusal naming it\nstderr: %s", subject, code, stderr)
					}
					refused = append(refused, subject)
					text = cut
				}
				mustRunDomain(t, work, args(text)...)
			}
			within(vocab.TextProjectDescription, fmt.Sprintf("project %q", lang.Project), lang.Description, func(text string) []string {
				return []string{"domain", "define", "project", "--description", text}
			})
			for _, ctx := range lang.Contexts {
				within(vocab.TextContextDefinition, fmt.Sprintf("context %q", ctx.Name), ctx.Definition, func(text string) []string {
					return []string{"domain", "define", "bounded_context", ctx.Name, "--definition", text}
				})
				subject := func(concept, name, owner string) string {
					if owner != "" {
						return fmt.Sprintf("%s %q of %q in context %q", concept, name, owner, ctx.Name)
					}
					return fmt.Sprintf("%s %q in context %q", concept, name, ctx.Name)
				}
				define := func(kind vocab.TextKind, concept, name, owner, flag, text string, flags ...string) {
					t.Helper()
					within(kind, subject(strings.ReplaceAll(concept, "_", " "), name, owner), text, func(text string) []string {
						args := append([]string{"domain", "define", concept, name, "--context", ctx.Name, flag, text}, flags...)
						if owner != "" {
							args = append(args, "--owner", owner)
						}
						return args
					})
				}
				invariants := func(owner string, entries []vocab.Invariant) {
					for _, entry := range entries {
						define(vocab.TextStatement, "invariant", entry.Key, owner, "--statement", entry.Statement)
					}
				}
				assertions := func(owner string, entries []vocab.Assertion) {
					for _, entry := range entries {
						define(vocab.TextStatement, "assertion", entry.Key, owner, "--statement", entry.Statement, "--on", entry.On)
					}
				}
				for _, aggregate := range ctx.Aggregates {
					flags := []string{"--identity", aggregate.Identity}
					flags = append(flags, aliasFlags(aggregate.Aliases)...)
					if aggregate.Repository != "" {
						flags = append(flags, "--repository", aggregate.Repository)
					}
					if aggregate.Factory != "" {
						flags = append(flags, "--factory", aggregate.Factory)
					}
					define(vocab.TextDefinition, "aggregate", aggregate.Name, "", "--definition", aggregate.Definition, flags...)
					for _, entity := range aggregate.Entities {
						var flags []string
						if entity.Identity != "" {
							flags = append(flags, "--identity", entity.Identity)
						}
						define(vocab.TextDefinition, "entity", entity.Name, aggregate.Name, "--definition", entity.Definition, append(flags, aliasFlags(entity.Aliases)...)...)
					}
					invariants(aggregate.Name, aggregate.Invariants)
					assertions(aggregate.Name, aggregate.Assertions)
				}
				for _, value := range ctx.ValueObjects {
					define(vocab.TextDefinition, "value_object", value.Name, "", "--definition", value.Definition, aliasFlags(value.Aliases)...)
					invariants(value.Name, value.Invariants)
				}
				for _, event := range ctx.Events {
					define(vocab.TextDefinition, "domain_event", event.Name, "", "--definition", event.Definition, "--raised-by", event.RaisedBy)
				}
				for _, service := range ctx.Services {
					define(vocab.TextDefinition, "domain_service", service.Name, "", "--definition", service.Definition)
					assertions(service.Name, service.Assertions)
				}
				for _, specification := range ctx.Specifications {
					define(vocab.TextDefinition, "specification", specification.Name, "", "--definition", specification.Definition)
				}
				for _, question := range ctx.Questions {
					mustRunDomain(t, work, "domain", "define", "question", question.Key, "--context", ctx.Name, "--text", question.Text)
				}
			}
			for _, relation := range lang.Relations {
				mustRunDomain(t, work, "domain", "define", "relation", "--from", relation.From, "--to", relation.To, "--kind", string(relation.Kind), "--description", relation.Description)
			}
			var reported []string
			for _, o := range lang.OverlongTexts() {
				reported = append(reported, o.Subject)
			}
			if strings.Join(refused, "\n") != strings.Join(reported, "\n") {
				t.Errorf("define refused\n%s\nwhile check reports\n%s", strings.Join(refused, "\n"), strings.Join(reported, "\n"))
			}
			assertRecordedLanguage(t, definitionRepository(t, work, format), withinLimits(boxofficeLanguage(t)))
			t.Logf("recorded %s: %+v; %d text(s) refused over their limit; read-back Difference empty", lang.Project, lang.Counts(), len(refused))
		})
	}
}

// cutToLimit returns text cut to the WordLimit of its kind, and whether
// it was over the limit.
func cutToLimit(kind vocab.TextKind, text string) (string, bool) {
	limit, ok := vocab.WordLimitOf(kind)
	words := strings.Fields(text)
	if !ok || len(words) <= limit.Words() {
		return text, false
	}
	return strings.Join(words[:limit.Words()], " "), true
}

// withinLimits is the language with every text over its WordLimit cut
// to the limit, as the librarian commands record it.
func withinLimits(lang vocab.UbiquitousLanguage) vocab.UbiquitousLanguage {
	cut := func(kind vocab.TextKind, text *string) { *text, _ = cutToLimit(kind, *text) }
	cut(vocab.TextProjectDescription, &lang.Description)
	for i := range lang.Contexts {
		c := &lang.Contexts[i]
		cut(vocab.TextContextDefinition, &c.Definition)
		for j := range c.Aggregates {
			a := &c.Aggregates[j]
			cut(vocab.TextDefinition, &a.Definition)
			for k := range a.Entities {
				cut(vocab.TextDefinition, &a.Entities[k].Definition)
			}
			for k := range a.Invariants {
				cut(vocab.TextStatement, &a.Invariants[k].Statement)
			}
			for k := range a.Assertions {
				cut(vocab.TextStatement, &a.Assertions[k].Statement)
			}
		}
		for j := range c.ValueObjects {
			cut(vocab.TextDefinition, &c.ValueObjects[j].Definition)
			for k := range c.ValueObjects[j].Invariants {
				cut(vocab.TextStatement, &c.ValueObjects[j].Invariants[k].Statement)
			}
		}
		for j := range c.Events {
			cut(vocab.TextDefinition, &c.Events[j].Definition)
		}
		for j := range c.Services {
			cut(vocab.TextDefinition, &c.Services[j].Definition)
			for k := range c.Services[j].Assertions {
				cut(vocab.TextStatement, &c.Services[j].Assertions[k].Statement)
			}
		}
		for j := range c.Specifications {
			cut(vocab.TextDefinition, &c.Specifications[j].Definition)
		}
	}
	return lang
}

func aliasFlags(aliases []string) []string {
	var flags []string
	for _, alias := range aliases {
		flags = append(flags, "--alias", alias)
	}
	return flags
}
