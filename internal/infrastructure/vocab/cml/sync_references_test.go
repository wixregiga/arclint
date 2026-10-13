package cmlvocab

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/infrastructure/vocab/cml/syntax"
)

// Cross-reference assignments must stay exhaustive when the pinned grammar
// changes. Every registry entry is exercised against an authored corpus node.
func TestReferenceContractsCoverPinnedGrammars(t *testing.T) {
	rulePattern := regexp.MustCompile(`(?m)^([A-Za-z][A-Za-z0-9]*)\s*:`)
	referencePattern := regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9]*)\s*\+?=\s*\[(?:tacticdsl::)?([a-zA-Z][a-zA-Z0-9]*)\]`)
	found := map[string]bool{}
	for _, path := range []string{"ContextMappingDSL.xtext", "TacticDDDLanguage.xtext"} {
		data, err := os.ReadFile(filepath.Join("syntax/grammar", path))
		if err != nil {
			t.Fatal(err)
		}
		rules := rulePattern.FindAllSubmatchIndex(data, -1)
		for i, match := range rules {
			end := len(data)
			if i+1 < len(rules) {
				end = rules[i+1][0]
			}
			rule := string(data[match[2]:match[3]])
			for _, assignment := range referencePattern.FindAllSubmatch(data[match[1]:end], -1) {
				field := string(assignment[1])
				key := rule + "." + strings.ToUpper(field[:1]) + field[1:]
				contract, ok := referenceContracts[key]
				if !ok {
					t.Errorf("unhandled grammar reference %s", key)
					continue
				}
				if contract.target != string(assignment[2]) && !(contract.target == "UserRequirement" && (string(assignment[2]) == "UseCase" || string(assignment[2]) == "UserStory")) {
					t.Errorf("%s targets %s, want %s", key, contract.target, assignment[2])
				}
				found[key] = true
			}
		}
	}
	for key := range referenceContracts {
		if !found[key] {
			t.Errorf("reference contract absent from grammar: %s", key)
		}
	}
}

func detachSyntaxDeclaration(file *syntax.File, target *syntax.Node) {
	walkSyntax(file, func(v reflect.Value, _ string) {
		if v.Kind() != reflect.Struct {
			return
		}
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if !field.CanSet() || !field.CanInterface() {
				continue
			}
			if syntaxNode(field.Interface()) == target {
				field.SetZero()
			}
		}
	})
	for _, d := range syntaxDeclarations(file) {
		for i := 0; i < d.value.NumField(); i++ {
			field := d.value.Field(i)
			if field.Kind() != reflect.Slice {
				continue
			}
			for j := field.Len() - 1; j >= 0; j-- {
				if syntaxNode(field.Index(j).Interface()) == target {
					field.Set(reflect.AppendSlice(field.Slice(0, j), field.Slice(j+1, field.Len())))
				}
			}
		}
	}
}

func TestReferenceRepairOnGrammarCorpus(t *testing.T) {
	t.Parallel()
	exercised := map[string]bool{}
	paths, err := filepath.Glob(filepath.Join(corpusRoot, "dsl/org.contextmapper.dsl.tests/integ-test-files/*/*.cml"))
	if err != nil {
		t.Fatal(err)
	}
	examples, err := filepath.Glob(filepath.Join(corpusRoot, "examples/src/main/cml/*/*.cml"))
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, examples...)
	paths = append(paths, "testdata/reference_clauses.cml")
	for _, path := range paths {
		if path == "testdata/reference_clauses.cml" {
			t.Logf("%d reference fields exercised from the upstream corpus before supplemental grammar clauses", len(exercised))
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file, err := syntax.Parse(data)
		if err != nil {
			if path == "testdata/reference_clauses.cml" {
				t.Fatal(err)
			}
			continue
		} // corpus includes intentional syntax failures
		for _, bound := range bindReferences(file) {
			key := bound.owner.value.Type().Name() + "." + bound.field
			if exercised[key] || len(bound.targets) == 0 {
				continue
			}
			exercised[key] = true
			t.Run(key, func(t *testing.T) {
				// Start each exercise from the authored model, not a manufactured AST.
				file, err := syntax.Parse(data)
				if err != nil {
					t.Fatal(err)
				}
				var reference boundReference
				for _, candidate := range bindReferences(file) {
					if candidate.owner.value.Type().Name()+"."+candidate.field == key && len(candidate.targets) != 0 {
						reference = candidate
						break
					}
				}
				var target *syntax.Node
				for _, node := range reference.targets {
					target = node
					break
				}
				for _, declaration := range syntaxDeclarations(file) {
					if declaration.node == target {
						declaration.value.FieldByName("Name").SetString("RenamedTarget")
						break
					}
				}
				if err := repairReferences(file, []boundReference{reference}, "corpus.cml"); err != nil {
					t.Fatal(err)
				}
				field := reference.owner.value.FieldByName(reference.field)
				if !strings.Contains(reflect.ValueOf(field.Interface()).String(), "RenamedTarget") {
					// Ref and []Ref implement no String method; inspect their fields directly.
					found := false
					if field.Type() == reflect.TypeOf(syntax.Ref{}) {
						found = field.Interface().(syntax.Ref).Name == "RenamedTarget"
					} else {
						for i := 0; i < field.Len(); i++ {
							found = found || field.Index(i).Interface().(syntax.Ref).Name == "RenamedTarget"
						}
					}
					if !found {
						t.Fatal("rename did not follow bound declaration")
					}
				}
				// Rebind the renamed reference, then remove exactly its target.
				for _, candidate := range bindReferences(file) {
					if candidate.owner.node == reference.owner.node && candidate.field == reference.field {
						reference = candidate
						break
					}
				}
				detachSyntaxDeclaration(file, target)
				err = repairReferences(file, []boundReference{reference}, "corpus.cml")
				ownerExists := false
				for _, declaration := range syntaxDeclarations(file) {
					ownerExists = ownerExists || declaration.node == reference.owner.node
				}
				if !ownerExists {
					if err != nil {
						t.Fatal(err)
					}
					return
				}
				if reference.contract.optional {
					if err != nil {
						t.Fatal(err)
					}
					if _, err := syntax.FormatChecked(file); err != nil {
						t.Fatal(err)
					}
				} else if err == nil || !strings.Contains(err.Error(), reference.owner.value.Type().Name()) || !strings.Contains(err.Error(), "cannot be dropped") {
					t.Fatalf("missing positioned refusal: %v", err)
				}
			})
		}
	}
	for key := range referenceContracts {
		if !exercised[key] {
			t.Errorf("grammar reference was not exercised: %s", key)
		}
	}
	t.Logf("exercised %d cross-reference fields across %d upstream corpus files and the supplemental grammar clauses", len(exercised), len(paths)-1)
}

func TestRecordRefusesRequiredReferenceRemoval(t *testing.T) {
	source := `Domain Shop
