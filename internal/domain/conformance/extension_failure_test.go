package conformance_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

func mustTimeout(t *testing.T, source string, limit time.Duration) conformance.ExtensionFailure {
	t.Helper()
	f, err := conformance.NewExtensionTimeout(source, limit)
	if err != nil {
		t.Fatalf("NewExtensionTimeout: %v", err)
	}
	return f
}

// stoppedRequest pairs an extension Rule with a naming Rule that finds
// one Violation, so the test sees what survives a stopped extension.
func stoppedRequest(t *testing.T, err error) conformance.Request {
	t.Helper()
	req := extensionRequest(t, &fakeExtensions{err: err})
	snake, cerr := rule.NewCaseSpec("snake_case")
	if cerr != nil {
		t.Fatal(cerr)
	}
	obs, oerr := conformance.NewObservations([]conformance.ObservedFile{
		{Path: "m/clean.go"},
		{Path: "m/dirty.go"},
		{Path: "m/BadName.go"},
	}, nil)
	if oerr != nil {
		t.Fatal(oerr)
	}
	req.Observations = obs
	req.Rules = append(req.Rules, mustRule(t, rule.Spec{
		ID: "m/snake", Type: rule.TypeNaming,
		Params: rule.NamingParams{Case: snake}, Scope: zoneScope(t, "m"),
	}))
	return req
}

func TestExtensionTimeoutStopsOnlyItsRule(t *testing.T) {
	failure := mustTimeout(t, ".arclint/extensions/content.ts", 5*time.Second)
	// The evaluator may wrap the failure; the check still recognizes it.
	a, err := conformance.Run(stoppedRequest(t, fmt.Errorf("evaluate: %w", failure)))
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

	var stopped []conformance.Diagnostic
	for _, d := range a.Diagnostics() {
		if d.Kind() == conformance.DiagnosticNotEvaluated {
			stopped = append(stopped, d)
		}
	}
	if len(stopped) != 1 {
		t.Fatalf("not-evaluated diagnostics = %v, want one", diagKeys(a))
	}
	want := `rule t/p:m/no-panic: extension "forbid-content": .arclint/extensions/content.ts exceeded its 5s time limit`
	if stopped[0].RuleID() != "t/p:m/no-panic" || stopped[0].Message() != want {
		t.Errorf("not evaluated = %q %q\nwant %q", stopped[0].RuleID(), stopped[0].Message(), want)
	}
	for _, d := range a.Diagnostics() {
		if d.Kind() == conformance.DiagnosticCoverage && d.RuleID() == "t/p:m/no-panic" {
			t.Errorf("coverage %q repeats the stopped Rule; one entry says it", d.Message())
		}
	}
}

func TestExtensionFaultNamesTheError(t *testing.T) {
	failure, err := conformance.NewExtensionFault("acme/web@1.0.0/extensions/web.ts", `rule "x": TypeError: boom`)
	if err != nil {
		t.Fatal(err)
	}
	a, err := conformance.Run(stoppedRequest(t, failure))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := `rule t/p:m/no-panic: extension "forbid-content": acme/web@1.0.0/extensions/web.ts failed: rule "x": TypeError: boom`
	for _, d := range a.Diagnostics() {
		if d.Kind() == conformance.DiagnosticNotEvaluated {
			if d.Message() != want {
				t.Errorf("message = %q\nwant      %q", d.Message(), want)
			}
			return
		}
	}
	t.Fatalf("no not-evaluated diagnostic: %v", diagKeys(a))
}

func TestOtherExtensionErrorsStillAbort(t *testing.T) {
	if _, err := conformance.Run(stoppedRequest(t, errors.New("params do not match"))); err == nil {
		t.Error("a configuration error must abort the check")
	}
}

func TestExtensionFailureStatesItsCause(t *testing.T) {
	for name, build := range map[string]func() (conformance.ExtensionFailure, error){
		"timeout without source": func() (conformance.ExtensionFailure, error) {
			return conformance.NewExtensionTimeout(" ", time.Second)
		},
		"timeout without limit": func() (conformance.ExtensionFailure, error) {
			return conformance.NewExtensionTimeout("a.ts", 0)
		},
		"fault without source": func() (conformance.ExtensionFailure, error) {
			return conformance.NewExtensionFault("", "boom")
		},
		"fault without cause": func() (conformance.ExtensionFailure, error) {
			return conformance.NewExtensionFault("a.ts", " ")
		},
	} {
		if f, err := build(); err == nil {
			t.Errorf("%s: accepted %v", name, f)
		}
	}
	if _, err := conformance.NewNotEvaluated("r", "x", conformance.ExtensionFailure{}); err == nil {
		t.Error("NewNotEvaluated accepted a failure with no cause")
	}
}

func TestAssessmentRejectsAStoppedRuleWithAResult(t *testing.T) {
	d, err := conformance.NewNotEvaluated("m/x", "x", mustTimeout(t, "a.ts", time.Second))
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
