package user_test

import (
	"errors"
	"testing"

	"example.com/saasovation/identityaccess/user"
)

// emptyScheme is a protection scheme that yields an empty protected form,
// the case the not-empty assertion refuses.
type emptyScheme struct{}

func (emptyScheme) Protect(user.Password) (string, error) { return "", nil }

func (emptyScheme) Matches(user.Password, string) bool { return true }

// failingScheme is a protection scheme whose derivation fails.
type failingScheme struct{}

var errDerivation = errors.New("derivation failed")

func (failingScheme) Protect(user.Password) (string, error) { return "", errDerivation }

func (failingScheme) Matches(user.Password, string) bool { return false }

func password(t *testing.T, text string) user.Password {
	t.Helper()
	p, err := user.NewPassword(text)
	if err != nil {
		t.Fatalf("password %q: %v", text, err)
	}
	return p
}

func TestEncrypterRefusesAnEmptyProtectedForm(t *testing.T) {
	_, err := user.NewEncrypter(emptyScheme{}).Encrypt(password(t, "correct-H0rse-battery"))
	if !errors.Is(err, user.ErrEncryptedPasswordEmpty) {
		t.Fatalf("err = %v, want %v", err, user.ErrEncryptedPasswordEmpty)
	}
}

func TestEncrypterFailsWithItsScheme(t *testing.T) {
	_, err := user.NewEncrypter(failingScheme{}).Encrypt(password(t, "correct-H0rse-battery"))
	if !errors.Is(err, errDerivation) {
		t.Fatalf("err = %v, want %v", err, errDerivation)
	}
}

func TestEncrypterWithoutASchemeProtectsNothing(t *testing.T) {
	var encrypter user.Encrypter
	chosen := password(t, "correct-H0rse-battery")
	if _, err := encrypter.Encrypt(chosen); !errors.Is(err, user.ErrEncrypterWithoutScheme) {
		t.Fatalf("err = %v, want %v", err, user.ErrEncrypterWithoutScheme)
	}
	stored, err := user.NewEncryptedPassword("pbkdf2-sha256$600000$c2FsdA$a2V5")
	if err != nil {
		t.Fatalf("stored form: %v", err)
	}
	if encrypter.Matches(chosen, stored) {
		t.Fatal("an encrypter without a scheme must match nothing")
	}
}

func TestAssertNotEmptyRefusesAPasswordNeverEncrypted(t *testing.T) {
	if err := (user.Encrypter{}).AssertNotEmpty(user.EncryptedPassword{}); !errors.Is(err, user.ErrEncryptedPasswordEmpty) {
		t.Fatalf("err = %v, want %v", err, user.ErrEncryptedPasswordEmpty)
	}
}
