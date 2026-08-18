package conformance_test

import (
	"errors"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

type fakeExtensions struct {
	findings []conformance.ExtensionFinding
	err      error
	saw      struct {
		extension string
		params    map[string]any
		subjects  []string
	}
}

func (f *fakeExtensions) Evaluate(extension string, params map[string]any, subjects []string,
	modules []rule.Module, obs conformance.Observations,
) ([]conformance.ExtensionFinding, error) {
	f.saw.extension = extension
	f.saw.params = params
	f.saw.subjects = subjects
	return f.findings, f.err
}

func extensionRequest(t *testing.T, evaluator conformance.ExtensionEvaluator) conformance.Request {
	t.Helper()
	modules := []rule.Module{mustModule(t, "m", "m/**")}
	obs, err := conformance.NewObservations([]conformance.ObservedFile{
		{Path: "m/clean.go"},
		{Path: "m/dirty.go"},
	}, nil)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	r := mustRule(t, rule.Spec{
		ID:            "t:m/no-panic",
		Type:          rule.TypeExtension,
		Params:        rule.ExtensionParams{Uses: "forbid-content", With: map[string]any{"pattern": `\bpanic\(`}},
		Applicability: moduleScope(t, "m"),
	})
	return conformance.Request{
		Rules:        []rule.Rule{r},
		Modules:      modules,
		Observations: obs,
		Extensions:   evaluator,
	}
}

func TestExtensionRuleDelegatesHonestly(t *testing.T) {
	evaluator := &fakeExtensions{findings: []conformance.ExtensionFinding{
		{Path: "m/dirty.go", Line: 7, Message: "forbidden content", Remediation: "remove it"},
	}}
	a, err := conformance.Run(extensionRequest(t, evaluator))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if evaluator.saw.extension != "forbid-content" || evaluator.saw.params["pattern"] != `\bpanic\(` {
		t.Errorf("evaluator saw %q %v", evaluator.saw.extension, evaluator.saw.params)
	}
	if len(evaluator.saw.subjects) != 2 {
		t.Errorf("subjects = %v, want both selected files", evaluator.saw.subjects)
	}

	active := a.ActiveViolations()
	if len(active) != 1 || active[0].Path() != "m/dirty.go" || active[0].Line() != 7 {
		t.Fatalf("active = %v", violationKeys(active))
	}
	// Heuristic extension evidence: findings are suspected violations
	// and still gate at error severity; absence of findings is
	// undetermined, never conformance.
	if active[0].Outcome() != conformance.OutcomeSuspectedViolation || !active[0].FailsGate() {
		t.Errorf("outcome = %q gate %v", active[0].Outcome(), active[0].FailsGate())
	}
	outcomes := map[conformance.Outcome]int{}
	for _, e := range a.Evaluations() {
		outcomes[e.Outcome()]++
	}
	if outcomes[conformance.OutcomeUndetermined] != 1 || outcomes[conformance.OutcomeConforms] != 0 {
		t.Errorf("outcomes = %v; heuristic evidence must not produce conformance", outcomes)
	}
}

func TestExtensionRuleContractBreaches(t *testing.T) {
	outside := &fakeExtensions{findings: []conformance.ExtensionFinding{
		{Path: "elsewhere/file.go", Message: "out of scope"},
	}}
	if _, err := conformance.Run(extensionRequest(t, outside)); err == nil {
		t.Errorf("a finding outside the rule's applicability must be rejected")
	}

	broken := &fakeExtensions{err: errors.New("extension threw")}
	if _, err := conformance.Run(extensionRequest(t, broken)); err == nil {
		t.Errorf("an extension failure must stop the check loudly")
	}

	a, err := conformance.Run(extensionRequest(t, nil))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	unsupported := 0
	for _, e := range a.Evaluations() {
		if e.Outcome() == conformance.OutcomeUnsupported {
			unsupported++
		}
	}
	if unsupported != 2 {
		t.Errorf("without a mechanism every subject must evaluate unsupported, got %d", unsupported)
	}
}
