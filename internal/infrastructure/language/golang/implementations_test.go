package golang_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	golang "github.com/wixregiga/arclint/internal/infrastructure/language/golang"
)

// encrypterModule declares a domain service interface in one package and
// the shapes that implement it, or seem to, in others: a value
// receiver, a pointer receiver, a method promoted from an embedded
// struct, an embedded interface, a generic type, a type missing one
// method, a type whose method has another signature, and a fake in a
// test file.
func encrypterModule(t *testing.T) (string, []conformance.ObservedFile) {
	t.Helper()
	root := t.TempDir()
	files := []conformance.ObservedFile{
		write(t, root, "go.mod", "module example.com/id\n\ngo 1.24\n"),
		write(t, root, "user/password.go", `package user

type EncryptedPassword struct{ v string }

type Encrypter interface {
	Encrypt(p string) (EncryptedPassword, error)
	Matches(p string, e EncryptedPassword) bool
}

type Matcher interface {
	Matches(p string, e EncryptedPassword) bool
}

type Empty interface{}

type Box[T any] interface{ Get() T }
`),
		write(t, root, "adapters/encrypters.go", `package adapters

import "example.com/id/user"

type Value struct{}

func (Value) Encrypt(p string) (user.EncryptedPassword, error) { return user.EncryptedPassword{}, nil }
func (Value) Matches(p string, e user.EncryptedPassword) bool  { return false }

type Pointer struct{}

func (*Pointer) Encrypt(p string) (user.EncryptedPassword, error) { return user.EncryptedPassword{}, nil }
func (*Pointer) Matches(p string, e user.EncryptedPassword) bool  { return false }

type Promoted struct{ Value }

type Wrapped struct{ user.Encrypter }

type Generic[T any] struct{}

func (Generic[T]) Encrypt(p string) (user.EncryptedPassword, error) { return user.EncryptedPassword{}, nil }
func (Generic[T]) Matches(p string, e user.EncryptedPassword) bool  { return false }

type HalfDone struct{}

func (HalfDone) Encrypt(p string) (user.EncryptedPassword, error) { return user.EncryptedPassword{}, nil }

type OtherSignature struct{}

func (OtherSignature) Encrypt(p []byte) (user.EncryptedPassword, error) { return user.EncryptedPassword{}, nil }
func (OtherSignature) Matches(p string, e user.EncryptedPassword) bool  { return false }
`),
		write(t, root, "adapters/fake_test.go", `package adapters

import "example.com/id/user"

type Fake struct{}

func (Fake) Encrypt(p string) (user.EncryptedPassword, error) { return user.EncryptedPassword{}, nil }
func (Fake) Matches(p string, e user.EncryptedPassword) bool  { return false }
`),
	}
	return root, files
}

// implementationsOf spells every implementation a file's types carry as
// Type -> Abstraction@file [evidence] with each operation reached.
func implementationsOf(f conformance.LanguageFacts) []string {
	var out []string
	for _, d := range f.Declarations {
		for _, impl := range d.Implements {
			var ops []string
			for _, op := range impl.Operations {
				ops = append(ops, fmt.Sprintf("%s.%s@%s", op.Owner, op.Name, op.File))
			}
			out = append(out, fmt.Sprintf("%s -> %s@%s [%s] %s", d.Name, impl.Abstraction.Name, impl.Abstraction.File, impl.Evidence, strings.Join(ops, " ")))
		}
	}
	sort.Strings(out)
	return out
}

