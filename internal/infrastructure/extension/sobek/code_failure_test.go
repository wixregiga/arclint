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

// A check() that never returns is interrupted at the limit it was given
// and reported as code that timed out in check, with how long it ran and
// the file it last asked the host about.
func TestCheckTimeoutIsACodeFailure(t *testing.T) {
	root := writeExtensions(t, map[string]string{"spin.ts": `
import { defineRule } from "arclint";
export default defineRule({
  type: "spin",
  check(ctx) {
    ctx.read("a.go");
    ctx.read("b.go");
    for (;;) {}
  },
});
`})
	reg, err := sobekextension.LoadDir(root, sobekextension.Options{})
	if err != nil {
		t.Fatal(err)
	}
	rt := reg.Get("spin")
	params, err := rt.ValidateParams(nil)
	if err != nil {
		t.Fatal(err)
	}
	limit := 150 * time.Millisecond
	_, elapsed, err := rt.Check(fakeHost(map[string]string{"a.go": "package a", "b.go": "package b"}), params, limit)
	var failed *sobekextension.CodeFailure
	if !errors.As(err, &failed) {
		t.Fatalf("Check error = %v (%T), want a CodeFailure", err, err)
	}
	if !failed.TimedOut() || failed.Raised() != "" {
		t.Errorf("TimedOut = %v, Raised = %q; want a timeout with no raised error", failed.TimedOut(), failed.Raised())
	}
	if failed.Phase != conformance.PhaseCheck || failed.Limit != limit || failed.File != "b.go" {
		t.Errorf("failure = phase %q limit %s file %q; want check, %s, b.go", failed.Phase, failed.Limit, failed.File, limit)
	}
	if failed.Elapsed < limit || elapsed != failed.Elapsed {
		t.Errorf("Elapsed = %s (returned %s), want at least the %s limit", failed.Elapsed, elapsed, limit)
	}
	if failed.Source != ".arclint/extensions/spin.ts" {
		t.Errorf("Source = %q", failed.Source)
	}
}

// Registration that never finishes is a timeout of the extension's own
// code in the registration phase, not a malformed definition.
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
	if !failed.TimedOut() || failed.Phase != conformance.PhaseRegistration || failed.Limit != limit || failed.Elapsed < limit {
		t.Errorf("failure = timedOut %v phase %q limit %s elapsed %s; want a registration timeout at %s",
			failed.TimedOut(), failed.Phase, failed.Limit, failed.Elapsed, limit)
	}
	if failed.File != "" {
		t.Errorf("File = %q; registration reads no file", failed.File)
	}
}

