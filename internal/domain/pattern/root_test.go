package pattern_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/pattern"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

func patternFixture(t *testing.T) pattern.Spec {
	t.Helper()
	reference, err := pattern.NewReference("arclint", "vertical", "1.2.3")
	if err != nil {
		t.Fatalf("NewReference: %v", err)
	}
	glob, err := rule.NewGlob("internal/**")
	if err != nil {
		t.Fatalf("NewGlob: %v", err)
	}
	name, _ := rule.NewModuleName("domain")
	module, err := rule.NewModule(name, "Domain code", []rule.Glob{glob})
	if err != nil {
		t.Fatalf("NewModule: %v", err)
	}
	applicability, err := rule.ModuleApplicability([]rule.ModuleName{name})
	if err != nil {
		t.Fatalf("ModuleApplicability: %v", err)
	}
	carriedRule, err := rule.New(rule.Spec{
		ID:            "arclint:vertical/domain/stdlib-only",
		Type:          rule.TypeConsumes,
		Params:        rule.ConsumesParams{Stdlib: rule.ImportForbid},
		Applicability: applicability,
	})
	if err != nil {
		t.Fatalf("New Rule: %v", err)
	}
	extension, err := rule.NewExtension("forbid-imports", "extensions/forbid_imports.ts", []byte("export default {}\n"))
	if err != nil {
		t.Fatalf("NewExtension: %v", err)
	}
	test, err := rule.NewTest("stdlib", carriedRule.ID().Qualified(), []rule.TestFile{{Path: "internal/a.go", Content: "package internal"}}, nil)
	if err != nil {
		t.Fatalf("NewTest: %v", err)
	}
	manifest, _ := pattern.NewFile("pattern.yaml", []byte("pattern:\n  name: vertical\n"))
	extensionFile, _ := pattern.NewFile("extensions/forbid_imports.ts", extension.Bytes())
	testFile, _ := pattern.NewFile("tests/stdlib.yaml", []byte("name: stdlib\n"))
	return pattern.Spec{
		Reference:  reference,
		Modules:    []rule.Module{module},
		Rules:      []rule.Rule{carriedRule},
		Extensions: []rule.Extension{extension},
		Tests:      []rule.Test{test},
		Coverage:   []rule.Language{rule.LanguageGo},
		Files:      []pattern.File{testFile, manifest, extensionFile},
	}
}

func TestPatternCarriesRuleContextValuesAndStampsOrigin(t *testing.T) {
	spec := patternFixture(t)
	p, err := pattern.New(spec)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Reference() != spec.Reference || len(p.Modules()) != 1 || len(p.Rules()) != 1 || len(p.Extensions()) != 1 || len(p.Tests()) != 1 {
		t.Fatalf("Pattern did not retain its complete contract")
	}
	origin, ok := p.Rules()[0].Provenance()
	if !ok || origin != spec.Reference {
		t.Errorf("Rule provenance = %v, %v; want %v", origin, ok, spec.Reference)
	}
	if p.Digest() == "" || len(p.Digest()) != 64 {
		t.Errorf("Digest = %q, want lowercase SHA-256", p.Digest())
	}
}

func TestPatternTestsAndCoverageAreOptional(t *testing.T) {
	spec := patternFixture(t)
	spec.Tests = nil
	spec.Coverage = nil
	p, err := pattern.New(spec)
	if err != nil {
		t.Fatalf("New without tests or coverage: %v", err)
	}
	if len(p.Tests()) != 0 || len(p.Coverage()) != 0 {
		t.Errorf("optional values = tests %v coverage %v", p.Tests(), p.Coverage())
	}
}

func TestPatternRejectsInvalidCarriedValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*pattern.Spec)
		want   string
	}{
		{"missing reference", func(s *pattern.Spec) { s.Reference = pattern.Reference{} }, "unconstructed reference"},
		{"no rules", func(s *pattern.Spec) { s.Rules = nil }, "no rules"},
		{"duplicate module", func(s *pattern.Spec) { s.Modules = append(s.Modules, s.Modules[0]) }, "duplicate module"},
		{"duplicate rule", func(s *pattern.Spec) { s.Rules = append(s.Rules, s.Rules[0]) }, "duplicate rule id"},
		{"duplicate extension", func(s *pattern.Spec) { s.Extensions = append(s.Extensions, s.Extensions[0]) }, "duplicate extension"},
		{"unknown tested rule", func(s *pattern.Spec) {
			s.Tests[0], _ = rule.NewTest("unknown", "arclint:vertical/domain/unknown", []rule.TestFile{{Path: "x.go", Content: "package x"}}, nil)
		}, "unknown rule"},
		{"duplicate test", func(s *pattern.Spec) { s.Tests = append(s.Tests, s.Tests[0]) }, "duplicate rule test"},
		{"invalid coverage", func(s *pattern.Spec) { s.Coverage = []rule.Language{"ruby"} }, "coverage language"},
		{"duplicate coverage", func(s *pattern.Spec) { s.Coverage = append(s.Coverage, s.Coverage[0]) }, "duplicate coverage"},
		{"empty tree", func(s *pattern.Spec) { s.Files = nil }, "distribution tree is empty"},
		{"duplicate file", func(s *pattern.Spec) { s.Files = append(s.Files, s.Files[0]) }, "duplicate distribution file"},
		{"missing manifest", func(s *pattern.Spec) { s.Files = []pattern.File{s.Files[0], s.Files[2]} }, "does not contain pattern.yaml"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spec := patternFixture(t)
			tc.mutate(&spec)
			if _, err := pattern.New(spec); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("New error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestPatternPreservesExactTreeBytesDefensively(t *testing.T) {
	spec := patternFixture(t)
	p, err := pattern.New(spec)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	digest := p.Digest()
	input := spec.Files[1].Bytes()
	input[0] = 'X'
	returned := p.Files()
	returned[1], _ = pattern.NewFile(returned[1].Path(), []byte("changed"))
	if p.Digest() != digest || string(p.Files()[1].Bytes()) != "pattern:\n  name: vertical\n" {
		t.Errorf("Pattern tree was mutated through an external byte slice")
	}
}

func TestPatternTreeDigestIsStableAndCoversEveryByte(t *testing.T) {
	spec := patternFixture(t)
	first, err := pattern.New(spec)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	spec.Files[0], spec.Files[2] = spec.Files[2], spec.Files[0]
	second, err := pattern.New(spec)
	if err != nil {
		t.Fatalf("New reordered: %v", err)
	}
	if first.Digest() != second.Digest() {
		t.Fatalf("digest depends on input order: %s != %s", first.Digest(), second.Digest())
	}
	changed := patternFixture(t)
	changed.Files[0], _ = pattern.NewFile(changed.Files[0].Path(), append(changed.Files[0].Bytes(), '\n'))
	third, err := pattern.New(changed)
	if err != nil {
		t.Fatalf("New changed: %v", err)
	}
	if first.Digest() == third.Digest() {
		t.Errorf("digest did not cover a changed file byte")
	}
}

func TestPatternFileRejectsAmbiguousPaths(t *testing.T) {
	for _, name := range []string{"", "/pattern.yaml", "./pattern.yaml", "a/../pattern.yaml", "../pattern.yaml", `a\pattern.yaml`, "."} {
		if _, err := pattern.NewFile(name, nil); err == nil {
			t.Errorf("NewFile(%q): expected error", name)
		}
	}
}

func TestPatternReferenceRequiresExactSemver(t *testing.T) {
	for _, version := range []string{"", "latest", "1", "1.2", "01.2.3", "1.2.3-01", "1.2.3+"} {
		if _, err := pattern.NewReference("arclint", "vertical", version); err == nil {
			t.Errorf("NewReference(%q): expected error", version)
		}
	}
	for _, version := range []string{"0.0.0", "1.2.3", "1.2.3-rc.1", "1.2.3+build.7"} {
		if _, err := pattern.NewReference("arclint", "vertical", version); err != nil {
			t.Errorf("NewReference(%q): %v", version, err)
		}
	}
}
