package cmlvocab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	"github.com/wixregiga/arclint/internal/infrastructure/vocab/cml/syntax"
)

func projectableCorpus(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(corpusRoot, "examples/src/main/cml", path))
	if err != nil {
		t.Fatal(err)
	}
	file, err := syntax.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Domains) == 0 && file.Map == nil {
		file.Domains = []*syntax.Domain{{Name: "CorpusProject"}}
	}
	for _, bc := range file.BoundedContexts {
		if definition(bc.DomainVisionStatement, bc.Comment) == "" {
			bc.DomainVisionStatement = "The recorded " + bc.Name + " model."
		}
		for _, a := range allAggregates(bc) {
			for _, obj := range a.DomainObjects {
				if e, ok := obj.(*syntax.Entity); ok && e.AggregateRoot {
					e.Attributes = append(e.Attributes, &syntax.Attribute{Type: e.Name + "Identity", Name: "arclintCorpusIdentity", Key: true})
				}
			}
		}
		visitObjects(bc, func(obj syntax.SimpleDomainObject) {
			switch o := obj.(type) {
			case *syntax.Entity:
				if definition(o.Doc, o.Comment) == "" {
					o.Doc = "The recorded " + o.Name + " concept."
				}
			case *syntax.ValueObject:
				if definition(o.Doc, o.Comment) == "" {
					o.Doc = "The recorded " + o.Name + " value."
				}
			case *syntax.DomainEvent:
				if definition(o.Doc, o.Comment) == "" {
					o.Doc = "The recorded " + o.Name + " event."
				}
			}
		})
		visitServices(bc, func(s *syntax.Service) {
			if definition(s.Doc, s.Comment) == "" {
				s.Doc = "The recorded " + s.Name + " service."
			}
		})
	}
	data, err = syntax.FormatChecked(file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(data, recording(t)); err != nil {
		t.Fatal(err)
	}
	return data
}

func visitObjects(bc *syntax.BoundedContext, fn func(syntax.SimpleDomainObject)) {
	for _, a := range allAggregates(bc) {
		for _, obj := range append([]syntax.SimpleDomainObject(nil), a.DomainObjects...) {
			fn(obj)
		}
	}
	for _, m := range bc.Modules {
		for _, obj := range append([]syntax.SimpleDomainObject(nil), m.DomainObjects...) {
			fn(obj)
		}
	}
	if bc.Application != nil {
		for _, e := range append([]*syntax.DomainEvent(nil), bc.Application.Events...) {
			fn(e)
		}
	}
}

func visitServices(bc *syntax.BoundedContext, fn func(*syntax.Service)) {
	for _, s := range append([]*syntax.Service(nil), bc.DomainServices...) {
		fn(s)
	}
	for _, m := range bc.Modules {
		for _, s := range append([]*syntax.Service(nil), m.Services...) {
			fn(s)
		}
	}
	for _, a := range allAggregates(bc) {
		for _, s := range append([]*syntax.Service(nil), a.Services...) {
			fn(s)
		}
	}
}

// foreignProjection removes only modeled fields and synthetic declarations,
// retaining full exported grammar structure for attributes, references, plain
// operations, use cases, stakeholders, value registers, flows and relation extras.
func foreignProjection(t *testing.T, data []byte) []byte {
	t.Helper()
	return foreignProjectionWithModules(t, data, false)
}

