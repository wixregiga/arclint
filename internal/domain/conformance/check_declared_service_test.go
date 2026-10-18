package conformance_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// A domain service declared as an interface in one package returns a
// value object of another: the guarantee is the value's invariant only
// when the language's resolution names that value's own declaration as
// the operation's result, never a type that shares its name, a
// collection of it, or a value whose invariant has another key. A
// result resolution cannot follow leaves the guarantee undetermined. A
// top-level function of the context spelled from the key checks it
// too, unless only a test declares it.
func TestDeclaredServiceIsCheckedInTheModel(t *testing.T) {
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "identity", Definition: "d", Line: 1,
		ValueObjects: []vocab.ValueObject{
			{Name: "Password", Definition: "d", Line: 2, Invariants: []vocab.Invariant{{Key: "not-weak", Statement: "s", Line: 3}}},
			{Name: "Username", Definition: "d", Line: 4, Invariants: []vocab.Invariant{{Key: "length-within-bounds", Statement: "s", Line: 5}}},
		},
		Services: []vocab.DomainService{{
			Name: "PasswordGenerator", Definition: "d", Line: 6,
			Assertions: []vocab.Assertion{{Key: "not-weak", On: "GenerateStrongPassword", Statement: "s", Line: 7}},
		}},
	})
	id := "assertion/checked-by-its-operation"
	r := builtInRule(t, id)
	zone, err := rule.NewZone("identity", "", []rule.Glob{mustGlob(t, "internal/user/**"), mustGlob(t, "internal/provisioning/**")})
	if err != nil {
		t.Fatal(err)
	}
	zones := []rule.Zone{zone}
	const (
		userFile      = "internal/user/password.go"
		userTest      = "internal/user/password_test.go"
		generatorFile = "internal/provisioning/generator.go"
		foreignFile   = "internal/foreign/password.go"
	)
	values := []conformance.Declaration{
		{Kind: "struct", Name: "Password", Exported: true, StartLine: 3},
		{Kind: "func", Name: "NewPassword", Exported: true, StartLine: 6, Results: []string{"Password", "error"}},
		{Kind: "struct", Name: "Username", Exported: true, StartLine: 10},
		{Kind: "func", Name: "NewUsername", Exported: true, StartLine: 13, Results: []string{"Username", "error"}},
	}
	declared := func(file, name string) conformance.Reference {
		return conformance.Reference{Resolution: conformance.ResolutionDeclared, File: file, Name: name}
	}
	assertNotWeak := conformance.Declaration{Kind: "func", Name: "AssertNotWeak", Exported: true, StartLine: 20, Results: []string{"error"}}
	type state struct {
		result *conformance.Reference
		user   []conformance.Declaration
		test   []conformance.Declaration
	}
	run := func(s state) ([]string, conformance.Outcome) {
		t.Helper()
		generator := []conformance.Declaration{{Kind: "interface", Name: "PasswordGenerator", Exported: true, StartLine: 5}}
		if s.result != nil {
			generator = append(generator, conformance.Declaration{Kind: "method", Name: "GenerateStrongPassword", Owner: "PasswordGenerator", Exported: true, StartLine: 6, Results: []string{"user.Password", "error"}, Type: *s.result})
		}
		facts := map[string]conformance.LanguageFacts{
			userFile:      goDecls("user", append(append([]conformance.Declaration(nil), values...), s.user...)),
			userTest:      goDecls("user", s.test),
			generatorFile: goDecls("provisioning", generator),
			foreignFile:   goDecls("foreign", []conformance.Declaration{{Kind: "struct", Name: "Password", Exported: true, StartLine: 3}}),
		}
		a := runOne(t, r, zones, observed(userFile, userTest, generatorFile, foreignFile), facts, knowledge)
		var out []string
		for _, v := range a.Violations() {
			out = append(out, v.Message())
			if strings.Contains(v.Remediation(), ". ") {
				t.Errorf("a service's finding appends a statement to its repair: %q", v.Remediation())
			}
		}
		return out, outcomesOf(a)[id+"|context:identity"]
	}
	ref := func(r conformance.Reference) *conformance.Reference { return &r }
	noCheck := "declared service PasswordGenerator: not-weak has no check in context identity"
	noOperation := "service PasswordGenerator: assertion not-weak constrains operation GenerateStrongPassword, which the service does not declare"
	stringResult := ref(conformance.Reference{Resolution: conformance.ResolutionExternal, Name: "string"})
	for _, tc := range []struct {
		name    string
		s       state
		want    []string
		outcome conformance.Outcome
	}{
		{"the recorded value", state{result: ref(declared(userFile, "Password"))}, nil, conformance.OutcomeConforms},
		{"another package's type of the same name", state{result: ref(declared(foreignFile, "Password"))}, []string{noCheck}, conformance.OutcomeViolates},
		{"a collection of the value", state{result: ref(conformance.Reference{Resolution: conformance.ResolutionLocal, Name: "[]user.Password"})}, []string{noCheck}, conformance.OutcomeViolates},
		{"a value whose invariant has another key", state{result: ref(declared(userFile, "Username"))}, []string{noCheck}, conformance.OutcomeViolates},
		{"a result resolution cannot follow", state{result: ref(conformance.Reference{Resolution: conformance.ResolutionUndetermined, Reason: "the type checker could not type the result type user.Password"})}, nil, conformance.OutcomeUndetermined},
		{"a result no resolution reached", state{result: ref(conformance.Reference{})}, nil, conformance.OutcomeUndetermined},
		{"a function of the context", state{result: stringResult, user: []conformance.Declaration{assertNotWeak}}, nil, conformance.OutcomeConforms},
		{"a function only a test declares", state{result: stringResult, test: []conformance.Declaration{assertNotWeak}}, []string{noCheck}, conformance.OutcomeViolates},
		{"a method named like the check", state{result: stringResult, user: []conformance.Declaration{{Kind: "method", Name: "AssertNotWeak", Owner: "Password", StartLine: 20}}}, []string{noCheck}, conformance.OutcomeViolates},
		{"a function nested in another", state{result: stringResult, user: []conformance.Declaration{{Kind: "func", Name: "AssertNotWeak", Owner: "propose", StartLine: 21}}}, []string{noCheck}, conformance.OutcomeViolates},
		{"no operation, checked by a function", state{user: []conformance.Declaration{assertNotWeak}}, []string{noOperation}, conformance.OutcomeViolates},
		{"no operation and no check", state{}, []string{noCheck, noOperation}, conformance.OutcomeViolates},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, outcome := run(tc.s)
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") || outcome != tc.outcome {
				t.Errorf("\n got %q (%s)\nwant %q (%s)", got, outcome, tc.want, tc.outcome)
			}
		})
	}
}

