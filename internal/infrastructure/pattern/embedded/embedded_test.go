package embeddedpattern_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/distribution"
	"github.com/wixregiga/arclint/internal/domain/rule"
	embeddedpattern "github.com/wixregiga/arclint/internal/infrastructure/pattern/embedded"
)

// vertical returns the arclint/vertical Pattern from the built-in
// source.
func vertical(t *testing.T) rule.Pattern {
	t.Helper()
	patterns, err := embeddedpattern.NewSource().Patterns()
	if err != nil {
		t.Fatalf("Patterns: %v", err)
	}
	for _, p := range patterns {
		if p.Reference().Name() == "vertical" {
			return p
		}
	}
	t.Fatalf("arclint/vertical is not embedded; got %d patterns", len(patterns))
	return rule.Pattern{}
}

func TestBuiltInPatternsAreAvailableWithDigests(t *testing.T) {
	source := embeddedpattern.NewSource()
	available, err := source.Available()
	if err != nil {
		t.Fatalf("Available: %v", err)
	}
	want := []string{"arclint/domain-model@0.1.0", "arclint/vertical@0.1.0"}
	if len(available) != len(want) {
		t.Fatalf("available = %d, want %d", len(available), len(want))
	}
	for i, a := range available {
		if a.Reference().String() != want[i] {
			t.Errorf("available[%d] = %s, want %s", i, a.Reference(), want[i])
		}
		if a.Kind != distribution.SourceEmbedded || a.Authored {
			t.Errorf("%s: kind %q authored %v; a built-in is an embedded, vendored copy", a.Reference(), a.Kind, a.Authored)
		}
		if a.Digest().IsZero() {
			t.Errorf("%s: no digest", a.Reference())
		}
		if _, ok := a.Vendored.File("pattern.yaml"); !ok {
			t.Errorf("%s: pattern.yaml is not among the shipped files", a.Reference())
		}
		if _, ok := a.Vendored.File("extensions/package.json"); !ok {
			t.Errorf("%s: extensions/package.json must ship so the extension directory type-checks", a.Reference())
		}
	}
	names, err := source.Names()
	if err != nil || len(names) != 2 || names[0] != "domain-model" || names[1] != "vertical" {
		t.Errorf("Names = %v, %v", names, err)
	}
}

func TestDomainModelPatternLoads(t *testing.T) {
	patterns, err := embeddedpattern.NewSource().Patterns()
	if err != nil {
		t.Fatalf("Patterns: %v", err)
	}
	var p rule.Pattern
	for _, candidate := range patterns {
		if candidate.Reference().Name() == "domain-model" {
			p = candidate
		}
	}
	if p.Reference().IsZero() {
		t.Fatal("arclint/domain-model is not embedded")
	}
	wantUses := map[string]string{
		"arclint/domain-model:vocabulary/terms-carry-definitions":         "domain-model/require-defined-terms",
		"arclint/domain-model:vocabulary/invariants-name-recorded-owners": "domain-model/invariants-name-recorded-owners",
		"arclint/domain-model:contexts/respect-relations":                 "domain-model/respect-context-relations",
	}
	if len(p.Rules()) != len(wantUses) {
		t.Errorf("rules = %d, want %d", len(p.Rules()), len(wantUses))
	}
	for _, r := range p.Rules() {
		id := r.ID().Qualified()
		uses, ok := wantUses[id]
		if !ok {
			t.Errorf("unexpected rule %s", id)
			continue
		}
		params, isExt := r.Params().(rule.ExtensionParams)
		if !isExt || params.Uses != uses {
			t.Errorf("%s uses = %v, want %q", id, r.Params(), uses)
		}
		delete(wantUses, id)
		if id == "arclint/domain-model:contexts/respect-relations" && r.Severity() != rule.SeverityWarning {
			t.Errorf("%s severity = %s, want warning", id, r.Severity())
		}
	}
	if len(wantUses) != 0 {
		t.Errorf("missing rules: %v", wantUses)
	}
	modules := p.Modules()
	if len(modules) != 1 || modules[0].Name().String() != "vocabulary" || len(modules[0].SuggestedPaths()) != 1 ||
		modules[0].SuggestedPaths()[0].String() != "domain.arclint.yaml" {
		t.Errorf("modules = %+v, want vocabulary suggesting domain.arclint.yaml", modules)
	}
	exts := p.Extensions()
	if len(exts) != 3 {
		t.Fatalf("extensions = %d, want 3", len(exts))
	}
	for _, e := range exts {
		if !strings.Contains(e.Source(), `type: "domain-model/`) {
			t.Errorf("%s: the extension type must carry the pattern name", e.FileName())
		}
	}
	if len(p.Coverage()) != 2 {
		t.Errorf("coverage = %v, want go and ts", p.Coverage())
	}
}

func TestVerticalPatternLoads(t *testing.T) {
	source := embeddedpattern.NewSource()
	p := vertical(t)
	if p.Reference().String() != "arclint/vertical@0.1.0" {
		t.Errorf("reference = %q, want arclint/vertical@0.1.0", p.Reference())
	}
	if len(p.Rules()) != 16 {
		t.Errorf("rules = %d, want 16", len(p.Rules()))
	}
	var sawIndependence bool
	wantUses := map[string]string{
		"arclint/vertical:domain/no-context":              "vertical/forbid-imports",
		"arclint/vertical:domain/no-io":                   "vertical/forbid-imports",
		"arclint/vertical:application/repository-context": "vertical/repository-context",
		"arclint/vertical:application/usecase-contract":   "vertical/usecase",
		"arclint/vertical:shared/concerns":                "vertical/shared-concerns",
		"arclint/vertical:repositories/application-only":  "vertical/repository-location",
	}
	for _, r := range p.Rules() {
		id := r.ID().Qualified()
		if id == "arclint/vertical:features/independent" {
			sawIndependence = true
			if r.Type() != rule.TypeIndependence {
				t.Errorf("arclint/vertical:features/independent type = %q, want independence", r.Type())
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
		t.Errorf("missing arclint/vertical:features/independent")
	}
	if len(wantUses) != 0 {
		t.Errorf("missing extension rules: %v", wantUses)
	}
	if strings.TrimSpace(p.Documentation()) == "" {
		t.Errorf("the vertical pattern must document itself")
	}
	modules := p.Modules()
	wantModules := []string{"domain", "application", "infra", "app", "shared", "composition"}
	if len(modules) != len(wantModules) {
		t.Fatalf("modules = %d, want %d", len(modules), len(wantModules))
	}
	for i, m := range modules {
		if m.Name().String() != wantModules[i] {
			t.Errorf("modules[%d] = %q, want %q", i, m.Name(), wantModules[i])
		}
		if m.Description() == "" || len(m.SuggestedPaths()) == 0 {
			t.Errorf("module %s must carry a description and suggested paths", m.Name())
		}
	}
	for _, r := range p.Rules() {
		if r.Claim().String() == "" {
			t.Errorf("%s: a distributed Rule must carry a description", r.ID().Qualified())
		}
		if ref, ok := r.Provenance(); !ok || ref.String() != "arclint/vertical@0.1.0" {
			t.Errorf("%s provenance = %v %v", r.ID().Qualified(), ref, ok)
		}
	}
	if _, err := source.Names(); err != nil {
		t.Errorf("Names: %v", err)
	}
}

func TestVerticalPatternCarriesExtensions(t *testing.T) {
	exts := vertical(t).Extensions()
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
