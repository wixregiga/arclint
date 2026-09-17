package conformance_test

import (
	"reflect"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

type incomingFactsProbe struct{ t *testing.T }

func TestExcludedTargetRetainsIndependenceEvidence(t *testing.T) {
	obs, err := conformance.NewObservations(
		[]conformance.ObservedFile{{Path: "internal/a/a.go"}, {Path: "internal/b/b.go"}},
		map[string]conformance.LanguageFacts{
			"internal/a/a.go": {Language: rule.LanguageGo, ImportsAvailable: true, Imports: []conformance.Import{
				{Path: "example/internal/b", Line: 3, Class: conformance.ImportInternal, TargetDir: "internal/b"},
			}},
			"internal/b/b.go": {Language: rule.LanguageGo, ImportsAvailable: true},
		})
	if err != nil {
		t.Fatal(err)
	}
	obs = obs.WithContent(conformance.NewMapContent(map[string]string{"internal/a/a.go": "package a", "internal/b/b.go": "package b"}))
	for _, excluded := range []string{"", "internal/b/b.go", "internal/b/**", "internal/a/a.go"} {
		t.Run(excluded, func(t *testing.T) {
			scope := repoScope(t)
			if excluded != "" {
				ex, err := rule.NewExclusion([]rule.Glob{mustGlob(t, excluded)}, nil, "exclude file access and evaluation")
				if err != nil {
					t.Fatal(err)
				}
				scope = scope.Excluding(ex)
			}
			r := mustRule(t, rule.Spec{ID: "features/independent", Type: rule.TypeIndependence,
				Params: rule.IndependenceParams{Folders: []rule.Glob{mustGlob(t, "internal/*")}}, Scope: scope})
			assessment, err := conformance.Run(conformance.Request{Rules: []rule.Rule{r}, Observations: obs})
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if excluded == "internal/a/a.go" {
				want = 0
			}
			violations := assessment.ActiveViolations()
			if len(violations) != want {
				t.Fatalf("exclude %q: got %d violations, want %d", excluded, len(violations), want)
			}
			if want == 1 && (violations[0].Path() != "internal/a/a.go" || assessment.Evaluations()[0].Outcome() != conformance.OutcomeViolates) {
				t.Fatalf("outgoing dependency lost its original subject or outcome: %+v", assessment)
			}
			if excluded == "" {
				return
			}
			supplied, err := conformance.NewFacts(r, nil, obs)
			if err != nil {
				t.Fatal(err)
			}
			denied := "internal/b/b.go"
			if excluded == "internal/a/a.go" {
				denied = excluded
			}
			if _, err := supplied.Read(denied); err == nil {
				t.Fatal("Folder evidence granted excluded file content")
			}
			if _, ok := supplied.FactsFor(denied); ok {
				t.Fatal("Folder evidence granted excluded file facts")
			}
		})
	}
}

func (p incomingFactsProbe) Evaluate(_ string, _ map[string]any, facts conformance.Facts,
	_ vocab.UbiquitousLanguage,
) ([]conformance.ExtensionFinding, error) {
	p.t.Helper()
	const subject = "shared/private.ts"
	const importer = "client/client.ts"
	if _, found := facts.FactsFor(importer); found {
		p.t.Fatal("incoming dependency granted access to importer declarations")
	}
	if _, err := facts.Read(importer); err == nil {
		p.t.Fatal("incoming dependency granted access to importer content")
	}
	var findings []conformance.ExtensionFinding
	for _, dependency := range facts.DependenciesFor(subject) {
		kind, target := dependency.Target()
		switch dependency.Line {
		case 7:
			if kind != "file" || target != subject || !reflect.DeepEqual(dependency.TargetZones, []rule.ZoneName{"private"}) {
				p.t.Fatalf("exact target borrowed a sibling's membership: %+v", dependency)
			}
		case 9:
			if kind != "directory" || target != "shared" || !reflect.DeepEqual(dependency.TargetZones, []rule.ZoneName{"private", "public"}) {
				p.t.Fatalf("package target lost directory precision or membership union: %+v", dependency)
			}
		default:
			p.t.Fatalf("unrelated import supplied as incident evidence: %+v", dependency)
		}
		findings = append(findings, conformance.ExtensionFinding{
			SubjectPath: subject, Path: dependency.SourcePath, Line: dependency.Line, Message: "private import",
		})
	}
	return findings, nil
}

func TestNativeAndExtensionUseSameDependencyPrecision(t *testing.T) {
	zones := []rule.Zone{
		mustZone(t, "private", "shared/private.ts"),
		mustZone(t, "public", "shared/public.ts"),
		mustZone(t, "client", "client/**"),
	}
	obs, err := conformance.NewObservations([]conformance.ObservedFile{
		{Path: "client/client.ts"}, {Path: "shared/private.ts"}, {Path: "shared/public.ts"},
	}, map[string]conformance.LanguageFacts{
		"client/client.ts": {Language: rule.LanguageTypeScript, ImportsAvailable: true, Imports: []conformance.Import{
			{Path: "../shared/private", Class: conformance.ImportInternal, TargetFile: "shared/private.ts", Line: 7},
			{Path: "../shared", Class: conformance.ImportInternal, TargetDir: "shared", Line: 9},
			{Path: "../shared/public", Class: conformance.ImportInternal, TargetFile: "shared/public.ts", Line: 11},
		}},
		"shared/private.ts": {Language: rule.LanguageTypeScript, ImportsAvailable: true},
		"shared/public.ts":  {Language: rule.LanguageTypeScript, ImportsAvailable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	rules := []rule.Rule{
		mustRule(t, rule.Spec{ID: "native", Type: rule.TypeProtected, Params: rule.ProtectedParams{Zone: "private"}, Scope: repoScope(t)}),
		mustRule(t, rule.Spec{ID: "extension", Type: rule.TypeExtension, Params: rule.ExtensionParams{Uses: "incoming"}, Scope: zoneScope(t, "private")}),
	}
	assessment, err := conformance.Run(conformance.Request{Rules: rules, Zones: zones, Observations: obs, Extensions: incomingFactsProbe{t}})
	if err != nil {
		t.Fatal(err)
	}
	lines := map[string][]int{}
	for _, violation := range assessment.ActiveViolations() {
		if violation.Path() != "client/client.ts" {
			t.Fatalf("dependency location changed to judged subject: %s", violation.Path())
		}
		id := violation.Rule().Qualified()
		lines[id] = append(lines[id], violation.Line())
	}
	for _, id := range []string{"native", "extension"} {
		if !reflect.DeepEqual(lines[id], []int{7, 9}) {
			t.Errorf("%s import findings = %v, want exact-private and package imports only", id, lines[id])
		}
	}
}

func TestNativeFactsKeepFailureDistinctFromMissingAndEmpty(t *testing.T) {
	zones := []rule.Zone{mustZone(t, "src", "src/**")}
	obs, err := conformance.NewObservations([]conformance.ObservedFile{
		{Path: "src/failed.go"}, {Path: "src/missing.go"}, {Path: "src/empty.go"},
	}, map[string]conformance.LanguageFacts{
		"src/failed.go": {Language: rule.LanguageGo, ImportsAvailable: true, ParseFailure: "syntax error"},
		"src/empty.go":  {Language: rule.LanguageGo, ImportsAvailable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	allow, err := rule.NewAllowList()
	if err != nil {
		t.Fatal(err)
	}
	r := mustRule(t, rule.Spec{ID: "imports", Type: rule.TypeConsumes, Params: rule.ConsumesParams{Internal: &allow}, Scope: zoneScope(t, "src")})
	assessment, err := conformance.Run(conformance.Request{Rules: []rule.Rule{r}, Zones: zones, Observations: obs})
	if err != nil {
		t.Fatal(err)
	}
	outcomes := map[string]conformance.Outcome{}
	for _, evaluation := range assessment.Evaluations() {
		outcomes[evaluation.Subject().Identity()] = evaluation.Outcome()
	}
	want := map[string]conformance.Outcome{
		"src/failed.go": conformance.OutcomeFailed, "src/missing.go": conformance.OutcomeUnsupported, "src/empty.go": conformance.OutcomeConforms,
	}
	if !reflect.DeepEqual(outcomes, want) {
		t.Fatalf("fact preparation erased availability distinctions: got %v, want %v", outcomes, want)
	}
}

func TestExcludedZoneStillSuppliesCycleEvidence(t *testing.T) {
	zones := []rule.Zone{mustZone(t, "alpha", "alpha/**"), mustZone(t, "beta", "beta/**")}
	obs, err := conformance.NewObservations([]conformance.ObservedFile{{Path: "alpha/a.go"}, {Path: "beta/b.go"}},
		map[string]conformance.LanguageFacts{
			"alpha/a.go": {Language: rule.LanguageGo, ImportsAvailable: true, Imports: []conformance.Import{{Path: "example/beta", Class: conformance.ImportInternal, TargetDir: "beta", Line: 3}}},
			"beta/b.go":  {Language: rule.LanguageGo, ImportsAvailable: true, Imports: []conformance.Import{{Path: "example/alpha", Class: conformance.ImportInternal, TargetDir: "alpha", Line: 5}}},
		})
	if err != nil {
		t.Fatal(err)
	}
	exclusion, err := rule.NewExclusion(nil, []rule.ZoneName{"beta"}, "do not judge beta")
	if err != nil {
		t.Fatal(err)
	}
	r := mustRule(t, rule.Spec{ID: "cycle", Type: rule.TypeAcyclic, Params: rule.AcyclicParams{Zones: []rule.ZoneName{"alpha", "beta"}}, Scope: repoScope(t).Excluding(exclusion)})
	assessment, err := conformance.Run(conformance.Request{Rules: []rule.Rule{r}, Zones: zones, Observations: obs})
	if err != nil {
		t.Fatal(err)
	}
	outcomes := map[string]conformance.Outcome{}
	for _, evaluation := range assessment.Evaluations() {
		outcomes[evaluation.Subject().Identity()] = evaluation.Outcome()
	}
	if outcomes["alpha"] != conformance.OutcomeViolates || outcomes["beta"] != conformance.OutcomeNotApplicable {
		t.Fatalf("excluding a judged Zone erased evidence for another Zone: %v", outcomes)
	}
	violations := assessment.ActiveViolations()
	if len(violations) != 1 || violations[0].Path() != "alpha/a.go" || violations[0].Line() != 3 {
		t.Fatalf("cycle witness changed: %+v", violations)
	}
}

func TestNativeRuleReceivesOnlyRequiredFactClasses(t *testing.T) {
	zones := []rule.Zone{mustZone(t, "src", "src/**")}
	obs, err := conformance.NewObservations([]conformance.ObservedFile{{Path: "src/a.go"}}, map[string]conformance.LanguageFacts{
		"src/a.go": {Language: rule.LanguageGo, ImportsAvailable: true, DeclarationsAvailable: true, CallsAvailable: true,
			Declarations: []conformance.Declaration{{Kind: "func", Name: "CollectedForAnotherRule"}},
			Calls:        []conformance.Call{{Callee: "CollectedForAnotherRule"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	allow, err := rule.NewAllowList()
	if err != nil {
		t.Fatal(err)
	}
	r := mustRule(t, rule.Spec{ID: "imports", Type: rule.TypeConsumes, Params: rule.ConsumesParams{Internal: &allow}, Scope: zoneScope(t, "src")})
	facts, err := conformance.NewFacts(r, zones, obs)
	if err != nil {
		t.Fatal(err)
	}
	file, ok := facts.FactsFor("src/a.go")
	if !ok || !file.Supports(rule.FactImports) {
		t.Fatalf("required empty import evidence disappeared: %+v", file)
	}
	if file.DeclarationsAvailable || len(file.Declarations) != 0 || file.CallsAvailable || len(file.Calls) != 0 {
		t.Fatalf("native rule received classes requested only by another rule: %+v", file)
	}
}

func TestExternalImportCannotClaimRepositoryTargetMembership(t *testing.T) {
	zones := []rule.Zone{mustZone(t, "src", "src/**"), mustZone(t, "target", "target/**")}
	obs, err := conformance.NewObservations([]conformance.ObservedFile{{Path: "src/a.go"}, {Path: "target/b.go"}}, map[string]conformance.LanguageFacts{
		"src/a.go": {Language: rule.LanguageGo, ImportsAvailable: true, Imports: []conformance.Import{
			// Classification is authoritative even if an adapter supplies a
			// target field. These observations are accepted by the constructor.
			{Path: "example.org/external", Class: conformance.ImportExternal, TargetFile: "target/b.go", Line: 4},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	r := mustRule(t, rule.Spec{ID: "external", Type: rule.TypeExtension, Params: rule.ExtensionParams{Uses: "probe"}, Scope: zoneScope(t, "src")})
	facts, err := conformance.NewFacts(r, zones, obs)
	if err != nil {
		t.Fatal(err)
	}
	dependencies := facts.ImportsFor("src/a.go")
	if len(dependencies) != 1 {
		t.Fatalf("external import evidence lost: %+v", dependencies)
	}
	kind, target := dependencies[0].Target()
	if kind != "unresolved" || target != "" || len(dependencies[0].TargetZones) != 0 {
		t.Fatalf("external classification acquired a repository target: %+v", dependencies[0])
	}
}

func TestExcludedFileRetainsStructureEvidenceWithoutReadAccess(t *testing.T) {
	zones := []rule.Zone{mustZone(t, "src", "src/**")}
	obs, err := conformance.NewObservations([]conformance.ObservedFile{{Path: "src/required.go"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	obs = obs.WithContent(conformance.NewMapContent(map[string]string{"src/required.go": "package src"}))
	exclusion, err := rule.NewExclusion([]rule.Glob{mustGlob(t, "src/required.go")}, nil, "do not judge or read this file")
	if err != nil {
		t.Fatal(err)
	}
	r := mustRule(t, rule.Spec{ID: "structure", Type: rule.TypeStructure,
		Params: rule.StructureParams{Require: []rule.Glob{mustGlob(t, "src/required.go")}},
		Scope:  zoneScope(t, "src").Excluding(exclusion),
	})
	facts, err := conformance.NewFacts(r, zones, obs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := facts.Read("src/required.go"); err == nil {
		t.Fatal("Zone composition evidence granted read access to an excluded file")
	}
	assessment, err := conformance.Run(conformance.Request{Rules: []rule.Rule{r}, Zones: zones, Observations: obs})
	if err != nil {
		t.Fatal(err)
	}
	evaluations := assessment.Evaluations()
	if len(evaluations) != 1 || evaluations[0].Subject().Identity() != "src" || evaluations[0].Outcome() != conformance.OutcomeConforms {
		t.Fatalf("excluded file vanished from the judged Zone's composition: %+v", evaluations)
	}
	if len(assessment.ActiveViolations()) != 0 {
		t.Fatalf("existing required file was reported missing: %+v", assessment.ActiveViolations())
	}
}
