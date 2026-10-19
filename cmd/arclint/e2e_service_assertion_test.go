package main

// End-to-end coverage of a domain service's assertion through the
// binary, over real Go, TypeScript and Python source: the operation the
// assertion names calls the check, assert followed by the key, however
// the service is declared.

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

// serviceAssertionDomain records the Encrypter service with a guarantee
// of its operation Encrypt.
const serviceAssertionDomain = `version: 1
project: identity
contexts:
  identity:
    definition: Who may sign in, and with what.
    services:
      Encrypter:
        definition: Protects a password.
        assertions:
          not-empty:
            on: Encrypt
            statement: An encrypted password is never empty.
`

// serviceAssertionCase is one language's Encrypter: calling is the
// service whose Encrypt calls its check, and the call is the line that
// does, removed in the case without it.
type serviceAssertionCase struct {
	runtime string
	path    string
	calling string
	call    string
	// operationLine and checkLine are where Encrypt and the check are
	// declared in calling.
	operationLine, checkLine int
	finding                  string
	module                   map[string]string
}

func serviceAssertionCases() []serviceAssertionCase {
	return []serviceAssertionCase{
		{
			runtime: "go", path: "src/identity/encrypter.go",
			calling: `package identity

import (
	"errors"
	"strings"
)

// Encrypter protects a password.
type Encrypter struct{}

// Encrypt protects a password.
func (e Encrypter) Encrypt(plain string) (string, error) {
	encrypted := strings.Repeat("*", len(plain))
	if err := e.AssertNotEmpty(encrypted); err != nil {
		return "", err
	}
	return encrypted, nil
}

// AssertNotEmpty checks not-empty.
func (Encrypter) AssertNotEmpty(encrypted string) error {
	if encrypted == "" {
		return errors.New("an encrypted password is never empty")
	}
	return nil
}
`,
			call: `	if err := e.AssertNotEmpty(encrypted); err != nil {
		return "", err
	}
`,
			operationLine: 12, checkLine: 21,
			finding: "service Encrypter: operation Encrypt does not call AssertNotEmpty, so assertion not-empty is not checked when it completes",
			module:  map[string]string{"go.mod": "module example.com/identity\n\ngo 1.22\n"},
		},
		{
			runtime: "ts", path: "src/identity/encrypter.ts",
			calling: `export class Encrypter {
  encrypt(plain: string): string {
    const encrypted = "*".repeat(plain.length);
    this.assertNotEmpty(encrypted);
    return encrypted;
  }

  assertNotEmpty(encrypted: string): void {
    if (encrypted === "") throw new Error("an encrypted password is never empty");
  }
}
`,
			call:          "    this.assertNotEmpty(encrypted);\n",
			operationLine: 2, checkLine: 8,
			finding: "service Encrypter: operation encrypt does not call assertNotEmpty, so assertion not-empty is not checked when it completes",
		},
		{
			runtime: "py", path: "src/identity/encrypter.py",
			calling: `class Encrypter:
    def encrypt(self, plain: str) -> str:
        encrypted = "*" * len(plain)
        self.assert_not_empty(encrypted)
        return encrypted

    def assert_not_empty(self, encrypted: str) -> None:
        if not encrypted:
            raise ValueError("an encrypted password is never empty")
`,
			call:          "        self.assert_not_empty(encrypted)\n",
			operationLine: 2, checkLine: 7,
			finding: "service Encrypter: operation encrypt does not call assert_not_empty, so assertion not-empty is not checked when it completes",
		},
	}
}

// serviceAssertionProject writes a project of one language holding the
// Encrypter recording and the source.
func serviceAssertionProject(t *testing.T, runtime string, module map[string]string, path, source string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", "runtime: ["+runtime+"]\nzones:\n  src:\n    paths: [\"src/**\"]\n")
	write(t, root, "domain.arclint.yaml", serviceAssertionDomain)
	for p, content := range module {
		write(t, root, p, content)
	}
	write(t, root, path, source)
	return root
}