func foreignProjectionWithModules(t *testing.T, data []byte, keepModules bool) []byte {
	t.Helper()
	file, err := syntax.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range file.Domains {
		d.Name = ""
		d.DomainVisionStatement = ""
	}
	if file.Map != nil {
		file.Map.Name = ""
		file.Map.BoundedContexts = nil
		for _, r := range file.Map.Relationships {
			value := reflect.ValueOf(r).Elem()
			for _, field := range []string{"Participant1", "Participant2", "Upstream", "Downstream", "UpstreamRoles", "DownstreamRoles", "Notation"} {
				v := value.FieldByName(field)
				if v.IsValid() {
					v.Set(reflect.Zero(v.Type()))
				}
			}
		}
	}
	for _, bc := range file.BoundedContexts {
		bc.DomainVisionStatement = ""
		for _, a := range allAggregates(bc) {
			if strings.HasPrefix(a.Name, "Added") {
				removeDeclaration(bc, a)
			}
		}
		visitObjects(bc, func(obj syntax.SimpleDomainObject) {
			value := reflect.ValueOf(obj).Elem()
			if strings.HasPrefix(value.FieldByName("Name").String(), "Added") {
				removeDeclaration(bc, obj)
				return
			}
			doc := value.FieldByName("Doc")
			if doc.IsValid() {
				doc.SetString("")
			}
			switch o := obj.(type) {
			case *syntax.Entity:
				o.Hint = replaceHint(o.Hint, "alias", nil)
				o.Hint = replaceHint(o.Hint, "repository", nil)
				o.Operations = foreignOperations(o.Operations)
				var attrs []*syntax.Attribute
				for _, a := range o.Attributes {
					if a.Name != "arclintCorpusIdentity" {
						attrs = append(attrs, a)
					}
				}
				o.Attributes = attrs
			case *syntax.ValueObject:
				o.Hint = replaceHint(o.Hint, "alias", nil)
				o.Operations = foreignOperations(o.Operations)
			case *syntax.DomainEvent:
				o.BelongsTo = syntax.Ref{}
			}
		})
		visitServices(bc, func(s *syntax.Service) {
			if strings.HasPrefix(s.Name, "Added") {
				removeDeclaration(bc, s)
				return
			}
			s.Doc = ""
			var ops []*syntax.ServiceOperation
			for _, op := range s.Operations {
				if kind, _, ok := ownedOperation(op.Name, op.Doc, op.Hint, &op.Node); !ok || kind != "assert" {
					ops = append(ops, op)
				}
			}
			s.Operations = ops
		})
		// New empty modules hold only removed synthetic additions.
		var modules []*syntax.SculptorModule
		for _, m := range bc.Modules {
			if keepModules || len(m.DomainObjects)+len(m.Aggregates)+len(m.Services)+len(m.Resources)+len(m.Consumers) > 0 {
				modules = append(modules, m)
			}
		}
		bc.Modules = modules
	}
	visitNodes(file, func(n *syntax.Node) {
		n.Pos = syntax.Pos{}
		n.NamePos = syntax.Pos{}
		n.Present = nil
		n.Spellings = nil
		for _, kind := range []string{"name", "question", "description", "separate-ways"} {
			n.LeadingComments = replaceDirectives(n.LeadingComments, kind, nil)
			n.TrailingComments = replaceDirectives(n.TrailingComments, kind, nil)
		}
		for i := range n.LeadingComments {
			n.LeadingComments[i].Pos = syntax.Pos{}
		}
		for i := range n.TrailingComments {
			n.TrailingComments[i].Pos = syntax.Pos{}
		}
	})
	// Ref positions are source provenance too, not foreign semantics.
	var clear func(reflect.Value)
	clear = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		if v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
			if !v.IsNil() {
				clear(v.Elem())
			}
			return
		}
		switch v.Kind() {
		case reflect.Struct:
			if v.Type() == reflect.TypeOf(syntax.Pos{}) && v.CanSet() {
				v.Set(reflect.Zero(v.Type()))
				return
			}
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() {
					clear(v.Field(i))
				}
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				clear(v.Index(i))
			}
		}
	}
	clear(reflect.ValueOf(file))
	out, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func foreignOperations(ops []*syntax.DomainObjectOperation) []*syntax.DomainObjectOperation {
	var out []*syntax.DomainObjectOperation
	for _, op := range ops {
		if _, _, ok := ownedOperation(op.Name, op.Doc, op.Hint, &op.Node); !ok {
			out = append(out, op)
		}
	}
	return out
}

