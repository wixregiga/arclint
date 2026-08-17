package yamlrule_test

import (
	"os"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
	yamlrule "github.com/wixregiga/arclint/internal/infrastructure/rule/yaml"
)

// TestLoadTargetRuleset proves the loader against the real target
// ruleset of this repository.
func TestLoadTargetRuleset(t *testing.T) {
	repo, err := yamlrule.NewRepository("../../../../rules.yaml")
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	cfg, err := repo.ConfiguredRules()
	if err != nil {
		t.Fatalf("ConfiguredRules: %v", err)
	}
	if len(cfg.Modules) != 8 {
		t.Errorf("modules = %d, want 8", len(cfg.Modules))
	}
	if len(cfg.Rules) != 18 {
		t.Errorf("rules = %d, want 18", len(cfg.Rules))
	}
	if len(cfg.Languages) != 2 || cfg.Languages[0] != rule.LanguageGo || cfg.Languages[1] != rule.LanguageTypeScript {
		t.Errorf("languages = %v, want [go typescript]", cfg.Languages)
	}
	if cfg.Scan.UnknownImports != rule.UnknownImportsError {
		t.Errorf("unknown imports policy = %q, want error", cfg.Scan.UnknownImports)
	}
	byID := map[string]rule.Rule{}
	for _, r := range cfg.Rules {
		byID[r.ID().Qualified()] = r
	}
	stdlibOnly, ok := byID["arclint:domain/stdlib-only"]
	if !ok {
		t.Fatalf("missing arclint:domain/stdlib-only; have %v", keys(byID))
	}
	if stdlibOnly.Type() != rule.TypeConsumes {
		t.Errorf("stdlib-only type = %q", stdlibOnly.Type())
	}
	if !strings.Contains(stdlibOnly.Claim().Statement(), "no other declared Module") {
		t.Errorf("stdlib-only claim = %q", stdlibOnly.Claim())
	}
	if acyclic, ok := byID["arclint:dependencies/acyclic"]; !ok {
		t.Errorf("missing arclint:dependencies/acyclic")
	} else if acyclic.Type() != rule.TypeAcyclic {
		t.Errorf("acyclic type = %q", acyclic.Type())
	}
	if table, ok := byID["arclint:infrastructure/stdlib-table-present"]; !ok {
		t.Errorf("missing arclint:infrastructure/stdlib-table-present")
	} else if table.Type() != rule.TypeStructure {
		t.Errorf("stdlib-table rule type = %q, want structure", table.Type())
	}
}

func keys(m map[string]rule.Rule) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func loadString(t *testing.T, content string) (yamlrule.Document, error) {
	t.Helper()
	return yamlrule.Load([]byte(content), "test.yaml")
}

func TestLoadRejectsInvalidDocuments(t *testing.T) {
	cases := map[string]string{
		"missing id": `
modules:
  m:
    paths: ["m/**"]
contracts:
  m:
    consumes:
      internal: []
`,
		"unknown invariant kind": `
modules:
  m:
    paths: ["m/**"]
contracts:
  m:
    invariants:
      - id: "t:m/content"
        kind: content
        files: "m/**"
`,
		"scrapped expr kind": `
modules:
  m:
    paths: ["m/**"]
contracts:
  m:
    invariants:
      - id: "t:m/depth"
        kind: expr
        files: "m/**"
`,
		"unknown dependency kind": `
modules:
  m:
    paths: ["m/**"]
dependencies:
  - id: "t:deps/forbidden"
    kind: forbidden
`,
		"unknown top-level key": `
rulesets: []
`,
		"contract for undeclared module": `
contracts:
  ghost:
    consumes:
      id: "t:ghost/imports"
      internal: []
`,
		"duplicate rule id": `
modules:
  m:
    paths: ["m/**"]
  n:
    paths: ["n/**"]
contracts:
  m:
    consumes:
      id: "t:same"
      internal: []
  n:
    consumes:
      id: "t:same"
      internal: []
`,
	}
	for name, content := range cases {
		if _, err := loadString(t, content); err == nil {
			t.Errorf("%s: expected a load error", name)
		}
	}
}

func TestPatternHeaderIsNotARepositoryRuleset(t *testing.T) {
	dir := t.TempDir()
	file := dir + "/rules.yaml"
	content := `
pattern:
  namespace: arclint
  name: sample
  version: 1.0.0
modules:
  m:
    paths: ["m/**"]
contracts:
  m:
    consumes:
      id: "t:m/imports"
      internal: []
`
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	repo, err := yamlrule.NewRepository(file)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	if _, err := repo.ConfiguredRules(); err == nil {
		t.Errorf("a pattern distribution file must not load as a repository ruleset")
	}
	doc, err := yamlrule.Load([]byte(content), file)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if doc.Pattern == nil || doc.Pattern.Namespace != "arclint" || doc.Pattern.Version != "1.0.0" {
		t.Errorf("pattern header = %+v", doc.Pattern)
	}
}

func TestDiscoverPath(t *testing.T) {
	root := t.TempDir()
	nested := root + "/a/b"
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	target := root + "/rules.yaml"
	if err := os.WriteFile(target, []byte("runtime: [go]\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	found, err := yamlrule.DiscoverPath(nested, "rules.yaml")
	if err != nil {
		t.Fatalf("DiscoverPath: %v", err)
	}
	if found != target {
		t.Errorf("DiscoverPath = %q, want %q", found, target)
	}
	if _, err := yamlrule.DiscoverPath(t.TempDir(), "rules.yaml"); err == nil {
		t.Errorf("expected discovery failure in an empty tree")
	}
}
