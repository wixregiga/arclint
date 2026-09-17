package conformance_test

import (
	"reflect"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

func TestInspectionAndRuleFactsShareDependencySemantics(t *testing.T) {
	zones := []rule.Zone{mustZone(t, "source", "src/**")}
	obs, err := conformance.NewObservations([]conformance.ObservedFile{{Path: "src/main.go"}, {Path: "unowned/known.go"}}, map[string]conformance.LanguageFacts{
		"src/main.go": {Language: rule.LanguageGo, ImportsAvailable: true, DeclarationsAvailable: true,
			Declarations: []conformance.Declaration{{Kind: "struct", Name: "Unrequested"}},
			Imports: []conformance.Import{
				{Class: conformance.ImportInternal, TargetFile: "unowned/known.go"},
				{Class: conformance.ImportInternal, TargetDir: "unowned"},
				{Class: conformance.ImportInternal, TargetFile: "unowned/absent.go"},
				{Class: conformance.ImportInternal, TargetDir: "missing"},
				{Class: conformance.ImportInternal},
				{Class: conformance.ImportExternal, TargetFile: "unowned/known.go"},
			}},
	})
	if err != nil {
		t.Fatal(err)
	}
	inspection, err := conformance.NewInspectionFacts(zones, obs, rule.FactImports)
	if err != nil {
		t.Fatal(err)
	}
	allow, err := rule.NewAllowList()
	if err != nil {
		t.Fatal(err)
	}
	r := mustRule(t, rule.Spec{ID: "source/imports", Type: rule.TypeConsumes, Params: rule.ConsumesParams{Internal: &allow}, Scope: zoneScope(t, "source")})
	supplied, err := conformance.NewFacts(r, zones, obs)
	if err != nil {
		t.Fatal(err)
	}
	imports := inspection.ImportsFor("src/main.go")
	if !reflect.DeepEqual(imports, supplied.ImportsFor("src/main.go")) {
		t.Fatal("inspection reinterpreted rule facts")
	}
	for i, imp := range imports {
		if imp.TargetObserved != (i < 2) || len(imp.TargetZones) != 0 {
			t.Fatalf("target %d: %+v", i, imp)
		}
	}
	filtered, ok := inspection.FactsFor("src/main.go")
	if !ok || filtered.DeclarationsAvailable || len(filtered.Declarations) != 0 || len(inspection.Files()) != 0 {
		t.Fatal("inspection received unrequested classes")
	}
	if supplied.Contains("unowned/known.go") {
		t.Fatal("target observation granted Rule access")
	}
	if _, err := conformance.NewInspectionFacts(zones, obs, rule.Fact("invented")); err == nil {
		t.Fatal("unknown inspection class accepted")
	}
}
