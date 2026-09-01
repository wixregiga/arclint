package embeddedpattern_test

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
	embeddedpattern "github.com/wixregiga/arclint/internal/infrastructure/pattern/embedded"
	yamlrule "github.com/wixregiga/arclint/internal/infrastructure/rule/yaml"
)

func TestVerticalPatternLoads(t *testing.T) {
	source := embeddedpattern.NewSource()
	patterns, err := source.Patterns()
	if err != nil {
		t.Fatalf("Patterns: %v", err)
	}
	if len(patterns) != 1 {
		t.Fatalf("patterns = %d, want 1", len(patterns))
	}
	p := patterns[0]
	if p.Reference().String() != "arclint/vertical@0.1.0" {
		t.Errorf("reference = %q, want arclint/vertical@0.1.0", p.Reference())
	}
	if len(p.Rules()) != 16 {
		t.Errorf("rules = %d, want 16", len(p.Rules()))
	}
	if len(p.Modules()) != 6 {
		t.Errorf("modules = %d, want 6", len(p.Modules()))
	}
	if len(p.Tests()) != 1 {
		t.Errorf("tests = %d, want 1", len(p.Tests()))
	}
	if p.Digest() == "" {
		t.Error("full-tree digest is empty")
	}
	var sawIndependence bool
	wantUses := map[string]string{
		"vertical:domain/no-context":              "vertical/forbid-imports",
		"vertical:domain/no-io":                   "vertical/forbid-imports",
		"vertical:application/repository-context": "vertical/repository-context",
		"vertical:application/usecase-contract":   "vertical/usecase",
		"vertical:shared/concerns":                "vertical/shared-concerns",
		"vertical:repositories/application-only":  "vertical/repository-location",
	}
	for _, r := range p.Rules() {
		id := r.ID().Qualified()
		if id == "vertical:features/independent" {
			sawIndependence = true
			if r.Type() != rule.TypeIndependence {
				t.Errorf("vertical:features/independent type = %q, want independence", r.Type())
			}
		}
		if uses, ok := wantUses[id]; ok {
			if r.Type() != rule.TypeExtension {
				t.Errorf("%s type = %q, want extension", id, r.Type())
			}
			params, ok := r.Params().(rule.ExtensionParams)
			if !ok || params.Uses != uses {
				t.Errorf("%s uses = %v, want %q", id, r.Params(), uses)
			}
			delete(wantUses, id)
		}
	}
	if !sawIndependence {
		t.Errorf("missing vertical:features/independent")
	}
	if len(wantUses) != 0 {
		t.Errorf("missing extension rules: %v", wantUses)
	}
	scaffold, ok := source.Scaffold("vertical")
	if !ok {
		t.Fatal("Scaffold(vertical) missing")
	}
	if !strings.Contains(scaffold.Ruleset, "runtime: [go]") {
		t.Errorf("Scaffold text lacks runtime: [go] marker")
	}
}

func TestVerticalInitProjectionMatchesManifestRules(t *testing.T) {
	source := embeddedpattern.NewSource()
	patterns, err := source.Patterns()
	if err != nil {
		t.Fatalf("Patterns: %v", err)
	}
	scaffold, ok := source.Scaffold("vertical")
	if !ok {
		t.Fatal("Scaffold(vertical) missing")
	}
	doc, err := yamlrule.Load([]byte(scaffold.Ruleset), "vertical/rules.yaml", vocab.UbiquitousLanguage{})
	if err != nil {
		t.Fatalf("load init projection: %v", err)
	}
	manifestIDs := ruleIDs(patterns[0].Rules())
	projectionIDs := ruleIDs(doc.Configured.Rules)
	if strings.Join(manifestIDs, "\n") != strings.Join(projectionIDs, "\n") {
		t.Errorf("init rules.yaml drifted from pattern.yaml\nmanifest: %v\nprojection: %v", manifestIDs, projectionIDs)
	}
	if len(scaffold.Extensions) != len(patterns[0].Extensions()) {
		t.Errorf("init extensions = %d, manifest extensions = %d", len(scaffold.Extensions), len(patterns[0].Extensions()))
	}
	manifestModules := patterns[0].Modules()
	projectionModules := append([]rule.Module(nil), doc.Configured.Modules...)
	sort.Slice(manifestModules, func(i, j int) bool { return manifestModules[i].Name() < manifestModules[j].Name() })
	sort.Slice(projectionModules, func(i, j int) bool { return projectionModules[i].Name() < projectionModules[j].Name() })
	if !reflect.DeepEqual(manifestModules, projectionModules) {
		t.Errorf("init Modules drifted from pattern.yaml")
	}
	if !reflect.DeepEqual(patterns[0].Coverage(), doc.Configured.Languages) {
		t.Errorf("init runtime = %v, manifest coverage = %v", doc.Configured.Languages, patterns[0].Coverage())
	}
	manifestRules := patterns[0].Rules()
	projectionByID := make(map[string]rule.Rule, len(doc.Configured.Rules))
	for _, projected := range doc.Configured.Rules {
		projectionByID[projected.ID().Qualified()] = projected
	}
	for _, manifested := range manifestRules {
		projected, ok := projectionByID[manifested.ID().Qualified()]
		if !ok {
			continue
		}
		if manifested.Type() != projected.Type() ||
			manifested.Claim() != projected.Claim() ||
			manifested.Severity() != projected.Severity() ||
			!reflect.DeepEqual(manifested.Applicability(), projected.Applicability()) ||
			!reflect.DeepEqual(manifested.Params(), projected.Params()) {
			t.Errorf("init Rule %s drifted from pattern.yaml: manifest claim=%q app=%#v params=%#v; projection claim=%q app=%#v params=%#v", manifested.ID(), manifested.Claim(), manifested.Applicability(), manifested.Params(), projected.Claim(), projected.Applicability(), projected.Params())
		}
	}
}

func ruleIDs(rules []rule.Rule) []string {
	ids := make([]string, 0, len(rules))
	for _, r := range rules {
		ids = append(ids, r.ID().Qualified())
	}
	sort.Strings(ids)
	return ids
}

func TestVerticalPatternCarriesExtensions(t *testing.T) {
	source := embeddedpattern.NewSource()
	patterns, err := source.Patterns()
	if err != nil {
		t.Fatalf("Patterns: %v", err)
	}
	if len(patterns) != 1 {
		t.Fatalf("patterns = %d, want 1", len(patterns))
	}
	exts := patterns[0].Extensions()
	want := []string{
		"vertical_forbid_imports.ts",
		"vertical_repository_context.ts",
		"vertical_repository_location.ts",
		"vertical_shared_concerns.ts",
		"vertical_usecase.ts",
	}
	if len(exts) != len(want) {
		t.Fatalf("extensions = %d, want %d", len(exts), len(want))
	}
	for i, e := range exts {
		if e.FileName() != want[i] {
			t.Errorf("extensions[%d] = %q, want %q", i, e.FileName(), want[i])
		}
		if strings.TrimSpace(e.Source()) == "" {
			t.Errorf("%s: blank source", e.FileName())
		}
	}
	if !strings.Contains(exts[0].Source(), `type: "vertical/forbid-imports"`) {
		t.Errorf("forbid-imports source was not preserved")
	}
}
