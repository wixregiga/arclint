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
	"github.com/wixregiga/arclint/internal/domain/vocab"
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
// schema, the editor-side half of the invariant.
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
	// oneModule is the smallest repository ruleset every Rule case is
	// written against.
	const oneModule = "modules:\n  core: core/**\n"
	const twoModules = "modules:\n  core: core/**\n  app: app/**\n"
	const header = "pattern:\n  namespace: acme\n  name: hexagonal\n  version: 1.0.0\n"

	cases := []struct {
		name     string
		document string
		accepted bool
	}{
		{"repository rules.yaml", string(realRuleset), true},
		{"empty file", "", false},
		{"empty document", "{}\n", true},
		{"runtime, scan, modules, empty rules", `
runtime: [go, ts]
scan:
  unknown_imports: ignore
  exclude: ["vendor/**"]
  include_testdata: true
modules:
  core: core/**
rules: {}
`, true},

		// ---- module sugar ------------------------------------------------
		{"module as a glob", "modules:\n  core: core/**\n", true},
		{"module as a glob list", "modules:\n  core: [\"core/**\", \"pkg/core/**\"]\n", true},
		{"module as an object", "modules:\n  core:\n    paths: core/**\n    description: \"The core.\"\n", true},
		{"module object with a paths list", "modules:\n  core:\n    paths: [\"core/**\"]\n", true},
		{"module object without paths", "modules:\n  core:\n    description: \"The core.\"\n", false},
		{"module object with an unknown key", "modules:\n  core:\n    paths: core/**\n    globs: [\"core/**\"]\n", false},
		{"module with an empty glob list", "modules:\n  core: []\n", false},
		{"module with a brace glob", "modules:\n  core: \"core/{a,b}/**\"\n", false},
		{"module name with uppercase", "modules:\n  Core: core/**\n", false},

		// ---- one minimal Rule per assertion --------------------------------
		{"imports with an internal allow-list", oneModule + `
rules:
  core/stdlib-only:
    description: "The core imports nothing else."
    on: core
    imports:
      internal: []
      external: forbid
      stdlib: allow
`, true},
		{"imports forbidding external only", oneModule + `
rules:
  core/no-external:
    on: core
    imports:
      external: forbid
`, true},
		{"imports on several modules", twoModules + `
rules:
  core/imports:
    on: [core, app]
    imports:
      internal: []
`, true},
		{"imports declaring no restriction", oneModule + `
rules:
  core/imports:
    on: core
    imports: {}
`, false},
		{"imports allowing external explicitly with nothing else", oneModule + `
rules:
  core/imports:
    on: core
    imports:
      external: allow
`, false},
		{"structure requiring files", oneModule + `
rules:
  core/root-present:
    on: core
    structure:
      require: ["core/root.go"]
`, true},
		{"structure forbidding files", oneModule + `
rules:
  core/no-util:
    on: core
    structure:
      forbid: ["core/**/util.go"]
`, true},
		{"structure with empty require", oneModule + `
rules:
  core/root-present:
    on: core
    structure:
      require: []
`, false},
		{"structure with each", oneModule + `
rules:
  core/aggregates:
    severity: warning
    on: core
    structure:
      each: domain.aggregates
      require: ["core/{name:flatcase}/root.go", "core/{name:flatcase}/repository.go"]
`, true},
		{"structure with each over a plain glob", oneModule + `
rules:
  core/aggregates:
    on: core
    structure:
      each: domain.aggregates
      require: ["core/root.go"]
`, true},
		{"structure with each from an unknown source", oneModule + `
rules:
  core/aggregates:
    on: core
    structure:
      each: domain.services
      require: ["core/{name:flatcase}/root.go"]
`, false},
		{"structure with each and an unknown term case", oneModule + `
rules:
  core/aggregates:
    on: core
    structure:
      each: domain.aggregates
      require: ["core/{name:bogus}/root.go"]
`, false},
		{"structure with each and a stray brace", oneModule + `
rules:
  core/aggregates:
    on: core
    structure:
      each: domain.aggregates
      require: ["core/{name:flatcase/root.go"]
`, false},
		{"structure placeholder without each", oneModule + `
rules:
  core/aggregates:
    on: core
    structure:
      require: ["core/{name:flatcase}/root.go"]
`, false},
		{"naming as a scalar", oneModule + `
rules:
  core/snake:
    on: core
    files: "core/**/*.go"
    naming: snake_case
`, true},
		{"naming as an object with a regex alternative", oneModule + `
rules:
  core/kebab-or-digits:
    severity: warning
    on: core
    naming:
      case: "kebab-case|regex:[0-9]+"
`, true},
		{"naming with an unknown case", oneModule + `
rules:
  core/naming:
    on: core
    naming: SCREAMING_CASE
`, false},
		{"naming object without case", oneModule + `
rules:
  core/naming:
    on: core
    naming: {}
`, false},
		{"naming with each", oneModule + `
rules:
  core/naming:
    on: core
    naming:
      case: snake_case
      each: domain.aggregates
`, false},
		{"content on a module", oneModule + `
rules:
  core/no-panic:
    on: core
    files: ["core/**/*.go"]
    content:
      forbid: '\bpanic\('
`, true},
		{"content over the repository", `
rules:
  repo/no-todo:
    severity: info
    content:
      forbid: "TODO"
`, true},
		{"content without forbid", oneModule + `
rules:
  core/content:
    on: core
    content: {}
`, false},
		{"content with a blank forbid", oneModule + `
rules:
  core/content:
    on: core
    content:
      forbid: "  "
`, false},
		{"invariants open", oneModule + `
rules:
  core/invariants:
    on: core
    invariants: {}
`, true},
		{"invariants closed", oneModule + `
rules:
  core/invariants:
    on: core
    invariants:
      closed: true
`, true},
		{"invariants with an unknown key", oneModule + `
rules:
  core/invariants:
    on: core
    invariants:
      strict: true
`, false},
		{"layers", twoModules + `
rules:
  deps/inward:
    layers: [app, core]
`, true},
		{"layers with one module", oneModule + `
rules:
  deps/inward:
    layers: [core]
`, false},
		{"layers with on", twoModules + `
rules:
  deps/inward:
    on: core
    layers: [app, core]
`, false},
		{"imported_by", twoModules + `
rules:
  core/app-only:
    on: core
    imported_by: [app]
`, true},
		{"imported_by nobody", oneModule + `
rules:
  core/sealed:
    on: core
    imported_by: []
`, true},
		{"imported_by with two protected modules", twoModules + `
rules:
  core/app-only:
    on: [core, app]
    imported_by: [app]
`, false},
		{"independent", `
rules:
  features/independent:
    independent: ["internal/features/*"]
`, true},
		{"independent with no globs", `
rules:
  features/independent:
    independent: []
`, false},
		{"acyclic over every module", `
rules:
  deps/acyclic:
    acyclic: {}
`, true},
		{"acyclic over a list", twoModules + `
rules:
  deps/acyclic:
    acyclic: [core, app]
`, true},
		{"acyclic over one module", oneModule + `
rules:
  deps/acyclic:
    acyclic: [core]
`, false},
		{"uses on a module with parameters", oneModule + `
rules:
  core/checked:
    on: core
    files: "core/**/*.go"
    uses: acme/check
    with:
      depth: 2
`, true},
		{"uses over the repository", `
rules:
  repo/checked:
    uses: acme/check
`, true},
		{"uses with each", oneModule + `
rules:
  core/checked:
    on: core
    uses: acme/check
    each: domain.aggregates
`, false},
		{"uses with a blank name", oneModule + `
rules:
  core/checked:
    on: core
    uses: " "
`, false},

		// ---- the Rule envelope -------------------------------------------
		{"rule with two assertions", oneModule + `
rules:
  core/two:
    on: core
    imports:
      internal: []
    naming: snake_case
`, false},
		{"rule with a retired kind key", oneModule + `
rules:
  core/naming:
    kind: naming
    on: core
    case: snake_case
`, false},
		{"imports without on", oneModule + `
rules:
  core/imports:
    imports:
      internal: []
`, false},
		{"imports with an empty on", oneModule + `
rules:
  core/imports:
    on: []
    imports:
      internal: []
`, false},
		{"files on imports", oneModule + `
rules:
  core/imports:
    on: core
    files: "core/**/*.go"
    imports:
      internal: []
`, false},
		{"with on imports", oneModule + `
rules:
  core/imports:
    on: core
    imports:
      internal: []
    with:
      depth: 1
`, false},
		{"unknown severity", oneModule + `
rules:
  core/imports:
    severity: critical
    on: core
    imports:
      internal: []
`, false},
		{"disable with a reason", oneModule + `
rules:
  core/imports:
    on: core
    imports:
      internal: []
    disable: "the core is being rewritten; re-enable after AL-42"
`, true},
		{"disable without a reason", oneModule + `
rules:
  core/imports:
    on: core
    imports:
      internal: []
    disable: ""
`, false},
		{"exclude paths with a reason", oneModule + `
rules:
  core/imports:
    on: core
    imports:
      internal: []
    exclude:
      paths: ["core/generated/**"]
      reason: "generated code is not authored"
`, true},
		{"exclude modules with a reason", twoModules + `
rules:
  deps/acyclic:
    acyclic: {}
    exclude:
      modules: [app]
      reason: "app is the composition root"
`, true},
		{"exclude without a reason", oneModule + `
rules:
  core/imports:
    on: core
    imports:
      internal: []
    exclude:
      paths: ["core/generated/**"]
`, false},
		{"exclude naming no subject", oneModule + `
rules:
  core/imports:
    on: core
    imports:
      internal: []
    exclude:
      reason: "why"
`, false},
		{"suppress paths with a reason", oneModule + `
rules:
  core/imports:
    on: core
    imports:
      internal: []
    suppress:
      paths: ["core/legacy/**"]
      reason: "adopted debt tracked in the baseline"
`, true},
		{"suppress without paths", oneModule + `
rules:
  core/imports:
    on: core
    imports:
      internal: []
    suppress:
      reason: "why"
`, false},
		{"rule id starting with a slash", oneModule + `
rules:
  /core:
    on: core
    imports:
      internal: []
`, false},
		{"rule id ending with a slash", oneModule + `
rules:
  core/:
    on: core
    imports:
      internal: []
`, false},
		{"rules as a list", oneModule + `
rules:
  - id: core/imports
`, false},
		{"unknown top-level key", "rulesets: []\n", false},
		{"retired contracts key", oneModule + `
contracts:
  core:
    consumes:
      id: t:core/stdlib-only
      internal: []
`, false},
		{"retired repository key", `
repository:
  invariants: []
`, false},

		// ---- runtime, scan, extends --------------------------------------
		{"runtime with an unknown target", "runtime: [rust]\n", false},
		{"runtime listing a target twice", "runtime: [go, go]\n", false},
		{"runtime naming no language", "runtime: []\n", false},
		{"scan with an unknown policy", "scan:\n  unknown_imports: explode\n", false},
		{"scan with an unknown key", "scan:\n  follow_symlinks: true\n", false},
		{"extends entry without pattern", "extends:\n  - bind:\n      core: core/**\n", false},
		{"extends with an inexact version", "extends:\n  - pattern: acme/hexagonal@latest\n", false},
		{"extends with a bind list", "extends:\n  - pattern: acme/hexagonal@1.0.0\n    bind: [core]\n", false},
		{"extends with an unknown key", "extends:\n  - pattern: acme/hexagonal@1.0.0\n    version: 1.0.0\n", false},
		{"override with a description", oneModule + `
rules:
  acme:core/stdlib-only:
    description: "rewritten"
    severity: warning
`, false},
		{"override with on", oneModule + `
rules:
  acme:core/stdlib-only:
    on: core
    severity: warning
`, false},
		{"override changing nothing", `
rules:
  acme:core/stdlib-only: {}
`, false},

		// ---- pattern distribution files ----------------------------------
		{"pattern file", header + `
modules:
  core: "The domain core."
  ports:
    description: "Inbound and outbound ports."
    paths: ["internal/ports/**"]
rules:
  core/stdlib-only:
    description: "The core imports nothing else."
    on: core
    imports:
      internal: []
      external: forbid
`, true},
		{"pattern file with coverage and documentation", `
pattern:
  namespace: acme
  name: hexagonal
  version: 1.0.0-beta.1
  coverage: [go, ts]
  documentation: https://example.test/hexagonal
modules:
  core: "The domain core."
rules:
  core/stdlib-only:
    on: core
    imports:
      internal: []
`, true},
		{"pattern file with runtime", header + `
runtime: [go]
modules:
  core: "The domain core."
rules:
  core/stdlib-only:
    on: core
    imports:
      internal: []
`, false},
		{"pattern file with scan", header + `
scan:
  unknown_imports: warn
modules:
  core: "The domain core."
rules:
  core/stdlib-only:
    on: core
    imports:
      internal: []
`, false},
		{"pattern file with extends", header + `
extends: []
modules:
  core: "The domain core."
rules:
  core/stdlib-only:
    on: core
    imports:
      internal: []
`, false},
		{"pattern file with a glob-list module", header + `
modules:
  core: ["core/**"]
rules:
  core/stdlib-only:
    on: core
    imports:
      internal: []
`, false},
		{"pattern file module without a description", header + `
modules:
  core:
    paths: ["core/**"]
rules:
  core/stdlib-only:
    on: core
    imports:
      internal: []
`, false},
		{"pattern file module with a blank description", header + `
modules:
  core: " "
rules:
  core/stdlib-only:
    on: core
    imports:
      internal: []
`, false},
		{"pattern file with an override", header + `
modules:
  core: "The domain core."
rules:
  core/stdlib-only:
    severity: warning
`, false},
		{"pattern header missing version", "pattern:\n  namespace: acme\n  name: hexagonal\n", false},
		{"pattern header with an unknown key", header + "  author: me\n", false},
		{"pattern header with an inexact version", "pattern:\n  namespace: acme\n  name: hexagonal\n  version: latest\n", false},
		{"pattern header with a slash in the name", "pattern:\n  namespace: acme\n  name: hex/agonal\n  version: 1.0.0\n", false},
		{"pattern header with unknown coverage", header[:len(header)-1] + "\n  coverage: [rust]\n", false},
		{"pattern header with coverage spelled as a language name", header[:len(header)-1] + "\n  coverage: [typescript]\n", false},
		{"pattern header with repeated coverage", header[:len(header)-1] + "\n  coverage: [go, go]\n", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, loaderErr := yamlrule.Load([]byte(tc.document), tc.name, vocab.UbiquitousLanguage{}, nil)
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
