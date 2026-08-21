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
