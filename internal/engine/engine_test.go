package engine_test

// Provider-level tests over real temp repos: graph rules, invariants,
// correspondence with content-derived captures, unknown-import policy.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/engine"
	"github.com/wixregiga/arclint/internal/report"
)

// writeRepo materializes files (including rules.yaml) and loads the ruleset.
func writeRepo(t *testing.T, files map[string]string) *config.RuleSet {
	t.Helper()
	root := t.TempDir()
	for p, content := range files {
		abs := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rs, err := config.Load(filepath.Join(root, "rules.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

func check(t *testing.T, rs *config.RuleSet) *engine.Result {
	t.Helper()
	res, err := engine.Check(rs)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func ruleIDs(res *engine.Result) []string {
	var ids []string
	for _, v := range res.Violations {
		ids = append(ids, v.RuleID)
	}
	return ids
}

const graphRepoGoMod = "module example.com/app\n\ngo 1.24\n"

// graphRepo: three one-package layers with domain -> ui and app -> domain
// imports.
func graphRepo(rules string) map[string]string {
	return map[string]string{
		"rules.yaml":       rules,
		"go.mod":           graphRepoGoMod,
		"ui/ui.go":         "package ui\n\nvar U = 1\n",
		"app/app.go":       "package app\n\nimport _ \"example.com/app/domain\"\n",
		"domain/domain.go": "package domain\n\nimport _ \"example.com/app/ui\"\n",
	}
}

const graphModules = `
modules:
  ui: ["ui/**"]
  app: ["app/**"]
  domain: ["domain/**"]
`

func TestLayers(t *testing.T) {
	rs := writeRepo(t, graphRepo(`runtime: [go]`+graphModules+`
dependencies:
  - kind: layers
    layers: [ui, app, domain]
`))
	res := check(t, rs)
	if len(res.Violations) != 1 {
		t.Fatalf("violations: %+v", res.Violations)
	}
	v := res.Violations[0]
	if v.RuleID != "dependencies.layers[0]" || v.Path != "domain/domain.go" ||
		v.Blame != report.BlameConsumer || v.Line == nil {
		t.Errorf("unexpected violation: %+v", v)
	}
	if !strings.Contains(v.Message, "higher layer") {
		t.Errorf("message: %s", v.Message)
	}
}

func TestForbidden(t *testing.T) {
	rs := writeRepo(t, graphRepo(`runtime: [go]`+graphModules+`
dependencies:
  - kind: forbidden
    from: [app]
    to: [domain]
`))
	res := check(t, rs)
	if len(res.Violations) != 1 || res.Violations[0].Path != "app/app.go" {
		t.Fatalf("violations: %+v", res.Violations)
	}
}

func TestIndependence(t *testing.T) {
	rs := writeRepo(t, graphRepo(`runtime: [go]`+graphModules+`
dependencies:
  - kind: independence
    modules: [domain, ui]
`))
	res := check(t, rs)
	if len(res.Violations) != 1 || res.Violations[0].Path != "domain/domain.go" {
		t.Fatalf("violations: %+v", res.Violations)
	}
}

func TestProtected(t *testing.T) {
	files := graphRepo(`runtime: [go]` + graphModules + `
dependencies:
  - kind: protected
    module: ui
    allow: []
`)
	// An importer outside any declared module also violates protection.
	files["stray/stray.go"] = "package stray\n\nimport _ \"example.com/app/ui\"\n"
	rs := writeRepo(t, files)
	res := check(t, rs)
	if len(res.Violations) != 2 {
		t.Fatalf("violations: %+v", res.Violations)
	}
	paths := []string{res.Violations[0].Path, res.Violations[1].Path}
	if paths[0] != "domain/domain.go" || paths[1] != "stray/stray.go" {
		t.Errorf("paths: %v", paths)
	}
	if !strings.Contains(res.Violations[1].Message, "outside any declared module") {
		t.Errorf("message: %s", res.Violations[1].Message)
	}
}

func TestAcyclic(t *testing.T) {
	rs := writeRepo(t, map[string]string{
		"rules.yaml": `runtime: [go]
modules:
  a: ["a/**"]
  b: ["b/**"]
dependencies:
  - kind: acyclic
`,
		"go.mod": graphRepoGoMod,
		"a/a.go": "package a\n\nimport _ \"example.com/app/b\"\n",
		"b/b.go": "package b\n\nimport _ \"example.com/app/a\"\n",
	})
	res := check(t, rs)
	if len(res.Violations) != 1 {
		t.Fatalf("violations: %+v", res.Violations)
	}
	v := res.Violations[0]
	if v.RuleID != "dependencies.acyclic[0]" || !strings.Contains(v.Message, "a -> b") {
		t.Errorf("violation: %+v", v)
	}
}

func TestStdlibForbidAndInternalDeny(t *testing.T) {
	rs := writeRepo(t, map[string]string{
		"rules.yaml": `runtime: [go]
modules:
  pure: ["pure/**"]
  legacy: ["legacy/**"]
contracts:
  pure:
    consumes:
      internal: { deny: [legacy] }
      stdlib: forbid
`,
		"go.mod":           graphRepoGoMod,
		"pure/p.go":        "package pure\n\nimport (\n\t_ \"fmt\"\n\t_ \"example.com/app/legacy\"\n)\n",
		"legacy/legacy.go": "package legacy\n",
	})
	res := check(t, rs)
	if len(res.Violations) != 2 {
		t.Fatalf("violations: %+v", res.Violations)
	}
	ids := ruleIDs(res)
	if ids[0] != "pure.consumes.stdlib" || ids[1] != "pure.consumes.internal" {
		t.Errorf("ids: %v", ids)
	}
}

func TestUnknownImportPolicy(t *testing.T) {
	files := map[string]string{
		"rules.yaml": `runtime: [go]
modules:
  app: ["**"]
`,
		"go.mod":  graphRepoGoMod,
		"main.go": "package main\n\nimport _ \"github.com/nobody/nothing\"\n\nfunc main() {}\n",
	}
	// Default policy: warn — no violation, one warning.
	rs := writeRepo(t, files)
	res := check(t, rs)
	if len(res.Violations) != 0 {
		t.Fatalf("violations under warn policy: %+v", res.Violations)
	}
	found := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "github.com/nobody/nothing") {
			found = true
		}
	}
	if !found {
		t.Errorf("missing unknown-import warning; warnings: %v", res.Warnings)
	}

	// error policy: violation.
	files["rules.yaml"] = `runtime: [go]
scan:
  unknown_imports: error
modules:
  app: ["**"]
`
	rs = writeRepo(t, files)
	res = check(t, rs)
	if len(res.Violations) != 1 || res.Violations[0].RuleID != "scan.unknown-imports" {
		t.Fatalf("violations under error policy: %+v", res.Violations)
	}
}

func TestCorrespondenceEqualWithContentCaptures(t *testing.T) {
	rs := writeRepo(t, map[string]string{
		"rules.yaml": `runtime: [go]
modules:
  entities: ["internal/entities/**"]
  setup: ["internal/setup/**"]
contracts:
  entities:
    provides:
      - kind: correspondence
        of: { files: 'internal/entities/[^/]+_(?P<s>[a-z0-9]+)\.go', value: "{s}" }
        in:
          files: 'internal/setup/setup\.go'
          content: 'Setup\("(?P<s>[a-z0-9]+)"\)'
          value: "{s}"
        relation: equal
`,
		"go.mod":                             graphRepoGoMod,
		"internal/entities/user_postgres.go": "package entities\n",
		"internal/setup/setup.go":            "package setup\n\nfunc init() {\n\tSetup(\"postgres\")\n\tSetup(\"mysql\")\n}\n\nfunc Setup(string) {}\n",
	})
	res := check(t, rs)
	// postgres matches both sides; mysql exists only on the in side, so the
	// equal relation flags it, anchored at the content match line.
	if len(res.Violations) != 1 {
		t.Fatalf("violations: %+v", res.Violations)
	}
	v := res.Violations[0]
	if v.Path != "internal/setup/setup.go" || v.Line == nil || *v.Line != 5 {
		t.Errorf("anchor: %+v", v)
	}
	if !strings.Contains(v.Message, `"mysql"`) {
		t.Errorf("message: %s", v.Message)
	}
}

func TestNamingInvariant(t *testing.T) {
	rs := writeRepo(t, map[string]string{
		"rules.yaml": `runtime: [go]
modules:
  app: ["src/**"]
contracts:
  app:
    invariants:
      - kind: naming
        files: "src/**/*.go"
        case: snake_case
`,
		"go.mod":           graphRepoGoMod,
		"src/good_name.go": "package src\n",
		"src/BadName.go":   "package src\n",
	})
	res := check(t, rs)
	if len(res.Violations) != 1 || res.Violations[0].Path != "src/BadName.go" {
		t.Fatalf("violations: %+v", res.Violations)
	}
}

func TestStructureInvariant(t *testing.T) {
	rs := writeRepo(t, map[string]string{
		"rules.yaml": `runtime: [go]
modules:
  app: ["**"]
contracts:
  app:
    invariants:
      - kind: structure
        require: ["cmd/**"]
        forbid: ["**/*.tmp"]
`,
		"go.mod":  graphRepoGoMod,
		"x/y.tmp": "junk",
		"main.go": "package main\n\nfunc main() {}\n",
	})
	res := check(t, rs)
	if len(res.Violations) != 2 {
		t.Fatalf("violations: %+v", res.Violations)
	}
	if res.Violations[0].Path != "cmd" || !strings.Contains(res.Violations[0].Message, "missing a required file") {
		t.Errorf("require violation: %+v", res.Violations[0])
	}
	if res.Violations[1].Path != "x/y.tmp" {
		t.Errorf("forbid violation: %+v", res.Violations[1])
	}
}

func TestContentInvariant(t *testing.T) {
	rs := writeRepo(t, map[string]string{
		"rules.yaml": `runtime: [go]
modules:
  app: ["**"]
contracts:
  app:
    invariants:
      - kind: content
        files: "**/*.go"
        must_not: ['TODO']
`,
		"go.mod":  graphRepoGoMod,
		"main.go": "package main\n\n// TODO: remove\nfunc main() {}\n",
	})
	res := check(t, rs)
	if len(res.Violations) != 1 {
		t.Fatalf("violations: %+v", res.Violations)
	}
	v := res.Violations[0]
	if v.Path != "main.go" || v.Line == nil || *v.Line != 3 {
		t.Errorf("anchor: %+v", v)
	}
}

func TestExprInvariant(t *testing.T) {
	rs := writeRepo(t, map[string]string{
		"rules.yaml": `runtime: [go]
modules:
  app: ["**"]
contracts:
  app:
    invariants:
      - kind: expr
        files: "**/*.go"
        assert: 'file.lines <= 4 && !("os" in file.imports)'
        message: "files stay tiny and avoid os"
`,
		"go.mod":   graphRepoGoMod,
		"small.go": "package app\n",
		"big.go":   "package app\n\nimport _ \"os\"\n\nvar a = 1\n\nvar b = 2\n",
	})
	res := check(t, rs)
	if len(res.Violations) != 1 || res.Violations[0].Path != "big.go" {
		t.Fatalf("violations: %+v", res.Violations)
	}
	if res.Violations[0].Message != "files stay tiny and avoid os" {
		t.Errorf("message: %s", res.Violations[0].Message)
	}
}

func TestTestdataExcludedByDefaultIncludableByConfig(t *testing.T) {
	files := map[string]string{
		"rules.yaml": `runtime: [go]
modules:
  entities: ["internal/entities/**"]
contracts:
  entities:
    consumes: { external: forbid }
`,
		"go.mod":                                "module example.com/app\n\ngo 1.24\n\nrequire github.com/pkg/errors v0.9.1\n",
		"internal/entities/testdata/fixture.go": "package entities\n\nimport _ \"github.com/pkg/errors\"\n",
	}
	rs := writeRepo(t, files)
	res := check(t, rs)
	if len(res.Violations) != 0 {
		t.Fatalf("testdata should be excluded by default: %+v", res.Violations)
	}

	files["rules.yaml"] = `runtime: [go]
scan:
  include_testdata: true
modules:
  entities: ["internal/entities/**"]
contracts:
  entities:
    consumes: { external: forbid }
`
	rs = writeRepo(t, files)
	res = check(t, rs)
	if len(res.Violations) != 1 {
		t.Fatalf("include_testdata should scan testdata: %+v", res.Violations)
	}
}
