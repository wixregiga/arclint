package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestExtensionIncomingDependenciesEndToEnd(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", `runtime: [ts]
zones:
  domain: domain/**
  adapter: adapter/**
rules:
  incoming:
    on: domain
    uses: incoming
`)
	write(t, root, "domain/order.ts", "export const order = 1;\n")
	write(t, root, "adapter/handler.ts", "import { order } from '../domain/order';\nimport '../elsewhere/other';\n")
	write(t, root, "elsewhere/other.ts", "export const other = 2;\n")
	program := `import { defineRule } from "arclint";
export default defineRule({ type: "incoming", check(ctx) {
  const subject = "domain/order.ts";
  const facts = ctx.facts(subject);
  if (!facts.importsAvailable || facts.imports.length !== 0) throw new Error("empty imports failed");
  if (facts.dependencies.length !== 1) throw new Error("wrong incident evidence");
  const edge = facts.dependencies[0];
  if (edge.sourcePath !== "adapter/handler.ts" || edge.line !== 1 ||
      edge.targetKind !== "file" || edge.targetPath !== subject ||
      edge.sourceZones.join() !== "adapter" || edge.targetZones.join() !== "domain" || !edge.targetObserved) {
    throw new Error("incoming evidence lost attributes");
  }
  if (ctx.facts(edge.sourcePath) !== null || ctx.imports(edge.sourcePath).length || ctx.zoneOf(edge.sourcePath).length) {
    throw new Error("evidence granted broader facts");
  }
  let denied = false;
  try { ctx.read(edge.sourcePath); } catch (_) { denied = true; }
  if (!denied) throw new Error("evidence granted content");
  ctx.report(%s);
}});`
	for _, tc := range []struct {
		name, report string
		accepted     bool
	}{
		{"evidence", `{subjectPath: subject, path: edge.sourcePath, line: edge.line, message: "forbidden importer"}`, true},
		{"wrong line", `{subjectPath: subject, path: edge.sourcePath, line: 2, message: "forged evidence"}`, false},
		{"wrong subject", `{subjectPath: edge.sourcePath, path: edge.sourcePath, line: 1, message: "forged subject"}`, false},
		{"missing subject", `{path: edge.sourcePath, line: 1, message: "missing subject"}`, false},
		{"target location", `{subjectPath: subject, path: "elsewhere/other.ts", line: 1, message: "forged path"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			write(t, root, ".arclint/extensions/incoming.ts", fmt.Sprintf(program, tc.report))
			stdout, stderr, code := runBin(t, root, os.Environ(), "check", "--format", "json")
			if code != 1 {
				t.Fatalf("exit %d: %s\n%s", code, stdout, stderr)
			}
			var diagnostics []diagnosticDoc
			if err := json.Unmarshal([]byte(stdout), &diagnostics); err != nil {
				t.Fatal(err)
			}
			var violations, operational int
			for _, d := range diagnostics {
				if d.Kind == "violation" {
					violations++
					if d.Path != "adapter/handler.ts" || d.Line != 1 || d.Message != "forbidden importer" {
						t.Fatalf("lost import location: %+v", d)
					}
				}
				if d.Kind == "operational" {
					operational++
				}
			}
			if tc.accepted && (violations != 1 || operational != 0) || !tc.accepted && (violations != 0 || operational != 1) {
				t.Fatalf("evidence authorization: %s", stdout)
			}
		})
	}
}

// Exercise default facts through configuration, observation, the check's
// membership index, and the real sandbox. Targets never become subjects.
func TestExtensionDependencyObservationsEndToEnd(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".arclint/extensions/dependencies.ts", `
import { defineRule } from "arclint";
export default defineRule({
  type: "dependency-facts",
  check(ctx) {
    function assert(ok, message) { if (!ok) throw new Error(message); }
    function equal(actual, expected, message) {
      assert(JSON.stringify(actual) === JSON.stringify(expected), message + ": " + JSON.stringify(actual));
    }
    equal(ctx.files().map(f => f.path), ["src/empty.go", "src/kept.go", "src/kept.ts"], "Scope");
    const go = ctx.facts("src/kept.go");
    assert(go.importsAvailable && go.declarationsAvailable, "Go availability");
    equal(go.zones, ["source", "src"], "source Zones");
    equal(go.imports, [{path: "example.test/m/pkg", line: 2, class: "internal",
      targetDir: "pkg", targetFile: "", targetZones: ["package-a", "package-b"], targetObserved: true}], "package import");
    assert(go.decls.some(d => d.name === "Kept"), "declarations still present");
    const ts = ctx.facts("src/kept.ts");
    assert(ts.importsAvailable && ts.declarationsAvailable, "TypeScript availability");
    equal(ts.imports, [
      {path: "../dep/target", line: 1, class: "internal", targetDir: "dep",
       targetFile: "dep/target.ts", targetZones: ["dependency", "target"], targetObserved: true},
      {path: "./excluded", line: 2, class: "internal", targetDir: "src",
       targetFile: "src/excluded.ts", targetZones: ["source", "src"], targetObserved: true}
    ], "exact targets do not inherit sibling Zones");
    const empty = ctx.facts("src/empty.go");
    assert(empty.importsAvailable && empty.imports.length === 0, "observed empty imports");
    for (const target of [ts.imports[0].targetFile, ts.imports[1].targetFile, "pkg/a.go", "pkg/b.go"]) {
      assert(ctx.facts(target) === null, "target facts leaked: " + target);
      assert(ctx.imports(target).length === 0, "target imports leaked: " + target);
      assert(ctx.zoneOf(target).length === 0, "target membership lookup leaked: " + target);
      let denied = false;
      try { ctx.read(target); } catch (_) { denied = true; }
      assert(denied, "target content leaked: " + target);
    }
    equal(ctx.zones()["target"], [], "target Zone must have no selected files");
    equal(ctx.zones()["src"], ["src/empty.go", "src/kept.go", "src/kept.ts"], "excluded Zone members");
    // Returned metadata is detached from the shared membership index.
    go.zones.push("invented");
    ts.imports[0].targetZones.push("invented");
    equal(ctx.facts("src/kept.go").zones, ["source", "src"], "source membership immutable");
    equal(ctx.facts("src/kept.ts").imports[0].targetZones, ["dependency", "target"], "target membership immutable");
    ctx.report({path: "src/kept.go", message: "dependency facts and Scope preserved"});
  }
});
`)
	write(t, root, "rules.arclint.yaml", `runtime: [go, ts]
zones:
  src: src/**
  source: src/**
  dependency: dep/**
  target: dep/target.ts
  sibling: dep/sibling.ts
  package-a: pkg/a.go
  package-b: pkg/b.go
  outside: elsewhere/**
rules:
  dependencies:
    on: src
    uses: dependency-facts
    exclude:
      paths: [src/excluded.ts]
      reason: deliberately outside the extension's subjects
`)
	write(t, root, "go.mod", "module example.test/m\n\ngo 1.24\n")
	write(t, root, "src/kept.go", "package src\nimport _ \"example.test/m/pkg\"\nfunc Kept() {}\n")
	write(t, root, "src/empty.go", "package src\n")
	write(t, root, "src/kept.ts", "import { target } from '../dep/target';\nimport './excluded';\nexport const kept = target;\n")
	write(t, root, "src/excluded.ts", "export const excluded = 1;\n")
	write(t, root, "dep/target.ts", "export const target = 1;\n")
	write(t, root, "dep/sibling.ts", "export const sibling = 1;\n")
	write(t, root, "pkg/a.go", "package pkg\n")
	write(t, root, "pkg/b.go", "package pkg\n")

	stdout, stderr, code := runBin(t, root, os.Environ(), "check", "--format", "json")
	if code != 1 {
		t.Fatalf("exit %d, want probe report\n%s\n%s", code, stdout, stderr)
	}
	var diagnostics []diagnosticDoc
	if err := json.Unmarshal([]byte(stdout), &diagnostics); err != nil {
		t.Fatal(err)
	}
	var reports int
	for _, d := range diagnostics {
		if d.Kind == "operational" {
			t.Fatalf("facts probe failed: %+v", d)
		}
		if d.Kind == "violation" {
			if d.Path != "src/kept.go" || d.Message != "dependency facts and Scope preserved" {
				t.Fatalf("unexpected finding: %+v", d)
			}
			reports++
		}
	}
	if reports != 1 {
		t.Fatalf("wanted one successful probe report: %s", stdout)
	}
	// Files outside the selected Scope and its observed import targets do
	// not enter this Rule's supplied facts.
	write(t, root, "elsewhere/unselected.txt", "outside the supplied facts")
	after, afterStderr, afterCode := runBin(t, root, os.Environ(), "check", "--format", "json")
	if afterCode != code || after != stdout {
		t.Fatalf("file outside the supplied facts changed extension behavior:\nbefore: %s\nafter (%d): %s\n%s", stdout, afterCode, after, afterStderr)
	}

	// Knowing the resolved target does not authorize reporting against it,
	// whether excluded explicitly or outside the Rule's selected Zone.
	write(t, root, ".arclint/extensions/dependencies.ts", `
import { defineRule } from "arclint";
export default defineRule({
  type: "dependency-facts",
  check(ctx) {
    for (const imp of ctx.facts("src/kept.ts").imports) {
      ctx.report({path: imp.targetFile, message: "must be rejected"});
    }
  }
});
`)
	stdout, stderr, code = runBin(t, root, os.Environ(), "check", "--format", "json")
	if code != 1 {
		t.Fatalf("exit %d, want Scope breach\n%s\n%s", code, stdout, stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &diagnostics); err != nil {
		t.Fatal(err)
	}
	breaches := map[string]bool{}
	for _, d := range diagnostics {
		if d.Kind == "violation" {
			t.Fatalf("out-of-Scope target became a finding: %+v", d)
		}
		if d.Kind == "operational" && d.Severity == "error" && d.RuleID == "dependencies" {
			breaches[d.Path] = true
		}
	}
	if len(breaches) != 2 || !breaches["dep/target.ts"] || !breaches["src/excluded.ts"] {
		t.Fatalf("wanted both target Scope breaches: %s", stdout)
	}
}
