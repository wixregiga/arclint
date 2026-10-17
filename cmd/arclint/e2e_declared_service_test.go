package main

// End-to-end coverage of a domain service declared as an abstraction
// (a Go interface, a TypeScript interface or abstract class, a Python
// ABC or Protocol) owning an assertion: the guarantee is checked in the
// model, by a function of the context spelled from the key or by the
// value object its operation returns, and never by a method the
// abstraction cannot hold.

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

// declaredServiceDomain records the Encrypter service with a guarantee
// of its operation Encrypt, and the value object Encrypt can return,
// whose invariant carries the same key.
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

// declaredServiceCase is one language's declared service in three
// states: returning a plain value with no check in the model, with the
// check declared as a function of the context, and returning the
// recorded value object.
type declaredServiceCase struct {
	name     string
	runtime  string
	path     string
	bare     string
	function string
	value    string
	// checkLine is the line of the function in the function state, where
	// the overview anchors the guarantee.
	checkLine int
	// valueDoorLine is the line of the value object's constructor in the
	// value state, where the overview anchors the guarantee.
	valueDoorLine int
	finding       string
	// module is the file that makes the source one module its language
	// resolves, where it needs one.
	module map[string]string
	// implPath holds an implementation of the service: implFunction
	// calls the function of the context, implValue returns the value
	// object.
	implPath, implFunction, implValue string
}

const goEncryptedPassword = `package identity

import "errors"

// EncryptedPassword is the protected form of a password.
type EncryptedPassword struct{ text string }

// NewEncryptedPassword is the one door: invariant not-empty.
func NewEncryptedPassword(text string) (EncryptedPassword, error) {
	if text == "" {
		return EncryptedPassword{}, errors.New("an encrypted password is never empty")
	}
	return EncryptedPassword{text: text}, nil
}
`

const tsEncryptedPassword = `export class EncryptedPassword {
  private constructor(private readonly text: string) {}

  /** The one door: invariant not-empty. */
  static from(text: string): EncryptedPassword {
    if (text === "") throw new Error("an encrypted password is never empty");
    return new EncryptedPassword(text);
  }
}
`

const pyEncryptedPassword = `class EncryptedPassword:
    """The protected form of a password."""

    def __init__(self, text: str) -> None:
        if not text:
            raise ValueError("an encrypted password is never empty")
        self._text = text
`

