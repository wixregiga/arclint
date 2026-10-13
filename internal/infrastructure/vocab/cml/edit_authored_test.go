package cmlvocab

// Edits to hand-written Context Mapper models. The four models the sync tests
// project (the Insurance example, the DDD sample's Cargo booking, the same-day
// delivery stakeholders and the workflow sample) carry far more CML than the
// vocabulary models: attributes, references, operations, use cases, flows,
// stakeholders, optional punctuation and keyword alternatives. Recording an
// unchanged language must write nothing, and one edit must leave every token
// outside the edited element exactly where it was.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

var authoredModels = []string{
	"insurance-example/Insurance-Example-Stage-5.cml",
	"ddd-sample/DDD-Sample-Stage-5.cml",
	"same-day-delivery-shop/stakeholders-and-values.cml",
	"workflow-samples/InsuranceQuoteRequest.cml",
}

// token is one lexical unit of a CML file and the line it starts on. Comments
// and strings are single tokens; whitespace is not a token.
type token struct {
	text string
	line int
}

func isWordByte(c byte) bool {
	return c == '_' || c == '.' || c == '^' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func tokensOf(data []byte) []token {
	var out []token
	s := string(data)
	line := 1
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case strings.HasPrefix(s[i:], "//"):
			end := strings.IndexByte(s[i:], '\n')
			if end < 0 {
				end = len(s) - i
			}
			out = append(out, token{strings.TrimRight(s[i:i+end], " \t\r"), line})
			i += end
		case strings.HasPrefix(s[i:], "/*"):
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				end = len(s) - i - 4
			}
			text := s[i : i+end+4]
			out = append(out, token{text, line})
			line += strings.Count(text, "\n")
			i += len(text)
		case c == '"':
			j := i + 1
			for j < len(s) && s[j] != '"' {
				if s[j] == '\\' {
					j++
				}
				j++
			}
			out = append(out, token{s[i : j+1], line})
			i = j + 1
		case isWordByte(c):
			j := i
			for j < len(s) && isWordByte(s[j]) {
				j++
			}
			out = append(out, token{s[i:j], line})
			i = j
		default:
			out = append(out, token{s[i : i+1], line})
			i++
		}
	}
	return out
}

// tokensOutside returns the tokens of a recording that no touched element
// owns, in file order.
func tokensOutside(layout recordingLayout, data []byte, scope editScope) []string {
	var out []string
	for _, tok := range tokensOf(data) {
		if !scope.allows(layout.owners[tok.line-1]) {
			out = append(out, tok.text)
		}
	}
	return out
}

type authoredEdit struct {
	name string
	// foreign says the edit touches modeled content only, so the foreign
	// projection of the model must not change at all.
	foreign bool
	// variants are tried in order; the first that Record accepts is the edit
	// under test. A removal that leaves a required reference dangling is
	// refused, and a refusal must leave the file as it was.
	variants []func(vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, bool)
}

