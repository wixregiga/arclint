package rule_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

func mustModuleApplicability(t *testing.T, names ...string) rule.Applicability {
	t.Helper()
	modules := make([]rule.ModuleName, 0, len(names))
	for _, n := range names {
		m, err := rule.NewModuleName(n)
		if err != nil {
			t.Fatalf("NewModuleName(%q): %v", n, err)
		}
		modules = append(modules, m)
	}
	a, err := rule.ModuleApplicability(modules)
	if err != nil {
		t.Fatalf("ModuleApplicability(%v): %v", names, err)
	}
	return a
}

func mustRepoApplicability(t *testing.T) rule.Applicability {
	t.Helper()
	a, err := rule.RepositoryApplicability()
	if err != nil {
		t.Fatalf("RepositoryApplicability: %v", err)
	}
	return a
}

func emptyAllowList(t *testing.T) *rule.AllowList {
	t.Helper()
	l, err := rule.NewAllowList()
	if err != nil {
		t.Fatalf("NewAllowList: %v", err)
	}
	return &l
}

func validConsumesSpec(t *testing.T) rule.Spec {
	t.Helper()
	return rule.Spec{
		ID:            "arclint:domain/stdlib-only",
		Type:          rule.TypeConsumes,
		Params:        rule.ConsumesParams{Internal: emptyAllowList(t), External: rule.ImportForbid},
		Applicability: mustModuleApplicability(t, "domain"),
	}
}

func TestInvalidRulesCannotBeConstructed(t *testing.T) {
	snake, err := rule.NewCaseSpec("snake_case")
	if err != nil {
		t.Fatalf("NewCaseSpec: %v", err)
	}
	cases := []struct {
		name string
		spec rule.Spec
	}{
		{"empty id", func() rule.Spec { s := validConsumesSpec(t); s.ID = ""; return s }()},
		{"unpublished type", func() rule.Spec { s := validConsumesSpec(t); s.Type = "correspondence"; return s }()},
		{"params mismatch", func() rule.Spec {
			s := validConsumesSpec(t)
			s.Params = rule.NamingParams{Case: snake}
			return s
		}()},
		{"invalid severity", func() rule.Spec { s := validConsumesSpec(t); s.Severity = "warn"; return s }()},
		{"consumes without restriction", func() rule.Spec {
			s := validConsumesSpec(t)
			s.Params = rule.ConsumesParams{}
			return s
		}()},
		{"consumes without module scope", func() rule.Spec {
			s := validConsumesSpec(t)
			s.Applicability = mustRepoApplicability(t)
			return s
		}()},
		{"layers with one layer", rule.Spec{
			ID:            "t:one-layer",
			Type:          rule.TypeLayers,
			Params:        rule.LayersParams{Layers: []rule.ModuleName{"a"}},
			Applicability: mustRepoApplicability(t),
		}},
		{"layers with module scope", rule.Spec{
			ID:            "t:layers-scope",
			Type:          rule.TypeLayers,
			Params:        rule.LayersParams{Layers: []rule.ModuleName{"a", "b"}},
			Applicability: mustModuleApplicability(t, "a"),
		}},
		{"structure without globs", rule.Spec{
			ID:            "t:structure-empty",
			Type:          rule.TypeStructure,
			Params:        rule.StructureParams{},
			Applicability: mustModuleApplicability(t, "a"),
		}},
		{"naming without case", rule.Spec{
			ID:            "t:naming-empty",
			Type:          rule.TypeNaming,
			Params:        rule.NamingParams{},
			Applicability: mustModuleApplicability(t, "a"),
		}},
		{"independence without folders", rule.Spec{
			ID:            "t:independence-empty",
			Type:          rule.TypeIndependence,
			Params:        rule.IndependenceParams{},
			Applicability: mustRepoApplicability(t),
		}},
		{"independence with duplicate folders", rule.Spec{
			ID:            "t:independence-dup",
			Type:          rule.TypeIndependence,
			Params:        rule.IndependenceParams{Folders: []rule.Glob{mustGlob(t, "internal/*"), mustGlob(t, "internal/*")}},
			Applicability: mustRepoApplicability(t),
		}},
		{"independence with module scope", rule.Spec{
			ID:            "t:independence-scope",
			Type:          rule.TypeIndependence,
			Params:        rule.IndependenceParams{Folders: []rule.Glob{mustGlob(t, "internal/*")}},
			Applicability: mustModuleApplicability(t, "a"),
		}},
	}
	for _, c := range cases {
		if _, err := rule.New(c.spec); err == nil {
			t.Errorf("%s: expected construction error", c.name)
		}
	}
}