func declaredServiceCases() []declaredServiceCase {
	goInterface := goEncryptedPassword + `
// Encrypter protects a password.
type Encrypter interface {
	Encrypt(plain string) (string, error)
}
`
	goCheck := `
// AssertNotEmpty checks not-empty.
func AssertNotEmpty(encrypted string) error {
	if encrypted == "" {
		return errors.New("an encrypted password is never empty")
	}
	return nil
}
`
	tsInterface := tsEncryptedPassword + `
export interface Encrypter {
  encrypt(plain: string): string;
}
`
	tsAbstract := tsEncryptedPassword + `
export abstract class Encrypter {
  abstract encrypt(plain: string): string;
}
`
	tsCheck := `
export function assertNotEmpty(encrypted: string): void {
  if (encrypted === "") throw new Error("an encrypted password is never empty");
}
`
	pyABC := "from abc import ABC, abstractmethod\n\n\n" + pyEncryptedPassword + `

class Encrypter(ABC):
    @abstractmethod
    def encrypt(self, plain: str) -> str: ...
`
	pyProtocol := "from typing import Protocol\n\n\n" + pyEncryptedPassword + `

class Encrypter(Protocol):
    def encrypt(self, plain: str) -> str: ...
`
	pyCheck := `

def assert_not_empty(encrypted: str) -> None:
    if not encrypted:
        raise ValueError("an encrypted password is never empty")
`
	finding := "declared service Encrypter: not-empty has no check in context identity"
	goImpl := func(result, body string) string {
		return "package adapters\n\nimport \"example.com/identity/src/identity\"\n\n// Plain encrypts nothing.\ntype Plain struct{}\n\n// Encrypt protects a password.\nfunc (*Plain) Encrypt(plain string) " + result + " {\n" + body + "}\n"
	}
	goFunction := goImpl("(string, error)", "\tout := \"x\" + plain\n\tif err := identity.AssertNotEmpty(out); err != nil {\n\t\treturn \"\", err\n\t}\n\treturn out, nil\n")
	goValue := goImpl("(identity.EncryptedPassword, error)", "\treturn identity.NewEncryptedPassword(\"x\" + plain)\n")
	tsImpl := func(relation, imports, result, body string) string {
		return "import { " + imports + " } from \"../identity/password\";\n\nexport class Plain " + relation + " Encrypter {\n  encrypt(plain: string): " + result + " {\n" + body + "  }\n}\n"
	}
	tsFunction := "    const out = \"x\" + plain;\n    assertNotEmpty(out);\n    return out;\n"
	tsValue := "    return EncryptedPassword.from(\"x\" + plain);\n"
	pyImpl := func(imports, result, body string) string {
		return "from identity.password import " + imports + "\n\n\nclass Plain(Encrypter):\n    def encrypt(self, plain: str) -> " + result + ":\n" + body
	}
	pyFunction := pyImpl("Encrypter, assert_not_empty", "str", "        out = \"x\" + plain\n        assert_not_empty(out)\n        return out\n")
	pyValue := pyImpl("EncryptedPassword, Encrypter", "EncryptedPassword", "        return EncryptedPassword(\"x\" + plain)\n")
	pyModule := map[string]string{"src/identity/__init__.py": "", "src/adapters/__init__.py": ""}
	return []declaredServiceCase{
		{
			name: "go interface", runtime: "go", path: "src/identity/password.go",
			bare:      goInterface,
			function:  goInterface + goCheck,
			value:     strings.Replace(goInterface, "Encrypt(plain string) (string, error)", "Encrypt(plain string) (EncryptedPassword, error)", 1),
			checkLine: 22, valueDoorLine: 9, finding: finding,
			module:   map[string]string{"go.mod": "module example.com/identity\n\ngo 1.22\n"},
			implPath: "src/adapters/plain.go", implFunction: goFunction, implValue: goValue,
		},
		{
			name: "typescript interface", runtime: "ts", path: "src/identity/password.ts",
			bare:      tsInterface,
			function:  tsInterface + tsCheck,
			value:     strings.Replace(tsInterface, "encrypt(plain: string): string;", "encrypt(plain: string): EncryptedPassword;", 1),
			checkLine: 15, valueDoorLine: 2, finding: finding,
			implPath:     "src/adapters/plain.ts",
			implFunction: tsImpl("implements", "assertNotEmpty, type Encrypter", "string", tsFunction),
			implValue:    tsImpl("implements", "EncryptedPassword, type Encrypter", "EncryptedPassword", tsValue),
		},
		{
			name: "typescript abstract class", runtime: "ts", path: "src/identity/password.ts",
			bare:      tsAbstract,
			function:  tsAbstract + tsCheck,
			value:     strings.Replace(tsAbstract, "abstract encrypt(plain: string): string;", "abstract encrypt(plain: string): EncryptedPassword;", 1),
			checkLine: 15, valueDoorLine: 2, finding: finding,
			implPath:     "src/adapters/plain.ts",
			implFunction: tsImpl("extends", "assertNotEmpty, Encrypter", "string", tsFunction),
			implValue:    tsImpl("extends", "EncryptedPassword, Encrypter", "EncryptedPassword", tsValue),
		},
		{
			name: "python abc", runtime: "py", path: "src/identity/password.py",
			bare:      pyABC,
			function:  pyABC + pyCheck,
			value:     strings.Replace(pyABC, "def encrypt(self, plain: str) -> str: ...", "def encrypt(self, plain: str) -> EncryptedPassword: ...", 1),
			checkLine: 18, valueDoorLine: 7, finding: finding,
			module:   pyModule,
			implPath: "src/adapters/plain.py", implFunction: pyFunction, implValue: pyValue,
		},
		{
			name: "python protocol", runtime: "py", path: "src/identity/password.py",
			bare:      pyProtocol,
			function:  pyProtocol + pyCheck,
			value:     strings.Replace(pyProtocol, "def encrypt(self, plain: str) -> str: ...", "def encrypt(self, plain: str) -> EncryptedPassword: ...", 1),
			checkLine: 17, valueDoorLine: 7, finding: finding,
			module:   pyModule,
			implPath: "src/adapters/plain.py", implFunction: pyFunction, implValue: pyValue,
		},
	}
}

