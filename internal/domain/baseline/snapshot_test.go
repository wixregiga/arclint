package baseline_test

import (
	"testing"

	"github.com/wixregiga/arclint/internal/domain/baseline"
	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// assess runs a one-rule naming check over the given file names.
func assess(t *testing.T, paths ...string) conformance.Assessment {
	t.Helper()
	glob, err := rule.NewGlob("m/**")
	if err != nil {
		t.Fatalf("NewGlob: %v", err)
	}
	module, err := rule.NewModule("m", "", []rule.Glob{glob})
	if err != nil {
		t.Fatalf("NewModule: %v", err)
	}
	files := make([]conformance.ObservedFile, 0, len(paths))
	for _, p := range paths {
		files = append(files, conformance.ObservedFile{Path: p})
	}
	obs, err := conformance.NewObservations(files, nil)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	snake, err := rule.NewCaseSpec("snake_case")
	if err != nil {
		t.Fatalf("NewCaseSpec: %v", err)
	}
	scope, err := rule.ModuleApplicability([]rule.ModuleName{"m"})
	if err != nil {
		t.Fatalf("ModuleApplicability: %v", err)
	}
	r, err := rule.New(rule.Spec{
		ID:            "t:m/snake",
		Type:          rule.TypeNaming,
		Params:        rule.NamingParams{Case: snake},
		Applicability: scope,
	})
	if err != nil {
		t.Fatalf("rule.New: %v", err)
	}
	a, err := conformance.Run(conformance.Request{
		Rules:        []rule.Rule{r},
		Modules:      []rule.Module{module},
		Observations: obs,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return a
}

func TestCaptureAndApply(t *testing.T) {
	before := assess(t, "m/BadName.go", "m/ok.go")
	if got := len(before.ActiveViolations()); got != 1 {
		t.Fatalf("scenario produced %d active violations, want 1", got)
	}

	snapshot, err := baseline.Capture(before, "")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if len(snapshot.CapturedRules()) != 1 || snapshot.CapturedRules()[0] != "t:m/snake" {
		t.Errorf("captured rules = %v", snapshot.CapturedRules())
	}
	if !snapshot.Covers(before.ActiveViolations()[0]) {
		t.Errorf("snapshot must cover the finding it captured")
	}

	after, stale, err := snapshot.Apply(before)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(stale) != 0 {
		t.Errorf("stale = %v, want none", stale)
	}
	if len(after.ActiveViolations()) != 0 || len(after.BaselinedViolations()) != 1 {
		t.Errorf("apply: active %d baselined %d, want 0 and 1",
			len(after.ActiveViolations()), len(after.BaselinedViolations()))
	}
	if after.HasErrors() {
		t.Errorf("a fully baselined assessment must pass the gate")
	}
	if before.ActiveViolations()[0].Status() != conformance.StatusActive {
		t.Errorf("Apply mutated the original assessment")
	}
}

func TestStaleEntriesSignalRefresh(t *testing.T) {
	before := assess(t, "m/BadName.go", "m/ok.go")
	snapshot, err := baseline.Capture(before, "")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	fixed := assess(t, "m/good_name.go", "m/ok.go")
	stale, err := snapshot.StaleEntries(fixed)
	if err != nil {
		t.Fatalf("StaleEntries: %v", err)
	}
	if len(stale) != 1 || stale[0].RuleID() != "t:m/snake" || stale[0].Count() != 1 {
		t.Errorf("stale = %v, want the captured finding once", stale)
	}
}

func TestAmbiguousIdentitiesRejected(t *testing.T) {
	e, err := baseline.NewEntry("t:m/snake", "m/BadName.go", "broken", 1)
	if err != nil {
		t.Fatalf("NewEntry: %v", err)
	}
	if _, err := baseline.New(nil, []baseline.Entry{e, e}, ""); err == nil {
		t.Errorf("duplicate fingerprints must be rejected")
	}
	if _, err := baseline.NewEntry("t:m/snake", "", "broken", 1); err == nil {
		t.Errorf("partial finding identity must be rejected")
	}
}
