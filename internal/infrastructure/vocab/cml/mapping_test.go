package cmlvocab

import (
	"os"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	"github.com/wixregiga/arclint/internal/infrastructure/vocab/cml/syntax"
)

func TestMappingProblemsCollected(t *testing.T) {
	input := `Domain Example
ContextMap { contains A, B
 A -> B
 B -> A
 // @arclint bad "x"
}
BoundedContext A { Aggregate Items { Entity Item { aggregateRoot } } }
BoundedContext B { Module B { ValueObject AValue } }
`
	_, err := Parse([]byte(input), recording(t))
	if err == nil {
		t.Fatal("mapping accepted")
	}
	for _, fragment := range []string{"domain.cml:", "unknown arclint directive", "two relationships", "missing a definition", "no key property"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Errorf("missing %q in %v", fragment, err)
		}
	}
}

func TestGrammarCommentsAndDocumentOrder(t *testing.T) {
	input := `Domain Example
BoundedContext HTTPGateway {
 domainVisionStatement = "The gateway."
 Module First {
  Aggregate A { /* The root. */ Entity Root { aggregateRoot RootId id key } }
  /* A value. */ ValueObject Earlier { "Its amount is positive." def void ensureAmountPositive(); }
  "A predicate." ValueObject Predicate { def boolean isSatisfiedBy(); }
  "An event." DomainEvent EarlierEvent { belongsTo @Root }
  "A service." Service WithinModule { void Operate(); "A contract." void assertAvailable() hint = "arclint.on=Operate"; }
 }
 Aggregate Second { "Second root." Entity SecondRoot { aggregateRoot OtherId id key } "An event." DomainEvent LaterEvent }
 // @arclint question "what-now" "What happens?"
}
`
	lang, err := Parse([]byte(input), recording(t))
	if err != nil {
		t.Fatal(err)
	}
	c := lang.Contexts[0]
	if c.Name != "http_gateway" || c.Aggregates[0].Definition != "The root." || c.ValueObjects[0].Definition != "A value." || len(c.Specifications) != 1 || len(c.Services) != 1 || len(c.Questions) != 1 || c.Events[1].RaisedBy != "SecondRoot" {
		t.Fatalf("unexpected projection: %#v", c)
	}
	if c.Events[0].Name != "EarlierEvent" || c.Events[1].Name != "LaterEvent" {
		t.Fatal("events not in document order")
	}
	if c.Questions[0].Line != 12 {
		t.Fatalf("question line %d", c.Questions[0].Line)
	}
}

func TestForeignHintsAndOrdinaryChecks(t *testing.T) {
	input := `Domain Example
BoundedContext one { domainVisionStatement="A context."
 Aggregate A { "Root." Entity Root { aggregateRoot
 hint=" foreign=literal%2C,arclint.alias=one%2Ctwo, other=x=y,arclint.repository=storage%3Dname"
 RootId id key
 def void ensureOrdinary();
 "Ordinary assertion." def void assertOrdinary();
 "A constraint." def void ensureTier_2Pricing();
 "A promise." def void assertPromised() hint="arclint.on=Book%2Cnow";
 }}
}
`
	lang, err := Parse([]byte(input), recording(t))
	if err != nil {
		t.Fatal(err)
	}
	a := lang.Contexts[0].Aggregates[0]
	if len(a.Invariants) != 1 || len(a.Assertions) != 1 || a.Assertions[0].On != "Book,now" || a.Repository != "storage=name" || a.Aliases[0] != "one,two" {
		t.Fatalf("unexpected %#v", a)
	}
	hint := replaceHint("foreign=x,arclint.alias=old, foreign2=y", "alias", []string{"new,=%"})
	if hint != "foreign=x,arclint.alias=new%2C%3D%25, foreign2=y" {
		t.Fatal(hint)
	}
}

