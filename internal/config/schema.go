package config

import (
	"bytes"
	"encoding/json"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/jofyi/arclint/schema"
)

// compileRulesSchema compiles the embedded rules schema exactly once, on
// first use — never at package init, per the cold-start budget.
//
// sync.OnceValues caches whatever the func returns on its first call,
// including an error: if compilation ever failed, every later call in the
// process lifetime would keep getting that same cached error, with no
// retry. That is acceptable here specifically because the schema is
// embedded at compile time (schema.Rules, via go:embed) — it cannot fail
// transiently or change between calls within a single process, so "fails
// once, fails forever, identically for every caller" is the right behavior,
// not a hidden footgun. Do not add retry/reset logic around this.
var compileRulesSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	const url = "https://arclint.dev/schema/arclint-rules.schema.json"
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema.Rules))
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(url, doc); err != nil {
		return nil, err
	}
	return c.Compile(url)
})

// validateAgainstSchema checks the parsed YAML tree against the embedded
// JSON Schema. The tree is round-tripped through encoding/json first so the
// validator sees exactly the value kinds it expects (json.Number et al.)
// regardless of what integer types the YAML parser produced.
func validateAgainstSchema(tree any) error {
	sch, err := compileRulesSchema()
	if err != nil {
		return err
	}
	raw, err := json.Marshal(tree)
	if err != nil {
		return err
	}
	val, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	return sch.Validate(val)
}
