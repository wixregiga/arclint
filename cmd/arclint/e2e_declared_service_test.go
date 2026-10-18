package main

// End-to-end coverage of a domain service owning an assertion, in every
// supported language: the operation the assertion names has one place
// in the domain, and its body calls the check, assert followed by the
// key, declared as a method of the service or as a function of the
// context; the call is the one the language's resolution follows, never
// a name alone. An operation without a body, such as a Go or TypeScript
// interface method, is a finding like any other that does not call it.

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

// declaredServiceDomain records the Encrypter service with a guarantee
// of its operation Encrypt, and the value object Encrypt returns, whose
// invariant carries the same key.
const declaredServiceDomain = `version: 1
project: identity
contexts:
  identity:
    definition: Who may sign in, and with what.
    value_objects:
      EncryptedPassword:
        definition: The protected form of a password.
        invariants:
          not-empty: An encrypted password is never empty.
    services:
      Encrypter:
        definition: Protects a password.
        assertions:
          not-empty:
            on: Encrypt
            statement: An encrypted password is never empty.
`

// serviceSource is one language's Encrypter declared as a concrete
// type. Its operation Encrypt makes the call written at CALL, and the
// service declares the method written at METHOD; a function of the
// context and a method of another type, Validator, are spelled the same
// as the check.
type serviceSource struct {
	runtime, path, template string
	module                  map[string]string
	// method is the service's own check, put at METHOD.
	method string
	// calls are what Encrypt calls at CALL: the service's method, the
	// function of the context, Validator's method of the same name, and
	// a helper that is not the check.
	ownMethod, function, other, helper string
	// methodDecl and functionDecl begin the line declaring each check,
	// where the overview anchors the guarantee.
	methodDecl, functionDecl string
}

func serviceSources() []serviceSource {
	return []serviceSource{
		{
			runtime: "go", path: "src/identity/encrypter.go",
			module: map[string]string{"go.mod": "module example.com/identity\n\ngo 1.22\n"},
			template: `package identity

import "errors"

// EncryptedPassword is the protected form of a password.
type EncryptedPassword struct{ text string }

// NewEncryptedPassword is the one door: invariant not-empty.
func NewEncryptedPassword(text string) (EncryptedPassword, error) {
	if err := checkText(text); err != nil {
		return EncryptedPassword{}, err
	}
	return EncryptedPassword{text: text}, nil
}

// Validator checks text handed to the context.
type Validator struct{}

// AssertNotEmpty checks that text is not empty.
func (Validator) AssertNotEmpty(text string) error { return checkText(text) }

// AssertNotEmpty checks not-empty.
func AssertNotEmpty(encrypted string) error { return checkText(encrypted) }

func checkText(text string) error {
	if text == "" {
		return errors.New("an encrypted password is never empty")
	}
	return nil
}

// Encrypter protects a password.
type Encrypter struct{}
METHOD
// Encrypt protects a password.
func (e Encrypter) Encrypt(plain string, v Validator) (EncryptedPassword, error) {
	encrypted := "pbkdf2$" + plain
	if err := CALL; err != nil {
		return EncryptedPassword{}, err
	}
	return NewEncryptedPassword(encrypted)
}
`,
			method:     "\n// AssertNotEmpty checks not-empty on the service.\nfunc (Encrypter) AssertNotEmpty(encrypted string) error { return checkText(encrypted) }\n",
			ownMethod:  "e.AssertNotEmpty(encrypted)",
			function:   "AssertNotEmpty(encrypted)",
			other:      "v.AssertNotEmpty(encrypted)",
			helper:     "checkText(encrypted)",
			methodDecl: "func (Encrypter) AssertNotEmpty", functionDecl: "func AssertNotEmpty",
		},
		{
			runtime: "ts", path: "src/identity/encrypter.ts",
			template: `export class EncryptedPassword {
  private constructor(private readonly text: string) {}

  /** The one door: invariant not-empty. */
  static from(text: string): EncryptedPassword {
    checkText(text);
    return new EncryptedPassword(text);
  }
}

export class Validator {
  assertNotEmpty(text: string): void {
    checkText(text);
  }
}

export function assertNotEmpty(encrypted: string): void {
  checkText(encrypted);
}

function checkText(text: string): void {
  if (text === "") throw new Error("an encrypted password is never empty");
}

export class Encrypter {
METHOD
  encrypt(plain: string, v: Validator): EncryptedPassword {
    const encrypted = "pbkdf2$" + plain;
    CALL;
    return EncryptedPassword.from(encrypted);
  }
}
`,
			method:     "  assertNotEmpty(encrypted: string): void {\n    checkText(encrypted);\n  }\n",
			ownMethod:  "this.assertNotEmpty(encrypted)",
			function:   "assertNotEmpty(encrypted)",
			other:      "v.assertNotEmpty(encrypted)",
			helper:     "checkText(encrypted)",
			methodDecl: "  assertNotEmpty(encrypted", functionDecl: "export function assertNotEmpty",
		},
		{
			runtime: "py", path: "src/identity/encrypter.py",
			template: `class EncryptedPassword:
    """The protected form of a password."""

    def __init__(self, text: str) -> None:
        _check_text(text)
        self._text = text


def _check_text(text: str) -> None:
    if not text:
        raise ValueError("an encrypted password is never empty")


def assert_not_empty(encrypted: str) -> None:
    _check_text(encrypted)


class Validator:
    def assert_not_empty(self, text: str) -> None:
        _check_text(text)


class Encrypter:
METHOD
    def encrypt(self, plain: str, v: Validator) -> EncryptedPassword:
        encrypted = "pbkdf2$" + plain
        CALL
        return EncryptedPassword(encrypted)
`,
			method:     "    def assert_not_empty(self, encrypted: str) -> None:\n        _check_text(encrypted)\n",
			ownMethod:  "self.assert_not_empty(encrypted)",
			function:   "assert_not_empty(encrypted)",
			other:      "v.assert_not_empty(encrypted)",
			helper:     "_check_text(encrypted)",
			methodDecl: "    def assert_not_empty(self, encrypted", functionDecl: "def assert_not_empty(encrypted",
		},
	}
}

