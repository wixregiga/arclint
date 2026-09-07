package vocab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Schema returns the published domain.arclint.schema.json bytes: draft
// 2020-12, 2-space indented, trailing newline, key order and compact
// leaf objects matching the litmus file byte-for-byte. The shape is the
// meta-model's: every object is a building block's `records`, every
// property description is that block's property definition, and the
// relation kinds are the context_relation block's kinds, so the schema
// cannot say anything the meta-model does not.
func Schema() ([]byte, error) {
	var buf bytes.Buffer
	if err := writeSchema(&buf); err != nil {
		return nil, fmt.Errorf("marshal library schema: %w", err)
	}
	return buf.Bytes(), nil
}

// The published name patterns of the domain file, shared by the loader
// and the schema. A context name is spelled like a Zone name, so a Zone
// spelled with it can narrow its code; a term is any non-empty name
// without surrounding whitespace, in whatever case the language's type
// names take; a key is kebab-case.
const (
	ContextNamePattern = `^[a-z][a-z0-9_-]*$`
	TermNamePattern    = `^\S(.*\S)?$`
	KeyPattern         = `^[a-z][a-z0-9]*(-[a-z0-9]+)*$`
)

// schemaBuilder reads property definitions out of the meta-model and
// remembers the first property the meta-model does not record, so the
// schema fails to build rather than describing a property with nothing.
type schemaBuilder struct {
	model MetaModel
	err   error
}

// def returns the meta-model's definition of one recorded property.
func (b *schemaBuilder) def(term, property string) string {
	block, ok := b.model.Block(term)
	if !ok {
		b.fail(fmt.Errorf("schema: building block %q is not recorded", term))
		return ""
	}
	for _, p := range block.Records.Properties {
		if p.Name == property {
			return p.Definition
		}
	}
	b.fail(fmt.Errorf("schema: building block %q records no property %q", term, property))
	return ""
}

// meaning returns a building block's definition text, one paragraph.
func (b *schemaBuilder) meaning(term string) string {
	block, ok := b.model.Block(term)
	if !ok {
		b.fail(fmt.Errorf("schema: building block %q is not recorded", term))
		return ""
	}
	return strings.Join(strings.Fields(block.Definition.Text), " ")
}

func (b *schemaBuilder) fail(err error) {
	if b.err == nil {
		b.err = err
	}
}

func writeSchema(buf *bytes.Buffer) error {
	b := &schemaBuilder{model: DDD()}

	kindEnum := make([]any, 0, len(RelationKinds()))
	for _, k := range RelationKinds() {
		kindEnum = append(kindEnum, string(k))
	}

	// Compact leaf helpers matching the litmus single-line object style:
	// { "type": "string", "minLength": 1, "description": "..." }
	str := func(desc string) compactObject {
		return co("type", "string", "minLength", 1, "description", desc)
	}
	strItems := co("type", "string", "minLength", 1)
	names := func(desc string) orderedObject {
		return o("type", "array", "uniqueItems", true, "items", strItems, "description", desc)
	}
	// A reference repeats its target's description beside the $ref, so
	// a reader of the property sees the block's meaning without
	// following the pointer; both come from the same meta-model entry.
	ref := func(name, term string) compactObject {
		return co("$ref", "#/$defs/"+name, "description", b.meaning(term))
	}
	// keyed is a map whose keys are the instances' names or keys.
	keyed := func(desc, pattern string, value any) orderedObject {
		return o(
			"type", "object",
			"description", desc,
			"propertyNames", co("pattern", pattern),
			"additionalProperties", value,
		)
	}
	entry := func(term string, required []string, props orderedObject) orderedObject {
		return o(
			"type", "object",
			"description", b.meaning(term),
			"additionalProperties", false,
			"required", a(required...),
			"properties", props,
		)
	}

	root := o(
		"$id", SchemaID,
		"$schema", SchemaDraft,
		"title", SchemaTitle,
		"description", SchemaDescription,
		"type", "object",
		"additionalProperties", false,
		"required", a("version", "project", "contexts"),
		"properties", o(
			"version", o(
				"type", "integer",
				"const", UbiquitousLanguageVersion,
				"description", SchemaVersionDescription,
			),
			"project", str(b.def("domain", "project")),
			"description", str(b.def("domain", "description")),
			"contexts", keyed(b.def("domain", "contexts"), ContextNamePattern, ref("context", "bounded_context")),
			"relations", o(
				"type", "array",
				"uniqueItems", true,
				"description", b.def("domain", "relations"),
				"items", ref("relation", "context_relation"),
			),
		),
		"$defs", o(
			"context", entry("bounded_context", []string{"definition"}, o(
				"definition", str(b.def("bounded_context", "definition")),
				"aggregates", keyed(b.def("bounded_context", "aggregates"), TermNamePattern, ref("aggregate", "aggregate")),
				"value_objects", keyed(b.def("bounded_context", "value_objects"), TermNamePattern, ref("value_object", "value_object")),
				"events", keyed(b.def("bounded_context", "events"), TermNamePattern, ref("event", "domain_event")),
				"services", keyed(b.def("bounded_context", "services"), TermNamePattern, ref("service", "domain_service")),
				"specifications", keyed(b.def("bounded_context", "specifications"), TermNamePattern, ref("specification", "specification")),
				"questions", keyed(b.def("bounded_context", "questions"), KeyPattern, str(b.def("question", "text"))),
			)),
			"aggregate", entry("aggregate", []string{"definition", "identity"}, o(
				"definition", str(b.def("aggregate", "definition")),
				"identity", str(b.def("aggregate", "identity")),
				"aliases", names(b.def("aggregate", "aliases")),
				"entities", keyed(b.def("aggregate", "entities"), TermNamePattern, ref("entity", "entity")),
				"invariants", keyed(b.def("aggregate", "invariants"), KeyPattern, str(b.def("invariant", "statement"))),
				"assertions", keyed(b.def("aggregate", "assertions"), KeyPattern, ref("assertion", "assertion")),
				"repository", str(b.def("aggregate", "repository")),
				"factory", str(b.def("aggregate", "factory")),
			)),
			"entity", entry("entity", []string{"definition"}, o(
				"definition", str(b.def("entity", "definition")),
				"identity", str(b.def("entity", "identity")),
				"aliases", names(b.def("entity", "aliases")),
			)),
			"value_object", entry("value_object", []string{"definition"}, o(
				"definition", str(b.def("value_object", "definition")),
				"aliases", names(b.def("value_object", "aliases")),
				"invariants", keyed(b.def("value_object", "invariants"), KeyPattern, str(b.def("invariant", "statement"))),
			)),
			"assertion", entry("assertion", []string{"on", "statement"}, o(
				"on", str(b.def("assertion", "on")),
				"statement", str(b.def("assertion", "statement")),
			)),
			"event", entry("domain_event", []string{"definition"}, o(
				"definition", str(b.def("domain_event", "definition")),
				"raised_by", str(b.def("domain_event", "raised_by")),
			)),
			"service", entry("domain_service", []string{"definition"}, o(
				"definition", str(b.def("domain_service", "definition")),
			)),
			"specification", entry("specification", []string{"definition"}, o(
				"definition", str(b.def("specification", "definition")),
			)),
			"relation", entry("context_relation", []string{"from", "to", "kind"}, o(
				"from", str(b.def("context_relation", "from")),
				"to", str(b.def("context_relation", "to")),
				"kind", o(
					"type", "string",
					"enum", kindEnum,
					"description", SchemaKindDescription(),
				),
				"description", str(b.def("context_relation", "description")),
			)),
		),
	)
	if b.err != nil {
		return b.err
	}
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
