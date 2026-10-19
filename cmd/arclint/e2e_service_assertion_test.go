package main

// End-to-end coverage of assertion/checked-by-its-operation for a domain
// service in every supported language: the operation an assertion names
// calls the check in its own body, the same for every operation.

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

// serviceAssertionCase is one language's Encrypter in four states: its
// operation calls the check, its operation does not, its operation is
// declared with no body, and only a same-named method of another type
// calls the check.
type serviceAssertionCase struct {
	name    string
	runtime string
	path    string
	// checked has the operation call the check; the other states edit it.
	checked   string
	unchecked string
	bodiless  string
	elsewhere string
	// operation and check are the names as the language spells them;
	// checkDecl is the text that declares the check.
	operation, check, checkDecl string
	// module is the file that makes the source one module its language
	// resolves, where it needs one.
	module map[string]string
}

const goCheckedEncrypter = `package identity

import "errors"

// Encrypter protects a password.
type Encrypter struct{}

// Encrypt protects a password.
func (e Encrypter) Encrypt(plain string) (string, error) {
	encrypted := "pbkdf2$" + plain
	if err := e.AssertNotEmpty(encrypted); err != nil {
		return "", err
	}
	return encrypted, nil
}

// AssertNotEmpty checks not-empty on Encrypt.
func (Encrypter) AssertNotEmpty(encrypted string) error {
	if encrypted == "" {
		return errors.New("an encrypted password is never empty")
	}
	return nil
}
`

const tsCheckedEncrypter = `export class Encrypter {
  encrypt(plain: string): string {
    const encrypted = "pbkdf2$" + plain;
    this.assertNotEmpty(encrypted);
    return encrypted;
  }

  assertNotEmpty(encrypted: string): void {
    if (encrypted === "") throw new Error("an encrypted password is never empty");
  }
}
`

const pyCheckedEncrypter = `class Encrypter:
    """Protects a password."""

    def encrypt(self, plain: str) -> str:
        encrypted = "pbkdf2$" + plain
        self.assert_not_empty(encrypted)
        return encrypted

    def assert_not_empty(self, encrypted: str) -> None:
        if not encrypted:
            raise ValueError("an encrypted password is never empty")
`

func serviceAssertionCases(t *testing.T) []serviceAssertionCase {
	edit := func(content, old, replacement string) string {
		t.Helper()
		out := strings.Replace(content, old, replacement, 1)
		if out == content {
			t.Fatalf("fixture edit did not apply: %q", old)
		}
		return out
	}
	goUnchecked := edit(goCheckedEncrypter, "\tif err := e.AssertNotEmpty(encrypted); err != nil {\n\t\treturn \"\", err\n\t}\n", "")
	tsUnchecked := edit(tsCheckedEncrypter, "    this.assertNotEmpty(encrypted);\n", "")
	pyUnchecked := edit(pyCheckedEncrypter, "        self.assert_not_empty(encrypted)\n", "")
	return []serviceAssertionCase{
		{
			name: "go", runtime: "go", path: "src/identity/encrypter.go",
			checked:   goCheckedEncrypter,
			unchecked: goUnchecked,
			bodiless: `package identity

// Encrypter protects a password.
type Encrypter interface {
	Encrypt(plain string) (string, error)
	AssertNotEmpty(encrypted string) error
}
`,
			elsewhere: goUnchecked + `
// Draft is not the service.
type Draft struct{ encrypter Encrypter }

// Encrypt of a draft is not the service's operation.
func (d Draft) Encrypt(plain string) (string, error) {
	return plain, d.encrypter.AssertNotEmpty(plain)
}
`,
			operation: "Encrypt", check: "AssertNotEmpty", checkDecl: "func (Encrypter) AssertNotEmpty(",
			module: map[string]string{"go.mod": "module example.com/identity\n\ngo 1.22\n"},
		},
		{
			name: "typescript", runtime: "ts", path: "src/identity/encrypter.ts",
			checked:   tsCheckedEncrypter,
			unchecked: tsUnchecked,
			bodiless: `export abstract class Encrypter {
  abstract encrypt(plain: string): string;

  assertNotEmpty(encrypted: string): void {
    if (encrypted === "") throw new Error("an encrypted password is never empty");
  }
}
`,
			elsewhere: tsUnchecked + `
export class Draft {
  encrypt(plain: string): string {
    new Encrypter().assertNotEmpty(plain);
    return plain;
  }
}
`,
			operation: "encrypt", check: "assertNotEmpty", checkDecl: "  assertNotEmpty(encrypted",
		},
		{
			name: "python", runtime: "py", path: "src/identity/encrypter.py",
			checked:   pyCheckedEncrypter,
			unchecked: pyUnchecked,
			bodiless: `from typing import Protocol


class Encrypter(Protocol):
    def encrypt(self, plain: str) -> str: ...

    def assert_not_empty(self, encrypted: str) -> None: ...
`,
			elsewhere: pyUnchecked + `

class Draft:
    """Not the service."""

    def encrypt(self, plain: str) -> str:
        Encrypter().assert_not_empty(plain)
        return plain
`,
			operation: "encrypt", check: "assert_not_empty", checkDecl: "def assert_not_empty(",
		},
	}
}

