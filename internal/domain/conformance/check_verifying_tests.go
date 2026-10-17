package conformance

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// checkInvariantTestsDeclared judges the tests every invariant of the
// context names under verified_by: each is declared in the file it
// names. The test's existence is judged, never what it proves.
func checkInvariantTestsDeclared(r rule.Rule, subject rule.Subject, cc contextCode, code domainCode) ([]Violation, []string, error) {
	var vs []Violation
	var unresolved []string
	for _, oi := range cc.ctx.Invariants() {
		for _, entry := range oi.Invariant.VerifiedBy {
			v, gap, err := code.judgeVerifyingTest(r, subject, cc.recording, oi.Invariant.Line,
				fmt.Sprintf("invariant %s of %s", oi.Invariant.Key, oi.Owner), entry)
			if err != nil {
				return nil, nil, err
			}
			vs = append(vs, v...)
			if gap != "" {
				unresolved = append(unresolved, gap)
			}
		}
	}
	return vs, unresolved, nil
}

// checkAssertionTestsDeclared judges the tests every assertion of the
// context names under verified_by, as checkInvariantTestsDeclared does
// for invariants.
func checkAssertionTestsDeclared(r rule.Rule, subject rule.Subject, cc contextCode, code domainCode) ([]Violation, []string, error) {
	var vs []Violation
	var unresolved []string
	for _, oa := range cc.ctx.Assertions() {
		for _, entry := range oa.Assertion.VerifiedBy {
			v, gap, err := code.judgeVerifyingTest(r, subject, cc.recording, oa.Assertion.Line,
				fmt.Sprintf("assertion %s of %s", oa.Assertion.Key, oa.Owner), entry)
			if err != nil {
				return nil, nil, err
			}
			vs = append(vs, v...)
			if gap != "" {
				unresolved = append(unresolved, gap)
			}
		}
	}
	return vs, unresolved, nil
}

// judgeVerifyingTest judges one verified_by entry recorded for what at
// line of the recording: the violation when the file or the test is
// missing, or the reason the facts cannot tell. A Go or Python test is
// a declaration the language's facts carry; a TypeScript test is a call
// such as test("..."), so its name written in the file is all the check
// can read.
func (code domainCode) judgeVerifyingTest(r rule.Rule, subject rule.Subject, recording string, line int, what, entry string) ([]Violation, string, error) {
	finding := func(message, remediation string) ([]Violation, string, error) {
		v, err := newViolation(r, subject, recording, line, what+": "+message, remediation)
		if err != nil {
			return nil, "", err
		}
		return []Violation{v}, "", nil
	}
	t, err := vocab.ParseVerifyingTest(entry)
	if err != nil {
		return finding("verified_by "+err.Error(), "spell the test as its file's path from the repository root, a colon, and the test's name")
	}
	if !code.facts.observed(t.Path) {
		return finding(fmt.Sprintf("verified_by names %s, but no file %s is in the repository", t, t.Path),
			fmt.Sprintf("name the test's file by its path from the repository root, or remove %s from verified_by", t))
	}
	if !code.facts.Contains(t.Path) {
		return nil, fmt.Sprintf("%s: the file is outside this rule's scope", t), nil
	}
	switch rule.LanguageOf(t.Path) {
	case rule.LanguageGo:
		return code.judgeDeclaredTest(t, finding, goTestDeclared, "func "+t.Name+"(t *testing.T)")
	case rule.LanguagePython:
		return code.judgeDeclaredTest(t, finding, pythonTestDeclared, pythonTestSpelling(t.Name))
	case rule.LanguageTypeScript:
		content, err := code.facts.Read(t.Path)
		if err != nil {
			return nil, fmt.Sprintf("%s: %v", t, err), nil
		}
		if strings.Contains(content, t.Name) {
			return nil, "", nil
		}
		return finding(fmt.Sprintf("verified_by names %s, but the name %q is not written in %s", t, t.Name, t.Path),
			fmt.Sprintf("name the test as its test() or it() call in %s spells it, or remove %s from verified_by", t.Path, t))
	default:
		return finding(fmt.Sprintf("verified_by names %s, which is not a Go, Python or TypeScript file", t),
			"name a test in a .go, .py, .ts or .tsx file")
	}
}

