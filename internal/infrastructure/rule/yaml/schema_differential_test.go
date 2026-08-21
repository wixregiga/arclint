package yamlrule_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	sj "github.com/santhosh-tekuri/jsonschema/v6"
	yamlv3 "gopkg.in/yaml.v3"

	"github.com/wixregiga/arclint/internal/domain/rule"
	yamlrule "github.com/wixregiga/arclint/internal/infrastructure/rule/yaml"
)

// repoRoot locates the repository root from this source file, keeping
// the tests independent of the working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller: no source location")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
}

// TestPublishedSchemaMatchesDomain is the drift half of the Rule Schema
// invariant: the committed docs/rules.schema.json is byte-for-byte what
// rule.Schema() produces. Regenerate by writing rule.Schema() output
// over the file.
func TestPublishedSchemaMatchesDomain(t *testing.T) {
	want, err := rule.Schema()
	if err != nil {
		t.Fatalf("rule.Schema: %v", err)
	}
	published := filepath.Join(repoRoot(t), "docs", "rules.schema.json")
	got, err := os.ReadFile(published)
	if err != nil {
		t.Fatalf("read published schema: %v", err)
	}
	if !bytes.Equal(want, got) {
		t.Fatalf("docs/rules.schema.json drifted from rule.Schema(); regenerate it from rule.Schema() output")
	}
}

// compileRuleSchema compiles rule.Schema() with the same validator the
// engine uses for extension parameter schemas.
func compileRuleSchema(t *testing.T) *sj.Schema {
	t.Helper()
	data, err := rule.Schema()
	if err != nil {
		t.Fatalf("rule.Schema: %v", err)
	}
	doc, err := sj.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	const url = "https://raw.githubusercontent.com/wixregiga/arclint/main/docs/rules.schema.json"
	compiler := sj.NewCompiler()
	if err := compiler.AddResource(url, doc); err != nil {
		t.Fatalf("add schema resource: %v", err)
	}
	schema, err := compiler.Compile(url)
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	return schema
}

// validateAgainstSchema parses the YAML document generically, converts
// it to the JSON data model, and validates it against the compiled
// schema — the editor-side half of the invariant.
func validateAgainstSchema(t *testing.T, schema *sj.Schema, source []byte) error {
	t.Helper()
	var value any
	if err := yamlv3.Unmarshal(source, &value); err != nil {
		t.Fatalf("generic YAML parse: %v", err)
	}
	data, err := json.Marshal(jsonify(value))
	if err != nil {
		t.Fatalf("marshal generic document: %v", err)
	}
	instance, err := sj.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unmarshal generic document: %v", err)
	}
	return schema.Validate(instance)
}