func TestCorpusSync(t *testing.T) {
	t.Parallel()
	paths := []string{"insurance-example/Insurance-Example-Stage-5.cml", "ddd-sample/DDD-Sample-Stage-5.cml", "same-day-delivery-shop/stakeholders-and-values.cml", "workflow-samples/InsuranceQuoteRequest.cml"}
	for _, path := range sampledModels(paths) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			original := projectableCorpus(t, path)
			foreign := foreignServicePlacement(t, original)
			root := t.TempDir()
			repo, err := NewRepository(root, recording(t))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(repo.Path(), original, 0o600); err != nil {
				t.Fatal(err)
			}
			base, _, err := repo.RecordedLanguage()
			if err != nil {
				t.Fatal(err)
			}
			if err := repo.Record(base); err != nil {
				t.Fatal(err)
			}
			unchanged, _ := os.ReadFile(repo.Path())
			validateContextMapper(t, unchanged)
			if !bytes.Equal(original, unchanged) {
				t.Fatal("unchanged Record wrote bytes")
			}
			changed := addSyncElements(t, base)
			assertRecorded(t, repo, changed)
			changed.Contexts[0].Definition = "Changed context definition."
			a := &changed.Contexts[0].Aggregates[len(changed.Contexts[0].Aggregates)-1]
			a.Definition = "Changed aggregate."
			a.Identity = "AddedChangedIdentity"
			a.Aliases = []string{"changed alias"}
			a.Repository = "Changed storage"
			a.Factory = "AddedChangedFactory"
			a.Invariants[0].Statement = "Changed invariant."
			a.Assertions[0].On = "Change"
			a.Assertions[0].Statement = "Changed assertion."
			a.Entities[0].Definition = "Changed entity."
			a.Entities[0].Identity = "AddedChangedIdentity"
			a.Entities[0].Aliases = []string{"changed entity alias"}
			c := &changed.Contexts[0]
			c.ValueObjects[len(c.ValueObjects)-1].Definition = "Changed value."
			c.ValueObjects[len(c.ValueObjects)-1].Aliases = []string{"changed value alias"}
			c.ValueObjects[len(c.ValueObjects)-1].Invariants[0].Statement = "Changed value invariant."
			c.Specifications[len(c.Specifications)-1].Definition = "Changed specification."
			c.Events[len(c.Events)-1].Definition = "Changed event."
			c.Events[len(c.Events)-1].RaisedBy = ""
			c.Services[len(c.Services)-1].Definition = "Changed service."
			c.Services[len(c.Services)-1].Assertions[0].Statement = "Changed service assertion."
			c.Questions[len(c.Questions)-1].Text = "Changed question."
			assertRecorded(t, repo, changed)
			// Remove every synthetic kind while retaining the corpus's original modeled
			// declarations and all of their foreign grammar features.
			base.Contexts[0].Definition = changed.Contexts[0].Definition
			assertRecorded(t, repo, base)
			final, _ := os.ReadFile(repo.Path())
			got := foreignServicePlacement(t, final)
			if !bytes.Equal(foreign, got) {
				_ = os.WriteFile(filepath.Join(root, "foreign-before.json"), foreign, 0o600)
				_ = os.WriteFile(filepath.Join(root, "foreign-after.json"), got, 0o600)
				t.Fatalf("foreign grammar changed: %s", firstDifference(foreign, got))
			}
			if err := repo.Record(base); err != nil {
				t.Fatal(err)
			}
			after, _ := os.ReadFile(repo.Path())
			validateContextMapper(t, after)
			if !bytes.Equal(final, after) {
				t.Fatal("final unchanged Record changed bytes")
			}
		})
	}
}

