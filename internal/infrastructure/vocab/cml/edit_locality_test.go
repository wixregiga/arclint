package cmlvocab

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

func TestCommittedRecordingEditLocality(t *testing.T) {
	t.Parallel()
	for fileIndex, path := range []string{"../../../../domain.arclint.cml", "../../../../testing/boxoffice/domain.arclint.cml"} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for kindIndex, kind := range []string{"invariant", "assertion", "aggregate", "entity", "value_object", "event", "service", "specification", "question", "relation", "context"} {
			// A sampled run edits each kind in one of the two recordings.
			if !sampled(kindIndex+fileIndex, 2) {
				continue
			}
			t.Run(filepath.ToSlash(path)+"/"+kind, func(t *testing.T) {
				t.Parallel()
				lang, err := Parse(source, recording(t))
				if err != nil {
					t.Fatal(err)
				}
				old, added := editLocalityElement(t, &lang, kind)
				repo := syncFixtureRepository(t, string(source))
				if err := repo.Record(lang); err != nil {
					t.Fatal(err)
				}
				actual, err := os.ReadFile(repo.Path())
				if err != nil {
					t.Fatal(err)
				}
				if old != "" {
					expected := bytes.Replace(source, []byte(quote(old)), []byte(quote("Formatter locality change.")), 1)
					if !bytes.Equal(actual, expected) {
						t.Fatalf("%s edit changed bytes outside its doc/string line", kind)
					}
					t.Log("exactly one string line changed")
				} else {
					assertOnlyInsertedLines(t, source, actual, added)
					t.Logf("kind absent in committed recording; inserted only its %d declaration lines", len(added))
				}
				exportOracleCopy(t, filepath.Base(filepath.Dir(path))+"-"+kind+".cml", actual)
			})
		}
	}
}

func editLocalityElement(t *testing.T, lang *vocab.UbiquitousLanguage, kind string) (string, []string) {
	t.Helper()
	change := func(text *string) string { old := *text; *text = "Formatter locality change."; return old }
	for ci := range lang.Contexts {
		c := &lang.Contexts[ci]
		switch kind {
		case "context":
			return change(&c.Definition), nil
		case "aggregate":
			if len(c.Aggregates) > 0 {
				return change(&c.Aggregates[0].Definition), nil
			}
		case "entity":
			for ai := range c.Aggregates {
				if len(c.Aggregates[ai].Entities) > 0 {
					return change(&c.Aggregates[ai].Entities[0].Definition), nil
				}
			}
		case "value_object":
			if len(c.ValueObjects) > 0 {
				return change(&c.ValueObjects[0].Definition), nil
			}
		case "event":
			if len(c.Events) > 0 {
				return change(&c.Events[0].Definition), nil
			}
		case "service":
			if len(c.Services) > 0 {
				return change(&c.Services[0].Definition), nil
			}
		case "specification":
			if len(c.Specifications) > 0 {
				return change(&c.Specifications[0].Definition), nil
			}
		case "question":
			if len(c.Questions) > 0 {
				return change(&c.Questions[0].Text), nil
			}
		case "invariant":
			for ai := range c.Aggregates {
				if len(c.Aggregates[ai].Invariants) > 0 {
					return change(&c.Aggregates[ai].Invariants[0].Statement), nil
				}
			}
			for vi := range c.ValueObjects {
				if len(c.ValueObjects[vi].Invariants) > 0 {
					return change(&c.ValueObjects[vi].Invariants[0].Statement), nil
				}
			}
		case "assertion":
			for ai := range c.Aggregates {
				if len(c.Aggregates[ai].Assertions) > 0 {
					return change(&c.Aggregates[ai].Assertions[0].Statement), nil
				}
			}
			for si := range c.Services {
				if len(c.Services[si].Assertions) > 0 {
					return change(&c.Services[si].Assertions[0].Statement), nil
				}
			}
		case "relation":
			if len(lang.Relations) > 0 {
				return change(&lang.Relations[0].Description), nil
			}
		}
	}
	c := &lang.Contexts[0]
	switch kind {
	case "entity":
		for ci := range lang.Contexts {
			if len(lang.Contexts[ci].Aggregates) > 0 {
				lang.Contexts[ci].Aggregates[0].Entities = append(lang.Contexts[ci].Aggregates[0].Entities, vocab.Entity{Name: "FormatterMember", Definition: "Formatter locality change."})
				return "", []string{quote("Formatter locality change."), "Entity FormatterMember"}
			}
		}
	case "event":
		c.Events = append(c.Events, vocab.DomainEvent{Name: "FormatterEvent", Definition: "Formatter locality change."})
		return "", []string{quote("Formatter locality change."), "DomainEvent FormatterEvent"}
	case "service":
		c.Services = append(c.Services, vocab.DomainService{Name: "FormatterService", Definition: "Formatter locality change."})
		return "", []string{quote("Formatter locality change."), "Service FormatterService"}
	case "specification":
		c.Specifications = append(c.Specifications, vocab.Specification{Name: "FormatterSpecification", Definition: "Formatter locality change."})
		return "", []string{quote("Formatter locality change."), "ValueObject FormatterSpecification {", "def boolean satisfiedBy();", "}"}
	case "assertion":
		for ci := range lang.Contexts {
			if len(lang.Contexts[ci].Aggregates) > 0 {
				a := &lang.Contexts[ci].Aggregates[0]
				a.Assertions = append(a.Assertions, vocab.Assertion{Key: "formatter-promise", On: "Review", Statement: "Formatter locality change."})
				return "", []string{"def void Review();", quote("Formatter locality change."), `def void assertFormatterPromise() hint = "arclint.on=Review";`}
			}
		}
	}
	t.Fatalf("no insertion strategy for absent %s", kind)
	return "", nil
}