// TestServiceOperationCallsItsAssertion holds the assertion of a domain
// service to the call its operation makes, in every supported language:
// the operation calling the check conforms, and the same operation
// without the call is one finding at the operation.
func TestServiceOperationCallsItsAssertion(t *testing.T) {
	for _, tc := range serviceAssertionCases() {
		t.Run(tc.runtime, func(t *testing.T) {
			root := serviceAssertionProject(t, tc.runtime, tc.module, tc.path, tc.calling)
			if findings, status := serviceAssertionFindings(t, root, 0); len(findings) != 0 || status != "structurally_checked" {
				t.Fatalf("an operation calling its check must conform: %s %+v", status, findings)
			}
			stdout := mustRunDomain(t, root, "domain", "overview")
			if want := "source: " + tc.path + ":" + strconv.Itoa(tc.checkLine); !strings.Contains(stdout, want) {
				t.Fatalf("overview does not anchor the guarantee at its check (%s):\n%s", want, stdout)
			}

			uncalled := strings.Replace(tc.calling, tc.call, "", 1)
			if uncalled == tc.calling {
				t.Fatal("the fixture does not hold the call")
			}
			root = serviceAssertionProject(t, tc.runtime, tc.module, tc.path, uncalled)
			findings, _ := serviceAssertionFindings(t, root, 1)
			if len(findings) != 1 {
				t.Fatalf("an operation without the call: want one finding, got %+v", findings)
			}
			f := findings[0]
			if f.RuleID != "assertion/checked-by-its-operation" || f.Path != tc.path || f.Line != tc.operationLine || f.Message != tc.finding {
				t.Fatalf("finding:\n got %+v\nwant %s:%d %q", f, tc.path, tc.operationLine, tc.finding)
			}
		})
	}
}

// TestAbstractServiceOperationIsAFinding judges a domain service declared
// as an interface, an abstract class, an ABC or a Protocol like any
// other: its operation makes no call to the check, so it is a finding.
func TestAbstractServiceOperationIsAFinding(t *testing.T) {
	for _, tc := range []struct {
		name, runtime, path, source string
		module                      map[string]string
		finding                     string
	}{
		{"go interface", "go", "src/identity/encrypter.go",
			"package identity\n\n// Encrypter protects a password.\ntype Encrypter interface {\n\tEncrypt(plain string) (string, error)\n\tAssertNotEmpty(encrypted string) error\n}\n",
			map[string]string{"go.mod": "module example.com/identity\n\ngo 1.22\n"},
			"service Encrypter: operation Encrypt does not call AssertNotEmpty, so assertion not-empty is not checked when it completes"},
		{"typescript interface", "ts", "src/identity/encrypter.ts",
			"export interface Encrypter {\n  encrypt(plain: string): string;\n  assertNotEmpty(encrypted: string): void;\n}\n", nil,
			"service Encrypter: operation encrypt does not call assertNotEmpty, so assertion not-empty is not checked when it completes"},
		{"typescript abstract class", "ts", "src/identity/encrypter.ts",
			"export abstract class Encrypter {\n  abstract encrypt(plain: string): string;\n\n  assertNotEmpty(encrypted: string): void {\n    if (encrypted === \"\") throw new Error(\"empty\");\n  }\n}\n", nil,
			"service Encrypter: operation encrypt does not call assertNotEmpty, so assertion not-empty is not checked when it completes"},
		{"python abc", "py", "src/identity/encrypter.py",
			"from abc import ABC, abstractmethod\n\n\nclass Encrypter(ABC):\n    @abstractmethod\n    def encrypt(self, plain: str) -> str: ...\n\n    def assert_not_empty(self, encrypted: str) -> None:\n        if not encrypted:\n            raise ValueError(\"empty\")\n", nil,
			"service Encrypter: operation encrypt does not call assert_not_empty, so assertion not-empty is not checked when it completes"},
		{"python protocol", "py", "src/identity/encrypter.py",
			"from typing import Protocol\n\n\nclass Encrypter(Protocol):\n    def encrypt(self, plain: str) -> str: ...\n\n    def assert_not_empty(self, encrypted: str) -> None: ...\n", nil,
			"service Encrypter: operation encrypt does not call assert_not_empty, so assertion not-empty is not checked when it completes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := serviceAssertionProject(t, tc.runtime, tc.module, tc.path, tc.source)
			findings, _ := serviceAssertionFindings(t, root, 1)
			if len(findings) != 1 || findings[0].RuleID != "assertion/checked-by-its-operation" || findings[0].Message != tc.finding {
				t.Fatalf("want one finding %q, got %+v", tc.finding, findings)
			}
		})
	}
}

