package filesystemscaffold_test

import (
	"os"
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
	if len(cfg.Rules) != 10 {
		t.Errorf("rules = %d, want 10", len(cfg.Rules))
	}
	var sawIndependence bool
	for _, r := range cfg.Rules {
		if r.ID().Qualified() == "repo:features/independent" {
			sawIndependence = true
			if r.Type() != "independence" {
				t.Errorf("repo:features/independent type = %q", r.Type())
			}
		}
	}
	if !sawIndependence {
		t.Errorf("missing repo:features/independent")
	}
	if len(cfg.Languages) != 2 {
		t.Errorf("languages = %v, want go and typescript", cfg.Languages)
	}
}
