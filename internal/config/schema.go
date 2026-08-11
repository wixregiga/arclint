package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/invopop/jsonschema"
	sj "github.com/santhosh-tekuri/jsonschema/v6"
)

// The schema pipeline: one Go struct definition yields both the runtime
// validator and the published JSON Schema (docs/rules.schema.json, written
// by go:generate). Editor completion and host validation can never drift
// apart because both derive from the same source.

//go:generate go run ../../tools/genschema -out ../../docs/rules.schema.json

// SchemaJSON returns the published JSON Schema for rules.yaml.
func SchemaJSON() ([]byte, error) {
	doc, err := schemaDoc()
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(doc, "", "  ")
}

// schemaDoc reflects RuleSet and patches the spots reflection cannot see:
// the two polymorphic YAML shapes and the closed vocabularies.
func schemaDoc() (map[string]any, error) {
	r := &jsonschema.Reflector{ExpandedStruct: true}
	sch := r.Reflect(&RuleSet{})
	sch.ID = "https://raw.githubusercontent.com/wixregiga/arclint/main/docs/rules.schema.json"
	sch.Title = "arclint rules.yaml"
	sch.Description = "Architecture contracts: modules, consumes/provides/invariants clauses, and graph-wide dependency rules."

	raw, err := json.Marshal(sch)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}

	defs, _ := doc["$defs"].(map[string]any)
	if defs == nil {
		return nil, fmt.Errorf("schema reflection produced no $defs")
	}
	def := func(name string) map[string]any {
		d, _ := defs[name].(map[string]any)
		return d
	}
	props := func(d map[string]any) map[string]any {
		p, _ := d["properties"].(map[string]any)
		return p
	}
	setEnum := func(d map[string]any, field string, values ...any) error {
		p := props(d)
		if p == nil {
			return fmt.Errorf("schema patch: no properties for enum field %s", field)
		}
		f, _ := p[field].(map[string]any)
		if f == nil {
			return fmt.Errorf("schema patch: missing field %s", field)
		}
		f["enum"] = values
		return nil
	}

	// runtime: closed target list.
	rootProps := props(doc)
	if rootProps == nil {
		return nil, fmt.Errorf("schema reflection produced no root properties")
	}
	runtime, _ := rootProps["runtime"].(map[string]any)
	if runtime == nil {
		return nil, fmt.Errorf("schema patch: missing runtime")
	}
	items, _ := runtime["items"].(map[string]any)
	if items == nil {
		return nil, fmt.Errorf("schema patch: runtime has no items")
	}
	items["enum"] = []any{"go", "ts", "py"}
	runtime["minItems"] = 1

	// InternalPolicy: a list is an allow-list; a mapping declares
	// allow/deny explicitly.
	defs["InternalPolicy"] = map[string]any{
		"oneOf": []any{
			map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"allow": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"deny":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				},
				"additionalProperties": false,
			},
		},
	}

	// ProvidesRule.in: module name (registration) or capture side
	// (correspondence).
	pr := def("ProvidesRule")
	if pr == nil {
		return nil, fmt.Errorf("schema patch: missing ProvidesRule")
	}
	prProps := props(pr)
	delete(prProps, "in_module")
	prProps["in"] = map[string]any{
		"oneOf": []any{
			map[string]any{"type": "string"},
			map[string]any{"$ref": "#/$defs/CaptureSide"},
		},
	}

	// Closed vocabularies.
	type enumPatch struct {
		def    string
		field  string
		values []any
	}
	sev := []any{"error", "warn", "info"}
	patches := []enumPatch{
		{"ScanConfig", "unknown_imports", []any{"warn", "error", "ignore"}},
		{"Consumes", "external", []any{"allow", "forbid"}},
		{"Consumes", "stdlib", []any{"allow", "forbid"}},
		{"Consumes", "severity", sev},
		{"ProvidesRule", "kind", []any{"registration", "correspondence"}},
		{"ProvidesRule", "relation", []any{"subset", "equal"}},
		{"ProvidesRule", "severity", sev},
		{"InvariantRule", "kind", []any{"naming", "structure", "content", "expr"}},
		{"InvariantRule", "severity", sev},
		{"GraphRule", "kind", []any{"layers", "forbidden", "independence", "protected", "acyclic"}},
		{"GraphRule", "severity", sev},
	}
	for _, p := range patches {
		d := def(p.def)
		if d == nil {
			return nil, fmt.Errorf("schema patch: missing $defs.%s", p.def)
		}
		if err := setEnum(d, p.field, p.values...); err != nil {
			return nil, err
		}
	}
	return doc, nil
}

var (
	schemaOnce     sync.Once
	compiledSchema *sj.Schema
	schemaErr      error
)

// validateAgainstSchema checks a YAML-decoded document against the
// published schema. The document is round-tripped through JSON so the
// validator sees exactly the JSON data model (and non-string keys fail
// loudly).
func validateAgainstSchema(raw any) error {
	schemaOnce.Do(func() {
		doc, err := schemaDoc()
		if err != nil {
			schemaErr = err
			return
		}
		data, err := json.Marshal(doc)
		if err != nil {
			schemaErr = err
			return
		}
		parsed, err := sj.UnmarshalJSON(bytes.NewReader(data))
		if err != nil {
			schemaErr = err
			return
		}
		c := sj.NewCompiler()
		if err := c.AddResource("file:///arclint/rules.schema.json", parsed); err != nil {
			schemaErr = err
			return
		}
		compiledSchema, schemaErr = c.Compile("file:///arclint/rules.schema.json")
	})
	if schemaErr != nil {
		return fmt.Errorf("internal schema error: %w", schemaErr)
	}

	jsonBytes, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("rules.yaml is not JSON-representable: %w", err)
	}
	instance, err := sj.UnmarshalJSON(bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}
	if err := compiledSchema.Validate(instance); err != nil {
		return fmt.Errorf("schema validation failed:\n%v", err)
	}
	return nil
}
