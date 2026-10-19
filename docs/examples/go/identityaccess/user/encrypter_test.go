package user_test

import (
	"errors"
	"strings"
	"testing"

	"example.com/saasovation/identityaccess/user"
)

// reversing derives a stored form by reversing the text, or what it is
// told to; it stands in for a key derivation function.
type reversing struct {
	derived string
	err     error
}

func (r reversing) Derive(plain string) (string, error) {
	if r.err != nil || r.derived != "" {
		return r.derived, r.err
	}
	runes := []rune(plain)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes), nil
}

func (r reversing) Verify(plain, stored string) bool {
	derived, err := reversing{}.Derive(plain)
	return err == nil && derived == stored
}

func strongPassword(t *testing.T) user.Password {
	t.Helper()
	p, err := user.NewPassword("s3cret-Pass")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEncryptStoresWhatTheKeyDerivationDerives(t *testing.T) {
	encrypter := user.NewEncrypter(reversing{})
	p := strongPassword(t)
	encrypted, err := encrypter.Encrypt(p)
	if err != nil {
		t.Fatal(err)
	}
	if encrypted.String() != "ssaP-terc3s" {
		t.Fatalf("stored form %q", encrypted.String())
	}
	if !encrypter.Matches(p, encrypted) {
		t.Fatal("the password does not match its own stored form")
	}
	other, err := user.NewPassword("an0ther-Pass")
	if err != nil {
		t.Fatal(err)
	}
	if encrypter.Matches(other, encrypted) {
		t.Fatal("another password matches the stored form")
	}
}

func TestEncryptRefusesAnEmptyStoredForm(t *testing.T) {
	// A password never built derives nothing.
	_, err := user.NewEncrypter(reversing{}).Encrypt(user.Password{})
	if !errors.Is(err, user.ErrEncryptedPasswordEmpty) {
		t.Fatalf("an empty derivation: %v", err)
	}
	if err := (user.Encrypter{}).AssertNotEmpty(user.EncryptedPassword{}); !errors.Is(err, user.ErrEncryptedPasswordEmpty) {
		t.Fatalf("AssertNotEmpty of a value never built: %v", err)
	}
}

func TestEncryptReportsAFailingKeyDerivation(t *testing.T) {
	failure := errors.New("no entropy")
	_, err := user.NewEncrypter(reversing{err: failure}).Encrypt(strongPassword(t))
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), "encrypt password") {
		t.Fatalf("a failing derivation: %v", err)
	}
}

func TestEncrypterWithoutAKeyDerivationRefuses(t *testing.T) {
	var encrypter user.Encrypter
	if _, err := encrypter.Encrypt(strongPassword(t)); !errors.Is(err, user.ErrEncrypterWithoutKeyDerivation) {
		t.Fatalf("Encrypt: %v", err)
	}
	if encrypter.Matches(strongPassword(t), user.EncryptedPassword{}) {
		t.Fatal("Matches without a key derivation")
	}
}
