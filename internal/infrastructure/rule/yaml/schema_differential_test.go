package yamlrule_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	sj "github.com/santhosh-tekuri/jsonschema/v6"
	yamlv3 "gopkg.in/yaml.v3"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
	embeddedpattern "github.com/wixregiga/arclint/internal/infrastructure/pattern/embedded"
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
// invariant: the release copy under docs/schemas and the dogfood copy
// under .arclint/schemas are both byte-for-byte what rule.Schema()
// produces. Regenerate with make schemas.
func TestPublishedSchemaMatchesDomain(t *testing.T) {
	want, err := rule.Schema()
	if err != nil {
		t.Fatalf("rule.Schema: %v", err)
	}
	for _, dir := range []string{"docs/schemas", ".arclint/schemas"} {
		published := filepath.Join(repoRoot(t), filepath.FromSlash(dir), rule.SchemaFileName)
		got, err := os.ReadFile(published)
		if err != nil {
			t.Fatalf("read published schema: %v", err)
		}
		if !bytes.Equal(want, got) {
			t.Fatalf("%s/%s drifted from rule.Schema(); run make schemas", dir, rule.SchemaFileName)
		}
	}
}

// TestPublishedSchemaIdentifiesItself pins the $id to the release copy
// and every $ref to a described $defs entry whose description the
// reference repeats, the contract the Spectral ruleset enforces on the
// committed file and editors rely on for hover text.
func TestPublishedSchemaIdentifiesItself(t *testing.T) {
	data, err := rule.Schema()
	if err != nil {
		t.Fatalf("rule.Schema: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if want := "https://raw.githubusercontent.com/wixregiga/arclint/main/docs/schemas/" + rule.SchemaFileName; doc["$id"] != want {
		t.Fatalf("$id = %v, want %q", doc["$id"], want)
	}
	defs, ok := doc["$defs"].(map[string]any)
	if !ok {
		t.Fatal("schema has no $defs object")
	}
	for name, def := range defs {
		entry, ok := def.(map[string]any)
		if !ok {
			t.Fatalf("$defs/%s is not an object", name)
		}
		if text, _ := entry["description"].(string); text == "" {
			t.Errorf("$defs/%s has no description", name)
		}
	}
	var walk func(path string, node any)
	walk = func(path string, node any) {
		switch typed := node.(type) {
		case map[string]any:
			if ref, ok := typed["$ref"].(string); ok {
				const prefix = "#/$defs/"
				if !strings.HasPrefix(ref, prefix) {
					t.Errorf("%s: $ref %q does not point into $defs", path, ref)
					return
				}
				target, ok := defs[strings.TrimPrefix(ref, prefix)].(map[string]any)
				if !ok {
					t.Errorf("%s: $ref %q has no $defs target", path, ref)
					return
				}
				if typed["description"] != target["description"] {
					t.Errorf("%s: $ref %q description %v differs from its target's %v", path, ref, typed["description"], target["description"])
				}
			}
			for key, child := range typed {
				walk(path+"/"+key, child)
			}
		case []any:
			for i, child := range typed {
				walk(fmt.Sprintf("%s/%d", path, i), child)
			}
		}
	}
	walk("#", doc)
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
	compiler := sj.NewCompiler()
	if err := compiler.AddResource(rule.SchemaID, doc); err != nil {
		t.Fatalf("add schema resource: %v", err)
	}
	schema, err := compiler.Compile(rule.SchemaID)
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
	realRuleset, err := os.ReadFile(filepath.Join(repoRoot(t), rule.RulesetFileName))
	if err != nil {
		t.Fatalf("read repository %s: %v", rule.RulesetFileName, err)
	}
	// oneZone is the smallest repository ruleset every Rule case is
	// written against.
	const oneZone = "zones:\n  core: core/**\n"
	const twoZones = "zones:\n  core: core/**\n  app: app/**\n"
	const header = "pattern:\n  namespace: acme\n  name: hexagonal\n  version: 1.0.0\n"

	cases := []struct {
		name     string
		document string
		accepted bool
	}{
		{"repository " + rule.RulesetFileName, string(realRuleset), true},
		{"empty file", "", false},
		{"empty document", "{}\n", true},
		{"runtime, scan, zones, empty rules", `
runtime: [go, ts]
scan:
  unknown_imports: ignore
  exclude: ["vendor/**"]
  include_testdata: true
zones:
  core: core/**
rules: {}
`, true},

		// ---- zone sugar ------------------------------------------------
		{"zone as a glob", "zones:\n  core: core/**\n", true},
		{"zone as a glob list", "zones:\n  core: [\"core/**\", \"pkg/core/**\"]\n", true},
		{"zone as an object", "zones:\n  core:\n    paths: core/**\n    description: \"The core.\"\n", true},
		{"zone object with a paths list", "zones:\n  core:\n    paths: [\"core/**\"]\n", true},
		{"zone object without paths", "zones:\n  core:\n    description: \"The core.\"\n", false},
		{"zone object with an unknown key", "zones:\n  core:\n    paths: core/**\n    globs: [\"core/**\"]\n", false},
		{"zone with an empty glob list", "zones:\n  core: []\n", false},
		{"zone with a brace glob", "zones:\n  core: \"core/{a,b}/**\"\n", false},
		{"zone listing a glob twice", "zones:\n  core: [\"core/**\", \"core/**\"]\n", false},
		{"zone object listing a glob twice", "zones:\n  core:\n    paths: [\"core/**\", \"core/**\"]\n", false},
		{"zone name with uppercase", "zones:\n  Core: core/**\n", false},

		// ---- one minimal Rule per constraint --------------------------------
		{"imports with an internal allow-list", oneZone + `
rules:
  core/stdlib-only:
    description: "The core imports nothing else."
    on: core
    imports:
      internal: []
      external: forbid
      stdlib: allow
`, true},
		{"imports forbidding external only", oneZone + `
rules:
  core/no-external:
    on: core
    imports:
      external: forbid
`, true},
		{"imports on several zones", twoZones + `
rules:
  core/imports:
    on: [core, app]
    imports:
      internal: []
`, true},
		{"imports declaring no restriction", oneZone + `
rules:
  core/imports:
    on: core
    imports: {}
`, false},
		{"imports allowing external explicitly with nothing else", oneZone + `
rules:
  core/imports:
    on: core
    imports:
      external: allow
`, false},
		{"structure requiring files", oneZone + `
rules:
  core/root-present:
    on: core
    structure:
      require: ["core/root.go"]
`, true},
		{"structure forbidding files", oneZone + `
rules:
  core/no-util:
    on: core
    structure:
      forbid: ["core/**/util.go"]
`, true},
		{"structure requiring a glob twice", oneZone + `
rules:
  core/shape:
    on: core
    structure:
      require: ["root.go", "root.go"]
`, false},
		{"structure with each listing a glob twice", oneZone + `
rules:
  core/aggregates:
    on: core
    structure:
      each: domain.aggregates
      require: ["core/{name:flatcase}/root.go", "core/{name:flatcase}/root.go"]
`, false},
		{"structure with empty require", oneZone + `
rules:
  core/root-present:
    on: core
    structure:
      require: []
`, false},
		{"structure with each", oneZone + `
rules:
  core/aggregates:
    severity: warning
    on: core
    structure:
      each: domain.aggregates
      require: ["core/{name:flatcase}/root.go", "core/{name:flatcase}/repository.go"]
`, true},
		{"structure with each over a plain glob", oneZone + `
rules:
  core/aggregates:
    on: core
    structure:
      each: domain.aggregates
      require: ["core/root.go"]
`, true},
		{"structure with each from an unknown source", oneZone + `
rules:
  core/aggregates:
    on: core
    structure:
      each: domain.services
      require: ["core/{name:flatcase}/root.go"]
`, false},
		{"structure with each and an unknown term case", oneZone + `
rules:
  core/aggregates:
    on: core
    structure:
      each: domain.aggregates
      require: ["core/{name:bogus}/root.go"]
`, false},
		{"structure with each and a stray brace", oneZone + `
rules:
  core/aggregates:
    on: core
    structure:
      each: domain.aggregates
      require: ["core/{name:flatcase/root.go"]
`, false},
		{"structure placeholder without each", oneZone + `
rules:
  core/aggregates:
    on: core
    structure:
      require: ["core/{name:flatcase}/root.go"]
`, false},
		{"naming as a scalar", oneZone + `
rules:
  core/snake:
    on: core
    files: "core/**/*.go"
    naming: snake_case
`, true},
		{"naming as an object with a regex alternative", oneZone + `
rules:
  core/kebab-or-digits:
    severity: warning
    on: core
    naming:
      case: "kebab-case|regex:[0-9]+"
`, true},
		{"naming with an unknown case", oneZone + `
rules:
  core/naming:
    on: core
    naming: SCREAMING_CASE
`, false},
		{"naming object without case", oneZone + `
rules:
  core/naming:
    on: core
    naming: {}
`, false},
		{"naming with each", oneZone + `
rules:
  core/naming:
    on: core
    naming:
      case: snake_case
      each: domain.aggregates
`, false},
		{"content on a zone", oneZone + `
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
		{"content without forbid", oneZone + `
rules:
  core/content:
    on: core
    content: {}
`, false},
		{"content with a blank forbid", oneZone + `
rules:
  core/content:
    on: core
    content:
      forbid: "  "
`, false},
		{"retired invariants constraint", oneZone + `
rules:
  core/invariants:
    on: core
    invariants: {}
`, false},
		{"layers", twoZones + `
rules:
  deps/inward:
    layers: [app, core]
`, true},
		{"layers with one zone", oneZone + `
rules:
  deps/inward:
    layers: [core]
`, false},
		{"layers with on", twoZones + `
rules:
  deps/inward:
    on: core
    layers: [app, core]
`, false},
		{"imported_by", twoZones + `
rules:
  core/app-only:
    on: core
    imported_by: [app]
`, true},
		{"imported_by nobody", oneZone + `
rules:
  core/sealed:
    on: core
    imported_by: []
`, true},
		{"imported_by with two protected zones", twoZones + `
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
		{"acyclic over every zone", `
rules:
  deps/acyclic:
    acyclic: {}
`, true},
		{"acyclic over a list", twoZones + `
rules:
  deps/acyclic:
    acyclic: [core, app]
`, true},
		{"acyclic over one zone", oneZone + `
rules:
  deps/acyclic:
    acyclic: [core]
`, false},
		{"uses on a zone with parameters", oneZone + `
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
		{"uses with each", oneZone + `
rules:
  core/checked:
    on: core
    uses: acme/check
    each: domain.aggregates
`, false},
		{"uses with a blank name", oneZone + `
rules:
  core/checked:
    on: core
    uses: " "
`, false},

		// ---- the Rule envelope -------------------------------------------
		{"rule with two constraints", oneZone + `
rules:
  core/two:
    on: core
    imports:
      internal: []
    naming: snake_case
`, false},
		{"rule with a retired kind key", oneZone + `
rules:
  core/naming:
    kind: naming
    on: core
    case: snake_case
`, false},
		{"imports without on", oneZone + `
rules:
  core/imports:
    imports:
      internal: []
`, false},
		{"imports naming a zone twice", twoZones + `
rules:
  core/imports:
    on: [core, core]
    imports:
      internal: []
`, false},
		{"imports allowing a zone twice", twoZones + `
rules:
  core/imports:
    on: core
    imports:
      internal: [app, app]
`, false},
		{"imports with an empty on", oneZone + `
rules:
  core/imports:
    on: []
    imports:
      internal: []
`, false},
		{"files on imports", oneZone + `
rules:
  core/imports:
    on: core
    files: "core/**/*.go"
    imports:
      internal: []
`, false},
		{"with on imports", oneZone + `
rules:
  core/imports:
    on: core
    imports:
      internal: []
    with:
      depth: 1
`, false},
		{"unknown severity", oneZone + `
rules:
  core/imports:
    severity: critical
    on: core
    imports:
      internal: []
`, false},
		{"disable with a reason", oneZone + `
rules:
  core/imports:
    on: core
    imports:
      internal: []
    disable: "the core is being rewritten; re-enable after AL-42"
`, true},
		{"disable without a reason", oneZone + `
rules:
  core/imports:
    on: core
    imports:
      internal: []
    disable: ""
`, false},
		{"exclude paths with a reason", oneZone + `
rules:
  core/imports:
    on: core
    imports:
      internal: []
    exclude:
      paths: ["core/generated/**"]
      reason: "generated code is not authored"
`, true},
		{"exclude zones with a reason", twoZones + `
rules:
  deps/acyclic:
    acyclic: {}
    exclude:
      zones: [app]
      reason: "app is the composition root"
`, true},
		{"exclude listing a path twice", oneZone + `
rules:
  core/imports:
    on: core
    imports:
      internal: []
    exclude:
      paths: ["core/generated/**", "core/generated/**"]
      reason: "generated code is not authored"
`, false},
		{"exclude listing a zone twice", twoZones + `
rules:
  deps/acyclic:
    acyclic: {}
    exclude:
      zones: [app, app]
      reason: "app is the composition root"
`, false},
		{"exclude without a reason", oneZone + `
rules:
  core/imports:
    on: core
    imports:
      internal: []
    exclude:
      paths: ["core/generated/**"]
`, false},
		{"exclude naming no subject", oneZone + `
rules:
  core/imports:
    on: core
    imports:
      internal: []
    exclude:
      reason: "why"
`, false},
		{"suppress paths with a reason", oneZone + `
rules:
  core/imports:
    on: core
    imports:
      internal: []
    suppress:
      paths: ["core/legacy/**"]
      reason: "adopted debt tracked in the baseline"
`, true},
		{"suppress listing a path twice", oneZone + `
rules:
  core/imports:
    on: core
    imports:
      internal: []
    suppress:
      paths: ["core/legacy/**", "core/legacy/**"]
      reason: "adopted debt tracked in the baseline"
`, false},
		{"suppress without paths", oneZone + `
rules:
  core/imports:
    on: core
    imports:
      internal: []
    suppress:
      reason: "why"
`, false},
		{"rule id starting with a slash", oneZone + `
rules:
  /core:
    on: core
    imports:
      internal: []
`, false},
		{"rule id ending with a slash", oneZone + `
rules:
  core/:
    on: core
    imports:
      internal: []
`, false},
		{"rules as a list", oneZone + `
rules:
  - id: core/imports
`, false},
		{"unknown top-level key", "rulesets: []\n", false},
		{"retired contracts key", oneZone + `
contracts:
  core:
    consumes:
      id: t/p:core/stdlib-only
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
		{"scan excluding a glob twice", "scan:\n  exclude: [\"vendor/**\", \"vendor/**\"]\n", false},
		{"extends listing a pattern twice", "extends:\n  - pattern: arclint/vertical@0.1.0\n  - pattern: arclint/vertical@0.1.0\n", false},
		{"extends entry without pattern", "extends:\n  - bind:\n      core: core/**\n", false},
		{"extends with an inexact version", "extends:\n  - pattern: acme/hexagonal@latest\n", false},
		{"extends with a bind list", "extends:\n  - pattern: acme/hexagonal@1.0.0\n    bind: [core]\n", false},
		{"extends with an unknown key", "extends:\n  - pattern: acme/hexagonal@1.0.0\n    version: 1.0.0\n", false},
		{"override with a description", oneZone + `
rules:
  acme/hexagonal:core/stdlib-only:
    description: "rewritten"
    severity: warning
`, false},
		{"override with on", oneZone + `
rules:
  acme/hexagonal:core/stdlib-only:
    on: core
    severity: warning
`, false},
		{"override changing nothing", `
rules:
  acme/hexagonal:core/stdlib-only: {}
`, false},

		// ---- pattern distribution files ----------------------------------
		{"pattern file", header + `
zones:
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
		{"pattern file with canonical rationale", header + `
zones:
  core:
    description: "The domain core."
rules:
  core/stdlib-only:
    rationale: "Keep the domain independent of adapters."
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
zones:
  core: "The domain core."
rules:
  core/stdlib-only:
    on: core
    imports:
      internal: []
`, true},
		{"pattern file with runtime", header + `
runtime: [go]
zones:
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
zones:
  core: "The domain core."
rules:
  core/stdlib-only:
    on: core
    imports:
      internal: []
`, false},
		{"pattern file with extends", header + `
extends: []
zones:
  core: "The domain core."
rules:
  core/stdlib-only:
    on: core
    imports:
      internal: []
`, false},
		{"pattern file with a glob-list zone", header + `
zones:
  core: ["core/**"]
rules:
  core/stdlib-only:
    on: core
    imports:
      internal: []
`, false},
		{"pattern file zone without a description", header + `
zones:
  core:
    paths: ["core/**"]
rules:
  core/stdlib-only:
    on: core
    imports:
      internal: []
`, false},
		{"pattern file zone with a blank description", header + `
zones:
  core: " "
rules:
  core/stdlib-only:
    on: core
    imports:
      internal: []
`, false},
		{"pattern file with an override", header + `
zones:
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

	// The loader resolves extends against the embedded source, so a
	// case naming a built-in Pattern exercises real resolution; the
	// schema judges the document alone.
	embedded, err := embeddedpattern.NewSource().Patterns()
	if err != nil {
		t.Fatalf("embedded patterns: %v", err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, loaderErr := yamlrule.Load([]byte(tc.document), tc.name, vocab.UbiquitousLanguage{}, embedded)
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
