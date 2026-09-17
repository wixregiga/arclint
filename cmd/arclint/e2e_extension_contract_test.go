package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

// One extension and one set of observed inputs exercise both distribution
// paths and both CLI evaluation paths. A successful probe reports a sentinel;
// silently skipping the extension cannot pass the comparison.
const extensionContractProbe = `import { defineRule, s } from "arclint";
export default defineRule({
  type: "contract-probe",
  params: s.object({
    label: s.string().default("default"),
    forbidden: s.boolean().default(false),
  }),
  check(ctx, params) {
    function assert(ok, message) { if (!ok) throw new Error(message); }
    function canonical(value) {
      if (Array.isArray(value)) return value.map(canonical);
      if (value && typeof value === "object") {
        const result = {};
        for (const key of Object.keys(value).sort()) result[key] = canonical(value[key]);
        return result;
      }
      return value;
    }
    function equal(a, b) { assert(JSON.stringify(canonical(a)) === JSON.stringify(canonical(b)), JSON.stringify(a)); }
    assert(typeof params.label === "string" && typeof params.forbidden === "boolean" &&
      Object.keys(params).length === 2, "unvalidated params reached check");
    const {subject, importer, excluded, outside, content, declaration, edge, outgoing, empty, outgoingImport} = input;
    equal(ctx.files().map(f => f.path), [empty, subject]);
    equal(ctx.files("src/*").map(f => f.path), [empty, subject]);
    equal(ctx.files("outside/**"), []);
    equal(ctx.zoneOf(subject), ["selected"]);
    equal(ctx.zones().selected, [empty, subject]);
    equal(ctx.zones().adapter, []);
    assert(ctx.read(subject) === content, "wrong input content");
    const facts = ctx.facts(subject);
    assert(facts.importsAvailable && facts.declarationsAvailable, "facts unavailable");
    equal(facts.imports, [outgoingImport]);
    const emptyFacts = ctx.facts(empty);
    assert(emptyFacts.importsAvailable, "empty imports are available");
    equal(emptyFacts.imports, []);
    equal(ctx.imports(subject), facts.imports);
    assert(facts.decls.some(d => d.name === declaration), "missing declaration");
    equal(facts.dependencies, [edge, outgoing]);
    for (const path of [excluded, importer, outside]) {
      equal(ctx.facts(path), null);
      equal(ctx.imports(path), []);
      equal(ctx.zoneOf(path), []);
      let denied = false;
      try { ctx.read(path); } catch (_) { denied = true; }
      assert(denied, "unauthorized read: " + path);
    }
    equal(ctx.domain().contexts, []);
    equal(ctx.caseTerm("Order Item", "snake_case"), "order_item");
    ctx.report({subjectPath: subject, path: importer, line: edge.line,
      message: "contract " + params.label});
    if (params.forbidden) {
      ctx.report({path: outside, message: "unauthorized report"});
    }
  }
});`

type contractFinding struct {
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}

