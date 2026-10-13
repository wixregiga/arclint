package cmlvocab

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

func TestFreshRecordingLayoutAndHeader(t *testing.T) {
	lang, err := vocab.NewUbiquitousLanguage("Project\r\nsecond\nthird\rfourth", "", []vocab.BoundedContext{{
		Name: "one", Definition: "First context.",
		Aggregates:   []vocab.Aggregate{{Name: "Root", Definition: "Root definition.", Identity: "RootId", Entities: []vocab.Entity{{Name: "Member", Definition: "Member definition."}}}},
		ValueObjects: []vocab.ValueObject{{Name: "Activity", Definition: "Activity definition."}, {Name: "Map", Definition: "Map definition."}},
		Services:     []vocab.DomainService{{Name: "Work", Definition: "Work definition."}},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Render(lang)
	if err != nil {
		t.Fatal(err)
	}
	wantHeader := "// The Ubiquitous Language of Project second third fourth, recorded by ArcLint.\n\n"
	if !bytes.HasPrefix(data, []byte(wantHeader)) {
		t.Fatalf("wrong header:\n%s", data)
	}
	if regexp.MustCompile(`\{\s*\}`).Match(data) {
		t.Fatalf("fresh output has an empty body:\n%s", data)
	}
	for _, snippet := range []string{
		"\n\nBoundedContext one",
		"\n\n    \"Member definition.\"\n    Entity Member\n",
		"\n\n  Module one",
		"\n\n    \"Map definition.\"\n    ValueObject ^Map\n",
		"\n  \"Work definition.\"\n  Service Work\n",
	} {
		if !strings.Contains(string(data), snippet) {
			t.Errorf("missing layout %q:\n%s", snippet, data)
		}
	}
	service := bytes.Index(data, []byte("Service Work"))
	aggregate := bytes.Index(data, []byte("Aggregate Root"))
	module := bytes.Index(data, []byte("Module one"))
	if !(service < aggregate && aggregate < module) {
		t.Fatalf("fresh body must place services, aggregates, then Module:\n%s", data)
	}
	assertRoundTrip(t, lang, data)
	// Ordinary prose in a file header is never a carrier for the project name.
	replaced := bytes.Replace(data, []byte(wantHeader), []byte("// An entirely different ordinary comment.\n\n"), 1)
	assertRoundTrip(t, lang, replaced)
}
