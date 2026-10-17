package conformance_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// discounterContext records the service of issue #143: Discounter's
// Discount never exceeds the order total, verified by the given tests.
func discounterContext(t *testing.T, tests ...string) vocab.UbiquitousLanguage {
	t.Helper()
	return oneContext(t, vocab.BoundedContext{
		Name: "pricing", Definition: "d", Line: 1,
		Services: []vocab.DomainService{{
			Name: "Discounter", Definition: "d", Line: 2,
			Assertions: []vocab.Assertion{{Key: "never-exceeds-total", On: "Discount", Statement: "A discount never exceeds the order total.", VerifiedBy: tests, Line: 4}},
		}},
	})
}

// runWithContent runs one Rule over observations that also lend file
// content, as the repository reader does.
func runWithContent(t *testing.T, r rule.Rule, files []conformance.ObservedFile, facts map[string]conformance.LanguageFacts, content map[string]string, knowledge vocab.UbiquitousLanguage) conformance.Assessment {
	t.Helper()
	obs, err := conformance.NewObservations(files, facts)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	a, err := conformance.Run(conformance.Request{
		Rules:          []rule.Rule{r},
		Observations:   obs.WithContent(conformance.NewMapContent(content)),
		UnknownImports: rule.UnknownImportsWarn,
		Knowledge:      knowledge,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return a
}

// goTestFile is one parsed Go test file importing testing, under name
// when the spec writes one.
func goTestFile(pkg, name string, decls []conformance.Declaration) conformance.LanguageFacts {
	f := goDecls(pkg, decls)
	f.ImportsAvailable = true
	f.Imports = []conformance.Import{{Path: "testing", Name: name, Line: 3, Class: conformance.ImportStdlib}}
	return f
}

func messagesOf(a conformance.Assessment) []string {
	var out []string
	for _, v := range a.Violations() {
		out = append(out, v.Message())
	}
	return out
}

// A service declared as an interface, as Evans states a service, has
// no body to call AssertNeverExceedsTotal from. The rule asks for the
// tests that verify the assertion, with a remedy that can be followed,
// and conforms once the assertion names them; the operation itself
// must still be declared. The same holds for an abstract class in
// TypeScript and Python.
func TestAssertionOfABodilessOwnerIsVerifiedByTests(t *testing.T) {
	id := "assertion/checked-by-its-operation"
	r := builtInRule(t, id)
	goFile := "internal/pricing/discounter.go"
	iface := conformance.Declaration{Kind: "interface", Name: "Discounter", Exported: true, StartLine: 3}
	discount := conformance.Declaration{Kind: "method", Name: "Discount", Owner: "Discounter", Exported: true, StartLine: 4}
	assert := conformance.Declaration{Kind: "method", Name: "AssertNeverExceedsTotal", Owner: "Discounter", Exported: true, StartLine: 5}
	run := func(knowledge vocab.UbiquitousLanguage, decls ...conformance.Declaration) conformance.Assessment {
		return runOne(t, r, nil, observed(goFile), map[string]conformance.LanguageFacts{goFile: goDecls("pricing", decls)}, knowledge)
	}

	a := run(discounterContext(t), iface, discount, assert)
	got := a.Violations()
	if len(got) != 1 {
		t.Fatalf("interface without tests: %q", messagesOf(a))
	}
	if want := "service Discounter: Discounter is declared as an interface, which has no body to call AssertNeverExceedsTotal (go) from, and assertion never-exceeds-total names no tests that verify it"; got[0].Message() != want {
		t.Errorf("message:\n got %q\nwant %q", got[0].Message(), want)
	}
	if want := "arclint domain define assertion never-exceeds-total --context pricing --verified-by <test file>:<test name>"; !strings.Contains(got[0].Remediation(), want) {
		t.Errorf("remediation %q does not say how to record the tests", got[0].Remediation())
	}

	verified := discounterContext(t, "internal/pricing/discounter_test.go:TestDiscountNeverExceedsTotal")
	if a := run(verified, iface, discount); outcomesOf(a)[id+"|context:pricing"] != conformance.OutcomeConforms {
		t.Errorf("interface with tests: %q", messagesOf(a))
	}
	a = run(verified, iface)
	if got := messagesOf(a); len(got) != 1 || got[0] != "service Discounter: assertion never-exceeds-total constrains operation Discount, which the service does not declare" {
		t.Errorf("interface without the operation: %q", got)
	}

	// A concrete owner still calls the check; tests do not replace it.
	concrete := conformance.Declaration{Kind: "struct", Name: "Discounter", Exported: true, StartLine: 3}
	a = run(verified, concrete, discount, assert)
	if got := messagesOf(a); len(got) != 1 || got[0] != "service Discounter: operation Discount does not call AssertNeverExceedsTotal, so assertion never-exceeds-total is not checked when it completes" {
		t.Errorf("concrete owner with tests: %q", got)
	}

	for _, c := range []struct {
		lang rule.Language
		file string
		op   string
	}{
		{rule.LanguageTypeScript, "src/pricing/discounter.ts", "discount"},
		{rule.LanguagePython, "pricing/discounter.py", "discount"},
	} {
		facts := map[string]conformance.LanguageFacts{c.file: {Language: c.lang, DeclarationsAvailable: true, CallsAvailable: true, Declarations: []conformance.Declaration{
			{Kind: "class", Name: "Discounter", Exported: true, Abstract: true, StartLine: 1},
			{Kind: "method", Name: c.op, Owner: "Discounter", Exported: true, StartLine: 2},
		}}}
		a := runOne(t, r, nil, observed(c.file), facts, discounterContext(t))
		if got := messagesOf(a); len(got) != 1 || !strings.Contains(got[0], "Discounter is declared as an abstract class") {
			t.Errorf("%s abstract class without tests: %q", c.lang, got)
		}
		a = runOne(t, r, nil, observed(c.file), facts, verified)
		if got := outcomesOf(a)[id+"|context:pricing"]; got != conformance.OutcomeConforms {
			t.Errorf("%s abstract class with tests: outcome %s, %q", c.lang, got, messagesOf(a))
		}
	}
}

// Each test verified_by names is declared in the file it names: a Go
// func Test of a _test.go file taking one *testing.T, a Python test
// function or Class.test method, or a TypeScript name written in the
// file. A missing or renamed test is a finding at the recorded
// assertion; facts the check cannot read leave it undetermined.
func TestVerifyingTestsAreDeclared(t *testing.T) {
	id := "assertion/verifying-tests-declared"
	r := builtInRule(t, id)
	goTest := "internal/pricing/discounter_test.go"
	pyTest := "tests/test_discounter.py"
	tsTest := "src/pricing/discounter.test.ts"
	testingT := []conformance.DeclarationParam{{Name: "t", Type: "*testing.T"}}
	dotTest := "internal/pricing/dot_test.go"
	aliasTest := "internal/pricing/alias_test.go"
	files := observed(goTest, dotTest, aliasTest, pyTest, tsTest, "internal/pricing/discounter.go", "docs/discounter.md")
	facts := map[string]conformance.LanguageFacts{
		goTest: goTestFile("pricing", "", []conformance.Declaration{
			{Kind: "func", Name: "TestDiscountNeverExceedsTotal", Exported: true, StartLine: 5, Params: testingT},
			{Kind: "func", Name: "Testdiscount", Exported: true, StartLine: 9, Params: testingT},
			{Kind: "func", Name: "TestHelper", Exported: true, StartLine: 12, Params: []conformance.DeclarationParam{{Name: "b", Type: "*testing.B"}}},
			{Kind: "func", Name: "TestOtherT", Exported: true, StartLine: 15, Params: []conformance.DeclarationParam{{Name: "t", Type: "*other.T"}}},
			{Kind: "func", Name: "TestReturns", Exported: true, StartLine: 18, Params: testingT, Results: []string{"int"}},
		}),
		dotTest: goTestFile("pricing", ".", []conformance.Declaration{
			{Kind: "func", Name: "TestCapped", Exported: true, StartLine: 5, Params: []conformance.DeclarationParam{{Name: "t", Type: "*T"}}},
			{Kind: "func", Name: "TestQualified", Exported: true, StartLine: 6, Params: testingT},
		}),
		aliasTest: goTestFile("pricing", "tst", []conformance.Declaration{
			{Kind: "func", Name: "TestCapped", Exported: true, StartLine: 5, Params: []conformance.DeclarationParam{{Name: "t", Type: "*tst.T"}}},
		}),
		"internal/pricing/discounter.go": goTestFile("pricing", "", []conformance.Declaration{
			{Kind: "func", Name: "TestLike", Exported: true, StartLine: 3, Params: testingT},
		}),
		pyTest: {Language: rule.LanguagePython, DeclarationsAvailable: true, Declarations: []conformance.Declaration{
			{Kind: "class", Name: "TestCapping", Exported: true, StartLine: 1},
			{Kind: "method", Name: "test_never_exceeds_total", Owner: "TestCapping", Exported: true, StartLine: 2},
			{Kind: "func", Name: "test_never_exceeds_total", Exported: true, StartLine: 4},
			{Kind: "class", Name: "TestDiscounter", Exported: true, StartLine: 6},
			{Kind: "method", Name: "test_caps_at_total", Owner: "TestDiscounter", Exported: true, StartLine: 7},
			{Kind: "func", Name: "helper", Exported: true, StartLine: 9},
		}},
	}
	content := map[string]string{tsTest: `test("never exceeds the order total: 100%", () => {});`}
	judge := func(tests ...string) (conformance.Outcome, []string) {
		a := runWithContent(t, r, files, facts, content, discounterContext(t, tests...))
		return outcomesOf(a)[id+"|context:pricing"], messagesOf(a)
	}

	for _, ok := range []string{
		goTest + ":TestDiscountNeverExceedsTotal",
		dotTest + ":TestCapped",
		aliasTest + ":TestCapped",
		pyTest + ":test_never_exceeds_total",
		pyTest + ":TestDiscounter.test_caps_at_total",
		tsTest + ":never exceeds the order total: 100%",
	} {
		if outcome, got := judge(ok); outcome != conformance.OutcomeConforms {
			t.Errorf("%s: outcome %s, %q", ok, outcome, got)
		}
	}

	const prefix = "assertion never-exceeds-total of Discounter: verified_by names "
	for _, c := range []struct{ entry, want string }{
		{goTest + ":TestRenamed", prefix + goTest + ":TestRenamed, but " + goTest + " declares no test TestRenamed"},
		{"internal/pricing/gone_test.go:TestA", prefix + "internal/pricing/gone_test.go:TestA, but no file internal/pricing/gone_test.go is in the repository"},
		{goTest + ":Testdiscount", "func Testdiscount is not named as a test"},
		{goTest + ":TestHelper", "func TestHelper does not take one *testing.T"},
		{goTest + ":TestOtherT", "func TestOtherT does not take one *testing.T"},
		{dotTest + ":TestQualified", "func TestQualified does not take one *testing.T"},
		{goTest + ":TestReturns", "func TestReturns returns a value"},
		{"internal/pricing/discounter.go:TestLike", "go test runs tests only from _test.go files"},
		{pyTest + ":helper", "helper is not named as a test, beginning with test"},
		{pyTest + ":test_caps_at_total", "test_caps_at_total is a method of TestDiscounter; name it TestDiscounter.test_caps_at_total"},
		{pyTest + ":TestOther.test_caps_at_total", "declares no test TestOther.test_caps_at_total"},
		{tsTest + ":rounds down", `but the name "rounds down" is not written in ` + tsTest},
		{"docs/discounter.md:never exceeds", "which is not a Go, Python or TypeScript file"},
	} {
		outcome, got := judge(c.entry)
		if outcome != conformance.OutcomeViolates || len(got) != 1 || !strings.Contains(got[0], c.want) {
			t.Errorf("%s: outcome %s, %q; want a finding containing %q", c.entry, outcome, got, c.want)
		}
	}

	// A test file whose declarations or, in Go, imports are not
	// available leaves the test's existence undetermined, never
	// conforming.
	for name, f := range map[string]conformance.LanguageFacts{
		"unparsed":        {Language: rule.LanguageGo, ParseFailure: "expected declaration"},
		"without imports": goDecls("pricing", facts[goTest].Declarations),
	} {
		a := runWithContent(t, r, files, map[string]conformance.LanguageFacts{goTest: f}, content, discounterContext(t, goTest+":TestDiscountNeverExceedsTotal"))
		if got := outcomesOf(a)[id+"|context:pricing"]; got != conformance.OutcomeUndetermined {
			t.Errorf("%s test file: outcome %s, want undetermined", name, got)
		}
	}
}

// An invariant names its tests the same way, and its own Rule judges
// them; an invariant naming no tests is not judged by it.
func TestInvariantVerifyingTestsAreDeclared(t *testing.T) {
	id := "invariant/verifying-tests-declared"
	r := builtInRule(t, id)
	goTest := "internal/money/money_test.go"
	knowledge := func(tests ...string) vocab.UbiquitousLanguage {
		return oneContext(t, vocab.BoundedContext{
			Name: "billing", Definition: "d", Line: 1,
			ValueObjects: []vocab.ValueObject{{Name: "Money", Definition: "d", Line: 2,
				Invariants: []vocab.Invariant{{Key: "never-negative", Statement: "Money is never negative.", VerifiedBy: tests, Line: 4}}}},
		})
	}
	facts := map[string]conformance.LanguageFacts{goTest: goTestFile("money", "", []conformance.Declaration{
		{Kind: "func", Name: "TestNeverNegative", Exported: true, StartLine: 3, Params: []conformance.DeclarationParam{{Name: "t", Type: "*testing.T"}}},
	})}
	for _, c := range []struct {
		tests []string
		want  conformance.Outcome
	}{
		{nil, conformance.OutcomeConforms},
		{[]string{goTest + ":TestNeverNegative"}, conformance.OutcomeConforms},
		{[]string{goTest + ":TestNeverNegative", goTest + ":TestRenamed"}, conformance.OutcomeViolates},
	} {
		a := runOne(t, r, nil, observed(goTest), facts, knowledge(c.tests...))
		if got := outcomesOf(a)[id+"|context:billing"]; got != c.want {
			t.Errorf("%v: outcome %s, want %s; %q", c.tests, got, c.want, messagesOf(a))
		}
	}
}

// A contract that names its tests is covered by the Rule that judges
// them besides the Rule that judges its code, each reporting its own
// outcome and the limitation that it reads existence, not proof; a
// contract naming none is not checked for tests.
func TestContractCoverageIncludesTheVerifyingTests(t *testing.T) {
	goFile, goTest := "internal/pricing/discounter.go", "internal/pricing/discounter_test.go"
	facts := map[string]conformance.LanguageFacts{
		goFile: goDecls("pricing", []conformance.Declaration{
			{Kind: "interface", Name: "Discounter", Exported: true, StartLine: 3},
			{Kind: "method", Name: "Discount", Owner: "Discounter", Exported: true, StartLine: 4},
		}),
		goTest: goTestFile("pricing", "", []conformance.Declaration{
			{Kind: "func", Name: "TestDiscountNeverExceedsTotal", Exported: true, StartLine: 5, Params: []conformance.DeclarationParam{{Name: "t", Type: "*testing.T"}}},
		}),
	}
	obs, err := conformance.NewObservations(observed(goFile, goTest), facts)
	if err != nil {
		t.Fatal(err)
	}
	coverage := func(tests ...string) conformance.ContractCoverage {
		t.Helper()
		a, err := conformance.Run(conformance.Request{
			Rules:          []rule.Rule{builtInRule(t, "assertion/checked-by-its-operation"), builtInRule(t, "assertion/verifying-tests-declared")},
			Observations:   obs,
			UnknownImports: rule.UnknownImportsWarn,
			Knowledge:      discounterContext(t, tests...),
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(a.Contracts()) != 1 {
			t.Fatalf("contracts = %d, want 1", len(a.Contracts()))
		}
		return a.Contracts()[0]
	}

	c := coverage(goTest + ":TestDiscountNeverExceedsTotal")
	if c.Status() != "structurally_checked" {
		t.Errorf("verified contract: status %s", c.Status())
	}
	rules := map[string]conformance.Evaluation{}
	for _, e := range c.Checks() {
		rules[e.Rule().Qualified()] = e
	}
	tests, ok := rules["assertion/verifying-tests-declared"]
	if len(rules) != 2 || !ok || tests.Outcome() != conformance.OutcomeConforms {
		t.Fatalf("checks of a verified contract: %v", rules)
	}
	if !strings.Contains(strings.Join(tests.Limitations(), " "), "matched as text") {
		t.Errorf("the verifying-tests check lost its limitation: %q", tests.Limitations())
	}

	if c := coverage(goTest + ":TestRenamed"); c.Status() != "violates" {
		t.Errorf("renamed test: status %s", c.Status())
	}
	c = coverage()
	if len(c.Checks()) != 1 || c.Checks()[0].Rule().Qualified() != "assertion/checked-by-its-operation" {
		t.Errorf("a contract naming no tests is checked for tests: %v", c.Checks())
	}
}
