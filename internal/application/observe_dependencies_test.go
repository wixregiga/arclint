package application_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

func TestDependencyAndDomainInspectionObserveOnce(t *testing.T) {
	cfg, knowledge, source := domainFixture(t)
	uc, err := application.NewGetArchitecturalContext(fakeRepository{cfg}, knowledge)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := uc.WithObservations(source).Execute(application.ContextRequest{Dependencies: true})
	if err != nil {
		t.Fatal(err)
	}
	if source.calls != 1 || !reflect.DeepEqual(source.facts, []rule.Fact{rule.FactImports, rule.FactDeclarations}) {
		t.Fatalf("observation calls=%d, requirements=%v", source.calls, source.facts)
	}
	if ctx.Dependencies == nil || ctx.Domain == nil || !ctx.Domain.Located {
		t.Fatalf("combined inspection: %+v", ctx)
	}
	billing := contextNamed(t, ctx.Domain, "billing")
	if len(billing.Invariants) != 1 || billing.Invariants[0].Source != "m/money.go:4" {
		t.Fatalf("shared input changed contract location: %+v", billing)
	}
}

func TestDependencyInspectionDistinguishesUnobservedAndUnzonedTargets(t *testing.T) {
	_, source := dependencyContext(t, map[string]conformance.LanguageFacts{
		"m/source.go": {Language: rule.LanguageGo, ImportsAvailable: true, Imports: []conformance.Import{
			{Path: "unresolved", Class: conformance.ImportInternal},
			{Path: "missing-file", Class: conformance.ImportInternal, TargetFile: "unowned/missing.go"},
			{Path: "missing-dir", Class: conformance.ImportInternal, TargetDir: "missing"},
			{Path: "existing-file", Class: conformance.ImportInternal, TargetFile: "unowned/known.go"},
			{Path: "existing-dir", Class: conformance.ImportInternal, TargetDir: "unowned"},
		}},
		"unowned/known.go": {Language: rule.LanguageGo, ImportsAvailable: true},
	}, "m/source.go", "unowned/known.go")
	// Use no Zones so observation status cannot be inferred from membership.
	cfg, _ := fixture(t)
	cfg.Zones = nil
	cfg.Rules = nil
	cfg.Languages = []rule.Language{rule.LanguageGo}
	uc, err := application.NewGetArchitecturalContext(fakeRepository{cfg}, emptyKnowledge())
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := uc.WithObservations(source).Execute(application.ContextRequest{Dependencies: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.Dependencies.Diagnostics) != 3 {
		t.Fatalf("gaps: %+v", ctx.Dependencies.Diagnostics)
	}
	codes := []string{}
	for _, d := range ctx.Dependencies.Diagnostics {
		codes = append(codes, d.Code)
	}
	if !reflect.DeepEqual(codes, []string{"TARGET_UNRESOLVED", "TARGET_NOT_OBSERVED", "TARGET_NOT_OBSERVED"}) {
		t.Fatalf("codes: %v", codes)
	}
	for i, edge := range ctx.Dependencies.Edges {
		if len(edge.TargetZones) != 0 || edge.TargetObserved != (i >= 3) {
			t.Fatalf("target status: %+v", edge)
		}
	}
}

func dependencyContext(t *testing.T, facts map[string]conformance.LanguageFacts, paths ...string) (application.GetArchitecturalContext, *fakeObservations) {
	t.Helper()
	cfg, _ := fixture(t)
	cfg.Languages = []rule.Language{rule.LanguageGo, rule.LanguageTypeScript}
	for name, pattern := range map[string]string{"all": "**", "target": "target/**", "precise": "target/one.ts"} {
		glob, err := rule.NewGlob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		zone, err := rule.NewZone(rule.ZoneName(name), name, []rule.Glob{glob})
		if err != nil {
			t.Fatal(err)
		}
		cfg.Zones = append(cfg.Zones, zone)
	}
	files := []conformance.ObservedFile{}
	for _, p := range paths {
		files = append(files, conformance.ObservedFile{Path: p})
	}
	obs, err := conformance.NewObservations(files, facts)
	if err != nil {
		t.Fatal(err)
	}
	source := &fakeObservations{obs: obs}
	uc, err := application.NewGetArchitecturalContext(fakeRepository{cfg}, emptyKnowledge())
	if err != nil {
		t.Fatal(err)
	}
	return uc.WithObservations(source), source
}

func TestObservedDependenciesPreserveNativeGranularityAndOverlappingZones(t *testing.T) {
	uc, source := dependencyContext(t, map[string]conformance.LanguageFacts{
		"m/main.go": {Language: rule.LanguageGo, ImportsAvailable: true, Imports: []conformance.Import{
			{Path: "example/target", Line: 2, Class: conformance.ImportInternal, TargetDir: "target"},
			{Path: "fmt", Line: 3, Class: conformance.ImportStdlib},
		}},
		"m/main.ts": {Language: rule.LanguageTypeScript, ImportsAvailable: true, Imports: []conformance.Import{
			{Path: "../target/two", Line: 1, Class: conformance.ImportInternal, TargetFile: "target/two.ts"},
		}},
		"target/one.ts": {Language: rule.LanguageTypeScript, ImportsAvailable: true},
		"target/two.ts": {Language: rule.LanguageTypeScript, ImportsAvailable: true},
	}, "target/two.ts", "m/main.ts", "README.md", "target/one.ts", "m/main.go")
	ctx, err := uc.Execute(application.ContextRequest{Dependencies: true})
	if err != nil {
		t.Fatal(err)
	}
	d := ctx.Dependencies
	if !reflect.DeepEqual(source.facts, []rule.Fact{rule.FactImports}) {
		t.Fatalf("requested facts: %v", source.facts)
	}
	if d == nil || !d.Coverage.Complete || d.Coverage.SourceFiles != 4 || d.Coverage.FilesWithImports != 4 || len(d.Edges) != 3 {
		t.Fatalf("dependencies: %+v", d)
	}
	edge := d.Edges[0]
	if edge.TargetKind != "directory" || edge.TargetPath != "target" || !reflect.DeepEqual(edge.SourceZones, []string{"all", "m"}) || !reflect.DeepEqual(edge.TargetZones, []string{"all", "precise", "target"}) {
		t.Fatalf("package import: %+v", edge)
	}
	precise := d.Edges[2]
	if precise.TargetKind != "file" || precise.TargetPath != "target/two.ts" || !reflect.DeepEqual(precise.TargetZones, []string{"all", "target"}) {
		t.Fatalf("file import used package union: %+v", precise)
	}
	if d.Edges[1].TargetPath != "" || d.Edges[1].Classification != conformance.ImportStdlib {
		t.Fatalf("stdlib import: %+v", d.Edges[1])
	}
	if d.Files[0].Path != "README.md" || d.Files[0].ImportsAvailable {
		t.Fatalf("non-source file: %+v", d.Files[0])
	}
}

func TestObservedDependenciesDoNotTurnMissingFactsIntoEmptyGraph(t *testing.T) {
	uc, _ := dependencyContext(t, map[string]conformance.LanguageFacts{
		"m/bad.go": {Language: rule.LanguageGo, ImportsAvailable: true, ParseFailure: "syntax error"},
		"m/known.go": {Language: rule.LanguageGo, ImportsAvailable: true, Imports: []conformance.Import{
			{Path: "unknown", Class: conformance.ImportUnknown},
			{Path: "example/missing", Class: conformance.ImportInternal, TargetDir: "missing"},
		}},
	}, "m/bad.go", "m/no_facts.go", "m/known.go")
	ctx, err := uc.Execute(application.ContextRequest{Dependencies: true})
	if err != nil {
		t.Fatal(err)
	}
	d := ctx.Dependencies
	if d.Coverage.Complete || d.Coverage.FilesWithImports != 1 || len(d.Diagnostics) != 4 {
		t.Fatalf("missing-facts coverage: %+v", d)
	}
	if len(d.Edges) != 2 {
		t.Fatalf("usable imports were discarded: %+v", d.Edges)
	}
	for _, row := range d.Diagnostics {
		if row.Code == "IMPORTS_UNAVAILABLE" && row.Path == "m/bad.go" && row.Message != "syntax error" {
			t.Fatal(row)
		}
	}
}

func TestObservedDependenciesAreOptInAndRepositoryScoped(t *testing.T) {
	uc, source := dependencyContext(t, nil)
	ctx, err := uc.Execute(application.ContextRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Dependencies != nil || source.facts != nil {
		t.Fatalf("default context observed imports: %+v", ctx)
	}
	for _, req := range []application.ContextRequest{{Dependencies: true, Paths: []string{"m"}}, {Dependencies: true, Zones: []string{"m"}}} {
		if _, err := uc.Execute(req); err == nil || !strings.Contains(err.Error(), "repository scope") {
			t.Fatalf("scoped request error: %v", err)
		}
	}
	ctx, err = uc.Execute(application.ContextRequest{Dependencies: true})
	if err != nil {
		t.Fatal(err)
	}
	if !ctx.Dependencies.Coverage.Complete || ctx.Dependencies.Edges == nil || ctx.Dependencies.Files == nil {
		t.Fatalf("legitimate empty observation: %+v", ctx.Dependencies)
	}
}
