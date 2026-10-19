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
	finding := func(message, remediation string) ([]Violation, error) {
		v, err := newViolation(r, subject, recording, line, what+": "+message, remediation)
		if err != nil {
			return nil, err
		}
		return []Violation{v}, nil
	}
	t, err := vocab.ParseVerifyingTest(entry)
	if err != nil {
		return judged(finding("verified_by "+err.Error(), "spell the test as its file's path from the repository root, a colon, and the test's name"))
	}
	file, gap, exists := code.testFileOf(t)
	switch {
	case !exists:
		return judged(finding(fmt.Sprintf("verified_by names %s, but no file %s is in the repository", t, t.Path),
			fmt.Sprintf("name the test's file by its path from the repository root, or remove %s from verified_by", t)))
	case gap != "":
		return nil, gap, nil
	}
	missing := func(message, remediation string) ([]Violation, error) {
		if file.named {
			message += "; " + namedTestRead
		}
		return finding(message, remediation)
	}
	switch rule.LanguageOf(t.Path) {
	case rule.LanguageGo:
		return judgeDeclaredTest(t, file, missing, goTestDeclared, "func "+t.Name+"(t *testing.T)")
	case rule.LanguagePython:
		return judgeDeclaredTest(t, file, missing, pythonTestDeclared, pythonTestSpelling(t.Name))
	case rule.LanguageTypeScript:
		content, err := file.read()
		if err != nil {
			return nil, fmt.Sprintf("%s: %v", t, err), nil
		}
		if strings.Contains(content, t.Name) {
			return nil, "", nil
		}
		return judged(missing(fmt.Sprintf("verified_by names %s, but the name %q is not written in %s", t, t.Name, t.Path),
			fmt.Sprintf("name the test as its test() or it() call in %s spells it, or remove %s from verified_by", t.Path, t)))
	default:
		return judged(finding(fmt.Sprintf("verified_by names %s, which is not a Go, Python or TypeScript file", t),
			"name a test in a .go, .py, .ts or .tsx file"))
	}
}

// judged is a judgment of a test that left nothing undetermined.
func judged(vs []Violation, err error) ([]Violation, string, error) { return vs, "", err }

// namedTestRead says why the check read a file the scan leaves out.
const namedTestRead = "read because verified_by names it; scan.exclude keeps it out of rule evaluation"

// testFile is what the check reads of the file a test names: an
// observed file the Rule selects, or one the walk left out that the
// recording names, which named marks.
type testFile struct {
	facts LanguageFacts
	read  func() (string, error)
	named bool
}

// testFileOf finds the file a test names. exists is false when the
// repository holds no such file; gap says why a file that exists
// cannot be read for the test.
func (code domainCode) testFileOf(t vocab.VerifyingTest) (file testFile, gap string, exists bool) {
	if code.facts.observed(t.Path) {
		if !code.facts.Contains(t.Path) {
			return testFile{}, fmt.Sprintf("%s: the file is outside this rule's scope", t), true
		}
		facts, _ := code.facts.FactsFor(t.Path)
		return testFile{facts: facts, read: func() (string, error) { return code.facts.Read(t.Path) }}, "", true
	}
	if facts, present, readable := code.facts.namedTest(t.Path); present {
		if !readable {
			return testFile{}, fmt.Sprintf("%s: the file is outside this rule's scope", t), true
		}
		return testFile{facts: facts, read: func() (string, error) { return code.facts.readNamedTest(t.Path) }, named: true}, "", true
	}
	if code.facts.held(t.Path) {
		return testFile{}, fmt.Sprintf("%s: the scan does not observe %s, and the file was not read for verified_by", t, t.Path), true
	}
	return testFile{}, "", false
}

// testDeclared finds the declaration of a test in a file's facts, and
// when there is none, says what the file declares under the name
// instead, or "" when nothing.
type testDeclared func(t vocab.VerifyingTest, facts LanguageFacts) (Declaration, bool, string)

// judgeDeclaredTest judges a test of a language whose facts carry
// declarations. spelling is the declaration a remedy asks for.
func judgeDeclaredTest(t vocab.VerifyingTest, file testFile, finding func(string, string) ([]Violation, error), declared testDeclared, spelling string) ([]Violation, string, error) {
	if gap := readableTests(t, file.facts); gap != "" {
		return nil, gap, nil
	}
	_, ok, instead := declared(t, file.facts)
	if ok {
		return nil, "", nil
	}
	message := fmt.Sprintf("verified_by names %s, but %s declares no test %s", t, t.Path, t.Name)
	if instead != "" {
		message += "; " + instead
	}
	return judged(finding(message, fmt.Sprintf("declare %s in %s, or name the test that verifies the statement in verified_by", spelling, t.Path)))
}