ContextMap { contains sales, billing }
BoundedContext sales {
 domainVisionStatement "Selling."
 "Checks out." Service Checkout { void place(); }
}
BoundedContext billing {
 domainVisionStatement "Billing."
 Application { Coordination Invoice { sales::Checkout::place; } }
}`
	repo := syncFixtureRepository(t, source)
	lang := readSyncLanguage(t, repo)
	lang.Contexts = lang.Contexts[1:]
	err := repo.Record(lang)
	if err == nil || !strings.Contains(err.Error(), "CoordinationStep.BoundedContext") || !strings.Contains(err.Error(), ":9:") {
		t.Fatalf("missing required-reference refusal: %v", err)
	}
	if readSyncFile(t, repo) != source {
		t.Fatal("refused removal wrote file")
	}
}

func TestRecordClearsOptionalContextReferences(t *testing.T) {
	source := `Domain Shop
ContextMap { contains sales, billing, crm }
BoundedContext sales {
 domainVisionStatement "Selling."
 Aggregate Order { owner billing "An order." Entity Order { aggregateRoot OrderId id key } }
}
BoundedContext billing { domainVisionStatement "Billing." }
BoundedContext crm realizes sales, billing refines billing { type = TEAM domainVisionStatement "CRM." }
Stakeholders of billing, sales { Stakeholder Accountant }
ValueRegister Accounting for billing { Value Accuracy }
`
	repo := syncFixtureRepository(t, source)
	lang := readSyncLanguage(t, repo)
	lang.Contexts = lang.Contexts[:1]
	assertRecorded(t, repo, lang)
	file, err := syntax.Parse([]byte(readSyncFile(t, repo)))
	if err != nil {
		t.Fatal(err)
	}
	crm := file.BoundedContexts[1]
	if crm.RefinedBoundedContext.Name != "" || len(crm.RealizedBoundedContexts) != 1 || crm.RealizedBoundedContexts[0].Name != "sales" {
		t.Fatal("context references were not selectively cleared")
	}
	if file.BoundedContexts[0].Aggregates[0].Owner.Name != "" || file.ValueRegisters[0].Context.Name != "" || len(file.Stakeholders[0].Contexts) != 1 || file.Stakeholders[0].Contexts[0].Name != "sales" {
		t.Fatal("optional references remain")
	}
	if !strings.Contains(readSyncFile(t, repo), "Stakeholder Accountant") || !strings.Contains(readSyncFile(t, repo), "Value Accuracy") {
		t.Fatal("referencing constructs were removed")
	}
}

func TestRecordRefusesFlowEventRemoval(t *testing.T) {
	source := `Domain Shop