// source writes the service with or without its own check method,
// making call at CALL.
func (s serviceSource) source(withMethod bool, call string) string {
	method := ""
	if withMethod {
		method = s.method
	}
	return strings.Replace(strings.Replace(s.template, "METHOD\n", method, 1), "CALL", call, 1)
}

// lineOf is the line of the first line of text beginning with prefix.
func lineOf(t *testing.T, text, prefix string) int {
	t.Helper()
	for i, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, prefix) {
			return i + 1
		}
	}
	t.Fatalf("no line begins with %q", prefix)
	return 0
}

// TestServiceOperationCallsItsCheck holds a concrete service's
// assertion to the call its operation makes: the service's own method
// or the function of the context conforms, and the guarantee is
// anchored at the one the operation calls, even when the service also
// declares a method of the same name; a method of another type spelled
// the same, or a helper that is not the check, is a finding, though a
// function of the context spelled like the check exists.
func TestServiceOperationCallsItsCheck(t *testing.T) {
	finding := "service Encrypter: operation "
	for _, s := range serviceSources() {
		for _, tc := range []struct {
			name       string
			withMethod bool
			call       string
			conforms   bool
			anchor     string
		}{
			{"own method", true, s.ownMethod, true, s.methodDecl},
			{"function of the context", false, s.function, true, s.functionDecl},
			{"function of the context beside a method of the same name", true, s.function, true, s.functionDecl},
			{"another type's method of the same name", false, s.other, false, ""},
			{"a helper that is not the check", true, s.helper, false, ""},
		} {
			t.Run(s.runtime+" "+tc.name, func(t *testing.T) {
				root := t.TempDir()
				write(t, root, "rules.arclint.yaml", "runtime: ["+s.runtime+"]\nzones:\n  src:\n    paths: [\"src/**\"]\n")
				write(t, root, "domain.arclint.yaml", declaredServiceDomain)
				for path, content := range s.module {
					write(t, root, path, content)
				}
				text := s.source(tc.withMethod, tc.call)
				write(t, root, s.path, text)
				if !tc.conforms {
					findings, status := declaredServiceFindings(t, root, 1)
					if len(findings) != 1 || findings[0].RuleID != "assertion/checked-by-its-operation" || !strings.HasPrefix(findings[0].Message, finding) || !strings.Contains(findings[0].Message, " does not call ") || status != "violates" {
						t.Fatalf("want one finding %q..., got %s %+v", finding, status, findings)
					}
					return
				}
				if findings, status := declaredServiceFindings(t, root, 0); len(findings) != 0 || status != "structurally_checked" {
					t.Fatalf("the operation calls the check: %s %+v", status, findings)
				}
				stdout := mustRunDomain(t, root, "domain", "overview")
				if want := "source: " + s.path + ":" + strconv.Itoa(lineOf(t, text, tc.anchor)); !strings.Contains(stdout, want) {
					t.Fatalf("overview does not anchor the guarantee at the check (%s):\n%s", want, stdout)
				}
			})
		}
	}
}