// The Go compiler decides which types implement an interface, through
// the method sets of the type and of a pointer to it. The facts list
// exactly those, for every interface of the repository with methods,
// with the method each operation reaches: the type's own, or the one
// an embedded field promotes, which for an embedded interface is the
// interface's own method.
func TestImplementationsFollowTheMethodSets(t *testing.T) {
	root, files := encrypterModule(t)
	facts, err := golang.NewProducer().Facts(root, files, []rule.Fact{rule.FactDeclarations, rule.FactImplementations})
	if err != nil {
		t.Fatal(err)
	}
	adapters := facts["adapters/encrypters.go"]
	if !adapters.ImplementationsAvailable || !adapters.Supports(rule.FactImplementations) {
		t.Fatalf("implementations not available: %+v", adapters)
	}
	got := implementationsOf(adapters)
	want := []string{
		"OtherSignature -> Matcher@user/password.go [exact] OtherSignature.Matches@adapters/encrypters.go",
		"Pointer -> Encrypter@user/password.go [exact] Pointer.Encrypt@adapters/encrypters.go Pointer.Matches@adapters/encrypters.go",
		"Pointer -> Matcher@user/password.go [exact] Pointer.Matches@adapters/encrypters.go",
		"Promoted -> Encrypter@user/password.go [exact] Value.Encrypt@adapters/encrypters.go Value.Matches@adapters/encrypters.go",
		"Promoted -> Matcher@user/password.go [exact] Value.Matches@adapters/encrypters.go",
		"Value -> Encrypter@user/password.go [exact] Value.Encrypt@adapters/encrypters.go Value.Matches@adapters/encrypters.go",
		"Value -> Matcher@user/password.go [exact] Value.Matches@adapters/encrypters.go",
		"Wrapped -> Encrypter@user/password.go [exact] Encrypter.Encrypt@user/password.go Encrypter.Matches@user/password.go",
		"Wrapped -> Matcher@user/password.go [exact] Encrypter.Matches@user/password.go",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("implementations:\n got %s\nwant %s", strings.Join(got, "\n     "), strings.Join(want, "\n     "))
	}
	// An interface implements nothing itself, and a type the type check
	// does not reach implements what nobody can tell.
	if got := implementationsOf(facts["user/password.go"]); len(got) != 0 {
		t.Errorf("interfaces carry implementations: %q", got)
	}
	if got, want := implementationsOf(facts["adapters/fake_test.go"]), []string{"Fake -> @ [undetermined] "}; strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("a test file is not type-checked, so what its types implement is undetermined: %q", got)
	}
	if reason := facts["adapters/fake_test.go"].Declarations[0].Implements[0].Reason; reason != "test files are not type-checked" {
		t.Errorf("reason %q", reason)
	}
	if _, err := conformance.NewObservations(files, facts); err != nil {
		t.Errorf("the facts do not validate: %v", err)
	}
}

// Implementations are computed only when asked for.
func TestImplementationsOnlyWhenRequested(t *testing.T) {
	root, files := encrypterModule(t)
	facts, err := golang.NewProducer().Facts(root, files, []rule.Fact{rule.FactDeclarations, rule.FactFieldTypes})
	if err != nil {
		t.Fatal(err)
	}
	if f := facts["adapters/encrypters.go"]; f.ImplementationsAvailable || len(implementationsOf(f)) != 0 {
		t.Errorf("implementations supplied without being requested: %+v", f)
	}
}

// A comparison of signatures the type checker could not type, or that
// name types arclint knows by name only, proves neither that a type
// implements an interface nor that it does not: os.FileMode is an alias
// of io/fs.FileMode, which a stub of each package cannot tell, and a dot
// import leaves its names untyped. Each such type is undetermined, and
// a type that matches through fully known types stays exact.
func TestImplementationsWithUnknownTypesAreUndetermined(t *testing.T) {
	root := t.TempDir()
	files := []conformance.ObservedFile{
		write(t, root, "go.mod", "module example.com/id\n\ngo 1.24\n"),
		write(t, root, "user/ports.go", `package user

import (
	. "context"
	"os"
)

type Encrypter interface {
	Encrypt(c Context) string
}

type Chmoder interface {
	Chmod(m os.FileMode) string
}
`),
		write(t, root, "adapters/adapters.go", `package adapters

import (
	"io/fs"
	. "io"
	"os"
)

type NotAnEncrypter struct{}

func (NotAnEncrypter) Encrypt(r Reader) string { return "" }

type SameMode struct{}

func (SameMode) Chmod(m os.FileMode) string { return "" }

type AliasMode struct{}

func (AliasMode) Chmod(m fs.FileMode) string { return "" }
`),
	}
	facts, err := golang.NewProducer().Facts(root, files, []rule.Fact{rule.FactDeclarations, rule.FactImplementations})
	if err != nil {
		t.Fatal(err)
	}
	got := implementationsOf(facts["adapters/adapters.go"])
	want := []string{
		"AliasMode -> Chmoder@user/ports.go [undetermined] AliasMode.Chmod@adapters/adapters.go",
		"NotAnEncrypter -> Encrypter@user/ports.go [undetermined] NotAnEncrypter.Encrypt@adapters/adapters.go",
		"SameMode -> Chmoder@user/ports.go [exact] SameMode.Chmod@adapters/adapters.go",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("implementations:\n got %s\nwant %s", strings.Join(got, "\n     "), strings.Join(want, "\n     "))
	}
	if _, err := conformance.NewObservations(files, facts); err != nil {
		t.Errorf("the facts do not validate: %v", err)
	}
}
