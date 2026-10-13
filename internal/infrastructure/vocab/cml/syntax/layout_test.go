package syntax

import (
	"bytes"
	"strings"
	"testing"
)

func TestDeclarationLayout(t *testing.T) {
	source := `import "first.cml"
import "second.cml"
Domain Project { Subdomain First { Entity FirstEntity Service FirstService } Subdomain Second }
ContextMap { contains Alpha, Beta Alpha -> Beta // Second relationship
Beta -> Alpha }
BoundedContext Alpha {
  domainVisionStatement "Alpha."
  type SYSTEM
  Aggregate Group {
    Entity First { hint = "kept" aggregateRoot String code - Other ref def void work(); }
    // Value comment
    ValueObject Second
    Service Third
  }
  // Module comment
  Module Mod { ValueObject Fourth ValueObject Fifth }
  Application App { DomainEvent Ev Service Sixth Flow Work Coordination Steps }
}
BoundedContext Beta
UseCase Req
Stakeholders of Alpha
ValueRegister Register`
	file, err := Parse([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	data, err := FormatChecked(file)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, consecutive := range []string{
		"import \"first.cml\"\nimport \"second.cml\"\n\nDomain",
		"Subdomain Second\n}\n\nContextMap",
		"Alpha -> Beta\n\n  // Second relationship\n  Beta -> Alpha",
		"Service FirstService\n  }\n\n  Subdomain Second",
		"\n\n    // Value comment\n    ValueObject Second",
		"ValueObject Second\n\n    Service Third",
		"\n\n  // Module comment\n  Module Mod",
		"ValueObject Fourth\n\n    ValueObject Fifth",
		"DomainEvent Ev\n\n    Service Sixth\n\n    Flow Work\n\n    Coordination Steps",
		"BoundedContext Beta\n\nUseCase Req\n\nStakeholders",
		"Stakeholders of Alpha\n\nValueRegister Register",
	} {
		if !strings.Contains(text, consecutive) {
			t.Errorf("missing declaration layout %q:\n%s", consecutive, data)
		}
	}
	entity := file.BoundedContexts[0].Aggregates[0].DomainObjects[0].(*Entity)
	printed := printerForTest(t, "Entity", entity)
	if bytes.Contains(printed, []byte("\n\n")) {
		t.Fatalf("domain object properties and operations must remain together:\n%s", printed)
	}
	assertRoundTrip(t, file)
}

func printerForTest(t *testing.T, rule string, value any) []byte {
	t.Helper()
	g, err := cmlGrammar()
	if err != nil {
		t.Fatal(err)
	}
	p := printer{grammar: g}
	tokens, ok := p.node(rule, value)
	if !ok {
		t.Fatal("cannot format node")
	}
	return layout(tokens)
}

func TestOptionalBodyProvenance(t *testing.T) {
	for _, rule := range []string{"BoundedContext", "Domain", "Subdomain", "Aggregate", "SculptorModule", "Application", "Service", "Entity", "ValueObject", "DomainEvent", "CommandEvent", "Trait", "DataTransferObject", "BasicType", "Repository", "Flow", "Coordination", "UseCase"} {
		t.Run(rule, func(t *testing.T) {
			keyword := rule
			if rule == "SculptorModule" {
				keyword = "Module"
			}
			for _, braces := range []bool{false, true} {
				source := keyword + " Sample"
				if braces {
					source += " {}"
				}
				value := parseRule(t, rule, source)
				node := nodeOf(value)
				if node.ExplicitBody != braces {
					t.Fatalf("ExplicitBody = %v, want %v", node.ExplicitBody, braces)
				}
				data := printerForTest(t, rule, value)
				if braces && !bytes.Contains(data, []byte("{}")) {
					t.Fatalf("authored empty braces must stay compact:\n%s", data)
				}
				if bytes.Contains(data, []byte("{")) != braces {
					t.Fatalf("body presence changed:\n%s", data)
				}
				node.ExplicitBody = false
				if bytes.Contains(printerForTest(t, rule, value), []byte("{")) {
					t.Fatal("caller cannot omit an empty optional body")
				}
			}
		})
	}
	constructed := &File{BoundedContexts: []*BoundedContext{{Name: "Alpha", Modules: []*SculptorModule{{Name: "Mod", DomainObjects: []SimpleDomainObject{&ValueObject{Name: "Activity"}}}}}}}
	data, err := FormatChecked(constructed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("ValueObject Activity\n")) || bytes.Contains(data, []byte("ValueObject Activity {")) {
		t.Fatalf("new empty optional body must be omitted:\n%s", data)
	}
}
