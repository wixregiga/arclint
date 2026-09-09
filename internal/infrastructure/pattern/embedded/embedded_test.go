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
	want := []string{"arclint/vertical@0.1.0"}
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
	if err != nil || len(names) != 1 || names[0] != "vertical" {
		t.Errorf("Names = %v, %v", names, err)
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
	zones := p.Zones()
	wantZones := []string{"domain", "application", "infra", "app", "shared", "composition"}
	if len(zones) != len(wantZones) {
		t.Fatalf("zones = %d, want %d", len(zones), len(wantZones))
	}
	for i, m := range zones {
		if m.Name().String() != wantZones[i] {
			t.Errorf("zones[%d] = %q, want %q", i, m.Name(), wantZones[i])
		}
		if m.Description() == "" || len(m.SuggestedPaths()) == 0 {
			t.Errorf("zone %s must carry a rationale and suggested paths", m.Name())
		}
	}
	for _, r := range p.Rules() {
		if r.Rationale().String() == "" {
			t.Errorf("%s: a distributed Rule must carry a rationale", r.ID().Qualified())
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
