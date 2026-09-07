package vocab_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// repoRoot locates the repository root from this source file, keeping
// the tests independent of the working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller: no source location")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

// TestSchemaIsDeterministicIndentedJSON proves the published bytes are
// reproducible: identical across calls, valid JSON, indented, and
// newline-terminated so the committed file compares byte-for-byte.
func TestSchemaIsDeterministicIndentedJSON(t *testing.T) {
	first, err := vocab.Schema()
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	second, err := vocab.Schema()
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

// TestSchemaMatchesPublishedSchema is the drift half of the domain
// schema invariant: vocab.Schema() must equal the published
// docs/schemas/domain.arclint.schema.json byte-for-byte.
//
// On failure: regenerate the committed schemas via make schemas, or fix
// the generator; never edit the committed copies by hand.
func TestSchemaMatchesPublishedSchema(t *testing.T) {
	wantPath := filepath.Join(repoRoot(t), "docs", "schemas", vocab.SchemaFileName)
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read published %s: %v", vocab.SchemaFileName, err)
	}
	got, err := vocab.Schema()
	if err != nil {
		t.Fatalf("vocab.Schema: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("vocab.Schema() drifted from published %s; run make schemas, or fix the generator; never edit the committed copies by hand\n--- first diff hint ---\n%s",
			vocab.SchemaFileName, firstDiff(got, want))
	}
}

// schemaDoc is the part of the published schema the tests read back.
type schemaDoc struct {
	ID         string                `json:"$id"`
	Schema     string                `json:"$schema"`
	Required   []string              `json:"required"`
	Properties map[string]schemaNode `json:"properties"`
	Defs       map[string]schemaNode `json:"$defs"`
}

type schemaNode struct {
	Type                 string                    `json:"type"`
	Description          string                    `json:"description"`
	Required             []string                  `json:"required"`
	Properties           map[string]schemaNode     `json:"properties"`
	PropertyNames        *struct{ Pattern string } `json:"propertyNames"`
	AdditionalProperties json.RawMessage           `json:"additionalProperties"`
	Enum                 []string                  `json:"enum"`
	Const                any                       `json:"const"`
}

func publishedSchema(t *testing.T) schemaDoc {
	t.Helper()
	raw, err := vocab.Schema()
	if err != nil {
		t.Fatalf("vocab.Schema: %v", err)
	}
	var doc schemaDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	return doc
}

func TestSchemaIdentifiesItself(t *testing.T) {
	doc := publishedSchema(t)
	if doc.ID != vocab.SchemaID {
		t.Errorf("$id = %q, want %q", doc.ID, vocab.SchemaID)
	}
	if doc.Schema != vocab.SchemaDraft {
		t.Errorf("$schema = %q, want %q", doc.Schema, vocab.SchemaDraft)
	}
	if v := doc.Properties["version"]; v.Const != float64(vocab.UbiquitousLanguageVersion) {
		t.Errorf("version const = %v, want %d", v.Const, vocab.UbiquitousLanguageVersion)
	}
	if got := doc.Required; !slices.Equal(got, []string{"version", "project", "contexts"}) {
		t.Errorf("required = %v", got)
	}
}

