package yamlrule_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
	yamlrule "github.com/wixregiga/arclint/internal/infrastructure/rule/yaml"
)

// installationFixture drafts the Installation of a Pattern with one
// Module suggesting a path and one left unbound.
func installationFixture(t *testing.T, version string) rule.Installation {
	t.Helper()
	scope, err := rule.ModuleApplicability([]rule.ModuleName{"domain"})
	if err != nil {
		t.Fatalf("ModuleApplicability: %v", err)
	}
	internal := rule.AllowList{}
	r, err := rule.New(rule.Spec{
		ID:            "acme:domain/stdlib-only",
		Type:          rule.TypeConsumes,
		Params:        rule.ConsumesParams{Internal: &internal, External: rule.ImportForbid},
		Applicability: scope,
	})
	if err != nil {
		t.Fatalf("rule.New: %v", err)
	}
	glob, err := rule.NewGlob("src/domain/**")
	if err != nil {
		t.Fatalf("NewGlob: %v", err)
	}
	domain, err := rule.NewPatternModule("domain", "The domain model.", []rule.Glob{glob})
	if err != nil {
		t.Fatalf("NewPatternModule: %v", err)
	}
	app, err := rule.NewPatternModule("app", "The application layer.", nil)
	if err != nil {
		t.Fatalf("NewPatternModule: %v", err)
	}
	p, err := rule.NewPattern(rule.PatternSpec{
		Namespace: "acme",
		Name:      "layers",
		Version:   version,
		Coverage:  []rule.Language{rule.LanguageGo},
		Modules:   []rule.PatternModule{domain, app},
		Rules:     []rule.Rule{r},
	})
	if err != nil {
		t.Fatalf("NewPattern: %v", err)
	}
	inst, err := rule.NewInstallation(p)
	if err != nil {
		t.Fatalf("NewInstallation: %v", err)
	}
	return inst
}

func writeRuleset(t *testing.T, content string) yamlrule.Editor {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rules.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	editor, err := yamlrule.NewEditor(path)
	if err != nil {
		t.Fatalf("NewEditor: %v", err)
	}
	return editor
}

func readRuleset(t *testing.T, editor yamlrule.Editor) string {
	t.Helper()
	data, err := os.ReadFile(editor.Path())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(data)
}

func TestExtendInsertsAnExtendsSectionBeforeModules(t *testing.T) {
	editor := writeRuleset(t, `# ArcLint architecture contracts.
runtime: [go]

scan:
  unknown_imports: warn

# Modules of this repository.
modules:
  web: "cmd/web/**"

rules:
  web/stdlib-only:
    description: "Web imports nothing."
    on: web
    imports:
      internal: []
`)
	inst := installationFixture(t, "1.0.0")
	change, err := editor.Extend(inst)
	if err != nil {
		t.Fatalf("Extend: %v", err)
	}
	if change.Replaced != "" || len(change.Adopted) != 0 || change.Path != editor.Path() {
		t.Errorf("change = %+v", change)
	}
	if len(change.Installation.Bindings()) != 1 || len(change.Installation.Unbound()) != 1 {
		t.Errorf("installation as written = %+v", change.Installation)
	}
	want := `# ArcLint architecture contracts.
runtime: [go]

scan:
  unknown_imports: warn

extends:
  - pattern: acme/layers@1.0.0
    bind:
      domain: "src/domain/**"
      # The application layer.
      # app: <glob>

# Modules of this repository.
modules:
  web: "cmd/web/**"

rules:
  web/stdlib-only:
    description: "Web imports nothing."
    on: web
    imports:
      internal: []
`
	if got := readRuleset(t, editor); got != want {
		t.Errorf("ruleset after extend:\n%s\nwant:\n%s", got, want)
	}
}

func TestExtendAppendsToAnExistingList(t *testing.T) {
	editor := writeRuleset(t, `runtime: [go]
extends:
  - pattern: "arclint/vertical@0.1.0"
    bind:
      domain: internal/*/domain/**
      application: internal/*/application/**
      infra: internal/*/infra/**
      app: internal/app/**
      shared: internal/shared/**
      composition: cmd/**

  # House modules follow.
modules:
  tools: "tools/**"
`)
	inst := installationFixture(t, "1.0.0")
	if _, err := editor.Extend(inst); err != nil {
		t.Fatalf("Extend: %v", err)
	}
	got := readRuleset(t, editor)
	want := `      composition: cmd/**
  - pattern: acme/layers@1.0.0
    bind:
      domain: "src/domain/**"
      # The application layer.
      # app: <glob>

  # House modules follow.
modules:
`
	if !strings.Contains(got, want) {
		t.Errorf("new entry must follow the last entry, before the trailing comment:\n%s", got)
	}
}

func TestExtendReplacesTheVersionAndKeepsBindings(t *testing.T) {
	editor := writeRuleset(t, `runtime: [go]
extends:
  - pattern: 'acme/layers@0.9.0'  # pinned
    bind:
      domain: [internal/domain/**, pkg/domain/**]
      app: internal/app/**
modules:
  web: "cmd/web/**"
`)
	inst := installationFixture(t, "1.0.0")
	change, err := editor.Extend(inst)
	if err != nil {
		t.Fatalf("Extend: %v", err)
	}
	if change.Replaced != "0.9.0" {
		t.Errorf("replaced = %q, want 0.9.0", change.Replaced)
	}
	bindings := change.Installation.Bindings()
	if len(bindings) != 2 || len(bindings[0].Paths()) != 2 || bindings[1].Paths()[0].String() != "internal/app/**" {
		t.Errorf("existing bindings must be reported as written: %+v", bindings)
	}
	if len(change.Installation.Unbound()) != 0 {
		t.Errorf("no module is unbound once the entry binds both: %+v", change.Installation.Unbound())
	}
	got := readRuleset(t, editor)
	if !strings.Contains(got, "  - pattern: 'acme/layers@1.0.0'  # pinned\n    bind:\n      domain: [internal/domain/**, pkg/domain/**]\n      app: internal/app/**\nmodules:\n") {
		t.Errorf("the version must change in place with its quoting and comment:\n%s", got)
	}
	again, err := editor.Extend(inst)
	if err != nil {
		t.Fatalf("Extend again: %v", err)
	}
	if again.Replaced != "" || readRuleset(t, editor) != got {
		t.Errorf("extending the same version again must change nothing: %+v", again)
	}
}

