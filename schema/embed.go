// Package schema carries the published arclint JSON Schemas, embedded so
// the binary validates configuration with zero filesystem lookups. The
// .json files in this directory are the single source of truth — they are
// both published for editor tooling and compiled into the binary.
package schema

import _ "embed"

// Rules is schema/arclint-rules.schema.json (JSON Schema draft 2020-12),
// validating .arclint/rules.yaml.
//
//go:embed arclint-rules.schema.json
var Rules []byte