func TestExtensionContractAcrossExecutionPaths(t *testing.T) {
	for _, language := range []struct {
		name, ext, content, empty, importer, excluded, outside, declaration string
		specifier, targetKind, target, outgoingSpecifier, outgoingTarget    string
		line                                                                int
	}{
		{
			name: "ts", ext: "ts", declaration: "order", line: 1,
			content: "import '../outside/other';\nexport const order = 1;\n", empty: "export const empty = 0;\n",
			importer: "import { order } from '../src/order';\nimport '../outside/other';\n",
			excluded: "import './order';\n", outside: "export const other = 2;\n",
			specifier: "../src/order", targetKind: "file", target: "src/order.ts",
			outgoingSpecifier: "../outside/other", outgoingTarget: "outside/other.ts",
		},
		{
			name: "go", ext: "go", declaration: "Order", line: 2,
			content: "package src\nimport _ \"example.test/contract/outside\"\nfunc Order() {}\n", empty: "package src\n",
			importer: "package adapter\nimport _ \"example.test/contract/src\"\nimport _ \"example.test/contract/outside\"\n",
			excluded: "package src\nimport _ \"example.test/contract/src\"\n", outside: "package outside\n",
			specifier: "example.test/contract/src", targetKind: "directory", target: "src",
			outgoingSpecifier: "example.test/contract/outside", outgoingTarget: "outside",
		},
		{
			name: "py", ext: "py", declaration: "order", line: 1,
			content: "import outside.other\ndef order():\n    pass\n", empty: "empty = 0\n",
			importer: "from src.order import order\nimport outside.other\n",
			excluded: "from src.order import order\n", outside: "other = 2\n",
			specifier: "src.order", targetKind: "file", target: "src/order.py",
			outgoingSpecifier: "outside.other", outgoingTarget: "outside/other.py",
		},
	} {
		subject, importer := "src/order."+language.ext, "adapter/handler."+language.ext
		excluded, outside := "src/excluded."+language.ext, "outside/other."+language.ext
		empty := "src/empty." + language.ext
		files := map[string]string{
			empty:   language.empty,
			subject: language.content, importer: language.importer,
			excluded: language.excluded, outside: language.outside,
		}
		if language.name == "go" {
			files["go.mod"] = "module example.test/contract\n\ngo 1.22\n"
		}
		targetFile := outside
		if language.targetKind == "directory" {
			targetFile = ""
		}
		input, err := json.Marshal(map[string]any{
			"subject": subject, "importer": importer, "excluded": excluded, "outside": outside,
			"content": language.content, "declaration": language.declaration, "empty": empty,
			"outgoingImport": map[string]any{"path": language.outgoingSpecifier, "line": language.line,
				"class": "internal", "targetDir": "outside", "targetFile": targetFile,
				"targetZones": []string{}, "targetObserved": true},
			"outgoing": map[string]any{"sourcePath": subject, "targetPath": language.outgoingTarget,
				"targetKind": language.targetKind, "specifier": language.outgoingSpecifier, "line": language.line,
				"classification": "internal", "sourceZones": []string{"selected"},
				"targetZones": []string{}, "targetObserved": true},
			"edge": map[string]any{"sourcePath": importer, "targetPath": language.target,
				"targetKind": language.targetKind, "specifier": language.specifier, "line": language.line,
				"classification": "internal", "sourceZones": []string{"adapter"},
				"targetZones": []string{"selected"}, "targetObserved": true},
		})
		if err != nil {
			t.Fatal(err)
		}
		program := "const input = " + string(input) + ";\n" + extensionContractProbe
		for _, source := range []string{"repository", "pattern"} {
			for _, tc := range []struct {
				name, params, label, invalid string
				forbidden                    bool
			}{
				{name: "defaults", params: "{}", label: "default"},
				{name: "explicit", params: `{label: chosen}`, label: "chosen"},
				{name: "invalid-type", params: `{label: 42}`, invalid: "got number, want string"},
				{name: "unknown-param", params: `{unknown: true}`, invalid: "additional properties"},
				{name: "forbidden-report", params: `{forbidden: true}`, forbidden: true},
			} {
				t.Run(language.name+"/"+source+"/"+tc.name, func(t *testing.T) {
					root := t.TempDir()
					for path, content := range files {
						write(t, root, path, content)
					}
					ruleID, pattern := "contract", ""
					rules := fmt.Sprintf(`rules:
  contract:
    on: selected
    uses: contract-probe
    exclude:
      paths: [%s]
      reason: excluded subjects grant no access
    with: %s
`, excluded, tc.params)
					if source == "repository" {
						write(t, root, ".arclint/extensions/probe.ts", program)
						write(t, root, "rules.arclint.yaml", "runtime: ["+language.name+"]\nzones:\n  selected: src/**\n  adapter: adapter/**\n"+rules)
					} else {
						ruleID, pattern = "acme/contracts:contract", "acme/contracts@1.0.0"
						write(t, root, ".arclint/patterns/acme/contracts/extensions/probe.ts", program)
						write(t, root, ".arclint/patterns/acme/contracts/pattern.yaml", fmt.Sprintf(`pattern:
  namespace: acme
  name: contracts
  version: 1.0.0
  coverage: [%s]
  documentation: Extension compatibility fixture.
zones:
  selected: The subjects checked by the extension.
  adapter: Incoming dependency sources.
`, language.name)+rules)
						write(t, root, "rules.arclint.yaml", fmt.Sprintf(`runtime: [%s]
extends:
  - pattern: acme/contracts@1.0.0
    bind:
      selected: src/**
      adapter: adapter/**
`, language.name))
					}

					stdout, stderr, code := runBin(t, root, os.Environ(), "check", "--format", "json")
					var expected []contractFinding
					if tc.invalid != "" {
						if code == 0 || !strings.Contains(stdout+stderr, tc.invalid) || strings.Contains(stdout+stderr, "unvalidated params reached check") {
							t.Fatalf("expected parameter validation error: exit %d\n%s\n%s", code, stdout, stderr)
						}
					} else {
						var diagnostics []struct {
							contractFinding
							RuleID  string `json:"ruleId"`
							Pattern string `json:"pattern"`
						}
						if err := json.Unmarshal([]byte(stdout), &diagnostics); err != nil || code != 1 {
							t.Fatalf("check: exit %d, decode %v\n%s\n%s", code, err, stdout, stderr)
						}
						var violations, operational int
						for _, d := range diagnostics {
							expected = append(expected, d.contractFinding)
							switch d.Kind {
							case "violation":
								violations++
								if d.RuleID != ruleID || d.Pattern != pattern || d.Path != importer || d.Line != language.line || d.Message != "contract "+tc.label {
									t.Fatalf("finding or provenance differs: %+v", d)
								}
							case "operational":
								operational++
								if d.RuleID != ruleID || d.Path != outside || !strings.Contains(d.Message, "outside the rule's scope") {
									t.Fatalf("unexpected operational failure: %+v", d)
								}
							}
						}
						if tc.forbidden && (violations != 0 || operational != 1) || !tc.forbidden && (violations != 1 || operational != 0) {
							t.Fatalf("wrong evaluation outcome: %s", stdout)
						}
					}

					// Compare the entire pre-Baseline assessment, including coverage.
					fixtureExpected := expected
					if tc.forbidden {
						// Rule-wide coverage has no path and cannot be authored as an
						// expectation. Compare the unexpected assessment below instead.
						fixtureExpected = nil
					}
					fixture, err := json.Marshal(map[string]any{"rule": ruleID, "files": files, "expect": fixtureExpected})
					if err != nil {
						t.Fatal(err)
					}
					write(t, root, ".arclint/tests/contract.yaml", string(fixture))
					// The Rule test must use its own supplied content.
					write(t, root, subject, language.content+"\n")
					stdout, stderr, code = runBin(t, root, os.Environ(), "rules", "test", "--format", "json")
					var results []struct {
						RuleID     string            `json:"ruleId"`
						Passed     bool              `json:"passed"`
						Error      string            `json:"error"`
						Unexpected []contractFinding `json:"unexpected"`
					}
					if err := json.Unmarshal([]byte(stdout), &results); err != nil || len(results) != 1 || results[0].RuleID != ruleID {
						t.Fatalf("Rule test identity: exit %d, decode %v\n%s\n%s", code, err, stdout, stderr)
					}
					if tc.invalid != "" {
						if code != 1 || results[0].Passed || !strings.Contains(results[0].Error, tc.invalid) || strings.Contains(results[0].Error, "unvalidated params reached check") {
							t.Fatalf("Rule test bypassed parameter validation: %s", stdout)
						}
					} else if tc.forbidden {
						if code != 1 || results[0].Passed || results[0].Error != "" || !reflect.DeepEqual(results[0].Unexpected, expected) {
							t.Fatalf("check and Rule test disagree on rejected reports:\nwant %+v\ngot %s", expected, stdout)
						}
					} else if code != 0 || !results[0].Passed {
						t.Fatalf("check and Rule test disagree: exit %d\n%s\n%s", code, stdout, stderr)
					}
				})
			}
		}
	}
}
