package syntax

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestReferenceModels(t *testing.T) {
	files, err := filepath.Glob("testdata/docs/*.md")
	if err != nil {
		t.Fatal(err)
	}
	pre := regexp.MustCompile(`(?s)<pre(?:\s[^>]*)?>(.*?)</pre>`)
	tags := regexp.MustCompile(`<[^>]*>`)
	data, err := os.ReadFile("testdata/docs_fragments.json")
	if err != nil {
		t.Fatal(err)
	}
	var fragments map[string]string
	if err = json.Unmarshal(data, &fragments); err != nil {
		t.Fatal(err)
	}
	failureData, err := os.ReadFile("testdata/docs_expected_failures.json")
	if err != nil {
		t.Fatal(err)
	}
	var failures map[string]string
	if err = json.Unmarshal(failureData, &failures); err != nil {
		t.Fatal(err)
	}
	seenFailures := map[string]bool{}
	seen := map[string]bool{}
	accepted := 0
	for _, path := range files {
		doc, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for i, block := range pre.FindAllSubmatch(doc, -1) {
			key := fmt.Sprintf("%s:%d", filepath.Base(path), i)
			source := html.UnescapeString(tags.ReplaceAllString(string(block[1]), ""))
			if reason := fragments[key]; reason != "" {
				seen[key] = true
				t.Logf("fragment %s: %s", key, reason)
				continue
			}
			file, err := ParseFile(key, []byte(source))
			if reason := failures[key]; reason != "" {
				seenFailures[key] = true
				if err == nil {
					t.Errorf("documented grammar violation unexpectedly parsed: %s", key)
				} else {
					t.Logf("expected documentation failure %s: %s", key, reason)
				}
				continue
			}
			if err != nil {
				t.Errorf("complete reference model: %s", err)
				continue
			}
			output, err := FormatChecked(file)
			if err != nil {
				t.Errorf("%s: %s", key, err)
				continue
			}
			again, err := ParseFile(key, output)
			if err != nil {
				t.Errorf("%s: %s", key, err)
				continue
			}
			if !reflect.DeepEqual(semantic(reflect.ValueOf(file)), semantic(reflect.ValueOf(again))) {
				t.Errorf("%s: formatted model changed its public AST", key)
			}
			if !reflect.DeepEqual(bodyPresence(reflect.ValueOf(file)), bodyPresence(reflect.ValueOf(again))) {
				t.Errorf("%s: authored optional body presence changed", key)
			}
			if string(formatted(t, again)) != string(output) {
				t.Errorf("%s: format is not a fixed point\nFIRST\n%s\nSECOND\n%s", key, output, formatted(t, again))
			}
			accepted++
		}
	}
	for key := range fragments {
		if !seen[key] {
			t.Errorf("unexercised fragment entry %s", key)
		}
	}
	for key := range failures {
		if !seenFailures[key] {
			t.Errorf("unexercised documentation failure %s", key)
		}
	}
	t.Logf("accepted complete reference models %d, expected documentation failures %d, listed fragments %d", accepted, len(seenFailures), len(seen))
}

func TestDocumentedDescriptionRequiresEscape(t *testing.T) {
	data, err := os.ReadFile("testdata/docs/application-and-process-layer.md")
	if err != nil {
		t.Fatal(err)
	}
	pre := regexp.MustCompile(`(?s)<pre(?:\s[^>]*)?>(.*?)</pre>`)
	tags := regexp.MustCompile(`<[^>]*>`)
	blocks := pre.FindAllSubmatch(data, -1)
	if len(blocks) == 0 {
		t.Fatal("reference page has no code blocks")
	}
	source := html.UnescapeString(tags.ReplaceAllString(string(blocks[0][1]), ""))
	_, err = ParseFile("application-and-process-layer.md:0", []byte(source))
	diagnostic, ok := err.(*Error)
	if !ok {
		t.Fatalf("want a syntax diagnostic, got %v", err)
	}
	if diagnostic.Pos.Line != 16 || diagnostic.Pos.Column != 14 || !strings.Contains(diagnostic.Message, `found "description" (description)`) {
		t.Fatalf("want the reserved Attribute.name at 16:14, got %s", diagnostic)
	}
	// ContextMappingDSL:390 reserves description; TacticDDDLanguage:374 requires ID.
	// Escaping the identifier corrects that documented grammar violation.
	corrected := strings.Replace(source, "String description", "String ^description", 1)
	if _, err := Parse([]byte(corrected)); err != nil {
		t.Fatalf("escaped documented model: %s", err)
	}
}

func TestReferenceFragmentReasons(t *testing.T) {
	// This check keeps reasons reviewable rather than accepting unexplained skips.
	data, err := os.ReadFile("testdata/docs_fragments.json")
	if err != nil {
		t.Fatal(err)
	}
	var reasons map[string]string
	if err = json.Unmarshal(data, &reasons); err != nil {
		t.Fatal(err)
	}
	for key, reason := range reasons {
		if !strings.Contains(reason, "fragment") && !strings.Contains(reason, "elision") {
			t.Errorf("%s: incomplete reason", key)
		}
	}
}