// TestDeclaredServiceAssertion holds a declared service's guarantee to
// a check in the model in every supported language: the model-side
// finding when neither shape exists, and conformance through a function
// of the context or through the value object the operation returns.
func TestDeclaredServiceAssertion(t *testing.T) {
	for _, tc := range declaredServiceCases() {
		t.Run(tc.name, func(t *testing.T) {
			if tc.value == tc.bare {
				t.Fatal("the value state did not change the bare fixture")
			}
			root := t.TempDir()
			write(t, root, "rules.arclint.yaml", "runtime: ["+tc.runtime+"]\nzones:\n  src:\n    paths: [\"src/**\"]\n")
			write(t, root, "domain.arclint.yaml", declaredServiceDomain)
			for path, content := range tc.module {
				write(t, root, path, content)
			}

			write(t, root, tc.path, tc.bare)
			findings, _ := declaredServiceFindings(t, root, 1)
			if len(findings) != 1 {
				t.Fatalf("bare declared service: want one finding, got %+v", findings)
			}
			f := findings[0]
			if f.RuleID != "assertion/checked-by-its-operation" || f.Path != tc.path || !strings.Contains(f.Message, tc.finding) {
				t.Fatalf("bare declared service finding:\n got %+v\nwant message containing %q", f, tc.finding)
			}
			for _, impossible := range []string{"names no checking method", "does not call"} {
				if strings.Contains(f.Message, impossible) {
					t.Fatalf("a declared service is asked for a method its declaration cannot hold: %s", f.Message)
				}
			}

			write(t, root, tc.path, tc.function)
			// A check in the model with no implementation observed is
			// undetermined, never conforming.
			if findings, status := declaredServiceFindings(t, root, 0); len(findings) != 0 || status != "undetermined" {
				t.Fatalf("a check with no implementation observed must be undetermined: %s %+v", status, findings)
			}
			write(t, root, tc.implPath, tc.implFunction)
			if findings, status := declaredServiceFindings(t, root, 0); len(findings) != 0 || status != "structurally_checked" {
				t.Fatalf("an implementation calling the function of the context must conform: %s %+v", status, findings)
			}
			stdout := mustRunDomain(t, root, "domain", "overview")
			if want := "source: " + tc.path + ":" + strconv.Itoa(tc.checkLine); !strings.Contains(stdout, want) {
				t.Fatalf("overview does not anchor the guarantee at its function (%s):\n%s", want, stdout)
			}

			write(t, root, tc.path, tc.value)
			write(t, root, tc.implPath, tc.implValue)
			if findings, status := declaredServiceFindings(t, root, 0); len(findings) != 0 || status != "structurally_checked" {
				t.Fatalf("an implementation returning the recorded value object must conform: %s %+v", status, findings)
			}
			stdout = mustRunDomain(t, root, "domain", "overview")
			if want := "not-empty (on Encrypt)"; !strings.Contains(stdout, want) {
				t.Fatalf("overview lacks the service assertion (%s):\n%s", want, stdout)
			}
			if want := "source: " + tc.path + ":" + strconv.Itoa(tc.valueDoorLine); strings.Count(stdout, want) < 2 {
				t.Fatalf("overview does not anchor the guarantee at the value's door (%s) as it anchors the value's invariant:\n%s", want, stdout)
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
	want := "  check: Encrypter is not declared yet. As an interface, an abstract class, an ABC, or a Protocol, it is checked in the model: the function AssertNotEmpty of context identity, or the invariant not-empty of a value object Encrypt returns, or else the tests named with --verified-by. As a concrete type, it declares AssertNotEmpty, and Encrypt calls it.\n"
	if !strings.HasPrefix(stdout, "Defined assertion not-empty under Encrypter in context identity.\n") || !strings.HasSuffix(stdout, want) {
		t.Fatalf("undeclared owner:\n got %q\nwant suffix %q", stdout, want)
	}

	write(t, root, "src/identity/encrypter.go", "package identity\n\n// Encrypter protects a password.\ntype Encrypter interface {\n\tEncrypt(plain string) (string, error)\n}\n")
	stdout = mustRunDomain(t, root, define...)
	if want := "Unchanged assertion not-empty under Encrypter in context identity.\n  check: Encrypter is declared as an interface, so the check is in the model: the function AssertNotEmpty of context identity, or the invariant not-empty of a value object Encrypt returns, or else the tests named with --verified-by.\n"; stdout != want {
		t.Fatalf("interface owner:\n got %q\nwant %q", stdout, want)
	}
	stdout = mustRunDomain(t, root, append(define, "--format", "json")...)
	var doc struct {
		Check struct {
			Home, Declared, Check, Operation string
		} `json:"check"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("define json: %v\n%s", err, stdout)
	}
	if doc.Check.Home != "model" || doc.Check.Declared != "interface" || doc.Check.Check != "AssertNotEmpty" || doc.Check.Operation != "Encrypt" {
		t.Fatalf("define json check: %+v\n%s", doc.Check, stdout)
	}

	write(t, root, "src/identity/encrypter.go", "package identity\n\n// Encrypter protects a password.\ntype Encrypter struct{}\n\n// Encrypt protects a password.\nfunc (Encrypter) Encrypt(plain string) (string, error) { return plain, nil }\n")
	stdout = mustRunDomain(t, root, define...)
	if want := "  check: Encrypter declares AssertNotEmpty, and Encrypt calls it.\n"; !strings.HasSuffix(stdout, want) {
		t.Fatalf("concrete owner:\n got %q\nwant suffix %q", stdout, want)
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
	if want := "the function assert-not-empty (spelled in the language's method case) of context identity"; !strings.Contains(stdout, want) {
		t.Fatalf("no runtime:\n got %q\nwant it to contain %q", stdout, want)
	}
}

// declaredServiceResult is one declared service whose operation's
// result names, or only seems to name, the recorded value object, with
// whether the language's resolution makes it that value.
type declaredServiceResult struct {
	name    string
	runtime string
	files   map[string]string
	value   bool
}

func declaredServiceResults() []declaredServiceResult {
	goValue := "package value\n\nimport \"errors\"\n\n// EncryptedPassword is the protected form of a password.\ntype EncryptedPassword struct{ text string }\n\n// NewEncryptedPassword is the one door: invariant not-empty.\nfunc NewEncryptedPassword(text string) (EncryptedPassword, error) {\n\tif text == \"\" {\n\t\treturn EncryptedPassword{}, errors.New(\"empty\")\n\t}\n\treturn EncryptedPassword{text: text}, nil\n}\n"
	goService := func(imports, result string) string {
		return "package service\n\nimport " + imports + "\n\n// Encrypter protects a password.\ntype Encrypter interface {\n\tEncrypt(plain string) " + result + "\n}\n"
	}
	goMod := "module example.com/identity\n\ngo 1.22\n"
	tsValue := "export class EncryptedPassword {\n  private constructor(private readonly text: string) {}\n  static from(text: string): EncryptedPassword {\n    if (text === \"\") throw new Error(\"empty\");\n    return new EncryptedPassword(text);\n  }\n}\n"
	tsOther := "export class OtherPassword {}\n"
	tsService := func(imports, result string) string {
		return imports + "\n\nexport interface Encrypter {\n  encrypt(plain: string): " + result + ";\n}\n"
	}
	pyValue := "class EncryptedPassword:\n    def __init__(self, text: str) -> None:\n        if not text:\n            raise ValueError(\"empty\")\n        self._text = text\n"
	pyService := func(imports, result string) string {
		return "from typing import Protocol\n" + imports + "\n\n\nclass Encrypter(Protocol):\n    def encrypt(self, plain: str) -> " + result + ": ...\n"
	}
	return []declaredServiceResult{
		{"go result through an aliased import", "go", map[string]string{"go.mod": goMod, "src/identity/value/password.go": goValue,
			"src/identity/service/encrypter.go": goService(`v "example.com/identity/src/identity/value"`, "(v.EncryptedPassword, error)")}, true},
		{"go callback taking the value", "go", map[string]string{"go.mod": goMod, "src/identity/value/password.go": goValue,
			"src/identity/service/encrypter.go": goService(`"example.com/identity/src/identity/value"`, "func(value.EncryptedPassword) string")}, false},
		{"go sized array of the value", "go", map[string]string{"go.mod": goMod, "src/identity/value/password.go": goValue,
			"src/identity/service/encrypter.go": goService(`"example.com/identity/src/identity/value"`, "[1]value.EncryptedPassword")}, false},
		{"typescript namespace import of a sibling module", "ts", map[string]string{"src/identity/value.ts": tsValue,
			"src/identity/encrypter.ts": tsService(`import * as value from "./value";`, "value.EncryptedPassword")}, true},
		{"typescript value imported under another name", "ts", map[string]string{"src/identity/value.ts": tsValue,
			"src/identity/encrypter.ts": tsService(`import { EncryptedPassword as Protected } from "./value";`, "Protected")}, true},
		{"typescript foreign type imported under the value's name", "ts", map[string]string{"src/identity/value.ts": tsValue, "src/identity/other.ts": tsOther,
			"src/identity/encrypter.ts": tsService(`import { OtherPassword as EncryptedPassword } from "./other";`, "EncryptedPassword")}, false},
		{"typescript promise of the value", "ts", map[string]string{"src/identity/value.ts": tsValue,
			"src/identity/encrypter.ts": tsService(`import { EncryptedPassword } from "./value";`, "Promise<EncryptedPassword>")}, true},
		{"typescript promise of a collection of the value", "ts", map[string]string{"src/identity/value.ts": tsValue,
			"src/identity/encrypter.ts": tsService(`import { EncryptedPassword } from "./value";`, "Promise<EncryptedPassword[]>")}, false},
		{"typescript parenthesized promise of the value", "ts", map[string]string{"src/identity/value.ts": tsValue,
			"src/identity/encrypter.ts": tsService(`import { EncryptedPassword } from "./value";`, "(Promise<EncryptedPassword>)")}, true},
		{"typescript alias of a promise of the value", "ts", map[string]string{"src/identity/value.ts": tsValue,
			"src/identity/encrypter.ts": tsService("import { EncryptedPassword } from \"./value\";\n\ntype Result<T> = Promise<T>;", "Result<EncryptedPassword>")}, true},
		{"typescript abstract operation beside a generic sibling", "ts", map[string]string{"src/identity/value.ts": tsValue,
			"src/identity/encrypter.ts": "import { EncryptedPassword } from \"./value\";\n\nexport abstract class Encrypter {\n  abstract other<EncryptedPassword>(): void;\n  abstract encrypt(plain: string): EncryptedPassword;\n}\n"}, true},
		{"typescript overloads on one line, one returning a string", "ts", map[string]string{"src/identity/value.ts": tsValue,
			"src/identity/encrypter.ts": "import { EncryptedPassword } from \"./value\";\n\nexport interface Encrypter {\n  encrypt(x: string): string; encrypt(x: Uint8Array): EncryptedPassword;\n}\n"}, false},
		{"typescript generic overload before one returning a string", "ts", map[string]string{"src/identity/value.ts": tsValue,
			"src/identity/encrypter.ts": "import { EncryptedPassword } from \"./value\";\n\nexport interface Encrypter {\n  encrypt<T>(x: T): T;\n  encrypt(x: string): string;\n}\n"}, false},
		{"python module attribute of a sibling module", "py", map[string]string{"src/identity/value.py": pyValue, "src/identity/__init__.py": "",
			"src/identity/encrypter.py": pyService("from identity import value", "value.EncryptedPassword")}, true},
	}
}

// TestDeclaredServiceResultResolvesThroughTheLanguage holds shape (b) to
// what the language's own resolution makes of the operation's result:
// the value object through any binding that names it, and never a type
// that only shares its name, a collection of it, or a function taking it.
func TestDeclaredServiceResultResolvesThroughTheLanguage(t *testing.T) {
	for _, tc := range declaredServiceResults() {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "rules.arclint.yaml", "runtime: ["+tc.runtime+"]\nzones:\n  src:\n    paths: [\"src/**\"]\n")
			write(t, root, "domain.arclint.yaml", declaredServiceDomain)
			for path, content := range tc.files {
				write(t, root, path, content)
			}
			want := 1
			if tc.value {
				want = 0
			}
			var findings []diagnosticDoc
			all, status := declaredServiceFindings(t, root, want)
			for _, f := range all {
				if f.RuleID == "assertion/checked-by-its-operation" {
					findings = append(findings, f)
				}
			}
			// With the value as the check and no implementation observed,
			// the guarantee is undetermined rather than a finding.
			if tc.value && (len(findings) != 0 || status != "undetermined") {
				t.Fatalf("the operation returns the recorded value: %s %+v", status, findings)
			}
			if !tc.value && (len(findings) != 1 || !strings.Contains(findings[0].Message, "declared service Encrypter: not-empty has no check in context identity")) {
				t.Fatalf("the operation does not return the recorded value: %+v", findings)
			}
		})
	}
}

// TestDefineAssertionOnAnAbstractRoot says that an aggregate whose root is
// declared as an abstract class is verified by its named tests, the
// only home the rule accepts for it.
func TestDefineAssertionOnAnAbstractRoot(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", "runtime: [ts]\nzones:\n  src:\n    paths: [\"src/**\"]\n")
	write(t, root, "src/identity/user.ts", "export abstract class User {\n  abstract changePassword(): void;\n}\n")
	mustRunDomain(t, root, "domain", "define", "bounded_context", "identity", "--definition", "Who may sign in, and with what.")
	mustRunDomain(t, root, "domain", "define", "aggregate", "User", "--definition", "A person who may sign in.", "--identity", "UserID")
	stdout := mustRunDomain(t, root, "domain", "define", "assertion", "credentials-current", "--owner", "User", "--on", "ChangePassword", "--statement", "A user signs in with the password it chose last.")
	if want := "  check: the root of User is declared as an abstract class, with no body to call assertCredentialsCurrent from, so the tests named with --verified-by verify it, and changePassword is declared on it.\n"; !strings.HasSuffix(stdout, want) {
		t.Fatalf("abstract root:\n got %q\nwant suffix %q", stdout, want)
	}
}

// skippingImplementationModule is a Go module whose Encrypter interface
// has its guarantee checked in the model by AssertNotEmpty, and two
// implementations: Checked calls the check, Skipping, with a pointer
// receiver, does not.
func skippingImplementationModule(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", "runtime: [go]\nzones:\n  identity:\n    paths: [\"src/identity/**\"]\n  adapters:\n    paths: [\"src/adapters/**\"]\n")
	write(t, root, "domain.arclint.yaml", declaredServiceDomain)
	write(t, root, "go.mod", "module example.com/identity\n\ngo 1.22\n")
	write(t, root, "src/identity/password.go", goEncryptedPassword+`
// Encrypter protects a password.
type Encrypter interface {
	Encrypt(plain string) (string, error)
}

// AssertNotEmpty checks not-empty.
func AssertNotEmpty(encrypted string) error {
	if encrypted == "" {
		return errors.New("an encrypted password is never empty")
	}
	return nil
}
`)
	write(t, root, "src/adapters/checked.go", `package adapters

import "example.com/identity/src/identity"

// Checked encrypts and checks the result.
type Checked struct{}

// Encrypt protects a password.
func (Checked) Encrypt(plain string) (string, error) {
	out := "x" + plain
	if err := identity.AssertNotEmpty(out); err != nil {
		return "", err
	}
	return out, nil
}
`)
	write(t, root, "src/adapters/skipping.go", `package adapters

// Skipping encrypts without the check.
type Skipping struct{}

// Encrypt protects a password.
func (*Skipping) Encrypt(plain string) (string, error) {
	return plain, nil
}
`)
	return root
}

// TestDeclaredServiceImplementationSkipsTheCheck holds every
// implementation of a declared service to the check in the model: the
// one whose operation skips it is the finding, anchored at that
// operation, and context and domain show name each implementation with
// what its operation must call.
func TestDeclaredServiceImplementationSkipsTheCheck(t *testing.T) {
	root := skippingImplementationModule(t)
	findings, status := declaredServiceFindings(t, root, 1)
	if len(findings) != 1 || status != "violates" {
		t.Fatalf("want one finding, got %s %+v", status, findings)
	}
	f := findings[0]
	want := "implementation Skipping of declared service Encrypter: Skipping.Encrypt does not call AssertNotEmpty"
	if f.RuleID != "assertion/checked-by-its-operation" || f.Path != "src/adapters/skipping.go" || f.Line != 7 || f.Message != want {
		t.Fatalf("finding:\n got %+v\nwant %q at src/adapters/skipping.go:7", f, want)
	}

	stdout := mustRunDomain(t, root, "context", "src/adapters/skipping.go")
	if want := "implements Encrypter: Skipping.Encrypt must call AssertNotEmpty (assertion not-empty): src/adapters/skipping.go:7"; !strings.Contains(stdout, want) {
		t.Fatalf("context does not name the implementation (%s):\n%s", want, stdout)
	}
	if strings.Contains(stdout, "Checked.Encrypt") {
		t.Fatalf("context lists an implementation outside the worksite:\n%s", stdout)
	}

	stdout = mustRunDomain(t, root, "domain", "show", "Encrypter")
	for _, want := range []string{
		"status: finding",
		"Checked.Encrypt (exact) must call AssertNotEmpty: conforms (src/adapters/checked.go:9)",
		"Skipping.Encrypt (exact) must call AssertNotEmpty: finding (src/adapters/skipping.go:7)",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("domain show lacks %q:\n%s", want, stdout)
		}
	}
	stdout = mustRunDomain(t, root, "domain", "show", "Encrypter", "--format", "json")
	var doc struct {
		Rules struct {
			Checks []struct {
				Implementations []struct {
					Type, Operation, Source, Evidence, Must, Status string
				} `json:"implementations"`
			} `json:"checks"`
		} `json:"rules"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("show json: %v\n%s", err, stdout)
	}
	if len(doc.Rules.Checks) != 1 || len(doc.Rules.Checks[0].Implementations) != 2 {
		t.Fatalf("show json implementations: %+v", doc.Rules.Checks)
	}
	if impl := doc.Rules.Checks[0].Implementations[1]; impl.Type != "Skipping" || impl.Status != "finding" || impl.Must != "call AssertNotEmpty" || impl.Evidence != "exact" {
		t.Fatalf("show json Skipping: %+v", impl)
	}

	// Once Skipping calls the check, every implementation reaches it.
	write(t, root, "src/adapters/skipping.go", `package adapters

import "example.com/identity/src/identity"

// Skipping now checks too.
type Skipping struct{}

// Encrypt protects a password.
func (*Skipping) Encrypt(plain string) (string, error) {
	return plain, identity.AssertNotEmpty(plain)
}
`)
	if findings, status := declaredServiceFindings(t, root, 0); len(findings) != 0 || status != "structurally_checked" {
		t.Fatalf("every implementation calls the check: %s %+v", status, findings)
	}
}
