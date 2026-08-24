package vocab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Schema returns the published library.schema.json bytes: draft 2020-12,
// 2-space indented, trailing newline, key order and compact leaf objects
// matching the litmus file byte-for-byte. Field descriptions are composed
// from taxonomy data (RelationKind meanings via SchemaKindDescription,
// distillation rule ids referenced in prose constants) so schema and
// VOCAB cannot drift on shared facts.
func Schema() ([]byte, error) {
	var buf bytes.Buffer
	if err := writeSchema(&buf); err != nil {
		return nil, fmt.Errorf("marshal library schema: %w", err)
	}
	return buf.Bytes(), nil
}

func writeSchema(buf *bytes.Buffer) error {
	// Verify referenced distillation rule ids exist so schema prose
	// cannot cite a removed rule.
	for _, id := range []string{"identity-test", "value-test", "language-fidelity", "synonym-collapse", "event-detection"} {
		if _, ok := DistillationRuleByID(id); !ok {
			return fmt.Errorf("schema references unknown distillation rule %q", id)
		}
	}

	kindEnum := make([]any, 0, len(RelationKinds()))
	for _, k := range RelationKinds() {
		kindEnum = append(kindEnum, string(k))
	}

	// Compact leaf helpers matching litmus single-line object style:
	// { "type": "string", "minLength": 1, "description": "..." }
	str := func(desc string) compactObject {
		return co(
			"type", "string",
			"minLength", 1,
			"description", desc,
		)
	}
	boolProp := func(desc string) compactObject {
		return co(
			"type", "boolean",
			"description", desc,
		)
	}
	strItems := co("type", "string", "minLength", 1)
	aliases := func(desc string) compactObject {
		return co(
			"type", "array",
			"items", strItems,
			"description", desc,
		)
	}

	root := o(
		"$id", SchemaID,
		"$schema", SchemaDraft,
		"title", SchemaTitle,
		"description", SchemaDescription,
		"type", "object",
		"additionalProperties", false,
		"required", a("version", "contexts"),
		"properties", o(
			"version", o(
				"const", UbiquitousLanguageVersion,
				"description", SchemaVersionDescription,
			),
			"contexts", o(
				"type", "array",
				"description", SchemaContextsDescription,
				"items", o(
					"type", "object",
					"additionalProperties", false,
					"required", a("name"),
					"properties", o(
						"name", o(
							"type", "string",
							"minLength", 1,
							"description", SchemaContextNameDescription,
						),
						"entities", o(
							"type", "array",
							"description", SchemaEntitiesDescription,
							"items", o(
								"type", "object",
								"additionalProperties", false,
								"required", a("name", "definition"),
								"properties", o(
									"name", str(SchemaCanonicalNameDescription),
									"definition", str(SchemaEntityDefinitionDescription),
									"aggregate", boolProp(SchemaAggregateFlagDescription),
									"aliases", aliases(SchemaEntityAliasesDescription),
								),
							),
						),
						"value_objects", o(
							"type", "array",
							"description", SchemaValueObjectsDescription,
							"items", o(
								"type", "object",
								"additionalProperties", false,
								"required", a("name", "definition"),
								"properties", o(
									"name", str(SchemaCanonicalNameDescription),
									"definition", str(SchemaValueDefinitionDescription),
									"aliases", aliases(SchemaValueAliasesDescription),
								),
							),
						),
						"invariants", o(
							"type", "array",
							"description", SchemaInvariantsDescription,
							"items", o(
								"type", "object",
								"additionalProperties", false,
								"required", a("statement", "owner"),
								"properties", o(
									"statement", str(SchemaStatementDescription),
									"owner", str(SchemaOwnerDescription),
								),
							),
						),
						"events", o(
							"type", "array",
							"description", SchemaEventsDescription,
							"items", o(
								"type", "object",
								"additionalProperties", false,
								"required", a("name", "definition"),
								"properties", o(
									"name", str(SchemaEventNameDescription),
									"definition", str(SchemaEventDefinitionDescription),
								),
							),
						),
					),
				),
			),
			"relations", o(
				"type", "array",
				"description", SchemaRelationsDescription,
				"items", o(
					"type", "object",
					"additionalProperties", false,
					"required", a("from", "to", "kind"),
					"properties", o(
						"from", str(SchemaFromDescription),
						"to", str(SchemaToDescription),
						"kind", o(
							"enum", kindEnum,
							"description", SchemaKindDescription(),
						),
					),
				),
			),
		),
	)

	if err := writeJSON(buf, root, 0); err != nil {
		return err
	}
	buf.WriteByte('\n')
	return nil
}

