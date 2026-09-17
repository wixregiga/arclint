package conformance_test

import (
	"reflect"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

func TestFactsPreserveDependencyEvidenceAndAccessBoundaries(t *testing.T) {
	zones := []rule.Zone{mustZone(t, "subject", "domain/**"), mustZone(t, "adapter", "adapter/**")}
	r := mustRule(t, rule.Spec{ID: "incoming", Type: rule.TypeExtension,
		Params: rule.ExtensionParams{Uses: "probe"}, Scope: zoneScope(t, "subject")})
	files := []conformance.ObservedFile{
		{Path: "domain/order.ts"}, {Path: "adapter/handler.ts"}, {Path: "elsewhere/a.ts"},
		{Path: "elsewhere/failed.ts"}, {Path: "elsewhere/missing.ts"},
	}
	observed := map[string]conformance.LanguageFacts{
		"domain/order.ts": {Language: rule.LanguageTypeScript, ImportsAvailable: true, Imports: []conformance.Import{
			{Path: "./order", Class: conformance.ImportInternal, TargetFile: "domain/order.ts", Line: 2},
		}},
		"adapter/handler.ts": {Language: rule.LanguageTypeScript, ImportsAvailable: true, Imports: []conformance.Import{
			{Path: "../domain/order", Class: conformance.ImportInternal, TargetFile: "domain/order.ts", Line: 7},
			{Path: "../domain", Class: conformance.ImportInternal, TargetDir: "domain", Line: 9},
			{Path: "elsewhere", Class: conformance.ImportInternal, TargetFile: "elsewhere/a.ts", Line: 11},
		}},
		"elsewhere/a.ts":      {Language: rule.LanguageTypeScript, ImportsAvailable: true},
		"elsewhere/failed.ts": {Language: rule.LanguageTypeScript, ParseFailure: "invalid syntax"},
	}
	obs, err := conformance.NewObservations(files, observed)
	if err != nil {
		t.Fatal(err)
	}
	all, err := conformance.NewInspectionFacts(zones, obs, rule.FactFileTree, rule.FactImports)
	if err != nil {
		t.Fatal(err)
	}
	// The index borrows stable observations. Neither input nor an accessor
	// may change them, including before the lazy index is first used.
	observed["adapter/handler.ts"].Imports[0].Line = 100
	accessed, _ := obs.FactsFor("adapter/handler.ts")
	accessed.Imports[0].Line = 200
	facts, err := conformance.NewFacts(r, zones, obs)
	if err != nil {
		t.Fatal(err)
	}
	edges := facts.DependenciesFor("domain/order.ts")
	if len(edges) != 3 {
		t.Fatalf("incident imports, self-loop once: %+v", edges)
	}
	if edges[0].SourcePath != "adapter/handler.ts" || edges[0].Line != 7 || edges[0].TargetFile != "domain/order.ts" {
		t.Fatalf("incoming location or precision lost: %+v", edges[0])
	}
	if edges[1].TargetFile != "" || edges[1].TargetDir != "domain" {
		t.Fatalf("package evidence invented a file target: %+v", edges[1])
	}
	var repository []conformance.DependencyImport
	for _, file := range all.Files() {
		repository = append(repository, all.ImportsFor(file.Path)...)
	}
	for _, edge := range edges {
		found := false
		for _, shared := range repository {
			if reflect.DeepEqual(edge, shared) {
				found = true
			}
		}
		if !found {
			t.Fatalf("extension reinterpreted repository evidence: %+v", edge)
		}
	}
	if !facts.AllowsFinding("domain/order.ts", "adapter/handler.ts", 7) ||
		facts.AllowsFinding("domain/order.ts", "adapter/handler.ts", 8) ||
		facts.AllowsFinding("domain/order.ts", "adapter/handler.ts", 11) ||
		facts.AllowsFinding("adapter/handler.ts", "adapter/handler.ts", 7) {
		t.Fatal("finding authorization did not require selected subject and incident evidence")
	}
	if _, ok := facts.FactsFor("adapter/handler.ts"); ok {
		t.Fatal("incoming evidence grants declarations")
	}
	if _, err := facts.Read("adapter/handler.ts"); err == nil {
		t.Fatal("incoming evidence grants content")
	}
	if len(facts.DependenciesFor("adapter/handler.ts")) != 0 {
		t.Fatal("incoming evidence grants traversal")
	}
	exclusion, err := rule.NewExclusion([]rule.Glob{mustGlob(t, "adapter/**")}, nil, "excluded importer")
	if err != nil {
		t.Fatal(err)
	}
	excludedRule, err := r.Exclude(exclusion)
	if err != nil {
		t.Fatal(err)
	}
	excludedFacts, err := conformance.NewFacts(excludedRule, zones, obs)
	if err != nil {
		t.Fatal(err)
	}
	if len(excludedFacts.DependenciesFor("domain/order.ts")) != 1 || excludedFacts.AllowsFinding("domain/order.ts", "adapter/handler.ts", 7) {
		t.Fatal("excluded importer escaped the evidence boundary")
	}
	edges[0].SourceZones[0] = "changed"
	repository[0].TargetZones[0] = "changed"
	if facts.DependenciesFor("domain/order.ts")[0].SourceZones[0] != "adapter" || all.ImportsFor("adapter/handler.ts")[0].TargetZones[0] != "subject" {
		t.Fatal("returned dependency metadata mutated shared facts")
	}
	for _, tc := range []struct {
		path      string
		available bool
		failure   string
	}{
		{"elsewhere/a.ts", true, ""}, {"elsewhere/missing.ts", false, ""}, {"elsewhere/failed.ts", false, "invalid syntax"},
	} {
		observed, _ := all.FactsFor(tc.path)
		available, failure := observed.Supports(rule.FactImports), observed.ParseFailure
		if available != tc.available || failure != tc.failure {
			t.Fatalf("availability for %s: %v, %q", tc.path, available, failure)
		}
	}
	// Adding facts that do not involve the subject changes no supplied value.
	files = append(files, conformance.ObservedFile{Path: "elsewhere/added.ts"})
	observed["elsewhere/added.ts"] = conformance.LanguageFacts{Language: rule.LanguageTypeScript, ParseFailure: "broken"}
	observed["adapter/handler.ts"].Imports[0].Line = 7
	other, err := conformance.NewObservations(files, observed)
	if err != nil {
		t.Fatal(err)
	}
	after, err := conformance.NewFacts(r, zones, other)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(facts.DependenciesFor("domain/order.ts"), after.DependenciesFor("domain/order.ts")) {
		t.Fatal("nonincident observation changed supplied dependencies")
	}
	// Reports retain the selected subject while anchoring at the importer.
	a, err := conformance.Run(conformance.Request{Rules: []rule.Rule{r}, Zones: zones, Observations: obs,
		Extensions: &fakeExtensions{findings: []conformance.ExtensionFinding{{
			SubjectPath: "domain/order.ts", Path: "adapter/handler.ts", Line: 7, Message: "forbidden importer",
		}}}})
	if err != nil {
		t.Fatal(err)
	}
	vs := a.ActiveViolations()
	if len(vs) != 1 || vs[0].Path() != "adapter/handler.ts" || vs[0].Line() != 7 || a.Evaluations()[0].Subject().Identity() != "domain/order.ts" {
		t.Fatalf("subject and evidence conflated: %+v", a)
	}
}
