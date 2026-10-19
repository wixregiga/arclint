package passwords_test

import (
	"testing"

	"example.com/saasovation/identityaccess/adapters/passwords"
	"example.com/saasovation/identityaccess/provisioning"
	"example.com/saasovation/identityaccess/user"
)

func TestPBKDF2ProtectsThePasswordTheEncrypterIsGiven(t *testing.T) {
	encrypter := user.NewEncrypter(passwords.PBKDF2KeyDerivation{})
	p, err := user.NewPassword("c0rrect-Horse")
	if err != nil {
		t.Fatal(err)
	}
	stored, err := encrypter.Encrypt(p)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if stored.String() == "" || stored.String() == p.Reveal() {
		t.Fatalf("stored form %q must be neither empty nor the plain text", stored)
	}
	if !encrypter.Matches(p, stored) {
		t.Fatal("the password must match its stored form")
	}
	other, err := user.NewPassword("wr0ng-Horse")
	if err != nil {
		t.Fatal(err)
	}
	if encrypter.Matches(other, stored) {
		t.Fatal("another password must not match")
	}
	again, err := encrypter.Encrypt(p)
	if err != nil {
		t.Fatal(err)
	}
	if again == stored {
		t.Fatal("two encryptions of one password must differ by their salt")
	}
}

func TestPBKDF2RefusesAMalformedStoredText(t *testing.T) {
	for _, stored := range []string{"", "plain", "pbkdf2-sha256$x$c2FsdA$a2V5", "md5$1$c2FsdA$a2V5", "pbkdf2-sha256$1$!!$a2V5", "pbkdf2-sha256$1$c2FsdA$"} {
		if (passwords.PBKDF2KeyDerivation{}).Matches("c0rrect-Horse", stored) {
			t.Errorf("%q matched", stored)
		}
	}
}

func TestSystemRandomGeneratesStrongPasswords(t *testing.T) {
	generator := provisioning.NewPasswordGenerator(passwords.SystemRandom{})
	for range 200 {
		p, err := generator.GenerateStrongPassword()
		if err != nil {
			t.Fatalf("GenerateStrongPassword: %v", err)
		}
		if _, err := user.NewPassword(p.Reveal()); err != nil {
			t.Fatalf("password %q is weak: %v", p.Reveal(), err)
		}
	}
	if _, err := (passwords.SystemRandom{}).Intn(0); err == nil {
		t.Fatal("a bound of zero must be refused")
	}
}
