package conformance_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

func mustFailure(t *testing.T, spec conformance.ExtensionFailureSpec) conformance.ExtensionFailure {
	t.Helper()
	f, err := conformance.NewExtensionFailure(spec)
	if err != nil {
		t.Fatalf("NewExtensionFailure: %v", err)
	}
	return f
}

func checkTimeout(t *testing.T) conformance.ExtensionFailure {
	t.Helper()
	return mustFailure(t, conformance.ExtensionFailureSpec{
		Source: ".arclint/extensions/content.ts", Phase: conformance.PhaseCheck,
		Limit: 5 * time.Second, Elapsed: 5003 * time.Millisecond, TimedOut: true, File: "m/dirty.go",
	})
}

// stoppedRequest pairs an extension Rule with a naming Rule that finds
// one Violation, so the test sees what survives a stopped extension.
func stoppedRequest(t *testing.T, evaluator *fakeExtensions) conformance.Request {
	t.Helper()
	req := extensionRequest(t, evaluator)
	snake, err := rule.NewCaseSpec("snake_case")
	if err != nil {
		t.Fatal(err)
	}
	obs, err := conformance.NewObservations([]conformance.ObservedFile{
		{Path: "m/clean.go"},
		{Path: "m/dirty.go"},
		{Path: "m/BadName.go"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Observations = obs
	req.Rules = append(req.Rules, mustRule(t, rule.Spec{
		ID: "m/snake", Type: rule.TypeNaming,
		Params: rule.NamingParams{Case: snake}, Scope: zoneScope(t, "m"),
	}))
	return req
}

func notEvaluated(a conformance.Assessment) []conformance.Diagnostic {
	var out []conformance.Diagnostic
	for _, d := range a.Diagnostics() {
		if d.Kind() == conformance.DiagnosticNotEvaluated {
			out = append(out, d)
		}
	}
	return out
}

func TestExtensionTimeoutStopsOnlyItsRule(t *testing.T) {
	failure := checkTimeout(t)
	// The evaluator may wrap the failure; the check still recognizes it.
	a, err := conformance.Run(stoppedRequest(t, &fakeExtensions{err: fmt.Errorf("evaluate: %w", failure)}))
	if err != nil {
		t.Fatalf("Run: %v; a timed-out extension must not abort the check", err)
	}
	if a.Complete() {
		t.Error("Complete = true with a Rule not evaluated")
	}
	if got, want := a.NotEvaluatedRules(), []string{"t/p:m/no-panic"}; !reflect.DeepEqual(got, want) {
		t.Errorf("NotEvaluatedRules = %v, want %v", got, want)
	}
	if got, want := a.AppliedRules(), []string{"m/snake"}; !reflect.DeepEqual(got, want) {
		t.Errorf("AppliedRules = %v, want %v; a stopped Rule was not applied", got, want)
	}
	for _, e := range a.Evaluations() {
		if e.Rule().Qualified() == "t/p:m/no-panic" {
			t.Errorf("stopped Rule has evaluation %s on %s; it has no result", e.Outcome(), e.Subject())
		}
	}
	if got := violationKeys(a.ActiveViolations()); len(got) != 1 {
		t.Errorf("active violations = %v, want the naming finding", got)
	}
	if !a.HasErrors() {
		t.Error("HasErrors = false; the other Rule's finding still gates")
	}

	stopped := notEvaluated(a)
	if len(stopped) != 1 {
		t.Fatalf("not-evaluated diagnostics = %v, want one", diagKeys(a))
	}
	want := `rule t/p:m/no-panic: extension "forbid-content" (.arclint/extensions/content.ts) reached its 5s time limit in check and was stopped after 5.003s, last reading m/dirty.go`
	if stopped[0].RuleID() != "t/p:m/no-panic" || stopped[0].Message() != want {
		t.Errorf("not evaluated = %q %q\nwant %q", stopped[0].RuleID(), stopped[0].Message(), want)
	}
	extension, carried, ok := stopped[0].NotEvaluated()
	if !ok || extension != "forbid-content" || carried != failure {
		t.Errorf("NotEvaluated = %q %v %v, want the extension and its failure", extension, carried, ok)
	}
	for _, d := range a.Diagnostics() {
		if d.Kind() == conformance.DiagnosticCoverage && d.RuleID() == "t/p:m/no-panic" {
			t.Errorf("coverage %q repeats the stopped Rule; one entry says it", d.Message())
		}
	}
}

func TestExtensionFaultNamesTheError(t *testing.T) {
	failure := mustFailure(t, conformance.ExtensionFailureSpec{
		Source: "acme/web@1.0.0/extensions/web.ts", Phase: conformance.PhaseRegistration,
		Limit: 5 * time.Second, Elapsed: 12 * time.Millisecond, Raised: "registration phase: TypeError:\n boom",
	})
	a, err := conformance.Run(stoppedRequest(t, &fakeExtensions{err: failure}))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	stopped := notEvaluated(a)
	want := `rule t/p:m/no-panic: extension "forbid-content" (acme/web@1.0.0/extensions/web.ts) failed in registration after 12ms of its 5s time limit: registration phase: TypeError: boom`
	if len(stopped) != 1 || stopped[0].Message() != want {
		t.Fatalf("not evaluated = %v\nwant %q", diagKeys(a), want)
	}
}

func TestOtherExtensionErrorsStillAbort(t *testing.T) {
	if _, err := conformance.Run(stoppedRequest(t, &fakeExtensions{err: errors.New("params do not match")})); err == nil {
		t.Error("a configuration error must abort the check")
	}
}

// The check runs under the Rule's TimeLimit, and a check that used more
// than half of it says so: a run twice as slow would stop.
func TestExtensionNearItsTimeLimitIsReported(t *testing.T) {
	limit, err := rule.NewTimeLimit(4 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		elapsed time.Duration
		near    bool
	}{
		{elapsed: 2 * time.Second, near: false},
		{elapsed: 2001 * time.Millisecond, near: true},
	} {
		evaluator := &fakeExtensions{elapsed: tc.elapsed}
		req := extensionRequest(t, evaluator)
		req.Rules[0], err = req.Rules[0].WithTimeLimit(limit)
		if err != nil {
			t.Fatal(err)
		}
		a, err := conformance.Run(req)
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if evaluator.saw.limit != limit {
			t.Errorf("evaluator ran under %s, want the Rule's %s", evaluator.saw.limit, limit)
		}
		var warned []string
		for _, d := range a.Diagnostics() {
			if d.Kind() == conformance.DiagnosticCoverage && strings.Contains(d.Message(), "time limit") {
				warned = append(warned, d.Message())
			}
		}
		if !tc.near {
			if len(warned) != 0 {
				t.Errorf("elapsed %s of %s warned %v; exactly half is not near", tc.elapsed, limit, warned)
			}
			continue
		}
		want := `rule t/p:m/no-panic: extension "forbid-content" took 2.001s of its 4s time limit; a run twice as slow would not be evaluated, so make the check faster or raise time_limit`
		if len(warned) != 1 || warned[0] != want {
			t.Errorf("warnings = %v\nwant %q", warned, want)
		}
		if !a.Complete() {
			t.Error("a check near its limit still evaluated its Rule")
		}
	}
}

func TestExtensionRuleRunsUnderTheDefaultLimit(t *testing.T) {
	evaluator := &fakeExtensions{}
	if _, err := conformance.Run(extensionRequest(t, evaluator)); err != nil {
		t.Fatal(err)
	}
	if evaluator.saw.limit != rule.DefaultTimeLimit {
		t.Errorf("limit = %s, want the default %s", evaluator.saw.limit, rule.DefaultTimeLimit)
	}
}

func TestExtensionFailureStatesItsCircumstances(t *testing.T) {
	valid := conformance.ExtensionFailureSpec{
		Source: "a.ts", Phase: conformance.PhaseCheck, Limit: time.Second, Elapsed: time.Second, TimedOut: true,
	}
	for name, change := range map[string]func(*conformance.ExtensionFailureSpec){
		"no source":            func(s *conformance.ExtensionFailureSpec) { s.Source = " " },
		"no phase":             func(s *conformance.ExtensionFailureSpec) { s.Phase = "" },
		"unknown phase":        func(s *conformance.ExtensionFailureSpec) { s.Phase = "evaluation" },
		"no limit":             func(s *conformance.ExtensionFailureSpec) { s.Limit = 0 },
		"negative elapsed":     func(s *conformance.ExtensionFailureSpec) { s.Elapsed = -time.Millisecond },
		"no cause":             func(s *conformance.ExtensionFailureSpec) { s.TimedOut = false },
		"blank raised error":   func(s *conformance.ExtensionFailureSpec) { s.TimedOut, s.Raised = false, " \n" },
		"two causes":           func(s *conformance.ExtensionFailureSpec) { s.Raised = "boom" },
		"file in registration": func(s *conformance.ExtensionFailureSpec) { s.Phase, s.File = conformance.PhaseRegistration, "a.go" },
	} {
		spec := valid
		change(&spec)
		if f, err := conformance.NewExtensionFailure(spec); err == nil {
			t.Errorf("%s: accepted %v", name, f)
		}
	}
	if _, err := conformance.NewExtensionFailure(valid); err != nil {
		t.Errorf("valid spec rejected: %v", err)
	}
	if _, err := conformance.NewNotEvaluated("r", "x", conformance.ExtensionFailure{}); err == nil {
		t.Error("NewNotEvaluated accepted a failure with no cause")
	}
}

func TestAssessmentRejectsAStoppedRuleWithAResult(t *testing.T) {
	d, err := conformance.NewNotEvaluated("m/x", "x", checkTimeout(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conformance.NewAssessment(nil, []conformance.Diagnostic{d}, []string{"m/x"}); err == nil {
		t.Error("a Rule both applied and not evaluated was accepted")
	}
	if _, err := conformance.NewAssessment(nil, []conformance.Diagnostic{d, d}, nil); err == nil {
		t.Error("a Rule not evaluated twice was accepted")
	}
	r := mustRule(t, rule.Spec{
		ID: "m/x", Type: rule.TypeExtension,
		Params: rule.ExtensionParams{Uses: "x"}, Scope: repoScope(t),
	})
	subject, err := rule.FileSubject("a.go")
	if err != nil {
		t.Fatal(err)
	}
	e, err := conformance.NewEvaluation(r.ID(), subject, conformance.OutcomeUndetermined, r.Enforcement(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conformance.NewAssessment([]conformance.Evaluation{e}, []conformance.Diagnostic{d}, nil); err == nil {
		t.Error("a Rule not evaluated with an Evaluation was accepted")
	}
	a, err := conformance.NewAssessment(nil, []conformance.Diagnostic{d}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.WithDiagnostics(d); err == nil {
		t.Error("WithDiagnostics added a not-evaluated entry after the check")
	}
}