// testDeclared reports whether a file's declarations declare a test,
// and when they do not, says what the file declares under the name
// instead, or "" when nothing.
type testDeclared func(t vocab.VerifyingTest, decls []Declaration) (bool, string)

// judgeDeclaredTest judges a test of a language whose facts carry
// declarations. spelling is the declaration a remedy asks for.
func (code domainCode) judgeDeclaredTest(t vocab.VerifyingTest, finding func(string, string) ([]Violation, string, error), declared testDeclared, spelling string) ([]Violation, string, error) {
	facts, _ := code.facts.FactsFor(t.Path)
	switch {
	case facts.ParseFailure != "":
		return nil, fmt.Sprintf("%s: the file does not parse: %s", t, facts.ParseFailure), nil
	case !facts.DeclarationsAvailable:
		return nil, fmt.Sprintf("%s: the declarations of %s are not available", t, t.Path), nil
	}
	ok, instead := declared(t, facts.Declarations)
	if ok {
		return nil, "", nil
	}
	message := fmt.Sprintf("verified_by names %s, but %s declares no test %s", t, t.Path, t.Name)
	if instead != "" {
		message += "; " + instead
	}
	return finding(message, fmt.Sprintf("declare %s in %s, or name the test that verifies the statement in verified_by", spelling, t.Path))
}

// goTestDeclared finds a Go test: a function of a _test.go file named
// Test followed by nothing or by a character that is not a lowercase
// letter, taking one *testing.T, as go test runs it.
func goTestDeclared(t vocab.VerifyingTest, decls []Declaration) (bool, string) {
	if !strings.HasSuffix(t.Path, "_test.go") {
		return false, "go test runs tests only from _test.go files"
	}
	for _, d := range decls {
		if d.Kind != declKindFunc || d.Owner != "" || d.Name != t.Name {
			continue
		}
		switch {
		case !goTestName(d.Name):
			return false, fmt.Sprintf("func %s is not named as a test, Test followed by a name that does not begin with a lowercase letter", d.Name)
		case len(d.Params) != 1 || !goTestingT(d.Params[0].Type):
			return false, fmt.Sprintf("func %s does not take one *testing.T", d.Name)
		}
		return true, ""
	}
	return false, ""
}

// goTestName reports whether go test runs a function of this name as a
// test.
func goTestName(name string) bool {
	rest, ok := strings.CutPrefix(name, "Test")
	if !ok {
		return false
	}
	if rest == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(rest)
	return !unicode.IsLower(r)
}

// goTestingT reports whether a parameter type is a pointer to the
// testing package's T under whatever name the file imports it.
func goTestingT(typ string) bool {
	pkg, ok := strings.CutPrefix(typ, "*")
	if !ok {
		return false
	}
	pkg, ok = strings.CutSuffix(pkg, ".T")
	return ok && pkg != "" && !strings.ContainsAny(pkg, ".*[] ")
}

// pythonTestDeclared finds a Python test: a module-level function whose
// name begins with test, or, spelled Class.test_name, such a method of
// the class.
func pythonTestDeclared(t vocab.VerifyingTest, decls []Declaration) (bool, string) {
	class, method, isMethod := strings.Cut(t.Name, ".")
	if !isMethod {
		method = t.Name
	}
	for _, d := range decls {
		switch {
		case isMethod && d.Kind == declKindMethod && d.Owner == class && d.Name == method,
			!isMethod && d.Kind == declKindFunc && d.Owner == "" && d.Name == method:
		case !isMethod && d.Kind == declKindMethod && d.Name == method:
			return false, fmt.Sprintf("%s is a method of %s; name it %s.%s", method, d.Owner, d.Owner, method)
		default:
			continue
		}
		if !strings.HasPrefix(method, "test") {
			return false, fmt.Sprintf("%s is not named as a test, beginning with test", method)
		}
		return true, ""
	}
	return false, ""
}

// pythonTestSpelling is the declaration a remedy asks for.
func pythonTestSpelling(name string) string {
	if class, method, ok := strings.Cut(name, "."); ok {
		return fmt.Sprintf("def %s(self) in class %s", method, class)
	}
	return "def " + name + "()"
}