// TestOperationWithoutTheCallIsAFinding holds a service whose
// operation has no body, or a body that does not call the check, to
// one finding, whatever function of the context or returned value
// object exists: a Go or TypeScript interface method and a TypeScript
// abstract method have no body; a Python abstract method or Protocol
// method has one that calls nothing.
func TestOperationWithoutTheCallIsAFinding(t *testing.T) {
	goMod := map[string]string{"go.mod": "module example.com/identity\n\ngo 1.22\n"}
	noBody := "service Encrypter: operation %s has no body, so it does not call %s and assertion not-empty is not checked"
	noCall := "service Encrypter: operation encrypt does not call assert_not_empty, so assertion not-empty is not checked"
	for _, tc := range []struct {
		name, runtime, path, source, want string
		module                            map[string]string
	}{
		{"go interface", "go", "src/identity/encrypter.go", "package identity\n\n// EncryptedPassword is the protected form of a password.\ntype EncryptedPassword struct{ text string }\n\n// NewEncryptedPassword is the one door: invariant not-empty.\nfunc NewEncryptedPassword(text string) (EncryptedPassword, error) { return EncryptedPassword{text: text}, nil }\n\n// Encrypter protects a password.\ntype Encrypter interface {\n\tEncrypt(plain string) (EncryptedPassword, error)\n}\n\n// AssertNotEmpty checks not-empty.\nfunc AssertNotEmpty(encrypted EncryptedPassword) error { return nil }\n", fmt.Sprintf(noBody, "Encrypt", "AssertNotEmpty"), goMod},
		{"typescript interface", "ts", "src/identity/encrypter.ts", "export class EncryptedPassword {\n  private constructor(private readonly text: string) {}\n  static from(text: string): EncryptedPassword {\n    return new EncryptedPassword(text);\n  }\n}\n\nexport interface Encrypter {\n  encrypt(plain: string): EncryptedPassword;\n}\n\nexport function assertNotEmpty(encrypted: EncryptedPassword): void {}\n", fmt.Sprintf(noBody, "encrypt", "assertNotEmpty"), nil},
		{"typescript abstract class", "ts", "src/identity/encrypter.ts", "export class EncryptedPassword {\n  private constructor(private readonly text: string) {}\n  static from(text: string): EncryptedPassword {\n    return new EncryptedPassword(text);\n  }\n}\n\nexport abstract class Encrypter {\n  abstract encrypt(plain: string): EncryptedPassword;\n}\n", fmt.Sprintf(noBody, "encrypt", "assertNotEmpty"), nil},
		{"python abc", "py", "src/identity/encrypter.py", "from abc import ABC, abstractmethod\n\n\nclass EncryptedPassword:\n    def __init__(self, text: str) -> None:\n        self._text = text\n\n\nclass Encrypter(ABC):\n    @abstractmethod\n    def encrypt(self, plain: str) -> EncryptedPassword: ...\n\n\ndef assert_not_empty(encrypted: EncryptedPassword) -> None:\n    pass\n", noCall, nil},
		{"python protocol", "py", "src/identity/encrypter.py", "from typing import Protocol\n\n\nclass EncryptedPassword:\n    def __init__(self, text: str) -> None:\n        self._text = text\n\n\nclass Encrypter(Protocol):\n    def encrypt(self, plain: str) -> EncryptedPassword: ...\n", noCall, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "rules.arclint.yaml", "runtime: ["+tc.runtime+"]\nzones:\n  src:\n    paths: [\"src/**\"]\n")
			write(t, root, "domain.arclint.yaml", declaredServiceDomain)
			for path, content := range tc.module {
				write(t, root, path, content)
			}
			write(t, root, tc.path, tc.source)
			findings, status := declaredServiceFindings(t, root, 1)
			if len(findings) != 1 || findings[0].RuleID != "assertion/checked-by-its-operation" || findings[0].Path != tc.path || findings[0].Message != tc.want || status != "violates" {
				t.Fatalf("want one finding %q, got %s %+v", tc.want, status, findings)
			}
		})
	}
}