// Every object of the schema is a building block's recording: the
// properties are the block's recorded properties (the map key aside),
// each required as the block marks it, each described by the block's
// definition of it. The map keys of each block are recorded by the
// block's own key or name property, so the schema cannot describe a
// property the meta-model does not record and cannot omit one it does.
func TestSchemaObjectsAreTheMetaModelRecordings(t *testing.T) {
	doc := publishedSchema(t)
	model := vocab.DDD()
	cases := []struct {
		term string
		node schemaNode
		key  string
	}{
		{"domain", schemaNode{Required: doc.Required, Properties: doc.Properties}, "version"},
		{"bounded_context", doc.Defs["context"], "name"},
		{"aggregate", doc.Defs["aggregate"], "name"},
		{"entity", doc.Defs["entity"], "name"},
		{"value_object", doc.Defs["value_object"], "name"},
		{"assertion", doc.Defs["assertion"], "key"},
		{"domain_event", doc.Defs["event"], "name"},
		{"domain_service", doc.Defs["service"], "name"},
		{"specification", doc.Defs["specification"], "name"},
		{"context_relation", doc.Defs["relation"], ""},
	}
	for _, c := range cases {
		block, ok := model.Block(c.term)
		if !ok {
			t.Fatalf("block %q missing", c.term)
		}
		var wantProps, wantRequired []string
		for _, p := range block.Records.Properties {
			if p.Name == c.key {
				continue
			}
			wantProps = append(wantProps, p.Name)
			if p.Required {
				wantRequired = append(wantRequired, p.Name)
			}
			node, ok := c.node.Properties[p.Name]
			if !ok {
				t.Errorf("%s: schema lacks recorded property %q", c.term, p.Name)
				continue
			}
			if node.Description != p.Definition && p.Name != "kind" && p.Name != "version" {
				t.Errorf("%s.%s: description %q, the meta-model records %q", c.term, p.Name, node.Description, p.Definition)
			}
		}
		var gotProps []string
		for name := range c.node.Properties {
			if name == c.key {
				continue
			}
			gotProps = append(gotProps, name)
		}
		sort.Strings(gotProps)
		sort.Strings(wantProps)
		if !slices.Equal(gotProps, wantProps) {
			t.Errorf("%s: schema properties %v, the meta-model records %v", c.term, gotProps, wantProps)
		}
		gotRequired := slices.DeleteFunc(slices.Clone(c.node.Required), func(s string) bool { return s == c.key })
		sort.Strings(gotRequired)
		sort.Strings(wantRequired)
		if !slices.Equal(gotRequired, wantRequired) {
			t.Errorf("%s: schema requires %v, the meta-model requires %v", c.term, gotRequired, wantRequired)
		}
		if c.term != "domain" && c.node.Description != block.Definition.Text {
			t.Errorf("%s: object description is not the block's definition", c.term)
		}
	}
}

// The maps keyed by name or key carry the published name patterns, and
// the leaf maps (invariants, questions) are keyed statements.
func TestSchemaKeyedMapsCarryTheNamePatterns(t *testing.T) {
	doc := publishedSchema(t)
	contexts := doc.Properties["contexts"]
	if contexts.PropertyNames == nil || contexts.PropertyNames.Pattern != vocab.ContextNamePattern {
		t.Errorf("contexts propertyNames = %+v", contexts.PropertyNames)
	}
	context := doc.Defs["context"]
	for _, section := range []string{"aggregates", "value_objects", "events", "services", "specifications"} {
		node := context.Properties[section]
		if node.PropertyNames == nil || node.PropertyNames.Pattern != vocab.TermNamePattern {
			t.Errorf("%s propertyNames = %+v", section, node.PropertyNames)
		}
	}
	aggregate := doc.Defs["aggregate"]
	for _, section := range []string{"invariants", "assertions"} {
		node := aggregate.Properties[section]
		if node.PropertyNames == nil || node.PropertyNames.Pattern != vocab.KeyPattern {
			t.Errorf("aggregate.%s propertyNames = %+v", section, node.PropertyNames)
		}
	}
	if node := context.Properties["questions"]; node.PropertyNames == nil || node.PropertyNames.Pattern != vocab.KeyPattern {
		t.Errorf("questions propertyNames = %+v", node.PropertyNames)
	}
	var leaf schemaNode
	if err := json.Unmarshal(aggregate.Properties["invariants"].AdditionalProperties, &leaf); err != nil || leaf.Type != "string" {
		t.Errorf("aggregate invariants are keyed statements; got %s (%v)", aggregate.Properties["invariants"].AdditionalProperties, err)
	}
	if err := json.Unmarshal(doc.Defs["value_object"].Properties["invariants"].AdditionalProperties, &leaf); err != nil || leaf.Type != "string" {
		t.Errorf("value object invariants are keyed statements; got %s (%v)", doc.Defs["value_object"].Properties["invariants"].AdditionalProperties, err)
	}
	kinds := doc.Defs["relation"].Properties["kind"].Enum
	var want []string
	for _, k := range vocab.RelationKinds() {
		want = append(want, string(k))
	}
	if !slices.Equal(kinds, want) {
		t.Errorf("kind enum = %v, want %v", kinds, want)
	}
}