// serviceAssertionFindings runs check and returns every violation and
// the coverage status of the Encrypter guarantee not-empty; the exit
// code says whether any error-severity finding remains.
func serviceAssertionFindings(t *testing.T, root string, wantExit int) ([]diagnosticDoc, string) {
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
// the one line that says where its guarantee is checked: the owner
// declares the check and the operation calls it, whether the owner is
// not declared yet, declared as an interface or a concrete type, or an
// aggregate, including one whose root is an abstract class.
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
	want := "  check: Encrypter is not declared yet; once it is, it declares AssertNotEmpty, and Encrypt calls it.\n"
	if !strings.HasPrefix(stdout, "Defined assertion not-empty under Encrypter in context identity.\n") || !strings.HasSuffix(stdout, want) {
		t.Fatalf("undeclared owner:\n got %q\nwant suffix %q", stdout, want)
	}

	for _, source := range []string{
		"package identity\n\n// Encrypter protects a password.\ntype Encrypter interface {\n\tEncrypt(plain string) (string, error)\n}\n",
		"package identity\n\n// Encrypter protects a password.\ntype Encrypter struct{}\n\n// Encrypt protects a password.\nfunc (Encrypter) Encrypt(plain string) (string, error) { return plain, nil }\n",
	} {
		write(t, root, "src/identity/encrypter.go", source)
		stdout = mustRunDomain(t, root, define...)
		if want := "Unchanged assertion not-empty under Encrypter in context identity.\n  check: Encrypter declares AssertNotEmpty, and Encrypt calls it.\n"; stdout != want {
			t.Fatalf("declared owner:\n got %q\nwant %q", stdout, want)
		}
	}
	stdout = mustRunDomain(t, root, append(define, "--format", "json")...)
	var doc struct {
		Check map[string]string `json:"check"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("define json: %v\n%s", err, stdout)
	}
	if want := map[string]string{"home": "service", "check": "AssertNotEmpty", "operation": "Encrypt"}; len(doc.Check) != len(want) || doc.Check["home"] != want["home"] || doc.Check["check"] != want["check"] || doc.Check["operation"] != want["operation"] {
		t.Fatalf("define json check: %+v\n%s", doc.Check, stdout)
	}

	stdout = mustRunDomain(t, root, "domain", "define", "assertion", "credentials-current", "--owner", "User", "--on", "ChangePassword", "--statement", "A user signs in with the password it chose last.")
	if want := "  check: the root of User declares AssertCredentialsCurrent, and ChangePassword calls it.\n"; !strings.HasSuffix(stdout, want) {
		t.Fatalf("aggregate owner:\n got %q\nwant suffix %q", stdout, want)
	}
	write(t, root, "src/identity/user.go", "package identity\n\n// User is a person who may sign in.\ntype User interface {\n\tChangePassword() error\n}\n")
	stdout = mustRunDomain(t, root, "domain", "define", "assertion", "credentials-current", "--owner", "User", "--on", "ChangePassword", "--statement", "A user signs in with the password it chose last.")
	if want := "  check: the root of User declares AssertCredentialsCurrent, and ChangePassword calls it.\n"; !strings.HasSuffix(stdout, want) {
		t.Fatalf("interface root:\n got %q\nwant suffix %q", stdout, want)
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
	if want := "it declares assert-not-empty (spelled in the language's method case), and Encrypt calls it."; !strings.Contains(stdout, want) {
		t.Fatalf("no runtime:\n got %q\nwant it to contain %q", stdout, want)
	}
}