func authoredEdits(base vocab.UbiquitousLanguage) []authoredEdit {
	inventory := inventoryOf(base)
	defineAt := func(concept vocab.Concept, at vocab.Locator, change vocab.Change) func(vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, bool) {
		return func(l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, bool) {
			next, _, err := l.Define(concept, at, change)
			return next, err == nil
		}
	}
	removeAt := func(concept vocab.Concept, at vocab.Locator) func(vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, bool) {
		return func(l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, bool) {
			next, _, err := l.Remove(concept, at)
			return next, err == nil
		}
	}
	changed := vocab.Change{SetDefinition: true, Definition: "A definition the edit wrote."}
	first := base.Contexts[0].Name
	var edits []authoredEdit
	add := func(name string, foreign bool, apply func(vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, bool)) {
		edits = append(edits, authoredEdit{name, foreign, []func(vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, bool){apply}})
	}
	addEach := func(name string, sites []site, concept vocab.Concept) {
		edit := authoredEdit{name: name}
		for _, s := range sites {
			edit.variants = append(edit.variants, removeAt(concept, s.locator()))
		}
		edits = append(edits, edit)
	}
	add("change the first context's definition", true, defineAt(vocab.ConceptBoundedContext, vocab.Locator{Name: first}, changed))
	if len(inventory.aggregates) > 0 {
		a := inventory.aggregates[0]
		add("change an aggregate's definition", true, defineAt(vocab.ConceptAggregate, a.locator(), changed))
		add("change an aggregate's aliases", false, defineAt(vocab.ConceptAggregate, a.locator(), vocab.Change{SetAliases: true, Aliases: []string{"Added alias"}}))
		add("add an entity", true, defineAt(vocab.ConceptEntity, vocab.Locator{Context: a.context, Owner: a.name, Name: "AddedEntity"}, changed))
		add("add an invariant to an aggregate", true, defineAt(vocab.ConceptInvariant, vocab.Locator{Context: a.context, Owner: a.name, Name: "added-invariant"}, vocab.Change{SetStatement: true, Statement: "An added promise."}))
		add("add an assertion to an aggregate", false, defineAt(vocab.ConceptAssertion, vocab.Locator{Context: a.context, Owner: a.name, Name: "added-assertion"}, vocab.Change{SetOn: true, On: "AddedOperation", SetStatement: true, Statement: "An added promise."}))
	}
	if len(inventory.entities) > 0 {
		add("change an entity's definition", true, defineAt(vocab.ConceptEntity, inventory.entities[0].locator(), changed))
	}
	if len(inventory.values) > 0 {
		v := inventory.values[0]
		add("change a value object's definition", true, defineAt(vocab.ConceptValueObject, v.locator(), changed))
		add("add an invariant to a value object", true, defineAt(vocab.ConceptInvariant, vocab.Locator{Context: v.context, Owner: v.name, Name: "added-invariant"}, vocab.Change{SetStatement: true, Statement: "An added promise."}))
		addEach("remove a value object", inventory.values, vocab.ConceptValueObject)
	}
	if len(inventory.events) > 0 {
		add("change an event's definition", true, defineAt(vocab.ConceptDomainEvent, inventory.events[0].locator(), changed))
		addEach("remove an event", inventory.events, vocab.ConceptDomainEvent)
	}
	if len(inventory.services) > 0 {
		s := inventory.services[0]
		add("change a service's definition", true, defineAt(vocab.ConceptDomainService, s.locator(), changed))
		add("add an assertion to a service", false, defineAt(vocab.ConceptAssertion, vocab.Locator{Context: s.context, Owner: s.name, Name: "added-assertion"}, vocab.Change{SetOn: true, On: "AddedOperation", SetStatement: true, Statement: "An added promise."}))
		addEach("remove a service", inventory.services, vocab.ConceptDomainService)
	}
	add("add an aggregate", true, defineAt(vocab.ConceptAggregate, vocab.Locator{Context: first, Name: "AddedAggregate"}, vocab.Change{SetDefinition: true, Definition: "An added aggregate.", SetIdentity: true, Identity: "AddedIdentity", SetAliases: true, Aliases: []string{"Added alias"}, SetRepository: true, Repository: "AddedRepository", SetFactory: true, Factory: "AddedFactory"}))
	add("add a value object", true, defineAt(vocab.ConceptValueObject, vocab.Locator{Context: first, Name: "AddedValue"}, changed))
	add("add a specification", true, defineAt(vocab.ConceptSpecification, vocab.Locator{Context: first, Name: "AddedSpecification"}, changed))
	add("add an event", true, defineAt(vocab.ConceptDomainEvent, vocab.Locator{Context: first, Name: "AddedEvent"}, changed))
	add("add a service", true, defineAt(vocab.ConceptDomainService, vocab.Locator{Context: first, Name: "AddedService"}, changed))
	add("add a question", true, defineAt(vocab.ConceptQuestion, vocab.Locator{Context: first, Name: "added-question"}, vocab.Change{SetText: true, Text: "An added question?"}))
	add("add a context", false, defineAt(vocab.ConceptBoundedContext, vocab.Locator{Name: "added_context"}, changed))
	for _, from := range base.Contexts {
		added := false
		for _, to := range base.Contexts {
			if _, related := base.Relation(from.Name, to.Name); from.Name != to.Name && !related {
				add("add a relation", false, func(l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, bool) {
					next, err := mutate(l, func(m *vocab.UbiquitousLanguage) error {
						m.Relations = append(m.Relations, vocab.ContextRelation{From: from.Name, To: to.Name, Kind: vocab.RelationSeparateWays, Description: "An added relation."})
						return nil
					})
					return next, err == nil
				})
				added = true
				break
			}
		}
		if added {
			break
		}
	}
	if len(base.Relations) > 0 {
		add("change a relation's description", true, func(l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, bool) {
			next, err := mutate(l, func(m *vocab.UbiquitousLanguage) error {
				m.Relations[0].Description = "A description the edit wrote."
				return nil
			})
			return next, err == nil
		})
	}
	return edits
}

