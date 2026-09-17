package conformance_test

import (
	"reflect"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

func TestFactsEnforcesTheDomainBoundary(t *testing.T) {
	zones := []rule.Zone{mustZone(t, "src", "src/**"), mustZone(t, "target", "target/**")}
	exclusion, err := rule.NewExclusion([]rule.Glob{mustGlob(t, "src/excluded.go")}, nil, "not selected")
	if err != nil {
		t.Fatal(err)
	}
	r := mustRule(t, rule.Spec{
		ID: "test/facts", Type: rule.TypeExtension,
		Params: rule.ExtensionParams{Uses: "probe"},
		Scope:  zoneScope(t, "src").Excluding(exclusion),
	})
	facts := conformance.LanguageFacts{
		Language: rule.LanguageGo, ImportsAvailable: true, DeclarationsAvailable: true, CallsAvailable: true,
		Imports: []conformance.Import{{Path: "example.test/target", Class: conformance.ImportInternal, TargetDir: "target"}},
		Declarations: []conformance.Declaration{{
			Kind: "func", Name: "Selected", Params: []conformance.DeclarationParam{{Name: "x"}}, Results: []string{"int"},
		}},
		Calls: []conformance.Call{{Callee: "collectedForAnotherRule"}},
	}
	obs, err := conformance.NewObservations([]conformance.ObservedFile{
		{Path: "src/selected.go"}, {Path: "src/excluded.go"}, {Path: "target/target.go"},
	}, map[string]conformance.LanguageFacts{
		"src/selected.go": facts, "src/excluded.go": facts, "target/target.go": facts,
	})
	if err != nil {
		t.Fatal(err)
	}
	obs = obs.WithContent(conformance.NewMapContent(map[string]string{
		"src/selected.go": "selected", "src/excluded.go": "excluded", "target/target.go": "target",
	}))
	supplied, err := conformance.NewFacts(r, zones, obs)
	if err != nil {
		t.Fatal(err)
	}
	if got := supplied.Files(); len(got) != 1 || got[0].Path != "src/selected.go" {
		t.Fatalf("files outside Scope or Exclusions escaped: %+v", got)
	}
	for _, path := range []string{"src/excluded.go", "target/target.go", "src/../target/target.go"} {
		if _, ok := supplied.FactsFor(path); ok {
			t.Fatalf("facts escaped for %s", path)
		}
		if _, err := supplied.Read(path); err == nil {
			t.Fatalf("content escaped for %s", path)
		}
		if len(supplied.ZoneOf(path)) != 0 || len(supplied.ImportsFor(path)) != 0 {
			t.Fatalf("membership escaped for %s", path)
		}
	}
	if content, err := supplied.Read("src/selected.go"); err != nil || content != "selected" {
		t.Fatalf("selected content: %q, %v", content, err)
	}
	selected, ok := supplied.FactsFor("src/selected.go")
	if !ok || !selected.ImportsAvailable || !selected.DeclarationsAvailable {
		t.Fatalf("required facts missing: %+v", selected)
	}
	if selected.CallsAvailable || len(selected.Calls) != 0 {
		t.Fatalf("facts required by another Rule escaped: %+v", selected.Calls)
	}
	if got := supplied.ImportsFor("src/selected.go")[0].TargetZones; !reflect.DeepEqual(got, []rule.ZoneName{"target"}) {
		t.Fatalf("observed import membership = %v", got)
	}
	// Neither a recipient nor the original observation's owner can mutate
	// the value, including nested declaration signature data.
	selected.Imports[0].Path = "changed"
	selected.Declarations[0].Params[0].Name = "changed"
	selected.Declarations[0].Results[0] = "changed"
	observed, _ := obs.FactsFor("src/selected.go")
	observed.Declarations[0].Results[0] = "changed at source"
	again, _ := supplied.FactsFor("src/selected.go")
	if again.Imports[0].Path != "example.test/target" || again.Declarations[0].Params[0].Name != "x" || again.Declarations[0].Results[0] != "int" {
		t.Fatalf("Facts was mutated: %+v", again)
	}
	members := supplied.Zones()
	members["src"][0] = "changed"
	if supplied.Zones()["src"][0] != "src/selected.go" || len(supplied.Zones()["target"]) != 0 {
		t.Fatal("Zone membership was mutated or widened")
	}
}

func TestUnconstructedFactsGrantsNoAccess(t *testing.T) {
	var facts conformance.Facts
	if len(facts.Files()) != 0 || len(facts.Zones()) != 0 {
		t.Fatal("unconstructed value exposes files or Zones")
	}
	if _, ok := facts.FactsFor("any.go"); ok {
		t.Fatal("unconstructed value exposes facts")
	}
	if _, err := facts.Read("any.go"); err == nil {
		t.Fatal("unconstructed value permits reading")
	}
	if _, err := conformance.NewFacts(rule.Rule{}, nil, conformance.Observations{}); err == nil {
		t.Fatal("unconstructed Rule accepted")
	}
}
