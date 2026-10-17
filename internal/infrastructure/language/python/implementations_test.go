package python

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// spellImplementations spells every implementation of a file's classes
// as Class -> Abstraction [evidence] with each operation reached.
func spellImplementations(f conformance.LanguageFacts) []string {
	var out []string
	for _, d := range f.Declarations {
		for _, impl := range d.Implements {
			var ops []string
			for _, op := range impl.Operations {
				if op.Resolution == conformance.ResolutionDeclared {
					ops = append(ops, fmt.Sprintf("%s.%s@%s", op.Owner, op.Name, op.File))
				} else {
					ops = append(ops, op.Name+":"+string(op.Resolution))
				}
			}
			abs := impl.Abstraction.Name + "@" + impl.Abstraction.File
			if impl.Abstraction.Resolution == conformance.ResolutionUndetermined {
				abs = impl.Abstraction.Name + "?"
			}
			out = append(out, strings.TrimSpace(fmt.Sprintf("%s -> %s [%s] %s", d.Name, abs, impl.Evidence, strings.Join(ops, " "))))
		}
	}
	sort.Strings(out)
	return out
}

// A class implements every abstract class its resolved bases reach,
// through the repository's classes, with each operation found where
// its method resolution order finds it. A class with every method of
// an abstract class but no base naming it is a heuristic
// implementation; a class registered through ABCMeta.register is
// undetermined; a base resolution cannot follow is undetermined with
// the name the class statement writes.
func TestImplementationsFollowTheClassHierarchy(t *testing.T) {
	root, files := writeFiles(t, map[string]string{
		"identity/__init__.py": "",
		"identity/password.py": `from abc import ABC, ABCMeta, abstractmethod
from typing import Protocol


class EncryptedPassword:
    def __init__(self, value: str) -> None:
        self.value = value


class Encrypter(ABC):
    @abstractmethod
    def encrypt(self, plain: str) -> EncryptedPassword: ...

    @abstractmethod
    def matches(self, plain: str, stored: EncryptedPassword) -> bool: ...


class Hasher(metaclass=ABCMeta):
    @abstractmethod
    def hash(self, plain: str) -> str: ...


class Generator(Protocol):
    def generate(self) -> str: ...
`,
		"identity/adapters.py": `from identity.password import EncryptedPassword, Encrypter, Generator, Hasher
from identity.missing import Unknown


class Pbkdf2Encrypter(Encrypter):
    def encrypt(self, plain: str) -> EncryptedPassword:
        return EncryptedPassword(plain)

    def matches(self, plain: str, stored: EncryptedPassword) -> bool:
        return False


class Salted(Pbkdf2Encrypter):
    def encrypt(self, plain: str) -> EncryptedPassword:
        return EncryptedPassword(plain + "salt")


class Sha(Hasher):
    def hash(self, plain: str) -> str:
        return plain


class Random:
    def generate(self) -> str:
        return "x"


class Registered:
    def hash(self, plain: str) -> str:
        return plain


Hasher.register(Registered)


@Encrypter.register
class Decorated:
    def encrypt(self, plain: str) -> EncryptedPassword:
        return EncryptedPassword(plain)

    def matches(self, plain: str, stored: EncryptedPassword) -> bool:
        return True


class Opaque(Unknown):
    pass
`,
	})
	facts, err := NewProducer().Facts(root, files, []rule.Fact{rule.FactDeclarations, rule.FactImplementations})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conformance.NewObservations(files, facts); err != nil {
		t.Fatal(err)
	}
	got := spellImplementations(facts["identity/adapters.py"])
	want := []string{
		"Decorated -> Encrypter@identity/password.py [undetermined] encrypt:undetermined matches:undetermined",
		"Opaque -> Unknown? [undetermined]",
		"Pbkdf2Encrypter -> Encrypter@identity/password.py [exact] Pbkdf2Encrypter.encrypt@identity/adapters.py Pbkdf2Encrypter.matches@identity/adapters.py",
		"Random -> Generator@identity/password.py [heuristic] Random.generate@identity/adapters.py",
		"Registered -> Hasher@identity/password.py [undetermined] Registered.hash@identity/adapters.py",
		"Salted -> Encrypter@identity/password.py [exact] Salted.encrypt@identity/adapters.py Pbkdf2Encrypter.matches@identity/adapters.py",
		"Sha -> Hasher@identity/password.py [exact] Sha.hash@identity/adapters.py",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("implementations:\n got %s\nwant %s", strings.Join(got, "\n     "), strings.Join(want, "\n     "))
	}
	if !facts["identity/adapters.py"].Supports(rule.FactImplementations) {
		t.Error("implementations not available")
	}
	// The method resolution order passes over abc.ABC, which declares no
	// method of the repository's, but a name only its metaclass provides
	// stays undetermined rather than becoming a method of the class.
	root, files = writeFiles(t, map[string]string{"port.py": `from abc import ABC


class Port(ABC):
    def run(self) -> None: ...


class Adapter:
    pass


Port.register(Adapter)
`})
	resolvedFacts, err := NewProducer().Facts(root, files, []rule.Fact{rule.FactResolvedCalls})
	if err != nil {
		t.Fatal(err)
	}
	calls := resolvedFacts["port.py"].Calls
	if len(calls) != 1 || calls[0].Target.Resolution != conformance.ResolutionUndetermined || !strings.Contains(calls[0].Target.Reason, "abc.ABCMeta may provide register") {
		t.Errorf("Port.register resolved to %+v", calls)
	}
}
