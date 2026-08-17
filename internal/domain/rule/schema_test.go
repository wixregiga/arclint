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
	assertStrings(t, "runtime enum",
		dig(t, doc, "properties", "runtime", "items", "enum"), []string{"go", "ts", "py"})
	languages := make([]string, 0, len(rule.Languages()))
	for _, l := range rule.Languages() {
		languages = append(languages, string(l))
	}
	assertStrings(t, "pattern coverage enum",
		dig(t, doc, "properties", "pattern", "properties", "coverage", "items", "enum"), languages)
}

// TestSchemaCoversEveryRuleType proves each published Rule Type owns
// exactly one shape in the document schema: consumes as the contract
// key, the invariant kinds, and the dependency kinds by const.
func TestSchemaCoversEveryRuleType(t *testing.T) {
	doc := schemaTree(t)
	kindDefs := map[rule.Type]string{
		rule.TypeStructure: "structureInvariant",
		rule.TypeNaming:    "namingInvariant",
		rule.TypeExtension: "extensionInvariant",
		rule.TypeLayers:    "layersDependency",
		rule.TypeProtected: "protectedDependency",
		rule.TypeAcyclic:   "acyclicDependency",
	}
	for _, typ := range rule.Types() {
		if typ == rule.TypeConsumes {
			if _, ok := dig(t, doc, "$defs", "consumes").(map[string]any); !ok {
				t.Errorf("missing consumes definition")
			}
			continue
		}
		def, ok := kindDefs[typ]
		if !ok {
			t.Errorf("rule type %q has no schema shape", typ)
			continue
		}
		if got := dig(t, doc, "$defs", def, "properties", "kind", "const"); got != string(typ) {
			t.Errorf("%s kind const = %v, want %q", def, got, typ)
		}
	}
}

// TestSchemaRejectsUnknownKeys proves the schema mirrors the loader's
// strict decoding: every object shape closes with
// additionalProperties: false and requires an explicit Rule ID where it
// describes a Rule.
func TestSchemaRejectsUnknownKeys(t *testing.T) {
	doc := schemaTree(t)
	if got := dig(t, doc, "additionalProperties"); got != false {
		t.Errorf("document additionalProperties = %v, want false", got)
	}
	ruleShapes := []string{
		"consumes", "structureInvariant", "namingInvariant", "extensionInvariant",
		"layersDependency", "protectedDependency", "acyclicDependency",
	}
	for _, name := range ruleShapes {
		if got := dig(t, doc, "$defs", name, "additionalProperties"); got != false {
			t.Errorf("%s additionalProperties = %v, want false", name, got)
		}
		required, ok := dig(t, doc, "$defs", name, "required").([]any)
		if !ok || len(required) == 0 || required[0] != "id" {
			t.Errorf("%s required = %v, want id first", name, required)
		}
	}
	for _, name := range []string{"pattern", "scan"} {
		if got := dig(t, doc, "properties", name, "additionalProperties"); got != false {
			t.Errorf("properties.%s additionalProperties = %v, want false", name, got)
		}
	}
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
