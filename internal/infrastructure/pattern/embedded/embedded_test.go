package embeddedpattern_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
	embeddedpattern "github.com/wixregiga/arclint/internal/infrastructure/pattern/embedded"
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
	var sawIndependence bool
	wantUses := map[string]string{
		"arclint:domain/no-context":              "vertical/forbid-imports",
		"arclint:domain/no-io":                   "vertical/forbid-imports",
		"arclint:application/repository-context": "vertical/repository-context",
		"arclint:application/usecase-contract":   "vertical/usecase",
		"arclint:shared/concerns":                "vertical/shared-concerns",
		"arclint:repositories/application-only":  "vertical/repository-location",
	}
	for _, r := range p.Rules() {
		id := r.ID().Qualified()
		if id == "arclint:features/independent" {
			sawIndependence = true
			if r.Type() != rule.TypeIndependence {
				t.Errorf("arclint:features/independent type = %q, want independence", r.Type())
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
		t.Errorf("missing arclint:features/independent")
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
	names, err := source.Names()
	if err != nil || len(names) != 1 || names[0] != "vertical" {
		t.Errorf("Names = %v, %v", names, err)
	}
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