func assertOnlyInsertedLines(t *testing.T, before, after []byte, added []string) {
	t.Helper()
	lines := func(data []byte) []string {
		var result []string
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) != "" {
				result = append(result, line)
			}
		}
		return result
	}
	a, b := lines(before), lines(after)
	// A subsequence match allows insertions, never rewrites or deletions. Expected
	// inserted braces can equal existing braces, so prefer matching existing lines.
	var match func(int, int, []string) bool
	match = func(i, j int, extra []string) bool {
		if j == len(b) {
			return i == len(a) && len(extra) == len(added)
		}
		if i < len(a) && a[i] == b[j] && match(i+1, j+1, extra) {
			return true
		}
		if len(extra) < len(added) && strings.TrimSpace(b[j]) == added[len(extra)] {
			return match(i, j+1, append(extra, b[j]))
		}
		return false
	}
	if !match(0, 0, nil) {
		t.Fatalf("edit did not insert exactly its declaration lines %q", added)
	}
}

func exportOracleCopy(t *testing.T, name string, data []byte) {
	t.Helper()
	dir := os.Getenv("CML_ORACLE_OUTPUT")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestReproducedDomainRecordingDefinitionEdit(t *testing.T) {
	source, err := os.ReadFile("../../../../domain.arclint.cml")
	if err != nil {
		t.Fatal(err)
	}
	// The edit that once restyled 312 lines: a new DomainRecording
	// definition in the committed copy. The new text is derived from the
	// current one, so the edit is a real change whatever the copy holds.
	lang, err := Parse(source, recording(t))
	if err != nil {
		t.Fatal(err)
	}
	old, definition := "", ""
	for ci := range lang.Contexts {
		for vi := range lang.Contexts[ci].ValueObjects {
			v := &lang.Contexts[ci].ValueObjects[vi]
			if v.Name == "DomainRecording" {
				old = v.Definition
				definition = old + " This sentence is the edit."
				v.Definition = definition
			}
		}
	}
	if old == "" {
		t.Fatal("the committed copy records no DomainRecording definition")
	}
	repo := syncFixtureRepository(t, string(source))
	if err := repo.Record(lang); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(repo.Path())
	if err != nil {
		t.Fatal(err)
	}
	expected := bytes.Replace(source, []byte(quote(old)), []byte(quote(definition)), 1)
	if !bytes.Equal(actual, expected) {
		t.Fatal("reproduced edit changed more than the definition string")
	}
	a, b := strings.Split(string(source), "\n"), strings.Split(string(actual), "\n")
	changed := 0
	for i := range a {
		if a[i] != b[i] {
			changed++
		}
	}
	if changed != 1 {
		t.Fatalf("changed %d lines, want 1", changed)
	}
	t.Log(fmt.Sprintf("%d removed line, %d added line; all other bytes identical", changed, changed))
	exportOracleCopy(t, "reproduced-definition.cml", actual)
}
