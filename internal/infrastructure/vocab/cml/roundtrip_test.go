package cmlvocab

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	"github.com/wixregiga/arclint/internal/infrastructure/vocab/cml/syntax"
	yamlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/yaml"
)

func recording(t *testing.T) vocab.DomainRecording {
	t.Helper()
	r, err := vocab.NewDomainRecording("domain.cml")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// TestRealDomains holds the two real recorded domains, arclint's own
// and the boxoffice proving ground, both kept in CML: each committed file
// is exactly what Render writes for the language it records, it passes the
// Context Mapper checks, and the language translates to YAML and back
// without changing.
func TestRealDomains(t *testing.T) {
	for _, path := range []string{"../../../../domain.arclint.cml", "../../../../testing/boxoffice/domain.arclint.cml"} {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			lang, err := Parse(data, recording(t))
			if err != nil {
				t.Fatal(err)
			}
			rendered, err := Render(lang)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, rendered) {
				t.Fatalf("%s is not the canonical rendering of the language it records", path)
			}
			assertRoundTrip(t, lang, rendered)

			yamlRecording := mustDefaultRecording(t)
			dir := t.TempDir()
			yamlRepo, err := yamlvocab.NewRepository(dir, yamlRecording)
			if err != nil {
				t.Fatal(err)
			}
			if err := yamlRepo.Record(lang); err != nil {
				t.Fatal(err)
			}
			viaYAML, _, err := yamlRepo.RecordedLanguage()
			if err != nil {
				t.Fatal(err)
			}
			if d := lang.Difference(viaYAML); d != "" {
				t.Fatalf("CML to YAML changed the language: %s", d)
			}
			back, err := Render(viaYAML)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(back, data) {
				t.Fatalf("CML to YAML to CML does not give the committed file back")
			}
		})
	}
}

// TestRepositoryCMLCopyRecordsTheYAMLLanguage keeps this repository's
// domain.arclint.cml a copy of its domain file, domain.arclint.yaml: the
// copy records the same language and is exactly what
// `arclint domain export domain.arclint.cml` writes for it. Refresh it
// with that command after changing the domain.
func TestRepositoryCMLCopyRecordsTheYAMLLanguage(t *testing.T) {
	yamlData, err := os.ReadFile("../../../../domain.arclint.yaml")
	if err != nil {
		t.Fatal(err)
	}
	lang, err := yamlvocab.Parse(yamlData, mustDefaultRecording(t))
	if err != nil {
		t.Fatal(err)
	}
	cmlData, err := os.ReadFile("../../../../domain.arclint.cml")
	if err != nil {
		t.Fatal(err)
	}
	copied, err := Parse(cmlData, recording(t))
	if err != nil {
		t.Fatal(err)
	}
	if d := lang.Difference(copied); d != "" {
		t.Fatalf("domain.arclint.cml records a different language than domain.arclint.yaml (%s); run `arclint domain export domain.arclint.cml`", d)
	}
	rendered, err := Render(lang)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rendered, cmlData) {
		t.Fatal("domain.arclint.cml is not what `arclint domain export domain.arclint.cml` writes for domain.arclint.yaml; run it")
	}
}

func assertRoundTrip(t *testing.T, want vocab.UbiquitousLanguage, data []byte) {
	t.Helper()
	tree, err := syntax.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := syntax.FormatChecked(tree)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, formatted) {
		t.Fatalf("Render is not a formatter fixed point:\n%s", formatted)
	}
	validateContextMapper(t, data)
	got, err := Parse(data, recording(t))
	if err != nil {
		t.Fatalf("%v\n%s", err, data)
	}
	if !sameLanguage(want, got) {
		t.Fatalf("round trip differs\nwant %#v\ngot %#v\n%s", normalized(want), normalized(got), data)
	}
}

