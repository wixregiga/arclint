package user_test

import (
	"errors"
	"testing"

	"example.com/saasovation/identityaccess/user"
)

// derivationOf is a KeyDerivation that answers with fixed text.
type derivationOf string

func (d derivationOf) Derive(string) (string, error) { return string(d), nil }
func (d derivationOf) Matches(_, stored string) bool { return stored == string(d) }

func mustPassword(t *testing.T, text string) user.Password {
	t.Helper()
	p, err := user.NewPassword(text)
	if err != nil {
		t.Fatalf("NewPassword(%q): %v", text, err)
	}
	return p
}

func TestEncryptStoresWhatTheDerivationProduces(t *testing.T) {
	encrypter := user.NewEncrypter(derivationOf("derived"))
	p := mustPassword(t, "c0rrect-Horse")
	stored, err := encrypter.Encrypt(p)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if stored.String() != "derived" {
		t.Fatalf("stored form = %q, want the derivation's text", stored)
	}
	if !encrypter.Matches(p, stored) {
		t.Fatal("the password must match its stored form")
	}
}

func TestEncryptRefusesAnEmptyStoredForm(t *testing.T) {
	_, err := user.NewEncrypter(derivationOf("")).Encrypt(mustPassword(t, "c0rrect-Horse"))
	if !errors.Is(err, user.ErrEncryptedPasswordEmpty) {
		t.Fatalf("err = %v, want ErrEncryptedPasswordEmpty", err)
	}
}

func TestAssertNotEmptyRefusesTheZeroStoredForm(t *testing.T) {
	encrypter := user.NewEncrypter(derivationOf("stored"))
	if err := encrypter.AssertNotEmpty(user.EncryptedPassword{}); !errors.Is(err, user.ErrEncryptedPasswordEmpty) {
		t.Fatalf("err = %v, want ErrEncryptedPasswordEmpty", err)
	}
	stored, err := user.NewEncryptedPassword("stored")
	if err != nil {
		t.Fatal(err)
	}
	if err := encrypter.AssertNotEmpty(stored); err != nil {
		t.Fatalf("a stored form with text: %v", err)
	}
}
