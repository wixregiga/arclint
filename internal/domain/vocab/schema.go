package vocab

import (
	"encoding/json"
	"fmt"
)

// Schema returns the published ubiquitous-language.yaml Schema: a
// deterministic, indented JSON Schema (draft 2020-12) document
// describing the committed project domain model file. Runtime
// validation and this published editor schema accept the same values;
// the committed docs/ubiquitous-language.schema.json holds exactly
// these bytes.
func Schema() ([]byte, error) {
	out, err := json.MarshalIndent(schemaDocument(), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal ubiquitous-language schema: %w", err)
	}
	return append(out, '\n'), nil
}

// schemaDocument builds the complete ubiquitous-language.yaml document
// schema. Property descriptions are sourced from ConceptDoc so editor
// hover reuses the single ArcLint-owned meanings. Map key order is
// irrelevant because encoding/json emits object keys sorted.
func schemaDocument() map[string]any {
	entityDoc := ConceptEntity.Doc()
	aggregateDoc := ConceptAggregate.Doc()
	valueObjectDoc := ConceptValueObject.Doc()
	businessRuleDoc := ConceptBusinessRule.Doc()
	eventDoc := ConceptEvent.Doc()

	return map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$id":     "https://raw.githubusercontent.com/wixregiga/arclint/main/docs/ubiquitous-language.schema.json",
		"title":   "ArcLint ubiquitous-language.yaml",
		"description": "The project's recorded Ubiquitous Language: Entities " +
			"(with optional Aggregate designations), Value Objects, Business Rules, " +
			"and Domain Events. ArcLint owns the concept meanings; the project " +
			"supplies names, definitions, and aliases. Unknown keys are rejected everywhere.",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"version"},
		"properties": map[string]any{
			"version": map[string]any{
				"description": "Document version. This arclint accepts version 1 only.",
				"const":       UbiquitousLanguageVersion,
			},
			"entities": map[string]any{
				"description": entityDoc.Meaning,
				"type":        "array",
				"items":       entityDefSchema(entityDoc, aggregateDoc),
			},
			"value_objects": map[string]any{
				"description": valueObjectDoc.Meaning,
				"type":        "array",
				"items":       defSchema(valueObjectDoc),
			},
			"business_rules": map[string]any{
				"description": businessRuleDoc.Meaning,
				"type":        "array",
				"items":       defSchema(businessRuleDoc),
			},
			"events": map[string]any{
				"description": eventDoc.Meaning,
				"type":        "array",
				"items":       defSchema(eventDoc),
			},
		},
	}
}

func entityDefSchema(entity, aggregate ConceptDoc) map[string]any {
	return strictObjectSchema(
		entity.Meaning,
		map[string]any{
			"name": map[string]any{
				"description": "Canonical project name for this Entity.",
				"type":        "string",
				"minLength":   1,
			},
			"definition": map[string]any{
				"description": "Meaning of this Entity in the project's Ubiquitous Language.",
				"type":        "string",
			},
			"aliases": map[string]any{
				"description": "Other names the project uses for this Entity.",
				"type":        "array",
				"items": map[string]any{
					"type":      "string",
					"minLength": 1,
				},
			},
			"aggregate": map[string]any{
				"description": aggregate.Meaning,
				"type":        "boolean",
			},
		},
		"name",
	)
}

func defSchema(doc ConceptDoc) map[string]any {
	return strictObjectSchema(
		doc.Meaning,
		map[string]any{
			"name": map[string]any{
				"description": "Canonical project name for this " + doc.Title + ".",
				"type":        "string",
				"minLength":   1,
			},
			"definition": map[string]any{
				"description": "Meaning of this " + doc.Title + " in the project's Ubiquitous Language.",
				"type":        "string",
			},
			"aliases": map[string]any{
				"description": "Other names the project uses for this " + doc.Title + ".",
				"type":        "array",
				"items": map[string]any{
					"type":      "string",
					"minLength": 1,
				},
			},
		},
		"name",
	)
}

// strictObjectSchema builds an object schema that rejects unknown keys,
// mirroring the loader's strict decoding.
func strictObjectSchema(description string, properties map[string]any, required ...string) map[string]any {
	out := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           properties,
	}
	if description != "" {
		out["description"] = description
	}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}
