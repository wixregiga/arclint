package conformance_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// A domain service's assertion is checked by the operation it
// protects: the operation's own body makes a call the language's
// resolution follows to assert followed by the key, declared as a
// method of the service or as a function of the context. A check that
// merely exists, a check another operation calls, a check only a test
// declares, a function of the same name outside the context, a returned
// value object with an invariant of the same key, and tests named under
// verified_by do not check it. A call spelling the check whose target
// resolution cannot follow, and a file without resolved calls, leave
// it undetermined.
func TestServiceOperationCallsItsCheck(t *testing.T) {
	const (
		serviceFile = "internal/user/encrypter.go"
		checkFile   = "internal/user/password.go"
		testFile    = "internal/user/password_test.go"
		foreignFile = "internal/foreign/check.go"
	)
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "identity", Definition: "d", Line: 1,
		ValueObjects: []vocab.ValueObject{
			{Name: "EncryptedPassword", Definition: "d", Line: 2, Invariants: []vocab.Invariant{{Key: "not-empty", Statement: "s", Line: 3}}},
		},
		Services: []vocab.DomainService{{
			Name: "Encrypter", Definition: "d", Line: 4,
			Assertions: []vocab.Assertion{{Key: "not-empty", On: "Encrypt", Statement: "s", Line: 5,
				VerifiedBy: []string{testFile + ":TestEncryptNeverEmpty"}}},
		}},
	})
	id := "assertion/checked-by-its-operation"
	r := builtInRule(t, id)
	zones := []rule.Zone{mustZone(t, "identity", "internal/user/**")}
	value := conformance.Reference{Resolution: conformance.ResolutionDeclared, File: checkFile, Name: "EncryptedPassword"}
	checkFunc := conformance.Declaration{Kind: "func", Name: "AssertNotEmpty", Exported: true, StartLine: 20, Results: []string{"error"}}
	checkMethod := conformance.Declaration{Kind: "method", Name: "AssertNotEmpty", Owner: "Encrypter", Exported: true, StartLine: 30, Results: []string{"error"}}
	type state struct {
		service   []conformance.Declaration
		user      []conformance.Declaration
		test      []conformance.Declaration
		foreign   []conformance.Declaration
		calls     []conformance.Call
		unresolve bool
	}
	encrypter := []conformance.Declaration{
		{Kind: "struct", Name: "Encrypter", Exported: true, StartLine: 5},
		{Kind: "method", Name: "Encrypt", Owner: "Encrypter", Exported: true, StartLine: 10, Results: []string{"EncryptedPassword", "error"}, Type: value},
		{Kind: "method", Name: "Matches", Owner: "Encrypter", Exported: true, StartLine: 15, Results: []string{"bool"}},
	}
	call := func(enclosing string, line int, target conformance.Reference) conformance.Call {
		return conformance.Call{Callee: "AssertNotEmpty", Line: line, Enclosing: enclosing, EnclosingOwner: "Encrypter", Target: target}
	}
	run := func(s state) ([]string, conformance.Outcome) {
		t.Helper()
		service := goDecls("user", append(append([]conformance.Declaration(nil), encrypter...), s.service...), s.calls...)
		if !s.unresolve {
			service.ResolvedCallsAvailable = true
		}
		facts := map[string]conformance.LanguageFacts{
			serviceFile: service,
			checkFile: resolvedGo("user", append([]conformance.Declaration{
				{Kind: "struct", Name: "EncryptedPassword", Exported: true, StartLine: 3},
				{Kind: "func", Name: "NewEncryptedPassword", Exported: true, StartLine: 6, Results: []string{"EncryptedPassword", "error"}},
			}, s.user...)),
			testFile:    resolvedGo("user", s.test),
			foreignFile: resolvedGo("foreign", s.foreign),
		}
		a := runOne(t, r, zones, observed(serviceFile, checkFile, testFile, foreignFile), facts, knowledge)
		var out []string
		for _, v := range a.Violations() {
			out = append(out, v.Message())
			if want := "call AssertNotEmpty from Encrypt and fail the operation when it fails"; !strings.HasPrefix(v.Remediation(), "declare AssertNotEmpty as a method of Encrypter or a function of context identity") && v.Remediation() != want {
				t.Errorf("a finding does not say which check to call from the operation: %q", v.Remediation())
			}
		}
		return out, outcomesOf(a)[id+"|context:identity"]
	}
	notCalled := "service Encrypter: operation Encrypt does not call AssertNotEmpty, so assertion not-empty is not checked"
	for _, tc := range []struct {
		name    string
		s       state
		want    []string
		outcome conformance.Outcome
	}{
		{"the operation calls the service's method",
			state{service: []conformance.Declaration{checkMethod}, calls: []conformance.Call{call("Encrypt", 11, declared(serviceFile, "Encrypter", "AssertNotEmpty"))}},
			nil, conformance.OutcomeConforms},
		{"the operation calls a function of the context",
			state{user: []conformance.Declaration{checkFunc}, calls: []conformance.Call{call("Encrypt", 11, declared(checkFile, "", "AssertNotEmpty"))}},
			nil, conformance.OutcomeConforms},
		{"a function of the context nothing calls",
			state{user: []conformance.Declaration{checkFunc}},
			[]string{notCalled}, conformance.OutcomeViolates},
		{"another operation calls the check",
			state{user: []conformance.Declaration{checkFunc}, calls: []conformance.Call{call("Matches", 16, declared(checkFile, "", "AssertNotEmpty"))}},
			[]string{notCalled}, conformance.OutcomeViolates},
		{"a function of the same name outside the context",
			state{foreign: []conformance.Declaration{checkFunc}, calls: []conformance.Call{call("Encrypt", 11, declared(foreignFile, "", "AssertNotEmpty"))}},
			[]string{notCalled}, conformance.OutcomeViolates},
		{"a function only a test declares",
			state{test: []conformance.Declaration{checkFunc}, calls: []conformance.Call{call("Encrypt", 11, declared(testFile, "", "AssertNotEmpty"))}},
			[]string{notCalled}, conformance.OutcomeViolates},
		{"a call resolution cannot follow",
			state{user: []conformance.Declaration{checkFunc}, calls: []conformance.Call{call("Encrypt", 11, undetermined("AssertNotEmpty is a function value; the function it holds is not declared"))}},
			nil, conformance.OutcomeUndetermined},
		{"a file without resolved calls",
			state{user: []conformance.Declaration{checkFunc}, calls: []conformance.Call{call("Encrypt", 11, conformance.Reference{})}, unresolve: true},
			nil, conformance.OutcomeUndetermined},
		{"only a returned value object and tests",
			state{},
			[]string{notCalled}, conformance.OutcomeViolates},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, outcome := run(tc.s)
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") || outcome != tc.outcome {
				t.Errorf("\n got %q (%s)\nwant %q (%s)", got, outcome, tc.want, tc.outcome)
			}
		})
	}
}