// declaredServiceFindings runs check and returns every violation and
// the coverage status of the Encrypter guarantee not-empty; the exit
// code says whether any error-severity finding remains.
func declaredServiceFindings(t *testing.T, root string, wantExit int) ([]diagnosticDoc, string) {
	t.Helper()
	stdout, stderr, code := runBin(t, root, os.Environ(), "check", "--format", "json")
	if code != wantExit {
		t.Fatalf("check: exit %d, want %d\nstdout: %s\nstderr: %s", code, wantExit, stdout, stderr)
	}
	var diagnostics []struct {
		diagnosticDoc
		Contract *struct {
			Owner  string `json:"owner"`
			Key    string `json:"key"`
			Status string `json:"status"`
		} `json:"contract"`
	}
	if err := json.Unmarshal([]byte(stdout), &diagnostics); err != nil {
		t.Fatalf("check json: %v\n%s", err, stdout)
	}
	var findings []diagnosticDoc
	status := ""
	for _, d := range diagnostics {
		if d.Kind == "violation" {
			findings = append(findings, d.diagnosticDoc)
		}
		if c := d.Contract; c != nil && c.Owner == "Encrypter" && c.Key == "not-empty" {
			status = c.Status
		}
	}
	return findings, status
}

// TestDefineAssertionSaysWhereItIsChecked defines an assertion and reads
// the one line that says where its guarantee is checked, chosen by how
// the owner is declared: not yet, as an interface, as a concrete type,
// or as an aggregate.
func TestDefineAssertionSaysWhereItIsChecked(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", "runtime: [go]\nzones:\n  src:\n    paths: [\"src/**\"]\n")
	for _, args := range [][]string{
		{"domain", "define", "bounded_context", "identity", "--definition", "Who may sign in, and with what."},
		{"domain", "define", "domain_service", "Encrypter", "--definition", "Protects a password."},
		{"domain", "define", "aggregate", "User", "--definition", "A person who may sign in.", "--identity", "UserID"},
	} {
		mustRunDomain(t, root, args...)
	}
	define := []string{"domain", "define", "assertion", "not-empty", "--owner", "Encrypter", "--on", "Encrypt", "--statement", "An encrypted password is never empty."}

	stdout := mustRunDomain(t, root, define...)
	want := "  check: Encrypter is not declared yet. Declared as a concrete type, Encrypt calls AssertNotEmpty, a method of Encrypter or a function of context identity.\n"
	if !strings.HasPrefix(stdout, "Defined assertion not-empty under Encrypter in context identity.\n") || !strings.HasSuffix(stdout, want) {
		t.Fatalf("undeclared owner:\n got %q\nwant suffix %q", stdout, want)
	}

	write(t, root, "src/identity/encrypter.go", "package identity\n\n// Encrypter protects a password.\ntype Encrypter interface {\n\tEncrypt(plain string) (string, error)\n}\n")
	stdout = mustRunDomain(t, root, define...)
	if want := "Unchanged assertion not-empty under Encrypter in context identity.\n  check: Encrypt calls AssertNotEmpty, a method of Encrypter or a function of context identity.\n"; stdout != want {
		t.Fatalf("interface owner:\n got %q\nwant %q", stdout, want)
	}
	stdout = mustRunDomain(t, root, append(define, "--format", "json")...)
	var doc struct {
		Check struct {
			Home, Check, Operation string
		} `json:"check"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("define json: %v\n%s", err, stdout)
	}
	if doc.Check.Home != "service" || doc.Check.Check != "AssertNotEmpty" || doc.Check.Operation != "Encrypt" {
		t.Fatalf("define json check: %+v\n%s", doc.Check, stdout)
	}

	write(t, root, "src/identity/encrypter.go", "package identity\n\n// Encrypter protects a password.\ntype Encrypter struct{}\n\n// Encrypt protects a password.\nfunc (Encrypter) Encrypt(plain string) (string, error) { return plain, nil }\n")
	stdout = mustRunDomain(t, root, define...)
	if want := "  check: Encrypt calls AssertNotEmpty, a method of Encrypter or a function of context identity.\n"; !strings.HasSuffix(stdout, want) {
		t.Fatalf("concrete owner:\n got %q\nwant suffix %q", stdout, want)
	}

	stdout = mustRunDomain(t, root, "domain", "define", "assertion", "credentials-current", "--owner", "User", "--on", "ChangePassword", "--statement", "A user signs in with the password it chose last.")
	if want := "  check: ChangePassword calls AssertCredentialsCurrent, a method of the root of User or a function of context identity.\n"; !strings.HasSuffix(stdout, want) {
		t.Fatalf("aggregate owner:\n got %q\nwant suffix %q", stdout, want)
	}
}

// TestDefineAssertionWithoutARuntime records an assertion in a project
// whose ruleset names no language: the line spells the check from the
// key, and the define succeeds.
func TestDefineAssertionWithoutARuntime(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", "zones:\n  src:\n    paths: [\"src/**\"]\n")
	mustRunDomain(t, root, "domain", "define", "bounded_context", "identity", "--definition", "Who may sign in, and with what.")
	mustRunDomain(t, root, "domain", "define", "domain_service", "Encrypter", "--definition", "Protects a password.")
	stdout := mustRunDomain(t, root, "domain", "define", "assertion", "not-empty", "--owner", "Encrypter", "--on", "Encrypt", "--statement", "An encrypted password is never empty.")
	if want := "Encrypt calls assert-not-empty (spelled in the language's method case), a method of Encrypter or a function of context identity"; !strings.Contains(stdout, want) {
		t.Fatalf("no runtime:\n got %q\nwant it to contain %q", stdout, want)
	}
}

// TestDefineAssertionOnAnAbstractRoot says where an aggregate's
// assertion is checked whatever its root is declared as: its command
// calls the check.
func TestDefineAssertionOnAnAbstractRoot(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", "runtime: [ts]\nzones:\n  src:\n    paths: [\"src/**\"]\n")
	write(t, root, "src/identity/user.ts", "export abstract class User {\n  abstract changePassword(): void;\n}\n")
	mustRunDomain(t, root, "domain", "define", "bounded_context", "identity", "--definition", "Who may sign in, and with what.")
	mustRunDomain(t, root, "domain", "define", "aggregate", "User", "--definition", "A person who may sign in.", "--identity", "UserID")
	stdout := mustRunDomain(t, root, "domain", "define", "assertion", "credentials-current", "--owner", "User", "--on", "ChangePassword", "--statement", "A user signs in with the password it chose last.")
	if want := "  check: changePassword calls assertCredentialsCurrent, a method of the root of User or a function of context identity.\n"; !strings.HasSuffix(stdout, want) {
		t.Fatalf("abstract root:\n got %q\nwant suffix %q", stdout, want)
	}
}
