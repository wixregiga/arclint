package embeddedpattern_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/distribution"
	"github.com/wixregiga/arclint/internal/domain/rule"
	embeddedpattern "github.com/wixregiga/arclint/internal/infrastructure/pattern/embedded"
)

// builtIn returns the named arclint Pattern from the built-in source.
func builtIn(t *testing.T, name string) rule.Pattern {
	t.Helper()
	patterns, err := embeddedpattern.NewSource().Patterns()
	if err != nil {
		t.Fatalf("Patterns: %v", err)
	}
	for _, p := range patterns {
		if p.Reference().Name() == name {
			return p
		}
	}
	t.Fatalf("arclint/%s is not embedded; got %d patterns", name, len(patterns))
	return rule.Pattern{}
}

// vertical returns the arclint/vertical Pattern from the built-in
// source.
func vertical(t *testing.T) rule.Pattern {
	t.Helper()
	return builtIn(t, "vertical")
}

func TestBuiltInPatternsAreAvailableWithDigests(t *testing.T) {
	source := embeddedpattern.NewSource()
	available, err := source.Available()
	if err != nil {
		t.Fatalf("Available: %v", err)
	}
	want := []string{"arclint/hexagonal@0.1.0", "arclint/vertical@0.1.0"}
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
		if len(a.Pattern.Extensions()) > 0 {
			if _, ok := a.Vendored.File("extensions/package.json"); !ok {
				t.Errorf("%s: extensions/package.json must ship so the extension directory type-checks", a.Reference())
			}
		}
	}
	names, err := source.Names()
	if err != nil || len(names) != 2 || names[0] != "hexagonal" || names[1] != "vertical" {
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

// TestHexagonalPatternLoads pins the shape of arclint/hexagonal: six
// zones around an isolated core, and seven built-in imports rules, one
// per zone boundary plus the domain's and the application's library
// policy, with no extension.
func TestHexagonalPatternLoads(t *testing.T) {
	p := builtIn(t, "hexagonal")
	if p.Reference().String() != "arclint/hexagonal@0.1.0" {
		t.Errorf("reference = %q, want arclint/hexagonal@0.1.0", p.Reference())
	}
	var coverage []string
	for _, l := range p.Coverage() {
		coverage = append(coverage, string(l))
	}
	if strings.Join(coverage, ",") != "go,typescript,python" {
		t.Errorf("coverage = %v, want go, typescript, python", coverage)
	}
	if strings.TrimSpace(p.Documentation()) == "" {
		t.Errorf("the hexagonal pattern must document itself")
	}
	wantZones := []string{"domain", "application", "driving", "driven", "shared", "composition"}
	zones := p.Zones()
	if len(zones) != len(wantZones) {
		t.Fatalf("zones = %d, want %d", len(zones), len(wantZones))
	}
	for i, m := range zones {
		if m.Name().String() != wantZones[i] {
			t.Errorf("zones[%d] = %q, want %q", i, m.Name(), wantZones[i])
		}
		if m.Description() == "" || len(m.SuggestedPaths()) == 0 {
			t.Errorf("zone %s must carry a description and suggested paths", m.Name())
		}
	}
	want := map[string]bool{
		"arclint/hexagonal:domain/depends-on-nothing-outside": true,
		"arclint/hexagonal:application/through-ports":         true,
		"arclint/hexagonal:application/core-libraries-only":   true,
		"arclint/hexagonal:driving/calls-the-application":     true,
		"arclint/hexagonal:driven/implements-ports":           true,
		"arclint/hexagonal:shared/product-free":               true,
	}
	if len(p.Rules()) != len(want) {
		t.Errorf("rules = %d, want %d", len(p.Rules()), len(want))
	}
	importRules := map[string]int{}
	for _, r := range p.Rules() {
		id := r.ID().Qualified()
		if !want[id] {
			t.Errorf("unexpected rule %s", id)
			continue
		}
		delete(want, id)
		if r.Rationale().String() == "" {
			t.Errorf("%s: a distributed Rule must carry a rationale", id)
		}
		if ref, ok := r.Provenance(); !ok || ref.String() != "arclint/hexagonal@0.1.0" {
			t.Errorf("%s provenance = %v %v", id, ref, ok)
		}
		for _, test := range []string{"internal/domain/order_test.go", "src/domain/order.test.ts", "src/routes/orders.spec.tsx", "app/domain/test_order.py"} {
			if !r.Scope().ExcludedFile(test) {
				t.Errorf("%s judges the test file %s; tests wire fakes across zones", id, test)
			}
		}
		if r.Scope().ExcludedFile("internal/domain/order.go") {
			t.Errorf("%s must judge production files", id)
		}
		if id == "arclint/hexagonal:application/core-libraries-only" {
			params, ok := r.Params().(rule.ExtensionParams)
			if r.Type() != rule.TypeExtension || !ok {
				t.Errorf("%s = %q, want an extension rule", id, r.Type())
				continue
			}
			if params.Uses != "hexagonal/allowed-packages" || fmt.Sprint(params.With["packages"]) != "[effect]" {
				t.Errorf("%s uses %q with %v, want hexagonal/allowed-packages with packages [effect]", id, params.Uses, params.With)
			}
			continue
		}
		params, ok := r.Params().(rule.ConsumesParams)
		if r.Type() != rule.TypeConsumes || !ok {
			t.Errorf("%s = %q, want a built-in imports rule", id, r.Type())
			continue
		}
		for _, z := range r.Scope().Zones() {
			importRules[z.String()]++
		}
		if params.Internal == nil {
			t.Errorf("%s must restrict internal imports", id)
		}
		wantExternal := rule.ImportAllow
		if id == "arclint/hexagonal:domain/depends-on-nothing-outside" {
			wantExternal = rule.ImportForbid
		}
		if params.External != wantExternal {
			t.Errorf("%s external = %q, want %q", id, params.External, wantExternal)
		}
	}
	// The Pattern gives every zone it constrains exactly one imports rule.
	for zone, n := range importRules {
		if n != 1 {
			t.Errorf("zone %s carries %d imports rules, want 1", zone, n)
		}
	}
	if len(want) != 0 {
		t.Errorf("missing rules: %v", want)
	}
	exts := p.Extensions()
	if len(exts) != 1 || exts[0].FileName() != "hexagonal_allowed_packages.ts" {
		t.Errorf("extensions = %v, want hexagonal_allowed_packages.ts", exts)
	}
}