func addSyncElements(t *testing.T, base vocab.UbiquitousLanguage) vocab.UbiquitousLanguage {
	t.Helper()
	lang, err := vocab.NewUbiquitousLanguage(base.Project, base.Description, base.Contexts, base.Relations)
	if err != nil {
		t.Fatal(err)
	}
	c := &lang.Contexts[0]
	c.Aggregates = append(c.Aggregates, vocab.Aggregate{Name: "AddedAggregate", Definition: "Added aggregate.", Identity: "AddedIdentity", Repository: "AddedRepository", Factory: "AddedFactory", Aliases: []string{"new alias"}, Entities: []vocab.Entity{{Name: "AddedEntity", Definition: "Added entity.", Identity: "AddedIdentity", Aliases: []string{"entity alias"}}}, Invariants: []vocab.Invariant{{Key: "balance-2", Statement: "A consistent balance."}}, Assertions: []vocab.Assertion{{Key: "credit-3", On: "ensureBalance_2", Statement: "Credit is available."}}})
	c.ValueObjects = append(c.ValueObjects, vocab.ValueObject{Name: "AddedValue", Definition: "Added value.", Aliases: []string{"value alias"}, Invariants: []vocab.Invariant{{Key: "value-2", Statement: "A consistent value."}}})
	c.Specifications = append(c.Specifications, vocab.Specification{Name: "AddedSpecification", Definition: "Added specification."})
	c.Events = append(c.Events, vocab.DomainEvent{Name: "AddedEvent", Definition: "Added event.", RaisedBy: "AddedAggregate"})
	c.Services = append(c.Services, vocab.DomainService{Name: "AddedService", Definition: "Added service.", Assertions: []vocab.Assertion{{Key: "service-2", On: "foreignOperation", Statement: "A service promise."}}})
	c.Questions = append(c.Questions, vocab.Question{Key: "added-question", Text: "What is the answer?"})
	lang, err = vocab.NewUbiquitousLanguage(lang.Project, lang.Description, lang.Contexts, lang.Relations)
	if err != nil {
		t.Fatal(err)
	}
	return lang
}

func assertRecorded(t *testing.T, repo Repository, want vocab.UbiquitousLanguage) {
	t.Helper()
	if err := repo.Record(want); err != nil {
		existing, _ := os.ReadFile(repo.Path())
		file, parseErr := syntax.Parse(existing)
		if parseErr == nil {
			old, p, projectErr := project(file, repo.Recording())
			if projectErr == nil {
				syncFile(p, old, want)
				data, formatErr := syntax.FormatChecked(file)
				if formatErr == nil {
					got, readErr := Parse(data, repo.Recording())
					t.Logf("edited CML:\n%s\nread error: %v\nwant: %#v\ngot: %#v", data, readErr, normalized(want), normalized(got))
				}
			}
		}
		t.Fatal(err)
	}
	data, err := os.ReadFile(repo.Path())
	if err != nil {
		t.Fatal(err)
	}
	validateContextMapper(t, data)
	got, exists, err := repo.RecordedLanguage()
	if err != nil || !exists {
		t.Fatalf("read exists=%v error=%v", exists, err)
	}
	if !sameLanguage(want, got) {
		t.Fatal("Record changed language")
	}
}

func TestRepositoryLifecycle(t *testing.T) {
	r, err := NewRepository(t.TempDir(), recording(t))
	if err != nil {
		t.Fatal(err)
	}
	zero, exists, err := r.RecordedLanguage()
	if err != nil || exists || !reflect.DeepEqual(zero, vocab.UbiquitousLanguage{}) {
		t.Fatal("missing recording did not return zero, false, nil")
	}
	lang, err := vocab.NewUbiquitousLanguage("Project", "Description", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertRecorded(t, r, lang)
	info, err := os.Stat(r.Path())
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("recording is not mode 0600")
	}
	if _, err := NewRepository(t.TempDir(), mustDefaultRecording(t)); err == nil {
		t.Fatal("YAML recording accepted")
	}
}

func firstDifference(a, b []byte) string {
	x, y := strings.Split(string(a), "\n"), strings.Split(string(b), "\n")
	for i := 0; i < len(x) && i < len(y); i++ {
		if x[i] != y[i] {
			return fmt.Sprintf("line %d: before %s; after %s", i+1, x[i], y[i])
		}
	}
	return fmt.Sprintf("length %d -> %d", len(a), len(b))
}

func mustDefaultRecording(t *testing.T) vocab.DomainRecording {
	t.Helper()
	recording, err := vocab.DefaultDomainRecording()
	if err != nil {
		t.Fatal(err)
	}
	return recording
}
