package filesystemscaffold_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
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
	repo, err := yamlrule.NewRepository(path, nil, embeddedpattern.NewSource())
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	cfg, err := repo.ConfiguredRules()
	if err != nil {
		t.Fatalf("the starter ruleset must load through the strict loader: %v", err)
	}
	if len(cfg.Zones) != 1 || string(cfg.Zones[0].Name()) != "source" {
		t.Errorf("zones = %+v, want the source zone", cfg.Zones)
	}
	if len(cfg.Rules) != 1 || cfg.Rules[0].ID().Qualified() != "source/dependencies" {
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
	repo, err := yamlrule.NewRepository(path, nil, embeddedpattern.NewSource())
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	cfg, err := repo.ConfiguredRules()
	if err != nil {
		t.Fatalf("the vertical ruleset must load through the strict loader: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".arclint", "extensions")); !os.IsNotExist(err) {
		t.Errorf("adopting a pattern must not copy its extensions into the repository, stat err = %v", err)
	}
	if len(cfg.Extensions) != 5 {
		t.Errorf("configured extensions = %d, want the five the pattern distributes", len(cfg.Extensions))
	}
	got := map[string]bool{}
	for _, m := range cfg.Zones {
		got[string(m.Name())] = true
	}
	for _, name := range []string{"domain", "application", "infra", "app", "shared", "composition"} {
		if !got[name] {
			t.Errorf("missing zone %q", name)
		}
	}
	if len(cfg.Zones) != 6 {
		t.Errorf("zones = %d, want 6", len(cfg.Zones))
	}
	if len(cfg.Rules) != 16 {
		t.Errorf("rules = %d, want 16", len(cfg.Rules))
	}
	var sawIndependence bool
	for _, r := range cfg.Rules {
		if r.ID().Qualified() == "arclint/vertical:features/independent" {
			sawIndependence = true
			if r.Type() != "independence" {
				t.Errorf("arclint/vertical:features/independent type = %q", r.Type())
			}
		}
	}
	if !sawIndependence {
		t.Errorf("missing arclint/vertical:features/independent")
	}
	if len(cfg.Languages) != 2 {
		t.Errorf("languages = %v, want go and typescript", cfg.Languages)
	}
}

// TestStarterRulesetNamesItsSchema proves the drafted file lands under
// the published ruleset name and opens with an editor modeline: the
// published $id when the project holds no schema copy, the project's
// copy under .arclint/schemas once one exists.
func TestStarterRulesetNamesItsSchema(t *testing.T) {
	dir := t.TempDir()
	writer, err := filesystemscaffold.NewWriter(dir)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	path, err := writer.Write("runtime: [go]\n", false)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if filepath.Base(path) != rule.RulesetFileName {
		t.Fatalf("path = %q, want base %q", path, rule.RulesetFileName)
	}
	remote, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if want := "# yaml-language-server: $schema=" + rule.SchemaID + "\nruntime: [go]\n"; string(remote) != want {
		t.Fatalf("fresh draft =\n%s\nwant\n%s", remote, want)
	}

	local := filepath.Join(dir, filepath.FromSlash(application.SchemaDirectory), rule.SchemaFileName)
	if err := os.MkdirAll(filepath.Dir(local), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(local, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile schema: %v", err)
	}
	if _, err := writer.Write("runtime: [go]\n", true); err != nil {
		t.Fatalf("forced Write: %v", err)
	}
	project, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if want := "# yaml-language-server: $schema=" + application.SchemaDirectory + "/" + rule.SchemaFileName + "\nruntime: [go]\n"; string(project) != want {
		t.Fatalf("draft beside a project schema =\n%s\nwant\n%s", project, want)
	}
}

// TestDraftedRulesetNamesItsDomainFile proves the drafted domain key
// against the loader that reads it: the file init was told to use is the
// DomainRecording the drafted ruleset names, for the starter and for an
// adopting ruleset, in either format.
func TestDraftedRulesetNamesItsDomainFile(t *testing.T) {
	for _, tc := range []struct {
		pattern string
		domain  string
		want    string
		format  vocab.DomainRecordingFormat
	}{
		{"", "docs/model.arclint.yaml", "docs/model.arclint.yaml", vocab.DomainRecordingYAML},
		{"", "./docs/../model.cml", "model.cml", vocab.DomainRecordingCML},
		{"", "docs/my model (v2).yml", "docs/my model (v2).yml", vocab.DomainRecordingYAML},
		{"vertical", "docs/model.arclint.yaml", "docs/model.arclint.yaml", vocab.DomainRecordingYAML},
		{"", "domain.arclint.yaml", vocab.DefaultDomainRecordingPath, vocab.DomainRecordingYAML},
		{"", "", vocab.DefaultDomainRecordingPath, vocab.DomainRecordingYAML},
	} {
		dir := t.TempDir()
		writer, err := filesystemscaffold.NewWriter(dir)
		if err != nil {
			t.Fatalf("NewWriter: %v", err)
		}
		initialize, err := application.NewInitializeRepository(writer, embeddedpattern.NewSource())
		if err != nil {
			t.Fatalf("NewInitializeRepository: %v", err)
		}
		path, err := initialize.Execute(application.InitializeRepositoryRequest{Pattern: tc.pattern, Domain: tc.domain})
		if err != nil {
			t.Fatalf("%q: Execute: %v", tc.domain, err)
		}
		named, err := yamlrule.ReadRecording(path)
		if err != nil {
			t.Fatalf("%q: ReadRecording: %v", tc.domain, err)
		}
		// Explicit choices are named even when discovery would currently
		// choose the same file. Only an omitted choice permits discovery.
		if named.IsZero() != (tc.domain == "") {
			t.Errorf("%q: the drafted ruleset names %q, want every explicit choice named", tc.domain, named.Path())
		}
		recording, _, err := vocab.ResolveDomainRecording(named, func(vocab.DomainRecording) (bool, error) { return false, nil })
		if err != nil {
			t.Fatalf("%q: ResolveDomainRecording: %v", tc.domain, err)
		}
		if recording.Path() != tc.want || recording.Format() != tc.format {
			t.Errorf("%q: recording = %s (%s), want %s (%s)", tc.domain, recording.Path(), recording.Format(), tc.want, tc.format)
		}
		if tc.pattern != "" {
			// An adopting ruleset leaves the Pattern's Zones for the owner
			// to bind, so only the starter loads whole.
			continue
		}
		repo, err := yamlrule.NewRepository(path, nil, embeddedpattern.NewSource())
		if err != nil {
			t.Fatalf("NewRepository: %v", err)
		}
		if _, err := repo.ConfiguredRules(); err != nil {
			t.Errorf("%q: the drafted ruleset must load through the strict loader: %v", tc.domain, err)
		}
	}
}