func TestExtendFoldsDeclaredModulesIntoBindings(t *testing.T) {
	editor := writeRuleset(t, `runtime: [go]

modules:
  # The domain model of this repository.
  domain:
    paths: internal/domain/**
    description: "Domain."
  # Application services.
  app: "internal/app/**"
  web: "cmd/web/**"

rules:
  domain/stdlib-only:
    description: "Domain imports nothing."
    on: domain
    imports:
      internal: []
`)
	inst := installationFixture(t, "1.0.0")
	change, err := editor.Extend(inst)
	if err != nil {
		t.Fatalf("Extend: %v", err)
	}
	if len(change.Adopted) != 2 || change.Adopted[0] != "domain" || change.Adopted[1] != "app" {
		t.Errorf("adopted = %v, want domain and app", change.Adopted)
	}
	bindings := change.Installation.Bindings()
	if len(bindings) != 2 || bindings[0].Paths()[0].String() != "internal/domain/**" || bindings[1].Paths()[0].String() != "internal/app/**" {
		t.Errorf("declared paths must win over the suggestion: %+v", bindings)
	}
	want := `runtime: [go]

extends:
  - pattern: acme/layers@1.0.0
    bind:
      domain: "internal/domain/**"
      app: "internal/app/**"

modules:
  web: "cmd/web/**"

rules:
  domain/stdlib-only:
    description: "Domain imports nothing."
    on: domain
    imports:
      internal: []
`
	if got := readRuleset(t, editor); got != want {
		t.Errorf("ruleset after extend:\n%s\nwant:\n%s", got, want)
	}
}

func TestExtendRemovesAnEmptiedModulesSection(t *testing.T) {
	editor := writeRuleset(t, `runtime: [go]

# Declared modules.
modules:
  domain: "internal/domain/**"

rules:
  domain/stdlib-only:
    description: "Domain imports nothing."
    on: domain
    imports:
      internal: []
`)
	inst := installationFixture(t, "1.0.0")
	if _, err := editor.Extend(inst); err != nil {
		t.Fatalf("Extend: %v", err)
	}
	want := `runtime: [go]

extends:
  - pattern: acme/layers@1.0.0
    bind:
      domain: "internal/domain/**"
      # The application layer.
      # app: <glob>

rules:
  domain/stdlib-only:
    description: "Domain imports nothing."
    on: domain
    imports:
      internal: []
`
	if got := readRuleset(t, editor); got != want {
		t.Errorf("ruleset after extend:\n%s\nwant:\n%s", got, want)
	}
}

func TestExtendHandlesEmptyAndFlowExtends(t *testing.T) {
	editor := writeRuleset(t, "runtime: [go]\nextends: [] # none yet\nrules: {}\n")
	inst := installationFixture(t, "1.0.0")
	if _, err := editor.Extend(inst); err != nil {
		t.Fatalf("Extend: %v", err)
	}
	if got := readRuleset(t, editor); !strings.HasPrefix(got, "runtime: [go]\nextends: # none yet\n  - pattern: acme/layers@1.0.0\n    bind:\n") {
		t.Errorf("an empty flow list becomes a block list:\n%s", got)
	}

	editor = writeRuleset(t, "runtime: [go]\nextends:\nrules: {}\n")
	if _, err := editor.Extend(inst); err == nil || !strings.Contains(err.Error(), "expected a list") {
		t.Errorf("a null extends is not a valid ruleset and is not edited, got %v", err)
	}

	editor = writeRuleset(t, "runtime: [go]\nextends: [{pattern: arclint/vertical@0.1.0}]\nrules: {}\n")
	if _, err := editor.Extend(inst); err == nil || !strings.Contains(err.Error(), "flow list") {
		t.Errorf("a populated flow list is refused with guidance, got %v", err)
	}
}

func TestExtendRefusesInvalidAndPatternFiles(t *testing.T) {
	inst := installationFixture(t, "1.0.0")
	editor := writeRuleset(t, "pattern:\n  namespace: acme\n  name: x\n  version: 1.0.0\nmodules:\n  m: \"M.\"\nrules:\n  m/r:\n    on: m\n    imports:\n      internal: []\n")
	if _, err := editor.Extend(inst); err == nil || !strings.Contains(err.Error(), "pattern distribution file") {
		t.Errorf("a pattern file is not a ruleset, got %v", err)
	}
	editor = writeRuleset(t, "runtime: [go]\nbogus: 1\n")
	if _, err := editor.Extend(inst); err == nil {
		t.Error("an invalid ruleset must not be edited")
	}
	missing, err := yamlrule.NewEditor(filepath.Join(t.TempDir(), "rules.yaml"))
	if err != nil {
		t.Fatalf("NewEditor: %v", err)
	}
	if exists, err := missing.Exists(); err != nil || exists {
		t.Errorf("Exists = %v, %v", exists, err)
	}
	if _, err := missing.Extend(inst); err == nil {
		t.Error("a missing ruleset cannot be extended")
	}
}
