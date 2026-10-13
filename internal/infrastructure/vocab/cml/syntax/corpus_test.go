package syntax

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCorpusParse(t *testing.T) {
	t.Parallel()
	accepted, failed := 0, 0
	manifestBytes, err := os.ReadFile("testdata/expected_failures.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]string
	if err = json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	index := -1
	err = filepath.WalkDir("testdata/corpus", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".cml" {
			return nil
		}
		index++
		if _, expectedFailure := manifest[strings.TrimPrefix(filepath.ToSlash(path), "testdata/corpus/")]; !expectedFailure && !sampled(index, 3) {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		file, err := ParseFile(path, source)
		if err != nil {
			failed++
			key := strings.TrimPrefix(filepath.ToSlash(path), "testdata/corpus/")
			if reason := manifest[key]; reason == "" {
				t.Errorf("unexplained failure: %s", err)
			} else {
				seen[key] = true
				t.Logf("expected failure %s: %s", key, reason)
			}
		} else {
			accepted++
			key := strings.TrimPrefix(filepath.ToSlash(path), "testdata/corpus/")
			if reason := manifest[key]; reason != "" {
				t.Errorf("expected failure now parses: %s (%s)", key, reason)
			}
			data, err := FormatChecked(file)
			if err != nil {
				t.Errorf("%s: format: %s", path, err)
				return nil
			}
			assertAuthoredTokens(t, source, data)
			again, err := ParseFile(path, data)
			if err != nil {
				t.Errorf("%s: formatted parse: %s\n%s", path, err, data)
				return nil
			}
			if !reflect.DeepEqual(semantic(reflect.ValueOf(file)), semantic(reflect.ValueOf(again))) {
				a, _ := json.MarshalIndent(semantic(reflect.ValueOf(file)), "", " ")
				b, _ := json.MarshalIndent(semantic(reflect.ValueOf(again)), "", " ")
				t.Errorf("%s: semantic round trip changed\nBEFORE\n%s\nAFTER\n%s", path, a, b)
			}
			if !reflect.DeepEqual(bodyPresence(reflect.ValueOf(file)), bodyPresence(reflect.ValueOf(again))) {
				t.Errorf("%s: authored optional body presence changed", path)
			}
			if string(formatted(t, again)) != string(data) {
				t.Errorf("%s: format is not idempotent\nFIRST\n%s\nSECOND\n%s", path, data, formatted(t, again))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for key := range manifest {
		if !seen[key] {
			t.Errorf("manifest entry was not exercised: %s", key)
		}
	}
	t.Logf("accepted %d, expected failures %d", accepted, failed)
}

func semantic(v reflect.Value) any {
	if !v.IsValid() {
		return nil
	}
	switch v.Kind() {
	case reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return semantic(v.Elem())
	case reflect.Pointer:
		if v.IsNil() {
			return nil
		}
		return semantic(v.Elem())
	case reflect.Struct:
		if v.Type() == reflect.TypeOf(Pos{}) {
			return nil
		}
		m := map[string]any{"$type": v.Type().Name()}
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if f.PkgPath != "" || f.Name == "ExplicitBody" || f.Type == reflect.TypeOf(Pos{}) {
				continue
			}
			m[f.Name] = semantic(v.Field(i))
		}
		return m
	case reflect.Slice:
		if v.Len() == 0 {
			return nil
		}
		var a []any
		for i := 0; i < v.Len(); i++ {
			a = append(a, semantic(v.Index(i)))
		}
		return a
	default:
		return v.Interface()
	}
}

// Body presence is syntax provenance, separate from domain semantics. Parsed
// corpus oracles retain it; constructor-based AST tests can omit it on new nodes.
func bodyPresence(v reflect.Value) []bool {
	if !v.IsValid() {
		return nil
	}
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			return nil
		}
		return bodyPresence(v.Elem())
	case reflect.Struct:
		if v.Type() == reflect.TypeOf(Node{}) {
			return []bool{v.FieldByName("ExplicitBody").Bool()}
		}
		var result []bool
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				result = append(result, bodyPresence(v.Field(i))...)
			}
		}
		return result
	case reflect.Slice:
		var result []bool
		for i := 0; i < v.Len(); i++ {
			result = append(result, bodyPresence(v.Index(i))...)
		}
		return result
	default:
		return nil
	}
}

func TestVendoredCorpusIntegrity(t *testing.T) {
	data, err := os.ReadFile("testdata/corpus_inventory.json")
	if err != nil {
		t.Fatal(err)
	}
	var inventory map[string]string
	if err = json.Unmarshal(data, &inventory); err != nil {
		t.Fatal(err)
	}
	if len(inventory) != 367 {
		t.Fatalf("corpus inventory has %d files, want 367", len(inventory))
	}
	examples, dsl := 0, 0
	for name, digest := range inventory {
		data, err := os.ReadFile(filepath.Join("testdata/corpus", filepath.FromSlash(name)))
		if err != nil {
			t.Error(err)
			continue
		}
		got := fmt.Sprintf("sha256:%x", sha256.Sum256(data))
		if got != digest {
			t.Errorf("vendored input was changed: %s", name)
		}
		if strings.HasPrefix(name, "examples/") {
			examples++
		} else if strings.HasPrefix(name, "dsl/") {
			dsl++
		} else {
			t.Errorf("unknown source repository: %s", name)
		}
	}
	if examples != 55 || dsl != 312 {
		t.Errorf("source counts: examples=%d want 55, dsl=%d want 312", examples, dsl)
	}
}

// Compare lexer tokens, retaining quoted strings, escapes and escaped IDs.
// Comments and layout are independent of authored grammar choices.
func assertAuthoredTokens(t *testing.T, before, after []byte) {
	t.Helper()
	g, err := cmlGrammar()
	if err != nil {
		t.Fatal(err)
	}
	a, err := lex("before", before, g)
	if err != nil {
		t.Fatal(err)
	}
	b, err := lex("after", after, g)
	if err != nil {
		t.Fatal(err)
	}
	texts := func(ts []token) []string {
		var result []string
		for _, token := range ts {
			result = append(result, token.text)
		}
		return result
	}
	if !reflect.DeepEqual(texts(a), texts(b)) {
		for i := 0; i < len(a) && i < len(b); i++ {
			if a[i].text != b[i].text {
				t.Errorf("authored token %d changed: %q at %v -> %q at %v", i, a[i].text, a[i].pos, b[i].text, b[i].pos)
				return
			}
		}
		t.Errorf("authored token count changed: %d -> %d", len(a), len(b))
	}
}