// An aggregate's command calls its assertion's check like a service's
// operation does: a function of the context counts, a name alone does
// not.
func TestRootCommandCallsItsCheck(t *testing.T) {
	const file = "internal/tenant/tenant.go"
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "tenancy", Definition: "d", Line: 1,
		Aggregates: []vocab.Aggregate{{
			Name: "Tenant", Definition: "d", Identity: "TenantID", Line: 2,
			Assertions: []vocab.Assertion{{Key: "only-while-active", On: "OfferInvitation", Statement: "s", Line: 3}},
		}},
	})
	zones := []rule.Zone{mustZone(t, "tenancy", "internal/tenant/**")}
	run := func(target conformance.Reference) []string {
		facts := map[string]conformance.LanguageFacts{file: resolvedGo("tenant", []conformance.Declaration{
			{Kind: "struct", Name: "Tenant", Exported: true, StartLine: 3},
			{Kind: "method", Name: "OfferInvitation", Owner: "Tenant", Exported: true, StartLine: 10, Results: []string{"error"}},
			{Kind: "func", Name: "AssertOnlyWhileActive", Exported: true, StartLine: 20, Results: []string{"error"}},
		}, conformance.Call{Callee: "AssertOnlyWhileActive", Line: 11, Enclosing: "OfferInvitation", EnclosingOwner: "Tenant", Target: target})}
		a := runOne(t, builtInRule(t, "assertion/checked-by-its-operation"), zones, observed(file), facts, knowledge)
		var out []string
		for _, v := range a.Violations() {
			out = append(out, v.Message())
		}
		return out
	}
	if got := run(declared(file, "", "AssertOnlyWhileActive")); len(got) != 0 {
		t.Errorf("a command calling the function of the context: %q", got)
	}
	want := "aggregate Tenant: operation OfferInvitation does not call AssertOnlyWhileActive, so assertion only-while-active is not checked"
	if got := run(external("example.com/guard", "", "AssertOnlyWhileActive")); len(got) != 1 || got[0] != want {
		t.Errorf("a command calling another package's function of the same name:\n got %q\nwant %q", got, want)
	}
}