func TestEditsKeepAuthoredCMLOutsideTheEditedElement(t *testing.T) {
	t.Parallel()
	for _, path := range sampledModels(authoredModels) {
		t.Run(path, func(t *testing.T) {
			original := projectableCorpus(t, path)
			originalLayout := layoutOf(t, original)
			foreignBefore := foreignContent(t, original)
			base := originalLayout.lang

			t.Run("recording the language as read writes nothing", func(t *testing.T) {
				repo := syncFixtureRepository(t, string(original))
				backdate(t, repo.Path())
				before := stateOf(t, repo.Path())
				if err := repo.Record(readSyncLanguage(t, repo)); err != nil {
					t.Fatal(err)
				}
				if message := before.differsFrom(repo.Path()); message != "" {
					t.Fatalf("the first Record of an unchanged language rewrote the file: %s", message)
				}
			})

			for index, edit := range authoredEdits(base) {
				if !sampled(index, 8) {
					continue
				}
				t.Run(edit.name, func(t *testing.T) {
					repo := syncFixtureRepository(t, string(original))
					var next vocab.UbiquitousLanguage
					accepted := false
					refused := 0
					for _, variant := range edit.variants {
						candidate, ok := variant(base)
						if !ok || base.Difference(candidate) == "" {
							t.Fatalf("the model offers no edit %q", edit.name)
						}
						backdate(t, repo.Path())
						settled := stateOf(t, repo.Path())
						err := repo.Record(candidate)
						if err == nil {
							next, accepted = candidate, true
							break
						}
						if !strings.Contains(err.Error(), "requires") {
							t.Fatalf("Record failed for a reason other than a required reference: %v", err)
						}
						if message := settled.differsFrom(repo.Path()); message != "" {
							t.Fatalf("a refused edit changed the file: %s", message)
						}
						refused++
					}
					if !accepted {
						t.Logf("Record refused all %d candidates because the model still refers to them, and left the file as it was each time", refused)
						return
					}
					after := stateOf(t, repo.Path())
					if d := next.Difference(readSyncLanguage(t, repo)); d != "" {
						t.Fatalf("the file does not read back as the requested language: %s", d)
					}
					validateContextMapper(t, after.data)

					afterLayout := layoutOf(t, after.data)
					scope := scopeOf(originalLayout, afterLayout).withServiceBlock()
					if violations := scopeViolations(scope, originalLayout, afterLayout); len(violations) > 0 {
						t.Fatalf("the edit touched lines of elements that did not change:\n  %s", strings.Join(violations, "\n  "))
					}
					want, got := tokensOutside(originalLayout, original, scope), tokensOutside(afterLayout, after.data, scope)
					if strings.Join(want, "\x00") != strings.Join(got, "\x00") {
						t.Fatalf("authored tokens outside the edited element changed: %s", firstTokenDifference(want, got))
					}
					if edit.foreign {
						if now := foreignContent(t, after.data); !bytes.Equal(foreignBefore, now) {
							t.Fatalf("foreign content changed: %s", firstDifference(foreignBefore, now))
						}
					}

					backdate(t, repo.Path())
					settled := stateOf(t, repo.Path())
					if err := repo.Record(next); err != nil {
						t.Fatal(err)
					}
					if message := settled.differsFrom(repo.Path()); message != "" {
						t.Fatalf("recording the edited language a second time rewrote the file: %s", message)
					}
					exportOracleCopy(t, fmt.Sprintf("authored-%s-%s.cml", strings.TrimSuffix(strings.ReplaceAll(path, "/", "-"), ".cml"), strings.ReplaceAll(edit.name, " ", "-")), after.data)
				})
			}
		})
	}
}

