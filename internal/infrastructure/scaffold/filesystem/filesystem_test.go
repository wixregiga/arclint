package filesystemscaffold_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	embeddedpattern "github.com/wixregiga/arclint/internal/infrastructure/pattern/embedded"
	yamlrule "github.com/wixregiga/arclint/internal/infrastructure/rule/yaml"
	filesystemscaffold "github.com/wixregiga/arclint/internal/infrastructure/scaffold/filesystem"
)

// TestStarterRulesetRoundTrips proves init's draft against the same
// strict loader that governs every ruleset: the generated file must
// load into valid Rules, not merely look plausible.
func TestStarterRulesetRoundTrips(t *testing.T) {
	dir := t.TempDir()
	writer, err := filesystemscaffold.NewWriter(dir)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	initialize, err := application.NewInitializeRepository(writer, embeddedpattern.NewSource())
	if err != nil {
		t.Fatalf("NewInitializeRepository: %v", err)
	}
	path, err := initialize.Execute(application.InitializeRepositoryRequest{Languages: []string{"go", "ts"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	repo, err := yamlrule.NewRepository(path)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	cfg, err := repo.ConfiguredRules()
	if err != nil {
		t.Fatalf("the starter ruleset must load through the strict loader: %v", err)
	}
	if len(cfg.Modules) != 1 || string(cfg.Modules[0].Name()) != "source" {
		t.Errorf("modules = %+v, want the source module", cfg.Modules)
	}
	if len(cfg.Rules) != 1 || cfg.Rules[0].ID().Qualified() != "repo:source/dependencies" {
		t.Errorf("rules = %d, want the starter consumes rule", len(cfg.Rules))
	}
	if len(cfg.Languages) != 2 {
		t.Errorf("languages = %v, want go and typescript", cfg.Languages)
	}
	if _, err := os.Stat(filepath.Join(dir, ".arclint", "extensions")); !os.IsNotExist(err) {
		t.Errorf("bare init must not write .arclint/extensions, stat err = %v", err)
	}

	if _, err := initialize.Execute(application.InitializeRepositoryRequest{}); err == nil {
		t.Errorf("a second init must refuse to overwrite without force")
	}
	if _, err := initialize.Execute(application.InitializeRepositoryRequest{Force: true}); err != nil {
		t.Errorf("forced init must overwrite: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("ruleset missing after forced init: %v", err)
	}
}

func TestVerticalRulesetRoundTrips(t *testing.T) {
	dir := t.TempDir()
	writer, err := filesystemscaffold.NewWriter(dir)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	initialize, err := application.NewInitializeRepository(writer, embeddedpattern.NewSource())
	if err != nil {
		t.Fatalf("NewInitializeRepository: %v", err)
	}
	path, err := initialize.Execute(application.InitializeRepositoryRequest{
		Languages: []string{"go", "ts"},
		Pattern:   "vertical",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	repo, err := yamlrule.NewRepository(path)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	cfg, err := repo.ConfiguredRules()
	if err != nil {
		t.Fatalf("the vertical ruleset must load through the strict loader: %v", err)
	}
	got := map[string]bool{}
	for _, m := range cfg.Modules {
		got[string(m.Name())] = true
	}
	for _, name := range []string{"domain", "application", "infra", "app", "shared", "composition"} {
		if !got[name] {
			t.Errorf("missing module %q", name)
		}
	}
	if len(cfg.Modules) != 6 {
		t.Errorf("modules = %d, want 6", len(cfg.Modules))
	}
	if len(cfg.Rules) != 16 {
		t.Errorf("rules = %d, want 16", len(cfg.Rules))
	}
	var sawIndependence bool
	for _, r := range cfg.Rules {
		if r.ID().Qualified() == "vertical:features/independent" {
			sawIndependence = true
			if r.Type() != "independence" {
				t.Errorf("vertical:features/independent type = %q", r.Type())
			}
		}
	}
	if !sawIndependence {
		t.Errorf("missing vertical:features/independent")
	}
	if len(cfg.Languages) != 2 {
		t.Errorf("languages = %v, want go and typescript", cfg.Languages)
	}
}

func TestVerticalInitWritesExtensions(t *testing.T) {
	dir := t.TempDir()
	writer, err := filesystemscaffold.NewWriter(dir)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	initialize, err := application.NewInitializeRepository(writer, embeddedpattern.NewSource())
	if err != nil {
		t.Fatalf("NewInitializeRepository: %v", err)
	}
	if _, err := initialize.Execute(application.InitializeRepositoryRequest{Pattern: "vertical"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	scaffold, ok := embeddedpattern.NewSource().Scaffold("vertical")
	if !ok {
		t.Fatal("Scaffold(vertical) missing")
	}
	if len(scaffold.Extensions) != 5 {
		t.Fatalf("embedded extensions = %d, want 5", len(scaffold.Extensions))
	}
	for _, e := range scaffold.Extensions {
		got, err := os.ReadFile(filepath.Join(dir, ".arclint", "extensions", e.FileName()))
		if err != nil {
			t.Fatalf("read %s: %v", e.FileName(), err)
		}
		if string(got) != e.Source() {
			t.Errorf("%s bytes do not match embedded source", e.FileName())
		}
	}
}

func TestVerticalInitPreflightBlocksPartialWrite(t *testing.T) {
	dir := t.TempDir()
	writer, err := filesystemscaffold.NewWriter(dir)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	extDir := filepath.Join(dir, ".arclint", "extensions")
	if err := os.MkdirAll(extDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	existing := filepath.Join(extDir, "vertical_usecase.ts")
	if err := os.WriteFile(existing, []byte("stale"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	initialize, err := application.NewInitializeRepository(writer, embeddedpattern.NewSource())
	if err != nil {
		t.Fatalf("NewInitializeRepository: %v", err)
	}
	if _, err := initialize.Execute(application.InitializeRepositoryRequest{Pattern: "vertical"}); err == nil {
		t.Fatal("pre-existing extension must block write without force")
	}
	if _, err := os.Stat(filepath.Join(dir, "rules.yaml")); !os.IsNotExist(err) {
		t.Errorf("rules.yaml must not be written when an extension already exists")
	}
	if _, err := os.Stat(filepath.Join(extDir, "vertical_forbid_imports.ts")); !os.IsNotExist(err) {
		t.Errorf("other extensions must not be written when preflight fails")
	}
	got, err := os.ReadFile(existing)
	if err != nil || string(got) != "stale" {
		t.Errorf("existing extension must be left untouched, got %q err %v", got, err)
	}
}

func TestVerticalInitForceOverwritesExtensions(t *testing.T) {
	dir := t.TempDir()
	writer, err := filesystemscaffold.NewWriter(dir)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	initialize, err := application.NewInitializeRepository(writer, embeddedpattern.NewSource())
	if err != nil {
		t.Fatalf("NewInitializeRepository: %v", err)
	}
	if _, err := initialize.Execute(application.InitializeRepositoryRequest{Pattern: "vertical"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	stale := filepath.Join(dir, ".arclint", "extensions", "vertical_usecase.ts")
	if err := os.WriteFile(stale, []byte("stale"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := initialize.Execute(application.InitializeRepositoryRequest{Pattern: "vertical", Force: true}); err != nil {
		t.Fatalf("forced Execute: %v", err)
	}
	got, err := os.ReadFile(stale)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) == "stale" {
		t.Errorf("force must overwrite extension bytes")
	}
}