// The operation an assertion names has one place in the domain: one
// declaration, with a body, on the type that spells the service, in the
// code of the service's context. No declaration there, two or more, and
// one without a body are each a finding, whatever the type is declared
// as; a declaration in a test file, or on another type outside the
// context, is not the operation's place.
func TestOperationHasOnePlaceInTheDomain(t *testing.T) {
	const (
		userFile    = "internal/user/encrypter.go"
		legacyFile  = "internal/user/legacy/encrypter.go"
		userTest    = "internal/user/encrypter_test.go"
		adapterFile = "internal/adapters/passwords/pbkdf2.go"
	)
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "identity", Definition: "d", Line: 1,
		Services: []vocab.DomainService{{
			Name: "Encrypter", Definition: "d", Line: 2,
			Assertions: []vocab.Assertion{{Key: "not-empty", On: "Encrypt", Statement: "s", Line: 3}},
		}},
	})
	id := "assertion/checked-by-its-operation"
	zones := []rule.Zone{mustZone(t, "identity", "internal/user/**")}
	run := func(facts map[string]conformance.LanguageFacts) ([]string, conformance.Outcome) {
		t.Helper()
		var paths []string
		for p := range facts {
			paths = append(paths, p)
		}
		a := runOne(t, builtInRule(t, id), zones, observed(paths...), facts, knowledge)
		var out []string
		for _, v := range a.Violations() {
			out = append(out, v.Message())
		}
		return out, outcomesOf(a)[id+"|context:identity"]
	}
	iface := conformance.Declaration{Kind: "interface", Name: "Encrypter", Exported: true, StartLine: 5}
	ifaceOp := conformance.Declaration{Kind: "method", Name: "Encrypt", Owner: "Encrypter", Exported: true, Bodiless: true, StartLine: 6}
	concrete := conformance.Declaration{Kind: "struct", Name: "Encrypter", Exported: true, StartLine: 5}
	op := conformance.Declaration{Kind: "method", Name: "Encrypt", Owner: "Encrypter", Exported: true, StartLine: 10}
	adapter := resolvedGo("passwords", []conformance.Declaration{
		{Kind: "struct", Name: "PBKDF2Encrypter", Exported: true, StartLine: 5},
		{Kind: "method", Name: "Encrypt", Owner: "PBKDF2Encrypter", Exported: true, StartLine: 10},
	})
	for _, tc := range []struct {
		name  string
		facts map[string]conformance.LanguageFacts
		want  []string
	}{
		{"no declaration in the context, one in an adapter",
			map[string]conformance.LanguageFacts{userFile: resolvedGo("user", []conformance.Declaration{concrete}), adapterFile: adapter},
			[]string{"service Encrypter: assertion not-empty constrains operation Encrypt, which is not declared on Encrypter in the code of context identity"}},
		{"only a test declares it",
			map[string]conformance.LanguageFacts{userFile: resolvedGo("user", []conformance.Declaration{concrete}), userTest: resolvedGo("user", []conformance.Declaration{op})},
			[]string{"service Encrypter: assertion not-empty constrains operation Encrypt, which is not declared on Encrypter in the code of context identity"}},
		{"two declarations in the context",
			map[string]conformance.LanguageFacts{
				userFile:   resolvedGo("user", []conformance.Declaration{concrete, op}),
				legacyFile: resolvedGo("legacy", []conformance.Declaration{concrete, op}),
			},
			[]string{"service Encrypter: operation Encrypt is declared 2 times in the code of context identity, at internal/user/encrypter.go:10, internal/user/legacy/encrypter.go:10; assertion not-empty needs it in one place"}},
		{"one declaration without a body, an adapter beside it",
			map[string]conformance.LanguageFacts{userFile: resolvedGo("user", []conformance.Declaration{iface, ifaceOp}), adapterFile: adapter},
			[]string{"service Encrypter: operation Encrypt has no body, so it does not call AssertNotEmpty and assertion not-empty is not checked"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, outcome := run(tc.facts)
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") || outcome != conformance.OutcomeViolates {
				t.Errorf("\n got %q (%s)\nwant %q (violates)", got, outcome, tc.want)
			}
		})
	}
}