// A class is a declared service only when it states it is abstract; a
// concrete class keeps the rule of the root and the concrete service,
// and a model-side check does not satisfy it.
func TestConcreteServiceIsNotCheckedByTheModel(t *testing.T) {
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "identity", Definition: "d", Line: 1,
		Services: []vocab.DomainService{{
			Name: "Encrypter", Definition: "d", Line: 2,
			Assertions: []vocab.Assertion{{Key: "not-empty", On: "encrypt", Statement: "s", Line: 3}},
		}},
	})
	id := "assertion/checked-by-its-operation"
	r := builtInRule(t, id)
	file := "src/identity/encrypter.ts"
	run := func(abstract bool) []string {
		facts := map[string]conformance.LanguageFacts{file: {
			Language: rule.LanguageTypeScript, ImportsAvailable: true, DeclarationsAvailable: true, CallsAvailable: true, FieldTypesAvailable: true,
			Declarations: []conformance.Declaration{
				{Kind: "class", Name: "Encrypter", Exported: true, Abstract: abstract, StartLine: 1},
				{Kind: "method", Name: "encrypt", Owner: "Encrypter", Exported: true, StartLine: 2, Results: []string{"string"}},
				{Kind: "func", Name: "assertNotEmpty", Exported: true, StartLine: 5},
			},
		}}
		a := runOne(t, r, nil, observed(file), facts, knowledge)
		var out []string
		for _, v := range a.Violations() {
			out = append(out, v.Message())
		}
		return out
	}
	if got := run(true); len(got) != 0 {
		t.Errorf("an abstract class checked by a function of the context: %q", got)
	}
	want := "service Encrypter: assertion not-empty names no checking method on the service; expected assertNotEmpty (typescript)"
	if got := run(false); len(got) != 1 || got[0] != want {
		t.Errorf("a concrete class:\n got %q\nwant %q", got, want)
	}
}