// TestServiceOperationCallsItsAssertion holds a domain service's
// operation to the call of its assertion's check in Go, TypeScript and
// Python: the call conforms; an operation without it, one declared with
// no body, and a call made only from a same-named method of another type
// are each the same finding at the operation.
func TestServiceOperationCallsItsAssertion(t *testing.T) {
	for _, tc := range serviceAssertionCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "rules.arclint.yaml", "runtime: ["+tc.runtime+"]\nzones:\n  src:\n    paths: [\"src/**\"]\n")
			write(t, root, "domain.arclint.yaml", serviceAssertionDomain)
			for path, content := range tc.module {
				write(t, root, path, content)
			}

			write(t, root, tc.path, tc.checked)
			if findings, status := serviceAssertionFindings(t, root, 0); len(findings) != 0 || status != "structurally_checked" {
				t.Fatalf("an operation calling its check must conform: %s %+v", status, findings)
			}
			stdout := mustRunDomain(t, root, "domain", "overview")
			if want := "source: " + tc.path + ":" + strconv.Itoa(lineOf(t, tc.checked, tc.checkDecl)); !strings.Contains(stdout, want) {
				t.Fatalf("overview does not anchor the guarantee at its check (%s):\n%s", want, stdout)
			}

			want := "service Encrypter: operation " + tc.operation + " does not call " + tc.check + ", so assertion not-empty is not checked when it completes"
			for _, state := range []struct{ name, content string }{
				{"unchecked", tc.unchecked},
				{"bodiless", tc.bodiless},
				{"called from another type", tc.elsewhere},
			} {
				write(t, root, tc.path, state.content)
				findings, status := serviceAssertionFindings(t, root, 1)
				if len(findings) != 1 || status != "violates" {
					t.Fatalf("%s: want one finding, got %s %+v", state.name, status, findings)
				}
				f := findings[0]
				if f.RuleID != "assertion/checked-by-its-operation" || f.Path != tc.path || f.Message != want ||
					f.Line != lineOf(t, state.content, tc.operation+"(") {
					t.Fatalf("%s finding:\n got %+v\nwant %q at %s:%d", state.name, f, want, tc.path, lineOf(t, state.content, tc.operation+"("))
				}
			}
		})
	}
}

// lineOf is the 1-based line of the first occurrence of text in content.
func lineOf(t *testing.T, content, text string) int {
	t.Helper()
	at := strings.Index(content, text)
	if at < 0 {
		t.Fatalf("%q is not in the fixture", text)
	}
	return strings.Count(content[:at], "\n") + 1
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
// the one line that names its check and the operation that calls it,
// before and after the owner is declared, and for an aggregate.
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
	want := "Defined assertion not-empty under Encrypter in context identity.\n  check: Encrypter is not declared yet. It declares AssertNotEmpty, and Encrypt calls it.\n"
	if stdout != want {
		t.Fatalf("undeclared owner:\n got %q\nwant %q", stdout, want)
	}

	write(t, root, "src/identity/encrypter.go", "package identity\n\n// Encrypter protects a password.\ntype Encrypter struct{}\n\n// Encrypt protects a password.\nfunc (Encrypter) Encrypt(plain string) (string, error) { return plain, nil }\n")
	stdout = mustRunDomain(t, root, define...)
	if want := "Unchanged assertion not-empty under Encrypter in context identity.\n  check: Encrypter declares AssertNotEmpty, and Encrypt calls it.\n"; stdout != want {
		t.Fatalf("declared owner:\n got %q\nwant %q", stdout, want)
	}
	stdout = mustRunDomain(t, root, append(define, "--format", "json")...)
	var doc struct {
		Check map[string]string `json:"check"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("define json: %v\n%s", err, stdout)
	}
	if len(doc.Check) != 3 || doc.Check["home"] != "service" || doc.Check["check"] != "AssertNotEmpty" || doc.Check["operation"] != "Encrypt" {
		t.Fatalf("define json check: %+v\n%s", doc.Check, stdout)
	}

	stdout = mustRunDomain(t, root, "domain", "define", "assertion", "credentials-current", "--owner", "User", "--on", "ChangePassword", "--statement", "A user signs in with the password it chose last.")
	if want := "  check: the root of User declares AssertCredentialsCurrent, and ChangePassword calls it.\n"; !strings.HasSuffix(stdout, want) {
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
	if want := "  check: Encrypter is not declared yet. It declares assert-not-empty (spelled in the language's method case), and Encrypt calls it.\n"; !strings.HasSuffix(stdout, want) {
		t.Fatalf("no runtime:\n got %q\nwant suffix %q", stdout, want)
	}
}