// readableTests says why a file's facts cannot tell whether it declares
// a test, or nothing when they can.
func readableTests(t vocab.VerifyingTest, facts LanguageFacts) string {
	switch {
	case facts.ParseFailure != "":
		return fmt.Sprintf("%s: the file does not parse: %s", t, facts.ParseFailure)
	case !facts.DeclarationsAvailable:
		return fmt.Sprintf("%s: the declarations of %s are not available", t, t.Path)
	case rule.LanguageOf(t.Path) == rule.LanguageGo && !facts.ImportsAvailable:
		// The name testing is imported by decides which T a Go test takes.
		return fmt.Sprintf("%s: the imports of %s are not available", t, t.Path)
	}
	return ""
}

// goTestDeclared finds a Go test as go test runs it: a function of a
// _test.go file named Test followed by nothing or by a character that
// is not a lowercase letter, declaring no type parameters, taking one
// *testing.T under the name the file imports testing by, and returning
// nothing.
func goTestDeclared(t vocab.VerifyingTest, facts LanguageFacts) (Declaration, bool, string) {
	if !strings.HasSuffix(t.Path, "_test.go") {
		return Declaration{}, false, "go test runs tests only from _test.go files"
	}
	for _, d := range facts.Declarations {
		if d.Kind != declKindFunc || d.Owner != "" || d.Name != t.Name {
			continue
		}
		switch {
		case !goTestName(d.Name):
			return Declaration{}, false, fmt.Sprintf("func %s is not named as a test, Test followed by a name that does not begin with a lowercase letter", d.Name)
		case len(d.Params) != 1 || !goTestingT(d.Params[0].Type, facts.Imports):
			return Declaration{}, false, fmt.Sprintf("func %s does not take one *testing.T", d.Name)
		case len(d.Results) != 0:
			return Declaration{}, false, fmt.Sprintf("func %s returns a value, and go test runs only a test that returns nothing", d.Name)
		case d.Generic:
			return Declaration{}, false, fmt.Sprintf("func %s declares type parameters, and go test refuses a test that does", d.Name)
		}
		return d, true, ""
	}
	return Declaration{}, false, ""
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

// goTestingT reports whether a parameter type, as written, is a pointer
// to the testing package's T under a name the file imports testing by:
// testing itself, an alias, or T alone through a dot import.
func goTestingT(typ string, imports []Import) bool {
	for _, imp := range imports {
		if imp.Path != "testing" {
			continue
		}
		switch imp.Name {
		case "":
			if typ == "*testing.T" {
				return true
			}
		case "_":
		case ".":
			if typ == "*T" {
				return true
			}
		default:
			if typ == "*"+imp.Name+".T" {
				return true
			}
		}
	}
	return false
}

// pythonTestDeclared finds a Python test: a module-level function whose
// name begins with test, or, spelled Class.test_name, such a method of
// the class. A bare name that only a method spells is answered with
// the spelling that names it.
func pythonTestDeclared(t vocab.VerifyingTest, facts LanguageFacts) (Declaration, bool, string) {
	class, method, isMethod := strings.Cut(t.Name, ".")
	if !isMethod {
		method = t.Name
	}
	instead := ""
	for _, d := range facts.Declarations {
		switch {
		case isMethod && d.Kind == declKindMethod && d.Owner == class && d.Name == method,
			!isMethod && d.Kind == declKindFunc && d.Owner == "" && d.Name == method:
			if !strings.HasPrefix(method, "test") {
				return Declaration{}, false, fmt.Sprintf("%s is not named as a test, beginning with test", method)
			}
			return d, true, ""
		case !isMethod && d.Kind == declKindMethod && d.Name == method && instead == "":
			instead = fmt.Sprintf("%s is a method of %s; name it %s.%s", method, d.Owner, d.Owner, method)
		}
	}
	return Declaration{}, false, instead
}

// pythonTestSpelling is the declaration a remedy asks for.
func pythonTestSpelling(name string) string {
	if class, method, ok := strings.Cut(name, "."); ok {
		return fmt.Sprintf("def %s(self) in class %s", method, class)
	}
	return "def " + name + "()"
}

// NamedTestsToRead lists the files the recording names under
// verified_by that a Rule judging those tests may read: an enabled
// verifying-tests Rule whose Scope selects the file and no file
// Exclusion of it removes. A file every such Rule excludes, or that no
// enabled one selects, is never read, whatever else names it.
func NamedTestsToRead(rules []rule.Rule, knowledge vocab.UbiquitousLanguage) []string {
	var readers []rule.Rule
	for _, r := range rules {
		check, ok := domainChecks[r.ID().Local()]
		if _, disabled := r.Disablement(); ok && check.readsTests && !disabled && r.Type() == rule.TypeDomain {
			readers = append(readers, r)
		}
	}
	var out []string
	for _, path := range knowledge.VerifyingTestPaths() {
		for _, r := range readers {
			if r.Scope().WouldSelectFile(path, nil) && !r.Scope().ExcludedFile(path) {
				out = append(out, path)
				break
			}
		}
	}
	return out
}