// The coverage of a declared service's guarantee judges the same shapes
// as the rule: the value object its operation returns is read while the
// contract is isolated from every other owner's.
func TestDeclaredServiceCoverageReadsTheReturnedValue(t *testing.T) {
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "identity", Definition: "d", Line: 1,
		ValueObjects: []vocab.ValueObject{
			{Name: "EncryptedPassword", Definition: "d", Line: 2, Invariants: []vocab.Invariant{{Key: "not-empty", Statement: "s", Line: 3}}},
		},
		Services: []vocab.DomainService{{
			Name: "Encrypter", Definition: "d", Line: 4,
			Assertions: []vocab.Assertion{{Key: "not-empty", On: "Encrypt", Statement: "s", Line: 5}},
		}},
	})
	file := "internal/user/password.go"
	facts := map[string]conformance.LanguageFacts{file: goDecls("user", []conformance.Declaration{
		{Kind: "struct", Name: "EncryptedPassword", Exported: true, StartLine: 3},
		{Kind: "func", Name: "NewEncryptedPassword", Exported: true, StartLine: 6, Results: []string{"EncryptedPassword", "error"}},
		{Kind: "interface", Name: "Encrypter", Exported: true, StartLine: 12},
		{Kind: "method", Name: "Encrypt", Owner: "Encrypter", Exported: true, StartLine: 13, Results: []string{"EncryptedPassword", "error"},
			Type: conformance.Reference{Resolution: conformance.ResolutionDeclared, File: file, Name: "EncryptedPassword"}},
	})}
	a := runOne(t, builtInRule(t, "assertion/checked-by-its-operation"), nil, observed(file), facts, knowledge)
	for _, c := range a.Contracts() {
		if c.Owner() != "Encrypter" {
			continue
		}
		if c.Status() != "structurally_checked" {
			t.Fatalf("coverage of a guarantee checked by the returned value: %#v", c)
		}
		return
	}
	t.Fatalf("no coverage for the Encrypter guarantee: %#v", a.Contracts())
}

// Every overload of a declared service's operation must return the
// recorded value for its invariant to check the guarantee: one overload
// returning anything else leaves the guarantee without a check, in
// whatever order the overloads are written.
func TestDeclaredServiceOverloadsAllReturnTheValue(t *testing.T) {
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "identity", Definition: "d", Line: 1,
		ValueObjects: []vocab.ValueObject{
			{Name: "EncryptedPassword", Definition: "d", Line: 2, Invariants: []vocab.Invariant{{Key: "not-empty", Statement: "s", Line: 3}}},
		},
		Services: []vocab.DomainService{{
			Name: "Encrypter", Definition: "d", Line: 4,
			Assertions: []vocab.Assertion{{Key: "not-empty", On: "encrypt", Statement: "s", Line: 5}},
		}},
	})
	file := "src/identity/password.ts"
	value := conformance.Reference{Resolution: conformance.ResolutionDeclared, File: file, Name: "EncryptedPassword"}
	text := conformance.Reference{Resolution: conformance.ResolutionExternal, Name: "string"}
	run := func(results ...conformance.Reference) int {
		decls := []conformance.Declaration{
			{Kind: "class", Name: "EncryptedPassword", Exported: true, StartLine: 1},
			{Kind: "method", Name: "from", Owner: "EncryptedPassword", Exported: true, StartLine: 2, Results: []string{"EncryptedPassword"}},
			{Kind: "interface", Name: "Encrypter", Exported: true, StartLine: 5},
		}
		for i, r := range results {
			decls = append(decls, conformance.Declaration{Kind: "method", Name: "encrypt", Owner: "Encrypter", Exported: true, StartLine: 6 + i, Type: r})
		}
		facts := map[string]conformance.LanguageFacts{file: {
			Language: rule.LanguageTypeScript, DeclarationsAvailable: true, CallsAvailable: true, FieldTypesAvailable: true, Declarations: decls,
		}}
		return len(runOne(t, builtInRule(t, "assertion/checked-by-its-operation"), nil, observed(file), facts, knowledge).Violations())
	}
	if got := run(value, value); got != 0 {
		t.Errorf("every overload returns the value: %d findings", got)
	}
	if got := run(value, text); got != 1 {
		t.Errorf("a later overload returns a string: %d findings, want 1", got)
	}
	if got := run(text, value); got != 1 {
		t.Errorf("an earlier overload returns a string: %d findings, want 1", got)
	}
}