// orderedObject preserves insertion order for multi-line JSON objects.
type orderedObject struct {
	keys   []string
	values []any
}

// o pairs keys with values in insertion order. Malformed pairs (odd
// count, non-string key) are skipped; the litmus byte-comparison test
// surfaces any such programmer error as schema drift.
func o(kv ...any) orderedObject {
	obj := orderedObject{
		keys:   make([]string, 0, len(kv)/2),
		values: make([]any, 0, len(kv)/2),
	}
	for len(kv) >= 2 {
		if key, ok := kv[0].(string); ok {
			obj.keys = append(obj.keys, key)
			obj.values = append(obj.values, kv[1])
		}
		kv = kv[2:]
	}
	return obj
}

// compactObject is emitted on one line: { "k": v, "k2": v2 }.
type compactObject struct {
	keys   []string
	values []any
}

// co pairs keys with values like o; the same litmus test guards misuse.
func co(kv ...any) compactObject {
	obj := compactObject{
		keys:   make([]string, 0, len(kv)/2),
		values: make([]any, 0, len(kv)/2),
	}
	for len(kv) >= 2 {
		if key, ok := kv[0].(string); ok {
			obj.keys = append(obj.keys, key)
			obj.values = append(obj.values, kv[1])
		}
		kv = kv[2:]
	}
	return obj
}

func a(vals ...string) []any {
	out := make([]any, len(vals))
	for i, v := range vals {
		out[i] = v
	}
	return out
}

func writeJSON(buf *bytes.Buffer, v any, depth int) error {
	switch val := v.(type) {
	case orderedObject:
		if len(val.keys) == 0 {
			buf.WriteString("{}")
			return nil
		}
		buf.WriteString("{\n")
		for i, key := range val.keys {
			writeIndent(buf, depth+1)
			kb, err := marshalJSONValue(key)
			if err != nil {
				return err
			}
			buf.Write(kb)
			buf.WriteString(": ")
			if err := writeJSON(buf, val.values[i], depth+1); err != nil {
				return err
			}
			if i < len(val.keys)-1 {
				buf.WriteByte(',')
			}
			buf.WriteByte('\n')
		}
		writeIndent(buf, depth)
		buf.WriteByte('}')
		return nil
	case compactObject:
		return writeCompactObject(buf, val)
	case []any:
		buf.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				buf.WriteString(", ")
			}
			if err := writeJSONCompact(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
		return nil
	case string:
		b, err := marshalJSONValue(val)
		if err != nil {
			return err
		}
		buf.Write(b)
		return nil
	case bool:
		if val {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
		return nil
	case int:
		buf.WriteString(strconv.Itoa(val))
		return nil
	default:
		b, err := marshalJSONValue(val)
		if err != nil {
			return err
		}
		buf.Write(b)
		return nil
	}
}

// marshalJSONValue wraps encoding/json errors with the document being
// built so failures name the ubiquitous-language schema.
func marshalJSONValue(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal ubiquitous-language schema value: %w", err)
	}
	return b, nil
}

func writeCompactObject(buf *bytes.Buffer, val compactObject) error {
	buf.WriteString("{ ")
	for i, key := range val.keys {
		if i > 0 {
			buf.WriteString(", ")
		}
		kb, err := marshalJSONValue(key)
		if err != nil {
			return err
		}
		buf.Write(kb)
		buf.WriteString(": ")
		if err := writeJSONCompact(buf, val.values[i]); err != nil {
			return err
		}
	}
	buf.WriteString(" }")
	return nil
}

func writeJSONCompact(buf *bytes.Buffer, v any) error {
	switch val := v.(type) {
	case compactObject:
		return writeCompactObject(buf, val)
	case string:
		b, err := marshalJSONValue(val)
		if err != nil {
			return err
		}
		buf.Write(b)
		return nil
	case int:
		buf.WriteString(strconv.Itoa(val))
		return nil
	case bool:
		if val {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
		return nil
	case []any:
		buf.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				buf.WriteString(", ")
			}
			if err := writeJSONCompact(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
		return nil
	default:
		b, err := marshalJSONValue(val)
		if err != nil {
			return err
		}
		buf.Write(b)
		return nil
	}
}

func writeIndent(buf *bytes.Buffer, depth int) {
	buf.WriteString(strings.Repeat("  ", depth))
}
