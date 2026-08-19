package ruletest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/rule"
	sobekextension "github.com/wixregiga/arclint/internal/infrastructure/extension/sobek"
	"github.com/wixregiga/arclint/internal/infrastructure/language/golang"
	yamlrule "github.com/wixregiga/arclint/internal/infrastructure/rule/yaml"
	"github.com/wixregiga/arclint/internal/infrastructure/ruletest"
)

const integrationRuleset = `runtime: [go]
scan:
  unknown_imports: error
modules:
  core:
    paths: ["core/**"]
  util:
    paths: ["util/**"]
contracts:
  core:
    consumes:
      id: "t:core/consumes"
      internal: []
      external: forbid
      stdlib: allow
    invariants:
      - id: "t:core/has-doc"
        kind: structure
        require: ["core/doc.go"]
`

// The structure test expects the exact violation the real evaluator
// produces for a fixture missing the required file: it passes.
const structureTest = `rule: "t:core/has-doc"
files:
  core/other.go: "package core\n"
expect:
  - path: core/doc.go
    message: 'Module "core" is missing a required file matching "core/doc.go"'
`

// The consumes test asserts complete conformance over a fixture whose
// Go source, parsed by the production Go fact producer, imports a
// Module outside the empty allow-list: it fails with one unexpected
// violation.
const consumesTest = `rule: "t:core/consumes"
files:
  go.mod: "module example.com/app\n\ngo 1.26\n"
  core/a.go: "package core\n\nimport _ \"example.com/app/util\"\n"
  util/u.go: "package util\n"
expect: []
`

// TestRunRuleTestsEndToEnd proves the whole seam without the CLI: the
// YAML source, the real ruleset repository, the fixture observer
// running the production Go parser, and the use case comparing real
// assessments against authored expectations.
func TestRunRuleTestsEndToEnd(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		target := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			t.Fatalf("MkdirAll %s: %v", rel, err)
		}
		if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile %s: %v", rel, err)
		}
	}
	write("rules.yaml", integrationRuleset)
	write(".arclint/tests/consumes_disallowed_import.yaml", consumesTest)
	write(".arclint/tests/structure_missing_doc.yaml", structureTest)

	repo, err := yamlrule.NewRepository(filepath.Join(root, "rules.yaml"))
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	uc, err := application.NewRunRuleTests(repo, ruletest.NewSource(root),
		ruletest.NewObserver(golang.NewProducer()), nil)
	if err != nil {
		t.Fatalf("NewRunRuleTests: %v", err)
	}
	results, err := uc.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want one per authored test", len(results))
	}

	failing := results[0]
	if failing.Name != "consumes_disallowed_import" || failing.RuleID != "t:core/consumes" {
		t.Fatalf("results[0] identity = %q %q", failing.Name, failing.RuleID)
	}
	if failing.Passed() || failing.Err != "" {
		t.Fatalf("consumes result = %+v, want a failed comparison without a test error", failing)
	}
	if len(failing.Missing) != 0 || len(failing.Unexpected) != 1 {
		t.Fatalf("consumes comparison = missing %v unexpected %v, want exactly one unexpected finding",
			failing.Missing, failing.Unexpected)
	}
	wantFinding := rule.Finding{
		Kind:    rule.FindingViolation,
		Path:    "core/a.go",
		Line:    3,
		Message: `import "example.com/app/util" resolves to Module(s) ["util"], not in the allow-list of Module "core"`,
	}
	if failing.Unexpected[0] != wantFinding {
		t.Errorf("unexpected finding = %+v, want %+v", failing.Unexpected[0], wantFinding)
	}

	passing := results[1]
	if passing.Name != "structure_missing_doc" || passing.RuleID != "t:core/has-doc" {
		t.Fatalf("results[1] identity = %q %q", passing.Name, passing.RuleID)
	}
	if !passing.Passed() {
		t.Errorf("structure result = %+v, want a pass: the authored expectation matches the real evaluator output",
			passing)
	}
}

const extensionRuleset = `runtime: [go]
modules:
  m:
    paths: ["m/**"]
contracts:
  m:
    invariants:
      - id: "t:m/no-panic"
        kind: extension
        files: "m/**/*.go"
        uses: forbid-content
        with:
          pattern: '\bpanic\('
`

// Fixture content contains panic; the repository production file does
// not. ctx.read must see the fixture bytes or the expectation misses.
const extensionFixtureTest = `rule: "t:m/no-panic"
files:
  m/a.go: |
    package m
    func f() { panic("x") }
expect:
  - path: m/a.go
    line: 2
    message: 'forbidden content matching /\bpanic\(/'
`

const forbidContentExtension = `import { defineRule, s } from "arclint";

export default defineRule({
  type: "forbid-content",
  description: "report lines matching a configured pattern",
  capability: "exact",
  params: s.object({
    pattern: s.string().describe("RegExp source matched against each line"),
  }),
  check(ctx, params) {
    const re = new RegExp(String(params.pattern));
    for (const file of ctx.files()) {
      const lines = ctx.read(file.path).split("\n");
      lines.forEach((line, index) => {
        if (re.test(line)) {
          ctx.report({
            path: file.path,
            line: index + 1,
            message: "forbidden content matching /" + params.pattern + "/",
            fixHint: "remove the content",
          });
        }
      });
    }
  },
});
`

// TestRuleTestExtensionReadsFixtureContent proves Extension ctx.read
// sees authored fixture bytes even when the repository has different
// content at the same path (or would, after the temp fixture tree is
// gone). A root-filesystem read would miss the panic and fail the test.
func TestRuleTestExtensionReadsFixtureContent(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		target := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			t.Fatalf("MkdirAll %s: %v", rel, err)
		}
		if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile %s: %v", rel, err)
		}
	}
	write("rules.yaml", extensionRuleset)
	write(".arclint/extensions/forbid_content.ts", forbidContentExtension)
	write(".arclint/tests/fixture_reads_panic.yaml", extensionFixtureTest)
	// Production path exists with clean content — the opposite of the fixture.
	write("m/a.go", "package m\n// clean production file\n")

	repo, err := yamlrule.NewRepository(filepath.Join(root, "rules.yaml"))
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	extensions, err := sobekextension.NewEvaluator(root)
	if err != nil {
		t.Fatalf("NewEvaluator: %v", err)
	}
	uc, err := application.NewRunRuleTests(repo, ruletest.NewSource(root),
		ruletest.NewObserver(golang.NewProducer()), extensions)
	if err != nil {
		t.Fatalf("NewRunRuleTests: %v", err)
	}
	results, err := uc.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	got := results[0]
	if !got.Passed() {
		t.Fatalf("result = %+v, want pass from fixture-driven ctx.read", got)
	}
}