// A malformed definition is not code that failed while running: the SDK
// marks it, and it stays a configuration error.
func TestMalformedDefinitionIsNotACodeFailure(t *testing.T) {
	for name, source := range map[string]string{
		"bare object": `export default { type: "bare" };`,
		"check not a function": `
import { defineRule } from "arclint";
export default defineRule({ type: "boom", check: "not a function" } as any);
`,
		"invalid capability": `
import { defineRule } from "arclint";
export default defineRule({ type: "cap", capability: "certain", check() {} } as any);
`,
		"missing type": `
import { defineRule } from "arclint";
export default defineRule({ type: "", check() {} });
`,
	} {
		t.Run(name, func(t *testing.T) {
			root := writeExtensions(t, map[string]string{"def.ts": source})
			_, err := sobekextension.LoadDir(root, sobekextension.Options{})
			var failed *sobekextension.CodeFailure
			if err == nil || errors.As(err, &failed) {
				t.Fatalf("LoadDir error = %v, want a configuration error", err)
			}
			if !strings.Contains(err.Error(), ".arclint/extensions/def.ts") {
				t.Errorf("error %q does not name the extension", err)
			}
		})
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
// with the error on one line and the Rule's limit, and keeps parameter
// errors as configuration errors.
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
	limit, err := rule.NewTimeLimit(7 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, err = evaluator.Evaluate("boom", nil, limit, failureFacts(t, "boom"), vocab.UbiquitousLanguage{})
	var failure conformance.ExtensionFailure
	if !errors.As(err, &failure) {
		t.Fatalf("Evaluate error = %v (%T), want an ExtensionFailure", err, err)
	}
	if failure.TimedOut() || failure.Phase() != conformance.PhaseCheck || failure.Limit() != 7*time.Second {
		t.Errorf("failure = %v; want a raised error in check under the 7s limit", failure)
	}
	if failure.Source() != ".arclint/extensions/boom.ts" {
		t.Errorf("Source = %q", failure.Source())
	}
	if raised := failure.Raised(); !strings.Contains(raised, "Error: cannot read the facts") || strings.Contains(raised, "\n") {
		t.Errorf("Raised = %q, want the thrown message on one line", raised)
	}

	_, err = evaluator.Evaluate("boom", map[string]any{"unknown": 1}, limit, failureFacts(t, "boom"), vocab.UbiquitousLanguage{})
	if err == nil || errors.As(err, &failure) {
		t.Errorf("params error = %v, want a configuration error, not an ExtensionFailure", err)
	}
}

// A check that finishes returns how long it ran.
func TestEvaluatorReportsElapsedTime(t *testing.T) {
	root := writeExtensions(t, map[string]string{"quick.ts": `
import { defineRule } from "arclint";
export default defineRule({ type: "quick", check() {} });
`})
	evaluator, err := sobekextension.NewEvaluator(root)
	if err != nil {
		t.Fatal(err)
	}
	run, err := evaluator.Evaluate("quick", nil, rule.DefaultTimeLimit, failureFacts(t, "quick"), vocab.UbiquitousLanguage{})
	if err != nil {
		t.Fatal(err)
	}
	if run.Elapsed <= 0 || run.Elapsed >= rule.DefaultTimeLimit.Duration() {
		t.Errorf("Elapsed = %s, want the time the check ran", run.Elapsed)
	}
}

// One extension that throws while registering leaves no registry, so
// every extension Rule names that extension and the registration phase.
// The inventory reports the same failure.
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
	_, err = evaluator.Evaluate("fine", nil, rule.DefaultTimeLimit, failureFacts(t, "fine"), vocab.UbiquitousLanguage{})
	var failure conformance.ExtensionFailure
	if !errors.As(err, &failure) {
		t.Fatalf("Evaluate error = %v (%T), want an ExtensionFailure", err, err)
	}
	if failure.Source() != ".arclint/extensions/aaa.ts" || failure.Phase() != conformance.PhaseRegistration ||
		!strings.Contains(failure.Raised(), "broken at load") || failure.Limit() != rule.DefaultTimeLimit.Duration() {
		t.Errorf("failure = %v, want aaa.ts failing in registration under the default limit", failure)
	}
	if _, err := evaluator.RegisteredExtensionRules(); !errors.As(err, &failure) {
		t.Errorf("RegisteredExtensionRules error = %v (%T), want the same ExtensionFailure", err, err)
	}
}

// Classifying what registration threw never runs extension code: a
// thrown object with a throwing name accessor, or a toString that throws
// or never returns, is still a CodeFailure with a readable cause.
func TestHostileThrownValuesAreCodeFailures(t *testing.T) {
	for name, tc := range map[string]struct{ throw, raised string }{
		"name accessor throws": {
			throw:  `throw Object.defineProperty({ toString() { return "plain" } }, "name", { get() { throw new Error("no name") } });`,
			raised: "registration phase: plain",
		},
		"toString throws": {
			throw:  `throw { toString() { throw new Error("no text") } };`,
			raised: "registration phase: a thrown value whose message could not be read",
		},
		"toString never returns": {
			throw:  `throw { toString() { for (;;) {} } };`,
			raised: "registration phase: a thrown value whose message could not be read",
		},
		"empty string": {
			throw:  `throw "";`,
			raised: "registration phase: at .arclint/extensions/hostile.ts:",
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := writeExtensions(t, map[string]string{"hostile.ts": `
import { defineRule } from "arclint";
` + tc.throw + `
export default defineRule({ type: "hostile", check() {} });
`})
			_, err := sobekextension.LoadDir(root, sobekextension.Options{RegisterTimeout: 300 * time.Millisecond})
			var failed *sobekextension.CodeFailure
			if !errors.As(err, &failed) {
				t.Fatalf("LoadDir error = %v (%T), want a CodeFailure", err, err)
			}
			if failed.TimedOut() || !strings.HasPrefix(failed.Raised(), tc.raised) {
				t.Errorf("Raised = %q (timed out %v), want it to start with %q", failed.Raised(), failed.TimedOut(), tc.raised)
			}
			if _, err := failed.ExtensionFailure(); err != nil {
				t.Errorf("ExtensionFailure: %v", err)
			}
		})
	}
}

