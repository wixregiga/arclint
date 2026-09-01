//nolint:goconst // JSON Schema keywords are clearer at their points of use.
package yaml

import (
	"encoding/json"
	"fmt"
)

// Schema returns the published draft 2020-12 JSON Schema for the exact
// pattern.yaml representation accepted by Load.
func Schema() ([]byte, error) {
	out, err := json.MarshalIndent(schemaDocument(), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal pattern schema: %w", err)
	}
	return append(out, '\n'), nil
}

const (
	moduleNamePattern  = `^[a-z0-9_-]+$`
	ruleIDPattern      = `^([a-z0-9_]([a-z0-9._/-]*[a-z0-9_-])?:)?[a-z0-9_]([a-z0-9._/-]*[a-z0-9_-])?$`
	exactSemverPattern = `^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-((0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(\.(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*))?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`
	// Runtime performs the remaining fs.ValidPath checks.
	safePathPattern = `^[^/\\](?:[^\\]*[^/\\])?$`
)

func schemaDocument() map[string]any {
	return map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  "https://raw.githubusercontent.com/wixregiga/arclint/main/docs/pattern.schema.json",
		"title":                "ArcLint Pattern manifest",
		"description":          "The complete source-neutral pattern.yaml distribution contract. Unknown keys are rejected everywhere.",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"pattern", "modules", "rules"},
		"properties": map[string]any{
			"pattern": map[string]any{
				"type": "object", "additionalProperties": false,
				"required": []string{"namespace", "name", "version"},
				"properties": map[string]any{
					"namespace": map[string]any{"type": "string", "minLength": 1, "pattern": `^[^/@\s]+$`},
					"name":      map[string]any{"type": "string", "minLength": 1, "pattern": `^[^/@\s]+$`},
					"version":   map[string]any{"type": "string", "pattern": exactSemverPattern},
				},
			},
			"coverage": map[string]any{
				"type": "array", "uniqueItems": true,
				"items": map[string]any{"enum": []string{"go", "typescript", "python"}},
			},
			"modules": map[string]any{
				"type": "array", "minItems": 1,
				"items": map[string]any{
					"type": "object", "additionalProperties": false,
					"required": []string{"name", "paths"},
					"properties": map[string]any{
						"name":        map[string]any{"type": "string", "pattern": moduleNamePattern},
						"description": map[string]any{"type": "string"},
						"paths":       stringList(),
					},
				},
			},
			"rules": map[string]any{
				"type": "array", "minItems": 1,
				"items": map[string]any{"oneOf": ruleSchemas()},
			},
			"extensions": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object", "additionalProperties": false,
					"required": []string{"name", "entry"},
					"properties": map[string]any{
						"name": map[string]any{"type": "string", "minLength": 1},
						"entry": map[string]any{
							"type": "string", "pattern": safePathPattern,
							"not": map[string]any{"pattern": `(^|/)\.{1,2}(/|$)`},
							"anyOf": []any{
								map[string]any{"pattern": `\.ts$`, "not": map[string]any{"pattern": `\.d\.ts$`}},
								map[string]any{"pattern": `\.js$`},
							},
						},
					},
				},
			},
			"tests": map[string]any{
				"type": "object", "additionalProperties": false,
				"required": []string{"root"},
				"properties": map[string]any{
					"root": map[string]any{
						"type": "string", "pattern": safePathPattern,
						"not": map[string]any{"pattern": `(^|/)\.{1,2}(/|$)`},
					},
				},
			},
		},
	}
}

func ruleSchemas() []any {
	return []any{
		ruleSchema("consumes", []string{"module"}, map[string]any{
			"module": moduleName(),
			"allow":  moduleList(0),
			"forbid": map[string]any{
				"type": "array", "minItems": 1, "uniqueItems": true,
				"items": map[string]any{"enum": []string{"external", "stdlib"}},
			},
		}, []any{
			map[string]any{"required": []string{"allow"}},
			map[string]any{"required": []string{"forbid"}},
		}),
		ruleSchema("structure", []string{"module"}, map[string]any{
			"module":  moduleName(),
			"each":    map[string]any{"enum": []string{"domain.aggregates", "domain.entities", "domain.value_objects", "domain.events", "domain.contexts"}},
			"require": stringList(),
			"forbid":  stringList(),
		}, []any{
			map[string]any{"required": []string{"require"}},
			map[string]any{"required": []string{"forbid"}},
		}),
		ruleSchema("naming", []string{"module", "case"}, map[string]any{
			"module": moduleName(), "files": map[string]any{"type": "string", "minLength": 1},
			"case": map[string]any{"type": "string", "minLength": 1},
		}, nil),
		ruleSchema("layers", []string{"layers"}, map[string]any{"layers": moduleList(2)}, nil),
		ruleSchema("protected", []string{"module"}, map[string]any{
			"module": moduleName(), "allow": moduleList(0),
		}, nil),
		ruleSchema("independence", []string{"folders"}, map[string]any{"folders": stringList()}, nil),
		ruleSchema("acyclic", nil, map[string]any{"modules": moduleList(0)}, nil),
		ruleSchema("extension", []string{"uses"}, map[string]any{
			"module": moduleName(), "files": map[string]any{"type": "string", "minLength": 1},
			"uses": map[string]any{"type": "string", "minLength": 1},
			"with": map[string]any{"type": "object"},
		}, nil),
	}
}

func ruleSchema(kind string, required []string, specific map[string]any, anyOf []any) map[string]any {
	properties := map[string]any{
		"id":       map[string]any{"type": "string", "pattern": ruleIDPattern},
		"kind":     map[string]any{"const": kind},
		"claim":    map[string]any{"type": "string"},
		"severity": map[string]any{"enum": []string{"error", "warning", "info"}},
	}
	for name, schema := range specific {
		properties[name] = schema
	}
	required = append([]string{"id", "kind"}, required...)
	out := map[string]any{
		"type": "object", "additionalProperties": false,
		"required": required, "properties": properties,
	}
	if len(anyOf) > 0 {
		out["anyOf"] = anyOf
	}
	return out
}

func moduleName() map[string]any {
	return map[string]any{"type": "string", "pattern": moduleNamePattern}
}

func moduleList(minItems int) map[string]any {
	return map[string]any{
		"type": "array", "minItems": minItems, "uniqueItems": true,
		"items": moduleName(),
	}
}

func stringList() map[string]any {
	return map[string]any{
		"type": "array", "minItems": 1, "uniqueItems": true,
		"items": map[string]any{"type": "string", "minLength": 1},
	}
}