// jsonify converts YAML-decoded values into the JSON data model,
// stringifying any non-string map keys.
func jsonify(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, entry := range typed {
			out[key] = jsonify(entry)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, entry := range typed {
			out[fmt.Sprintf("%v", key)] = jsonify(entry)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, entry := range typed {
			out[i] = jsonify(entry)
		}
		return out
	default:
		return value
	}
}

// TestSchemaAgreesWithLoader is the agreement half of the Rule Schema
// invariant: for every case the strict loader and JSON-Schema
// validation of the same document reach the same verdict, and that
// verdict is the expected one. A divergence in either direction fails
// naming the case.
func TestSchemaAgreesWithLoader(t *testing.T) {
	schema := compileRuleSchema(t)
	realRuleset, err := os.ReadFile(filepath.Join(repoRoot(t), "rules.yaml"))
	if err != nil {
		t.Fatalf("read repository rules.yaml: %v", err)
	}

	cases := []struct {
		name     string
		document string
		accepted bool
	}{
		{"repository rules.yaml", string(realRuleset), true},
		{"empty file", "", false},
		{"empty document", "{}\n", true},
		{
			"minimal consumes rule", `
modules:
  core:
    paths: ["core/**"]
contracts:
  core:
    consumes:
      id: t:core/stdlib-only
      internal: []
      external: forbid
      stdlib: allow
`, true,
		},
		{
			"minimal structure rule", `
modules:
  core:
    paths: ["core/**"]
contracts:
  core:
    invariants:
      - id: t:core/root-present
        kind: structure
        require: ["core/root.go"]
`, true,
		},
		{
			"minimal naming rule", `
modules:
  core:
    paths: ["core/**"]
contracts:
  core:
    invariants:
      - id: t:core/snake-case
        kind: naming
        files: "core/**/*.go"
        case: snake_case
`, true,
		},
		{
			"naming rule with regex alternative", `
modules:
  core:
    paths: ["core/**"]
contracts:
  core:
    invariants:
      - id: t:core/kebab-or-digits
        kind: naming
        case: "kebab-case|regex:[0-9]+"
        severity: warning
`, true,
		},
		{
			"minimal extension rule", `
modules:
  core:
    paths: ["core/**"]
contracts:
  core:
    invariants:
      - id: t:core/no-panic
        kind: extension
        files: "core/**/*.go"
        uses: forbid-content
        with:
          pattern: 'panic\('
`, true,
		},
		{
			"minimal layers rule", `
modules:
  app:
    paths: ["app/**"]
  core:
    paths: ["core/**"]
dependencies:
  - id: t:deps/layered
    kind: layers
    layers: [app, core]
`, true,
		},
		{
			"minimal protected rule", `
dependencies:
  - id: t:deps/protected
    kind: protected
    module: core
    allow: [app]
    severity: info
`, true,
		},
		{
			"minimal independence rule", `
dependencies:
  - id: t:deps/independent
    kind: independence
    folders: ["internal/*"]
`, true,
		},
		{
			"minimal acyclic rule", `
dependencies:
  - id: t:deps/acyclic
    kind: acyclic
`, true,
		},
		{
			"full scan settings", `
runtime: [go, ts, py]
scan:
  unknown_imports: ignore
  exclude: ["**/generated/**", "vendor/**"]
  include_testdata: true
`, true,
		},
		{
			"pattern distribution header", `
pattern:
  namespace: acme
  name: hexagonal
  version: 1.2.0
  coverage: [go, typescript]
modules:
  core:
    paths: ["core/**"]
contracts:
  core:
    consumes:
      id: acme:core/clean
      external: forbid
`, true,
		},
		{"unknown top-level key", "rules: []\n", false},
		{
			"unknown rule kind", `
modules:
  core:
    paths: ["core/**"]
contracts:
  core:
    invariants:
      - id: t:core/depth
        kind: expr
        files: "core/**"
`, false,
		},
		{
			"unknown dependency kind", `
dependencies:
  - id: t:deps/circular
    kind: circular
`, false,
		},
		{
			"structure rule carrying a naming field", `
modules:
  core:
    paths: ["core/**"]
contracts:
  core:
    invariants:
      - id: t:core/mixed
        kind: structure
        require: ["core/root.go"]
        case: snake_case
`, false,
		},
		{
			"bad severity", `
modules:
  core:
    paths: ["core/**"]
contracts:
  core:
    consumes:
      id: t:core/imports
      internal: []
      severity: fatal
`, false,
		},
		{"bad runtime language", "runtime: [rust]\n", false},
		{"bad unknown-import policy", "scan:\n  unknown_imports: panic\n", false},
		{
			"missing rule id", `
modules:
  core:
    paths: ["core/**"]
contracts:
  core:
    consumes:
      internal: []
`, false,
		},
		{
			"consumes without restriction", `
modules:
  core:
    paths: ["core/**"]
contracts:
  core:
    consumes:
      id: t:core/unrestricted
      external: allow
`, false,
		},
		{
			"duplicate internal modules", `
modules:
  core:
    paths: ["core/**"]
  app:
    paths: ["app/**"]
contracts:
  core:
    consumes:
      id: t:core/dupes
      internal: [app, app]
`, false,
		},
		{
			"single layer", `
dependencies:
  - id: t:deps/thin
    kind: layers
    layers: [core]
`, false,
		},
		{
			"duplicate layer modules", `
dependencies:
  - id: t:deps/twice
    kind: layers
    layers: [core, core]
`, false,
		},
		{
			"protected rule missing module", `
dependencies:
  - id: t:deps/unprotected
    kind: protected
    allow: [app]
`, false,
		},
		{
			"independence without folders", `
dependencies:
  - id: t:deps/nofolders
    kind: independence
`, false,
		},
		{
			"independence with modules", `
dependencies:
  - id: t:deps/with-modules
    kind: independence
    folders: ["internal/*"]
    modules: [app]
`, false,
		},
		{
			"protected rule carrying layers", `
dependencies:
  - id: t:deps/confused
    kind: protected
    module: core
    layers: [app, core]
`, false,
		},
		{"invalid module name", "modules:\n  Core:\n    paths: [\"core/**\"]\n", false},
		{"glob with brace alternation", "scan:\n  exclude: [\"{a,b}/**\"]\n", false},
		{
			"structure rule with no globs", `
modules:
  core:
    paths: ["core/**"]
contracts:
  core:
    invariants:
      - id: t:core/empty
        kind: structure
`, false,
		},
		{
			"naming rule missing case", `
modules:
  core:
    paths: ["core/**"]
contracts:
  core:
    invariants:
      - id: t:core/caseless
        kind: naming
        files: "core/**/*.go"
`, false,
		},
		{
			"bad case vocabulary", `
modules:
  core:
    paths: ["core/**"]
contracts:
  core:
    invariants:
      - id: t:core/shouting
        kind: naming
        case: SCREAMING_SNAKE
`, false,
		},
		{
			"extension rule missing uses", `
modules:
  core:
    paths: ["core/**"]
contracts:
  core:
    invariants:
      - id: t:core/unbound
        kind: extension
        files: "core/**/*.go"
`, false,
		},
		{
			"minimal repository extension", `
repository:
  invariants:
    - id: t:repos/application-only
      kind: extension
      uses: vertical/repository-location
      with:
        module: application
`, true,
		},
		{
			"repository extension with files", `
repository:
  invariants:
    - id: t:repos/go-only
      kind: extension
      files: "**/*.go"
      uses: vertical/repository-location
      with:
        module: application
`, true,
		},
		{
			"repository extension missing uses", `
repository:
  invariants:
    - id: t:repos/unbound
      kind: extension
`, false,
		},
		{
			"repository invariant wrong kind", `
repository:
  invariants:
    - id: t:repos/structure
      kind: structure
      require: ["internal/**"]
`, false,
		},
		{
			"repository unknown field", `
repository:
  extra: true
  invariants:
    - id: t:repos/extra
      kind: extension
      uses: vertical/repository-location
`, false,
		},
		{"pattern header missing version", "pattern:\n  namespace: acme\n  name: hexagonal\n", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, loaderErr := yamlrule.Load([]byte(tc.document), tc.name)
			schemaErr := validateAgainstSchema(t, schema, []byte(tc.document))
			loaderAccepts := loaderErr == nil
			schemaAccepts := schemaErr == nil
			if loaderAccepts != schemaAccepts {
				t.Fatalf("divergence: loader accepts=%v (err: %v), schema accepts=%v (err: %v)",
					loaderAccepts, loaderErr, schemaAccepts, schemaErr)
			}
			if loaderAccepts != tc.accepted {
				t.Fatalf("both sides agree on accepts=%v, but the case expects %v (loader err: %v)",
					loaderAccepts, tc.accepted, loaderErr)
			}
		})
	}
}
