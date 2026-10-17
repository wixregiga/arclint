package sobekextension_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
	sobekextension "github.com/wixregiga/arclint/internal/infrastructure/extension/sobek"
)

// A check() that never returns is interrupted at its limit and reported
// as code that timed out, with the limit and the extension's source.
func TestCheckTimeoutIsACodeFailure(t *testing.T) {
	root := writeExtensions(t, map[string]string{"spin.ts": `
import { defineRule } from "arclint";
export default defineRule({ type: "spin", check() { for (;;) {} } });
`})
	limit := 150 * time.Millisecond
	reg, err := sobekextension.LoadDir(root, sobekextension.Options{CheckTimeout: limit})
	if err != nil {
		t.Fatal(err)
	}
	rt := reg.Get("spin")
	params, err := rt.ValidateParams(nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = rt.Check(fakeHost(nil), params)
	var failed *sobekextension.CodeFailure
	if !errors.As(err, &failed) {
		t.Fatalf("Check error = %v (%T), want a CodeFailure", err, err)
	}
	if got, ok := failed.TimeLimit(); !ok || got != limit {
		t.Errorf("TimeLimit = %s %v, want %s true", got, ok, limit)
	}
	if failed.Source != ".arclint/extensions/spin.ts" {
		t.Errorf("Source = %q", failed.Source)
	}
}

// Registration that never finishes is a timeout of the extension's own
// code, not a malformed definition.
func TestRegistrationTimeoutIsACodeFailure(t *testing.T) {
	root := writeExtensions(t, map[string]string{"hang.ts": `
import { defineRule } from "arclint";
for (;;) {}
export default defineRule({ type: "hang", check() {} });
`})
	limit := 150 * time.Millisecond
	_, err := sobekextension.LoadDir(root, sobekextension.Options{RegisterTimeout: limit})
	var failed *sobekextension.CodeFailure
	if !errors.As(err, &failed) {
		t.Fatalf("LoadDir error = %v (%T), want a CodeFailure", err, err)
	}
	if got, ok := failed.TimeLimit(); !ok || got != limit {
		t.Errorf("TimeLimit = %s %v, want %s true", got, ok, limit)
	}
	if !strings.Contains(err.Error(), "registration exceeded the 150ms timeout") {
		t.Errorf("error = %q, want the registration limit named", err)
	}
}

// A malformed definition is not code that failed while running: it
// stays a configuration error.
func TestMalformedDefinitionIsNotACodeFailure(t *testing.T) {
	root := writeExtensions(t, map[string]string{"bare.ts": `export default { type: "bare" };`})
	_, err := sobekextension.LoadDir(root, sobekextension.Options{})
	var failed *sobekextension.CodeFailure
	if err == nil || errors.As(err, &failed) {
		t.Fatalf("LoadDir error = %v, want a configuration error", err)
	}
}

func failureFacts(t *testing.T, uses string) conformance.Facts {
	t.Helper()
	obs, err := conformance.NewObservations([]conformance.ObservedFile{{Path: "a.go"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	glob, err := rule.NewGlob("a.go")
	if err != nil {
		t.Fatal(err)
	}
	scope, err := rule.RepositoryScope(glob)
	if err != nil {
		t.Fatal(err)
	}
	r, err := rule.New(rule.Spec{
		ID: "test/failure", Type: rule.TypeExtension,
		Params: rule.ExtensionParams{Uses: uses}, Scope: scope,
	})
	if err != nil {
		t.Fatal(err)
	}
	facts, err := conformance.NewFacts(r, nil, obs)
	if err != nil {
		t.Fatal(err)
	}
	return facts
}

// The evaluator hands the domain an ExtensionFailure for code that threw,
// with the error on one line, and keeps parameter errors as configuration
// errors.
func TestEvaluatorReportsThrownErrorAsExtensionFailure(t *testing.T) {
	root := writeExtensions(t, map[string]string{"boom.ts": `
import { defineRule } from "arclint";
export default defineRule({
  type: "boom",
  check() { throw new Error("cannot read\nthe facts"); },
});
`})
	evaluator, err := sobekextension.NewEvaluator(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = evaluator.Evaluate("boom", nil, failureFacts(t, "boom"), vocab.UbiquitousLanguage{})
	var failure conformance.ExtensionFailure
	if !errors.As(err, &failure) {
		t.Fatalf("Evaluate error = %v (%T), want an ExtensionFailure", err, err)
	}
	if _, timedOut := failure.TimeLimit(); timedOut {
		t.Errorf("a thrown error is not a timeout: %v", failure)
	}
	if failure.Source() != ".arclint/extensions/boom.ts" {
		t.Errorf("Source = %q", failure.Source())
	}
	if cause := failure.Cause(); !strings.Contains(cause, "Error: cannot read the facts") || strings.Contains(cause, "\n") {
		t.Errorf("Cause = %q, want the thrown message on one line", cause)
	}

	_, err = evaluator.Evaluate("boom", map[string]any{"unknown": 1}, failureFacts(t, "boom"), vocab.UbiquitousLanguage{})
	if err == nil || errors.As(err, &failure) {
		t.Errorf("params error = %v, want a configuration error, not an ExtensionFailure", err)
	}
}

// One extension that throws while registering leaves no registry, so
// every extension Rule names that extension as the cause.
func TestEvaluatorReportsRegistrationErrorAsExtensionFailure(t *testing.T) {
	root := writeExtensions(t, map[string]string{
		"aaa.ts": `
import { defineRule } from "arclint";
throw new Error("broken at load");
export default defineRule({ type: "never", check() {} });
`,
		"ok.ts": `
import { defineRule } from "arclint";
export default defineRule({ type: "fine", check() {} });
`,
	})
	evaluator, err := sobekextension.NewEvaluator(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = evaluator.Evaluate("fine", nil, failureFacts(t, "fine"), vocab.UbiquitousLanguage{})
	var failure conformance.ExtensionFailure
	if !errors.As(err, &failure) {
		t.Fatalf("Evaluate error = %v (%T), want an ExtensionFailure", err, err)
	}
	if failure.Source() != ".arclint/extensions/aaa.ts" || !strings.Contains(failure.Cause(), "broken at load") {
		t.Errorf("failure = %v, want aaa.ts and its error", failure)
	}
}
