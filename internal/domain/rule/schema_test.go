package rule_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// TestSchemaIsDeterministicIndentedJSON proves the published bytes are
// reproducible: identical across calls, valid JSON, indented, and
// newline-terminated so the committed file compares byte-for-byte.
func TestSchemaIsDeterministicIndentedJSON(t *testing.T) {
	first, err := rule.Schema()
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	second, err := rule.Schema()
	if err != nil {
		t.Fatalf("Schema (second call): %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("Schema output differs between calls")
	}
	if !json.Valid(first) {
		t.Errorf("Schema output is not valid JSON")
	}
	if !bytes.HasPrefix(first, []byte("{\n  \"")) {
		t.Errorf("Schema output is not indented: starts %q", first[:min(len(first), 8)])
	}
	if !bytes.HasSuffix(first, []byte("}\n")) {
		t.Errorf("Schema output does not end with a newline-terminated object")
	}
}

// TestSchemaPublishesDomainEnums proves every finite vocabulary in the
// schema matches the domain's published values.
func TestSchemaPublishesDomainEnums(t *testing.T) {
	doc := schemaTree(t)
	wantSeverities := []string{
		string(rule.SeverityError), string(rule.SeverityWarning), string(rule.SeverityInfo),
	}
	assertStrings(t, "severity enum", dig(t, doc, "$defs", "severity", "enum"), wantSeverities)
	if got := dig(t, doc, "$defs", "severity", "default"); got != string(rule.DefaultSeverity) {
		t.Errorf("severity default = %v, want %q", got, rule.DefaultSeverity)
	}
	wantPolicies := []string{string(rule.ImportAllow), string(rule.ImportForbid)}
	assertStrings(t, "import policy enum", dig(t, doc, "$defs", "importPolicy", "enum"), wantPolicies)
	wantUnknown := []string{
		string(rule.UnknownImportsError), string(rule.UnknownImportsWarn), string(rule.UnknownImportsIgnore),
	}
	assertStrings(t, "unknown_imports enum",
		dig(t, doc, "properties", "scan", "properties", "unknown_imports", "enum"), wantUnknown)
	targets := []string{"go", "ts", "py"}
	assertStrings(t, "runtime enum",
		dig(t, doc, "properties", "runtime", "items", "enum"), targets)
	// One spelling of language targets across the format: coverage in a
	// Pattern file accepts exactly what runtime accepts.
	assertStrings(t, "pattern coverage enum",
		dig(t, doc, "properties", "pattern", "properties", "coverage", "items", "enum"), targets)
}

// TestSchemaCoversEveryRuleType proves each published Rule Type owns
// exactly one shape in the document schema, named after the Type and
// requiring the Type's one assertion key, and that the rule entry is
// the choice among those shapes plus the Override.
func TestSchemaCoversEveryRuleType(t *testing.T) {
	doc := schemaTree(t)
	alternatives, ok := dig(t, doc, "$defs", "rule", "oneOf").([]any)
	if !ok || len(alternatives) != len(rule.Types())+1 {
		t.Fatalf("rule oneOf = %v, want one shape per Type plus the override", alternatives)
	}
	for i, typ := range rule.Types() {
		def := string(typ) + "Rule"
		if ref := alternatives[i].(map[string]any)["$ref"]; ref != "#/$defs/"+def {
			t.Errorf("rule oneOf[%d] = %v, want %s", i, ref, def)
		}
		required, ok := dig(t, doc, "$defs", def, "required").([]any)
		if !ok || !containsString(required, typ.AssertionKey()) {
			t.Errorf("%s required = %v, want the assertion key %q", def, required, typ.AssertionKey())
		}
		props := dig(t, doc, "$defs", def, "properties").(map[string]any)
		if _, ok := props[typ.AssertionKey()]; !ok {
			t.Errorf("%s lacks its assertion key %q", def, typ.AssertionKey())
		}
		for _, other := range rule.AssertionKeys() {
			if other == typ.AssertionKey() {
				continue
			}
			if _, ok := props[other]; ok {
				t.Errorf("%s accepts a second assertion key %q", def, other)
			}
		}
		_, hasOn := props["on"]
		switch typ.Scope() {
		case rule.ScopeModules, rule.ScopeOneModule:
			if !hasOn || !containsString(required, "on") {
				t.Errorf("%s must require on", def)
			}
		case rule.ScopeRepository:
			if hasOn {
				t.Errorf("%s must not accept on", def)
			}
		case rule.ScopeModulesOrRepository:
			if !hasOn || containsString(required, "on") {
				t.Errorf("%s must accept an optional on", def)
			}
		}
		if _, hasFiles := props["files"]; hasFiles != typ.AcceptsFiles() {
			t.Errorf("%s files = %v, want %v", def, hasFiles, typ.AcceptsFiles())
		}
	}
	if ref := alternatives[len(alternatives)-1].(map[string]any)["$ref"]; ref != "#/$defs/override" {
		t.Errorf("rule oneOf last = %v, want the override", ref)
	}
	overrideProps := dig(t, doc, "$defs", "override", "properties").(map[string]any)
	for _, key := range rule.AssertionKeys() {
		if _, ok := overrideProps[key]; ok {
			t.Errorf("override must not accept assertion key %q", key)
		}
	}
	if _, ok := overrideProps["description"]; ok {
		t.Errorf("override must not accept a description")
	}
}

// TestSchemaRejectsUnknownKeys proves the schema mirrors the loader's
// strict decoding: every object shape closes with
// additionalProperties: false, and the Pattern branch forbids the
// repository-only keys.
func TestSchemaRejectsUnknownKeys(t *testing.T) {
	doc := schemaTree(t)
	if got := dig(t, doc, "additionalProperties"); got != false {
		t.Errorf("document additionalProperties = %v, want false", got)
	}
	shapes := []string{"override", "exclusion", "suppression"}
	for _, typ := range rule.Types() {
		shapes = append(shapes, string(typ)+"Rule")
	}
	for _, name := range shapes {
		if got := dig(t, doc, "$defs", name, "additionalProperties"); got != false {
			t.Errorf("%s additionalProperties = %v, want false", name, got)
		}
	}
	for _, name := range []string{"pattern", "scan"} {
		if got := dig(t, doc, "properties", name, "additionalProperties"); got != false {
			t.Errorf("properties.%s additionalProperties = %v, want false", name, got)
		}
	}
	if got := dig(t, doc, "properties", "extends", "items", "additionalProperties"); got != false {
		t.Errorf("extends item additionalProperties = %v, want false", got)
	}
	for _, key := range []string{"runtime", "scan", "extends"} {
		if got := dig(t, doc, "then", "properties", key); got != false {
			t.Errorf("pattern branch %s = %v, want false", key, got)
		}
	}
	patternRules, ok := dig(t, doc, "then", "properties", "rules", "additionalProperties", "oneOf").([]any)
	if !ok || len(patternRules) != len(rule.Types()) {
		t.Fatalf("pattern branch rules oneOf = %v, want one alternative per Type and no override", patternRules)
	}
	for _, alt := range patternRules {
		if ref := alt.(map[string]any)["$ref"]; ref == "#/$defs/override" {
			t.Errorf("a pattern file must not accept an override")
		}
	}
}

func containsString(list []any, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func schemaTree(t *testing.T) map[string]any {
	t.Helper()
	data, err := rule.Schema()
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	return doc
}

// dig walks nested schema objects and fails the test on a missing step.
func dig(t *testing.T, root map[string]any, path ...string) any {
	t.Helper()
	var current any = root
	for i, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("path %v: step %d is %T, not an object", path, i, current)
		}
		current, ok = object[key]
		if !ok {
			t.Fatalf("path %v: missing key %q", path, key)
		}
	}
	return current
}

func assertStrings(t *testing.T, what string, got any, want []string) {
	t.Helper()
	list, ok := got.([]any)
	if !ok {
		t.Fatalf("%s: %T is not a list", what, got)
	}
	if len(list) != len(want) {
		t.Fatalf("%s = %v, want %v", what, list, want)
	}
	for i, v := range list {
		if v != want[i] {
			t.Errorf("%s[%d] = %v, want %q", what, i, v, want[i])
		}
	}
}