func TestDerivedClaim(t *testing.T) {
	r, err := rule.New(validConsumesSpec(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	claim := r.Claim().Statement()
	for _, want := range []string{`Module "domain"`, "no other declared Module", "no external imports"} {
		if !strings.Contains(claim, want) {
			t.Errorf("derived claim %q lacks %q", claim, want)
		}
	}
	if r.Severity() != rule.SeverityError {
		t.Errorf("default severity = %q, want error", r.Severity())
	}
	if !r.Enforcement().CanEvaluate() {
		t.Errorf("builtin consumes enforcement must be evaluable")
	}
}

func TestConfigurationPreservesIdentity(t *testing.T) {
	r, err := rule.New(validConsumesSpec(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	configured, err := r.WithSeverity(rule.SeverityWarning)
	if err != nil {
		t.Fatalf("WithSeverity: %v", err)
	}
	if !configured.ID().Equals(r.ID()) {
		t.Errorf("configuration changed identity")
	}
	if r.Severity() != rule.SeverityError {
		t.Errorf("original rule mutated: immutability broken")
	}

	glob, err := rule.NewGlob("internal/domain/legacy/**")
	if err != nil {
		t.Fatalf("NewGlob: %v", err)
	}
	exclusion, err := rule.NewExclusion([]rule.Glob{glob}, nil, "legacy code adopted as-is")
	if err != nil {
		t.Fatalf("NewExclusion: %v", err)
	}
	excluded := r.Exclude(exclusion)
	member := []rule.ModuleName{"domain"}
	if excluded.AppliesToFile("internal/domain/legacy/x.go", member) {
		t.Errorf("excluded subject still selected")
	}
	if !excluded.AppliesToFile("internal/domain/rule/root.go", member) {
		t.Errorf("exclusion removed an unselected subject")
	}
	if r.AppliesToFile("internal/domain/legacy/x.go", member) != true {
		t.Errorf("original rule mutated by Exclude")
	}

	suppression, err := rule.NewSuppression([]rule.Glob{glob}, "known debt")
	if err != nil {
		t.Fatalf("NewSuppression: %v", err)
	}
	suppressed := r.Suppress(suppression)
	if reason, ok := suppressed.SuppressionFor("internal/domain/legacy/x.go"); !ok || reason != "known debt" {
		t.Errorf("SuppressionFor = (%q, %v), want (known debt, true)", reason, ok)
	}

	disablement, err := rule.NewDisablement("rule retired for this repository")
	if err != nil {
		t.Fatalf("NewDisablement: %v", err)
	}
	disabled := r.Disable(disablement)
	if !disabled.Disabled() || r.Disabled() {
		t.Errorf("Disable must produce a disabled copy and leave the original enabled")
	}
}

func TestIDQualification(t *testing.T) {
	id, err := rule.NewID("arclint:domain/stdlib-only")
	if err != nil {
		t.Fatalf("NewID: %v", err)
	}
	if id.Namespace() != "arclint" || id.Local() != "domain/stdlib-only" {
		t.Errorf("parsed id = %q/%q", id.Namespace(), id.Local())
	}
	if id.Qualified() != "arclint:domain/stdlib-only" {
		t.Errorf("Qualified = %q", id.Qualified())
	}
	local, err := rule.NewID("repo-rule")
	if err != nil {
		t.Fatalf("NewID: %v", err)
	}
	if local.Qualified() != "repo-rule" {
		t.Errorf("local Qualified = %q", local.Qualified())
	}
	for _, bad := range []string{"", ":x", "a:", "a:b:c", "UPPER", "a b", ".start"} {
		if _, err := rule.NewID(bad); err == nil {
			t.Errorf("NewID(%q): expected error", bad)
		}
	}
}

func TestCaseSpec(t *testing.T) {
	spec, err := rule.NewCaseSpec("snake_case|regex:^v[0-9]+$")
	if err != nil {
		t.Fatalf("NewCaseSpec: %v", err)
	}
	for stem, want := range map[string]bool{
		"good_name": true,
		"v12":       true,
		"BadName":   false,
	} {
		if spec.Matches(stem) != want {
			t.Errorf("Matches(%q) = %v, want %v", stem, !want, want)
		}
	}
	for _, bad := range []string{"", "SCREAMING_CASE", "regex:("} {
		if _, err := rule.NewCaseSpec(bad); err == nil {
			t.Errorf("NewCaseSpec(%q): expected error", bad)
		}
	}
}

func TestRuleProvenanceCannotBeRewritten(t *testing.T) {
	r, err := rule.New(validConsumesSpec(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	origin, err := rule.NewPatternOrigin("arclint", "ddd-flat", "1.0.0")
	if err != nil {
		t.Fatalf("NewPatternOrigin: %v", err)
	}
	stamped, err := r.WithProvenance(origin)
	if err != nil {
		t.Fatalf("WithProvenance: %v", err)
	}
	got, ok := stamped.Provenance()
	if !ok || got != origin {
		t.Fatalf("Provenance = %v, %v", got, ok)
	}
	other, _ := rule.NewPatternOrigin("arclint", "ddd-flat", "2.0.0")
	if _, err := stamped.WithProvenance(other); err == nil {
		t.Errorf("rewriting Rule origin must be rejected")
	}
	if _, ok := r.Provenance(); ok {
		t.Errorf("WithProvenance must not mutate the original Rule")
	}
}

func TestExtensionValidationAndDefensiveBytes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		entry string
	}{
		{"", "extensions/check.ts"},
		{"check", ""},
		{"check", `extensions\check.ts`},
		{"check", "/extensions/check.ts"},
		{"check", "../check.ts"},
		{"check", "extensions/.hidden.ts"},
		{"check", "extensions/types.d.ts"},
		{"check", "extensions/readme.md"},
	} {
		if _, err := rule.NewExtension(tc.name, tc.entry, []byte("export default {}")); err == nil {
			t.Errorf("NewExtension(%q, %q): expected error", tc.name, tc.entry)
		}
	}
	if _, err := rule.NewExtension("check", "extensions/check.ts", []byte(" \n")); err == nil {
		t.Errorf("blank source must be rejected")
	}
	source := []byte("export default {}")
	extension, err := rule.NewExtension("check", "extensions/check.ts", source)
	if err != nil {
		t.Fatalf("NewExtension: %v", err)
	}
	source[0] = 'X'
	got := extension.Bytes()
	got[0] = 'Y'
	if string(extension.Bytes()) != "export default {}" {
		t.Errorf("Extension must preserve defensive source bytes")
	}
	if extension.Name() != "check" || extension.Entry() != "extensions/check.ts" || extension.FileName() != "check.ts" {
		t.Errorf("Extension accessors returned unexpected values")
	}
}

func TestRuleTestCompare(t *testing.T) {
	files := []rule.TestFile{{Path: "a/x.go", Content: "package a"}}
	// The vocabulary's duplicate-Diagnostics concern is settled at
	// construction: an expectation listed twice is unconstructible.
	if _, err := rule.NewTest("duplicates", "t:a/b", files, []rule.ExpectedFinding{
		{Path: "a/x.go", Message: "broken"},
		{Path: "a/x.go", Message: "broken"},
	}); err == nil {
		t.Errorf("duplicate expected findings must be unconstructible")
	}
	rt, err := rule.NewTest("compare", "t:a/b", files, []rule.ExpectedFinding{
		{Path: "a/x.go", Line: 3, Message: "broken"},
	})
	if err != nil {
		t.Fatalf("NewTest: %v", err)
	}
	cmp := rt.Compare([]rule.Finding{
		{Kind: rule.FindingViolation, Path: "a/y.go", Message: "surprise"},
	})
	if cmp.Clean() {
		t.Fatalf("mismatched result compared clean")
	}
	if len(cmp.Missing) != 1 || cmp.Missing[0].Message != "broken" {
		t.Errorf("missing = %v, want the unmet expectation", cmp.Missing)
	}
	if len(cmp.Unexpected) != 1 || cmp.Unexpected[0].Path != "a/y.go" {
		t.Errorf("unexpected = %v", cmp.Unexpected)
	}
}