// A params declaration that is not built with s is a malformed
// definition, not code that failed while running.
func TestMalformedParamsAreDefinitionErrors(t *testing.T) {
	for name, tc := range map[string]struct{ source, want string }{
		"params not a schema": {`defineRule({ type: "p", params: {} as any, check() {} })`, "defineRule: params must be a schema built with s"},
		"object property not a schema": {`defineRule({ type: "p", params: s.object({ depth: 2 as any }), check() {} })`, `s.object: property "depth" must be a schema built with s`},
		"array items not a schema": {`defineRule({ type: "p", params: s.object({ list: s.array("x" as any) }), check() {} })`, "s.array: items must be a schema built with s"},
	} {
		t.Run(name, func(t *testing.T) {
			root := writeExtensions(t, map[string]string{"params.ts": `
import { defineRule, s } from "arclint";
export default ` + tc.source + `;
`})
			_, err := sobekextension.LoadDir(root, sobekextension.Options{})
			var failed *sobekextension.CodeFailure
			if err == nil || errors.As(err, &failed) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("LoadDir error = %v, want a configuration error naming %q", err, tc.want)
			}
		})
	}
}

// A thrown Proxy whose traps throw is classified without running them
// outside protection, in both phases.
func TestThrownProxyIsACodeFailure(t *testing.T) {
	const proxy = `new Proxy({}, { getPrototypeOf() { throw new Error("trap"); }, get() { throw new Error("trap"); } })`
	root := writeExtensions(t, map[string]string{"proxy.ts": `
import { defineRule } from "arclint";
export default defineRule({ type: "proxy", check() { throw ` + proxy + `; } });
`})
	reg, err := sobekextension.LoadDir(root, sobekextension.Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = reg.Get("proxy").Check(fakeHost(nil), map[string]any{}, testLimit)
	var failed *sobekextension.CodeFailure
	if !errors.As(err, &failed) || failed.Phase != conformance.PhaseCheck || failed.Raised() == "" {
		t.Fatalf("Check error = %v (%T), want a CodeFailure in check", err, err)
	}

	root = writeExtensions(t, map[string]string{"proxy.ts": `
import { defineRule } from "arclint";
throw ` + proxy + `;
export default defineRule({ type: "proxy", check() {} });
`})
	_, err = sobekextension.LoadDir(root, sobekextension.Options{})
	if !errors.As(err, &failed) || failed.Phase != conformance.PhaseRegistration {
		t.Fatalf("LoadDir error = %v (%T), want a CodeFailure in registration", err, err)
	}
}

// A malformed defineRule called while checking is still the author's
// definition error, not code that failed.
func TestDefinitionErrorInCheckIsNotACodeFailure(t *testing.T) {
	root := writeExtensions(t, map[string]string{"late.ts": `
import { defineRule } from "arclint";
export default defineRule({ type: "late", check() { defineRule({ type: "x", check: 42 } as any); } });
`})
	reg, err := sobekextension.LoadDir(root, sobekextension.Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = reg.Get("late").Check(fakeHost(nil), map[string]any{}, testLimit)
	var failed *sobekextension.CodeFailure
	if err == nil || errors.As(err, &failed) || !strings.Contains(err.Error(), "defineRule: check must be a function") {
		t.Fatalf("Check error = %v (%T), want a configuration error naming the definition", err, err)
	}
}