// foreignContent is the foreign projection of a model with an absent list and
// an empty one told apart no longer: removing the last declaration a slice held
// leaves it empty, and the projection must not call that a change.
func foreignContent(t *testing.T, data []byte) []byte {
	t.Helper()
	var value any
	if err := json.Unmarshal(foreignServicePlacement(t, data), &value); err != nil {
		t.Fatal(err)
	}
	var clean func(any) any
	clean = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			for key, child := range x {
				x[key] = clean(child)
			}
		case []any:
			if len(x) == 0 {
				return nil
			}
			for i, child := range x {
				x[i] = clean(child)
			}
		}
		return v
	}
	out, err := json.MarshalIndent(clean(value), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestUndoingAnAdditionRestoresTheAuthoredModel records each added element into
// the hand-written models and then the language they began with: the file must
// return to its original bytes, authored tokens, comments and layout included.
func TestUndoingAnAdditionRestoresTheAuthoredModel(t *testing.T) {
	t.Parallel()
	for _, path := range sampledModels(authoredModels) {
		t.Run(path, func(t *testing.T) {
			original := projectableCorpus(t, path)
			originalLayout := layoutOf(t, original)
			base := originalLayout.lang
			added := 0
			for _, edit := range authoredEdits(base) {
				if !strings.HasPrefix(edit.name, "add ") {
					continue
				}
				added++
				if !sampled(added-1, 8) {
					continue
				}
				t.Run(edit.name, func(t *testing.T) {
					next, ok := edit.variants[0](base)
					if !ok {
						t.Fatalf("the model offers no edit %q", edit.name)
					}
					repo := syncFixtureRepository(t, string(original))
					if err := repo.Record(next); err != nil {
						t.Fatal(err)
					}
					afterLayout := layoutOf(t, stateOf(t, repo.Path()).data)
					if err := repo.Record(base); err != nil {
						t.Fatal(err)
					}
					restored := stateOf(t, repo.Path()).data
					if bytes.Equal(restored, original) {
						return
					}
					restoredLayout := layoutOf(t, restored)
					if edit.name == "add a service" {
						// A documented service is kept in the context's direct block ahead of its
						// modules and aggregates, and the services before it are carried into the
						// block. The file keeps no record of where they stood, so they stay there
						// when the added service goes: nothing else may differ.
						if violations := scopeViolations(scopeOf(originalLayout, afterLayout).withServiceBlock(), originalLayout, restoredLayout); len(violations) > 0 {
							t.Fatalf("undoing the addition changed more than the context's services:\n  %s", strings.Join(violations, "\n  "))
						}
						return
					}
					violations, residue := residueViolations(originalLayout, restoredLayout, scopeOf(originalLayout, afterLayout))
					if len(violations) > 0 || !residue {
						t.Fatalf("undoing the addition did not restore the model (blank lines only: %v):\n  %s", !residue, strings.Join(violations, "\n  "))
					}
				})
			}
		})
	}
}

func firstTokenDifference(a, b []string) string {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			lo := max(0, i-4)
			return fmt.Sprintf("token %d: before %q (after %q); context %q", i, a[i], b[i], a[lo:min(len(a), i+3)])
		}
	}
	return fmt.Sprintf("%d tokens before, %d after", len(a), len(b))
}