func generatedLanguage(t *testing.T, rng *rand.Rand, i int) vocab.UbiquitousLanguage {
	t.Helper()
	terms := []string{"Event", "Entity", "Domain", "Map", "key", "Order item", "3D account", "Überweisung", "Account-v2", "HTTPGateway"}
	prose := []string{"A fact.", "Quotes \" and slash \\.", "First\nsecond\tthird.", "Unicode 日本語 and café.", "Percent %, comma , and equals =."}
	text := func() string { return prose[rng.Intn(len(prose))] }
	name := func(slot int) string { return terms[(i+slot)%len(terms)] }
	keys := []string{"total-is-sum", "tier-2-pricing", "x-1", "x1", "a-b2-c3d"}
	contexts := []vocab.BoundedContext{{Name: "first-context", Definition: text()}, {Name: "second_context2", Definition: text()}}
	for j := range contexts {
		c := &contexts[j]
		a := vocab.Aggregate{Name: name(0), Definition: text(), Identity: name(1), Aliases: []string{"alias,=%, Unicode Ω", "alias 2"}, Repository: []string{"AccountRepository", "Repository", "storage name"}[i%3], Factory: name(8), Entities: []vocab.Entity{{Name: name(2), Definition: text(), Identity: name(1), Aliases: []string{"member alias"}}}, Invariants: []vocab.Invariant{{Key: keys[i%len(keys)], Statement: text()}}, Assertions: []vocab.Assertion{{Key: "available-for-2", On: []string{"Publish", "reserve item", "Event", " Operation ", "Comma,equals=%\n\t"}[i%5], Statement: text()}}}
		c.Aggregates = []vocab.Aggregate{a}
		c.ValueObjects = []vocab.ValueObject{{Name: name(1), Definition: text(), Aliases: []string{"value alias"}, Invariants: []vocab.Invariant{{Key: "non-negative-2", Statement: text()}}}, {Name: name(3), Definition: text()}}
		c.Events = []vocab.DomainEvent{{Name: name(4), Definition: text(), RaisedBy: a.Name}, {Name: name(5), Definition: text()}}
		c.Services = []vocab.DomainService{{Name: name(6), Definition: text(), Assertions: []vocab.Assertion{{Key: "unique-for-3", On: "Book", Statement: text()}}}}
		c.Specifications = []vocab.Specification{{Name: name(7), Definition: text()}}
		c.Questions = []vocab.Question{{Key: "question-2", Text: text()}}
	}
	kinds := []vocab.RelationKind{vocab.RelationPartnership, vocab.RelationSharedKernel, vocab.RelationCustomerSupplier, vocab.RelationConformist, vocab.RelationAnticorruptionLayer, vocab.RelationOpenHostService, vocab.RelationPublishedLanguage, vocab.RelationSeparateWays}
	description := ""
	if (i/8)%2 == 0 {
		description = text()
	}
	lang, err := vocab.NewUbiquitousLanguage(name(9), text(), contexts, []vocab.ContextRelation{{From: contexts[0].Name, To: contexts[1].Name, Kind: kinds[i%len(kinds)], Description: description}})
	if err != nil {
		t.Fatal(err)
	}
	return lang
}

func TestGeneratedLanguages(t *testing.T) {
	t.Parallel()
	cases := 2000
	if !exhaustive() {
		cases = 10
	}
	rng := rand.New(rand.NewSource(71729031))
	for i := 0; i < cases; i++ {
		lang := generatedLanguage(t, rng, i)
		data, err := Render(lang)
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		assertRoundTrip(t, lang, data)
		if dir := os.Getenv("CML_ORACLE_OUTPUT"); dir != "" && i < 200 {
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("generated-%03d.cml", i)), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Logf("%d constructor-valid languages round-tripped", cases)
}

func TestKeyCase(t *testing.T) {
	rng := rand.New(rand.NewSource(273413))
	seen := map[string]string{}
	for i := 0; i < 20000; i++ {
		var b strings.Builder
		b.WriteByte(byte('a' + rng.Intn(26)))
		for j := 0; j < rng.Intn(12); j++ {
			alphabet := "abcdefghijklmnopqrstuvwxyz0123456789"
			b.WriteByte(alphabet[rng.Intn(len(alphabet))])
		}
		for j := 0; j < rng.Intn(5); j++ {
			b.WriteByte('-')
			for k := 0; k < 1+rng.Intn(8); k++ {
				alphabet := "abcdefghijklmnopqrstuvwxyz0123456789"
				b.WriteByte(alphabet[rng.Intn(len(alphabet))])
			}
		}
		key := b.String()
		encoded := keyCase(key)
		decoded, ok := decodeKey(encoded)
		if !ok || decoded != key {
			t.Fatalf("%q -> %q -> %q", key, encoded, decoded)
		}
		if prior, ok := seen[encoded]; ok && prior != key {
			t.Fatalf("collision %q and %q", prior, key)
		}
		seen[encoded] = key
	}
	t.Logf("20000 keys, %d distinct encodings", len(seen))
}

func TestConstructorRejectsInvalidLanguage(t *testing.T) {
	_, err := Render(vocab.UbiquitousLanguage{Project: "invalid", Contexts: []vocab.BoundedContext{{Name: "bad"}}})
	if err == nil {
		t.Fatal("invalid language rendered")
	}
}

func TestNormalizeDoesNotMutate(t *testing.T) {
	lang := generatedLanguage(t, rand.New(rand.NewSource(1)), 1)
	lang.Contexts[0].Line = 17
	want := lang.Contexts[0]
	_ = normalized(lang)
	if !reflect.DeepEqual(want, lang.Contexts[0]) {
		t.Fatal("normalization mutated source")
	}
}