// A TypeScript abstract method has no body, so it is a finding like any
// other operation that does not call its check, whatever tests the
// assertion names; an abstract class's method with a body is judged by
// its calls.
func TestAbstractOperationIsAFinding(t *testing.T) {
	const file = "src/identity/encrypter.ts"
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "identity", Definition: "d", Line: 1,
		Services: []vocab.DomainService{{
			Name: "Encrypter", Definition: "d", Line: 2,
			Assertions: []vocab.Assertion{{Key: "not-empty", On: "encrypt", Statement: "s", Line: 3,
				VerifiedBy: []string{"src/identity/encrypter.test.ts:encrypts to a non-empty value"}}},
		}},
	})
	id := "assertion/checked-by-its-operation"
	run := func(op conformance.Declaration, calls ...conformance.Call) []string {
		facts := map[string]conformance.LanguageFacts{file: {
			Language: rule.LanguageTypeScript, DeclarationsAvailable: true, CallsAvailable: true, ResolvedCallsAvailable: true,
			Declarations: []conformance.Declaration{
				{Kind: "class", Name: "Encrypter", Exported: true, StartLine: 1},
				op,
				{Kind: "func", Name: "assertNotEmpty", Exported: true, StartLine: 9},
			},
			Calls: calls,
		}}
		a := runOne(t, builtInRule(t, id), nil, observed(file), facts, knowledge)
		var out []string
		for _, v := range a.Violations() {
			out = append(out, v.Message())
		}
		return out
	}
	abstract := conformance.Declaration{Kind: "method", Name: "encrypt", Owner: "Encrypter", Exported: true, Bodiless: true, StartLine: 2}
	want := "service Encrypter: operation encrypt has no body, so it does not call assertNotEmpty and assertion not-empty is not checked"
	if got := run(abstract); len(got) != 1 || got[0] != want {
		t.Errorf("an abstract operation:\n got %q\nwant %q", got, want)
	}
	withBody := conformance.Declaration{Kind: "method", Name: "encrypt", Owner: "Encrypter", Exported: true, StartLine: 2}
	call := conformance.Call{Callee: "assertNotEmpty", Line: 3, Enclosing: "encrypt", EnclosingOwner: "Encrypter",
		Target: conformance.Reference{Resolution: conformance.ResolutionDeclared, File: file, Name: "assertNotEmpty"}}
	if got := run(withBody, call); len(got) != 0 {
		t.Errorf("an abstract class's operation with a body that calls the check: %q", got)
	}
}
