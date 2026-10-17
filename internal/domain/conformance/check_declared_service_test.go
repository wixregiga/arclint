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
// when the result names that value through the package the file
// imports, never a collection of it, and only when the value records
// an invariant under the assertion's key. A function of the context
// spelled from the key checks it too, unless only a test declares it.
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
	)
	userImport := conformance.Import{Path: "example.com/mod/internal/user", Line: 3, Class: conformance.ImportInternal, TargetDir: "internal/user"}
	values := []conformance.Declaration{
		{Kind: "struct", Name: "Password", Exported: true, StartLine: 3},
		{Kind: "func", Name: "NewPassword", Exported: true, StartLine: 6, Results: []string{"Password", "error"}},
		{Kind: "struct", Name: "Username", Exported: true, StartLine: 10},
		{Kind: "func", Name: "NewUsername", Exported: true, StartLine: 13, Results: []string{"Username", "error"}},
	}
	aliased := func(alias string) conformance.Import {
		imp := userImport
		imp.Alias = alias
		return imp
	}
	assertNotWeak := conformance.Declaration{Kind: "func", Name: "AssertNotWeak", Exported: true, StartLine: 20, Results: []string{"error"}}
	generator := func(results ...string) []conformance.Declaration {
		ds := []conformance.Declaration{{Kind: "interface", Name: "PasswordGenerator", Exported: true, StartLine: 5}}
		if results != nil {
			ds = append(ds, conformance.Declaration{Kind: "method", Name: "GenerateStrongPassword", Owner: "PasswordGenerator", Exported: true, StartLine: 6, Results: results})
		}
		return ds
	}
	type state struct {
		generator []conformance.Declaration
		imports   []conformance.Import
		user      []conformance.Declaration
		test      []conformance.Declaration
	}
	run := func(s state) []string {
		t.Helper()
		gen := goDecls("provisioning", s.generator)
		gen.Imports = s.imports
		facts := map[string]conformance.LanguageFacts{
			userFile:      goDecls("user", append(append([]conformance.Declaration(nil), values...), s.user...)),
			userTest:      goDecls("user", s.test),
			generatorFile: gen,
		}
		a := runOne(t, r, zones, observed(userFile, userTest, generatorFile), facts, knowledge)
		var out []string
		for _, v := range a.Violations() {
			out = append(out, v.Message())
		}
		if len(out) == 0 {
			if got := outcomesOf(a)[id+"|context:identity"]; got != conformance.OutcomeConforms {
				t.Errorf("no finding, but outcome %s", got)
			}
		}
		return out
	}
	noCheck := "declared service PasswordGenerator: not-weak has no check in context identity"
	noOperation := "service PasswordGenerator: assertion not-weak constrains operation GenerateStrongPassword, which the service does not declare"
	for _, tc := range []struct {
		name string
		s    state
		want []string
	}{
		{"value of another package through its import", state{generator: generator("user.Password", "error"), imports: []conformance.Import{userImport}}, nil},
		{"value spelled without the import that names it", state{generator: generator("user.Password", "error")}, []string{noCheck}},
		{"a collection of the value", state{generator: generator("[]user.Password", "error"), imports: []conformance.Import{userImport}}, []string{noCheck}},
		{"a value whose invariant has another key", state{generator: generator("user.Username", "error"), imports: []conformance.Import{userImport}}, []string{noCheck}},
		{"a type that only ends with the value's name", state{generator: generator("user.StrongPassword", "error"), imports: []conformance.Import{userImport}}, []string{noCheck}},
		{"value through an aliased import", state{generator: generator("v.Password", "error"), imports: []conformance.Import{aliased("v")}}, nil},
		{"the package name where the import binds an alias", state{generator: generator("user.Password", "error"), imports: []conformance.Import{aliased("v")}}, []string{noCheck}},
		{"value through a dot import", state{generator: generator("Password", "error"), imports: []conformance.Import{aliased(".")}}, nil},
		{"value spelled through a blank import", state{generator: generator("user.Password", "error"), imports: []conformance.Import{aliased("_")}}, []string{noCheck}},
		{"a sized array of the value", state{generator: generator("[0]user.Password", "error"), imports: []conformance.Import{userImport}}, []string{noCheck}},
		{"a function of the context", state{generator: generator("string", "error"), user: []conformance.Declaration{assertNotWeak}}, nil},
		{"a function nested in another", state{generator: generator("string", "error"), user: []conformance.Declaration{{Kind: "func", Name: "AssertNotWeak", Owner: "propose", StartLine: 21}}}, []string{noCheck}},
		{"a function only a test declares", state{generator: generator("string", "error"), test: []conformance.Declaration{assertNotWeak}}, []string{noCheck}},
		{"a method named like the check", state{generator: generator("string", "error"), user: []conformance.Declaration{{Kind: "method", Name: "AssertNotWeak", Owner: "Password", StartLine: 20}}}, []string{noCheck}},
		{"no operation, checked by a function", state{generator: generator(), user: []conformance.Declaration{assertNotWeak}}, []string{noOperation}},
		{"no operation and no check", state{generator: generator()}, []string{noCheck, noOperation}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := run(tc.s); strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("\n got %q\nwant %q", got, tc.want)
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
			Language: rule.LanguageTypeScript, ImportsAvailable: true, DeclarationsAvailable: true, CallsAvailable: true,
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
		{Kind: "method", Name: "Encrypt", Owner: "Encrypter", Exported: true, StartLine: 13, Results: []string{"EncryptedPassword", "error"}},
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

// In the value's own directory a result names it bare; a qualified name
// there is another package's type, whatever it ends with.
func TestDeclaredServiceResultInTheValuesDirectoryIsBare(t *testing.T) {
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
	run := func(result string) int {
		facts := map[string]conformance.LanguageFacts{file: goDecls("user", []conformance.Declaration{
			{Kind: "struct", Name: "EncryptedPassword", Exported: true, StartLine: 3},
			{Kind: "func", Name: "NewEncryptedPassword", Exported: true, StartLine: 6, Results: []string{"EncryptedPassword", "error"}},
			{Kind: "interface", Name: "Encrypter", Exported: true, StartLine: 12},
			{Kind: "method", Name: "Encrypt", Owner: "Encrypter", Exported: true, StartLine: 13, Results: []string{result, "error"}},
		})}
		return len(runOne(t, builtInRule(t, "assertion/checked-by-its-operation"), nil, observed(file), facts, knowledge).Violations())
	}
	if got := run("*EncryptedPassword"); got != 0 {
		t.Errorf("a bare result naming the value: %d findings", got)
	}
	if got := run("foreign.EncryptedPassword"); got != 1 {
		t.Errorf("another package's type of the same name: %d findings, want 1", got)
	}
}
