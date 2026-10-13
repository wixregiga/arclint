package cmlvocab

import (
	"reflect"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	"github.com/wixregiga/arclint/internal/infrastructure/vocab/cml/syntax"
)

// validateContextMapper implements the error-level checks required by the
// mapping contract. Sources are pinned in /tmp/cml-research/validators:
// UniquenessValidator.java:102-123 (model-wide Module/Aggregate names),
// AggregateSemanticsValidator.java:40-50 (at most one aggregateRoot),
// TacticDDDLanguageValidator.xtend:352-355,513-523,706-711 (Repository suffix,
// belongsTo and model-wide Repository name uniqueness),
// ContextMapSemanticsValidator.java:54-77 (participants),
// ValidationMessages.java:20-21 (Customer-Supplier roles). The supplied
// validator snapshot omits the relationship validator implementation; the
// self-relationship check is independently required by the mapping contract.
// Cross-reference scope follows ContextMappingDSL.xtext:136-173,350-361 and
// TacticDDDLanguage.xtext:372-386 (references and belongsTo).
func validateContextMapper(t *testing.T, data []byte) {
	t.Helper()
	file, err := syntax.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	validateContextMapperFile(t, file)
}

// validateContextMapperFile applies the same checks to a model that is already
// parsed.
func validateContextMapperFile(t *testing.T, file *syntax.File) {
	t.Helper()
	for _, bound := range bindReferences(file) {
		field := bound.owner.value.FieldByName(bound.field)
		var refs []syntax.Ref
		if field.Type() == reflect.TypeOf(syntax.Ref{}) {
			refs = []syntax.Ref{field.Interface().(syntax.Ref)}
		} else {
			for i := 0; i < field.Len(); i++ {
				refs = append(refs, field.Index(i).Interface().(syntax.Ref))
			}
		}
		for _, ref := range refs {
			if ref.Name != "" && bound.targets[ref.Name] == nil {
				t.Fatalf("%s.%s at line %d has unlinked %s %q", bound.owner.value.Type().Name(), bound.field, ref.Pos.Line, bound.contract.target, ref.Name)
			}
		}
	}
	repositories := map[string]bool{}
	visitRepositories(file, nil, func(repo *syntax.Repository) {
		if repositories[repo.Name] {
			t.Fatalf("duplicate Repository %q; TacticDDDLanguageValidator.xtend:706-711", repo.Name)
		}
		repositories[repo.Name] = true
	})
	aggregates, modules := map[string]bool{}, map[string]bool{}
	contexts := map[string]*syntax.BoundedContext{}
	for _, bc := range file.BoundedContexts {
		contexts[bc.Name] = bc
	}
	if file.Map != nil {
		contained := map[string]bool{}
		for _, ref := range file.Map.BoundedContexts {
			if contexts[ref.Name] == nil {
				t.Fatalf("contains unknown %q", ref.Name)
			}
			contained[ref.Name] = true
		}
		for _, r := range file.Map.Relationships {
			from, to, _, _ := relationshipValue(r)
			if !contained[from] || !contained[to] {
				t.Fatal("relationship participant not contained in map")
			}
			if from == to {
				t.Fatal("self relationship")
			}
			if cs, ok := r.(*syntax.CustomerSupplierRelationship); ok {
				for _, role := range cs.DownstreamRoles {
					if role == "CF" {
						t.Fatal("Customer-Supplier with CF")
					}
				}
				for _, role := range cs.UpstreamRoles {
					if role == "OHS" {
						t.Fatal("Customer-Supplier with OHS")
					}
				}
			}
		}
	}
	for _, bc := range file.BoundedContexts {
		roots := map[string]bool{}
		objects := map[string]bool{}
		for _, declaration := range syntaxDeclarations(file) {
			if referenceTargetMatches("SimpleDomainObject", declaration.value.Type().Name()) {
				objects[declaration.value.FieldByName("Name").String()] = true
			}
		}
		for _, m := range bc.Modules {
			if modules[m.Name] {
				t.Fatalf("duplicate Module %q", m.Name)
			}
			modules[m.Name] = true
			for _, obj := range m.DomainObjects {
				switch o := obj.(type) {
				case *syntax.ValueObject:
					objects[o.Name] = true
				case *syntax.DomainEvent:
					objects[o.Name] = true
				}
			}
		}
		for _, a := range allAggregates(bc) {
			if aggregates[a.Name] {
				t.Fatalf("duplicate Aggregate %q", a.Name)
			}
			aggregates[a.Name] = true
			rootCount := 0
			for _, obj := range a.DomainObjects {
				if e, ok := obj.(*syntax.Entity); ok {
					objects[e.Name] = true
					if e.AggregateRoot {
						rootCount++
						roots[e.Name] = true
					}
					if e.Repository != nil {
						if !e.AggregateRoot {
							t.Fatal("Repository on non-root")
						}
						if !strings.HasSuffix(e.Repository.Name, "Repository") {
							t.Fatal("invalid Repository suffix")
						}
					}
				}
			}
			if rootCount > 1 {
				t.Fatalf("Aggregate %q has %d roots", a.Name, rootCount)
			}
		}
		for _, a := range allAggregates(bc) {
			for _, obj := range a.DomainObjects {
				if e, ok := obj.(*syntax.Entity); ok {
					for _, ref := range e.References {
						if !objects[ref.DomainObjectType.Name] {
							t.Fatalf("unlinked reference %q", ref.DomainObjectType.Name)
						}
					}
					if e.BelongsTo.Name != "" && !roots[e.BelongsTo.Name] {
						t.Fatal("belongsTo does not name a root")
					}
				}
			}
		}
		for _, m := range bc.Modules {
			for _, obj := range m.DomainObjects {
				if e, ok := obj.(*syntax.DomainEvent); ok && e.BelongsTo.Name != "" && !roots[e.BelongsTo.Name] {
					t.Fatal("event belongsTo does not name a context root")
				}
			}
		}
	}
}

func TestMappingRepositoryNamesAreGloballyUnique(t *testing.T) {
	lang, err := vocab.NewUbiquitousLanguage("Project", "", []vocab.BoundedContext{
		{Name: "one", Definition: "First context.", Aggregates: []vocab.Aggregate{{Name: "First", Definition: "First root.", Identity: "FirstId", Repository: "CommonRepository"}}},
		{Name: "two", Definition: "Second context.", Aggregates: []vocab.Aggregate{{Name: "Second", Definition: "Second root.", Identity: "SecondId", Repository: "CommonRepository"}}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Render(lang)
	if err != nil {
		t.Fatal(err)
	}
	assertRoundTrip(t, lang, data)
	if strings.Count(string(data), "arclint.repository=CommonRepository") != 2 {
		t.Fatalf("shared Repository names must use hints on both roots:\n%s", data)
	}
}
