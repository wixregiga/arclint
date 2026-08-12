package config

import (
	"encoding/json"
	"testing"
)

// TestRuleDocsCoverSchemaKinds proves the doc table and the published
// schema cannot drift: every kind the schema admits has documentation,
// and no documentation names a kind the schema does not admit.
func TestRuleDocsCoverSchemaKinds(t *testing.T) {
	data, err := SchemaJSON()
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	defs := doc["$defs"].(map[string]any)
	kindEnum := func(def string) []string {
		d := defs[def].(map[string]any)
		enum := d["properties"].(map[string]any)["kind"].(map[string]any)["enum"].([]any)
		out := make([]string, len(enum))
		for i, v := range enum {
			out[i] = v.(string)
		}
		return out
	}

	want := map[string]bool{"modules": true, "consumes": true, "scan": true}
	for _, def := range []string{"ProvidesRule", "InvariantRule", "GraphRule"} {
		for _, k := range kindEnum(def) {
			want[k] = true
		}
	}

	have := map[string]bool{}
	for _, d := range RuleDocs {
		if have[d.Kind] {
			t.Errorf("duplicate doc for kind %q", d.Kind)
		}
		have[d.Kind] = true
		if d.Summary == "" || d.Doc == "" || d.Example == "" || d.Where == "" {
			t.Errorf("kind %q: incomplete doc (summary/doc/example/where all required)", d.Kind)
		}
	}
	for k := range want {
		if !have[k] {
			t.Errorf("schema kind %q has no entry in RuleDocs", k)
		}
	}
	for k := range have {
		if !want[k] {
			t.Errorf("RuleDocs documents %q, which the schema does not admit", k)
		}
	}
}
