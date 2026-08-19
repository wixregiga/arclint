package conformance_test

import (
	"errors"
	"strings"
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

func TestExtensionOutOfApplicabilityIsContained(t *testing.T) {
	// One out-of-scope report plus an in-scope finding: the whole
	// Extension run is untrustworthy, so neither becomes a Violation.
	outside := &fakeExtensions{findings: []conformance.ExtensionFinding{
		{Path: "m/dirty.go", Line: 3, Message: "in scope but discarded"},
		{Path: "elsewhere/b.go", Line: 2, Message: "second breach"},
		{Path: "elsewhere/a.go", Line: 9, Message: "first breach"},
		{Path: "elsewhere/a.go", Line: 4, Message: "earlier line"},
	}}
	req := extensionRequest(t, outside)
	// Keep one excluded subject to prove OutcomeNotApplicable survives.
	ex, err := rule.NewExclusion([]rule.Glob{mustGlob(t, "m/clean.go")}, nil, "legacy clean file")
	if err != nil {
		t.Fatalf("NewExclusion: %v", err)
	}
	req.Rules[0] = req.Rules[0].Exclude(ex)

	a, err := conformance.Run(req)
	if err != nil {
		t.Fatalf("Run: %v (Applicability breach must not abort the Assessment)", err)
	}
	if !a.HasErrors() {
		t.Errorf("HasErrors = false; error operational Diagnostics must fail the gate")
	}
	if got := a.ActiveViolations(); len(got) != 0 {
		t.Errorf("active violations = %v; breach must never become a Violation", violationKeys(got))
	}
	if got := a.Violations(); len(got) != 0 {
		t.Errorf("violations = %v; untrustworthy findings must be discarded", violationKeys(got))
	}

	outcomes := map[conformance.Outcome]int{}
	for _, e := range a.Evaluations() {
		outcomes[e.Outcome()]++
		switch e.Outcome() {
		case conformance.OutcomeFailed:
			if e.Subject().Identity() != "m/dirty.go" {
				t.Errorf("failed subject = %q, want only the selected file", e.Subject().Identity())
			}
		case conformance.OutcomeNotApplicable:
			if e.Subject().Identity() != "m/clean.go" {
				t.Errorf("not_applicable subject = %q, want the exclusion", e.Subject().Identity())
			}
		default:
			t.Errorf("unexpected outcome %q on %s", e.Outcome(), e.Subject().Identity())
		}
	}
	if outcomes[conformance.OutcomeFailed] != 1 || outcomes[conformance.OutcomeNotApplicable] != 1 {
		t.Errorf("outcomes = %v, want 1 failed selected + 1 not_applicable excluded", outcomes)
	}

	var ops []conformance.Diagnostic
	for _, d := range a.Diagnostics() {
		if d.Kind() != conformance.DiagnosticOperational || d.Severity() != rule.SeverityError {
			continue
		}
		ops = append(ops, d)
	}
	if len(ops) != 3 {
		t.Fatalf("error operational diagnostics = %d, want 3 breach reports; all=%v", len(ops), diagKeys(a))
	}
	// Deterministic by path, then line, then message.
	want := []struct {
		path string
		line int
	}{
		{"elsewhere/a.go", 4},
		{"elsewhere/a.go", 9},
		{"elsewhere/b.go", 2},
	}
	for i, w := range want {
		d := ops[i]
		if d.RuleID() != "t:m/no-panic" {
			t.Errorf("ops[%d].RuleID = %q, want t:m/no-panic", i, d.RuleID())
		}
		if d.Path() != w.path || d.Line() != w.line {
			t.Errorf("ops[%d] anchor = %s:%d, want %s:%d", i, d.Path(), d.Line(), w.path, w.line)
		}
		msg := d.Message()
		if !strings.Contains(msg, `extension "forbid-content"`) ||
			!strings.Contains(msg, w.path) ||
			!strings.Contains(msg, "outside the rule's applicability") ||
			!strings.Contains(msg, "t:m/no-panic") {
			t.Errorf("ops[%d].Message = %q, want rule/extension/path/applicability", i, msg)
		}
	}
}

func TestExtensionOutOfApplicabilityWithNoSelectedSubjects(t *testing.T) {
	// Declared module has no observed members: nothing is selected, yet
	// the Extension still reported a path. Diagnostics only — no
	// fabricated Evaluation.
	outside := &fakeExtensions{findings: []conformance.ExtensionFinding{
		{Path: "missing/registry.go", Line: 1, Message: "where is registry"},
	}}
	modules := []rule.Module{mustModule(t, "m", "m/**")}
	obs, err := conformance.NewObservations([]conformance.ObservedFile{
		{Path: "other/file.go"},
	}, nil)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	r := mustRule(t, rule.Spec{
		ID:            "t:m/require-registry",
		Type:          rule.TypeExtension,
		Params:        rule.ExtensionParams{Uses: "require-registry"},
		Applicability: moduleScope(t, "m"),
	})
	a, err := conformance.Run(conformance.Request{
		Rules:        []rule.Rule{r},
		Modules:      modules,
		Observations: obs,
		Extensions:   outside,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !a.HasErrors() {
		t.Errorf("HasErrors = false with breach Diagnostics")
	}
	if len(a.Evaluations()) != 0 {
		t.Errorf("evaluations = %d, want 0 when nothing was selected", len(a.Evaluations()))
	}
	if len(a.Violations()) != 0 {
		t.Errorf("violations = %v", violationKeys(a.Violations()))
	}
	var ops int
	for _, d := range a.Diagnostics() {
		if d.Kind() != conformance.DiagnosticOperational || d.Severity() != rule.SeverityError {
			continue
		}
		ops++
		if d.RuleID() != "t:m/require-registry" {
			t.Errorf("RuleID = %q", d.RuleID())
		}
		if d.Path() != "missing/registry.go" || d.Line() != 1 {
			t.Errorf("anchor = %s:%d", d.Path(), d.Line())
		}
		if !strings.Contains(d.Message(), `extension "require-registry"`) ||
			!strings.Contains(d.Message(), "missing/registry.go") {
			t.Errorf("Message = %q", d.Message())
		}
	}
	if ops != 1 {
		t.Errorf("error operational diagnostics = %d, want 1; all=%v", ops, diagKeys(a))
	}
}

func TestExtensionRuleContractBreaches(t *testing.T) {
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

func diagKeys(a conformance.Assessment) []string {
	var out []string
	for _, d := range a.Diagnostics() {
		out = append(out, string(d.Kind())+"|"+d.RuleID()+"|"+d.Path()+"|"+d.Message())
	}
	return out
}