func TestRelationSync(t *testing.T) {
	input := `Domain Example
ContextMap { contains One, Two
 // Keep the relationship comment.
 One [U,OHS,PL]->[D,ACL] Two : Link { implementationTechnology="SOAP" exposedAggregates=Items downstreamRights=INFLUENCER }
}
BoundedContext One { domainVisionStatement="One." Aggregate Items { "Root." Entity Root {aggregateRoot RootId id key} } }
BoundedContext Two {domainVisionStatement="Two."}
`
	repo, err := NewRepository(t.TempDir(), recording(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repo.Path(), []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	lang, _, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatal(err)
	}
	unchanged := lang
	unchanged.Project = "Changed project"
	assertRecorded(t, repo, unchanged)
	data, _ := os.ReadFile(repo.Path())
	file, err := syntax.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	r := file.Map.Relationships[0].(*syntax.UpstreamDownstreamRelationship)
	if len(r.UpstreamRoles) != 2 || len(r.UpstreamExposedAggregates) != 1 || r.DownstreamGovernanceRights != "INFLUENCER" {
		t.Fatal("unchanged relation lost extras")
	}
	kinds := []vocab.RelationKind{vocab.RelationConformist, vocab.RelationAnticorruptionLayer, vocab.RelationOpenHostService, vocab.RelationPublishedLanguage, vocab.RelationCustomerSupplier, vocab.RelationPartnership, vocab.RelationSharedKernel, vocab.RelationSeparateWays}
	for _, kind := range kinds {
		lang.Project = "Changed project"
		lang.Relations[0].Kind = kind
		lang.Relations[0].Description = "Changed description."
		assertRecorded(t, repo, lang)
		data, _ := os.ReadFile(repo.Path())
		if !strings.Contains(string(data), "Keep the relationship comment.") {
			t.Fatalf("%s lost comment", kind)
		}
		if kind != vocab.RelationSeparateWays {
			file, err = syntax.Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			v := file.Map.Relationships[0]
			if kind == vocab.RelationPartnership {
				s := v.(*syntax.Partnership)
				if s.Name != "Link" || s.ImplementationTechnology != "SOAP" {
					t.Fatal("symmetric relation lost name or technology")
				}
			}
		}
	}
	lang.Relations = nil
	assertRecorded(t, repo, lang)
}

func TestDirectiveErrors(t *testing.T) {
	for _, directiveText := range []string{`// @arclint nope "x"`, `// @arclint question "one"`, `// @arclint question "one" "bad\x20"`, `// @arclint name unquoted`} {
		input := "Domain Example\nBoundedContext one {domainVisionStatement=\"One.\"\n" + directiveText + "\n}"
		if _, err := Parse([]byte(input), recording(t)); err == nil {
			t.Fatal("invalid directive accepted: " + directiveText)
		}
	}
}

func TestBuiltinNamedIdentity(t *testing.T) {
	for _, name := range []string{"String", "Key", "boolean", "Object[]"} {
		lang, err := vocab.NewUbiquitousLanguage("Project", "", []vocab.BoundedContext{{Name: "one", Definition: "Context.", Aggregates: []vocab.Aggregate{{Name: "Root", Definition: "Root.", Identity: name}}}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		data, err := Render(lang)
		if err != nil {
			t.Fatalf("identity %q: %v", name, err)
		}
		assertRoundTrip(t, lang, data)
	}
}

func TestSyntaxPreservesEscapedIdentityType(t *testing.T) {
	input := []byte("Domain Project\nBoundedContext one { domainVisionStatement=\"Context.\" Aggregate Root { \"Root.\" Entity Root {aggregateRoot ^String id key} } }")
	ast, err := syntax.Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	data, err := syntax.FormatChecked(ast)
	if err != nil {
		t.Fatal(err)
	}
	before, err := Parse(input, recording(t))
	if err != nil {
		t.Fatal(err)
	}
	after, err := Parse(data, recording(t))
	if err != nil {
		t.Fatalf("formatter changed ^String identifier to built-in String: %v\n%s", err, data)
	}
	if !sameLanguage(before, after) {
		t.Fatal("formatter changed identity meaning")
	}
}

func TestSyntaxPreservesDomainObjectOrder(t *testing.T) {
	input := []byte(`Domain Project
BoundedContext one {domainVisionStatement="Context."
 Aggregate Root { "Root." Entity Root {aggregateRoot RootId id key} "First event." DomainEvent FirstEvent }
 Module one { "Second event." DomainEvent SecondEvent }
}`)
	ast, err := syntax.Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	data, err := syntax.FormatChecked(ast)
	if err != nil {
		t.Fatal(err)
	}
	before, err := Parse(input, recording(t))
	if err != nil {
		t.Fatal(err)
	}
	after, err := Parse(data, recording(t))
	if err != nil {
		t.Fatal(err)
	}
	if !sameLanguage(before, after) {
		t.Fatalf("formatter reordered aggregate/module events: before %v; after %v\n%s", before.Contexts[0].Events, after.Contexts[0].Events, data)
	}
}

func TestParserPort(t *testing.T) {
	input := []byte("Domain Project")
	want, err := Parse(input, recording(t))
	if err != nil {
		t.Fatal(err)
	}
	got, err := (Parser{}).ParseUbiquitousLanguage(input, recording(t))
	if err != nil {
		t.Fatal(err)
	}
	if !sameLanguage(got, want) {
		t.Fatal("Parser differs from Parse")
	}
}

func TestEmptyOwnedOperationCarriers(t *testing.T) {
	for _, op := range []string{`"" def void ensureTotalConsistent();`, `"Promise." def void assertAvailable() hint="arclint.on=";`} {
		input := `Domain Project
BoundedContext one {domainVisionStatement="Context." Aggregate Root { "Root." Entity Root {aggregateRoot RootId id key ` + op + ` } } }`
		if _, err := Parse([]byte(input), recording(t)); err == nil {
			t.Fatalf("empty carrier accepted: %s", op)
		}
	}
}

func TestExplicitContextNameIsNotNormalized(t *testing.T) {
	input := []byte(`Domain Project
// @arclint name "HTTPGateway"
BoundedContext HTTPGateway {domainVisionStatement="Context."}`)
	if _, err := Parse(input, recording(t)); err == nil {
		t.Fatal("invalid explicitly recorded context name was normalized")
	}
}

func TestSingleLineGrammarDefinition(t *testing.T) {
	input := []byte("Domain Project\n// The gateway domain.\nBoundedContext Gateway")
	lang, err := Parse(input, recording(t))
	if err != nil {
		t.Fatal(err)
	}
	if lang.Contexts[0].Definition != "The gateway domain." {
		t.Fatalf("comment markers retained: %q", lang.Contexts[0].Definition)
	}
}