BoundedContext sales {
 domainVisionStatement "Selling."
 Application {
  "Placed." DomainEvent Placed
  CommandEvent Place
  Flow Ordering { command Place emits event Placed }
 }
}`
	repo := syncFixtureRepository(t, source)
	lang := readSyncLanguage(t, repo)
	lang.Contexts[0].Events = nil
	err := repo.Record(lang)
	if err == nil || !strings.Contains(err.Error(), "SingleEventProduction.Events") || !strings.Contains(err.Error(), ":7:") {
		t.Fatalf("missing required Flow reference refusal: %v", err)
	}
	if readSyncFile(t, repo) != source {
		t.Fatal("refused Flow edit wrote file")
	}
}

func TestRecordDropsOptionalFlowDelegation(t *testing.T) {
	source := `Domain Shop
BoundedContext sales {
 domainVisionStatement "Selling."
 Aggregate Order {
  "An order." Entity Order { aggregateRoot OrderId id key }
  enum Status { OPENED, CLOSED }
  "Placed." DomainEvent Placed
 }
 Application {
  CommandEvent Place
  Flow Ordering { command Place delegates to Order [OPENED -> CLOSED] emits event Placed }
 }
}`
	repo := syncFixtureRepository(t, source)
	lang := readSyncLanguage(t, repo)
	lang.Contexts[0].Aggregates = nil
	lang.Contexts[0].Events[0].RaisedBy = ""
	assertRecorded(t, repo, lang)
	output := readSyncFile(t, repo)
	if strings.Contains(output, "delegates to") || strings.Contains(output, "OPENED -> CLOSED") || !strings.Contains(output, "emits event Placed") {
		t.Fatal("optional delegation and its transition were not removed together")
	}
}

func TestRecordResolvesQualifiedCoordinationReferences(t *testing.T) {
	source := `Domain Shop
BoundedContext sales { domainVisionStatement "Selling." "Sales checkout." Service Checkout { void place(); } }
BoundedContext billing {
 domainVisionStatement "Billing."
 "Billing checkout." Service Checkout { void place(); }
 Application { Coordination Invoice { sales::Checkout::place; } }
}`
	repo := syncFixtureRepository(t, source)
	lang := readSyncLanguage(t, repo)
	lang.Contexts[0].Services = nil
	err := repo.Record(lang)
	if err == nil || !strings.Contains(err.Error(), "CoordinationStep.Service") || !strings.Contains(err.Error(), ":6:") {
		t.Fatalf("qualified remote reference was not protected: %v", err)
	}
	if readSyncFile(t, repo) != source {
		t.Fatal("refused coordination edit wrote file")
	}
}
