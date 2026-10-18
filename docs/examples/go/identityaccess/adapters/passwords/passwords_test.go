package passwords_test

import (
	"testing"

	"example.com/saasovation/identityaccess/adapters/passwords"
	"example.com/saasovation/identityaccess/provisioning"
	"example.com/saasovation/identityaccess/user"
)

func TestEncrypterProtectsAndMatchesThroughPBKDF2(t *testing.T) {
	encrypter := user.NewEncrypter(passwords.PBKDF2Scheme{})
	chosen, err := user.NewPassword("correct-H0rse-battery")
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := encrypter.Encrypt(chosen)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if encrypted.String() == "" || encrypted.String() == chosen.Reveal() {
		t.Fatalf("encrypted form %q does not protect the password", encrypted)
	}
	if !encrypter.Matches(chosen, encrypted) {
		t.Fatal("the password must match its own encrypted form")
	}
	other, err := user.NewPassword("other-H0rse-battery")
	if err != nil {
		t.Fatal(err)
	}
	if encrypter.Matches(other, encrypted) {
		t.Fatal("another password must not match")
	}
	again, err := encrypter.Encrypt(chosen)
	if err != nil {
		t.Fatalf("encrypt again: %v", err)
	}
	if again.String() == encrypted.String() {
		t.Fatal("each encryption must use a fresh salt")
	}
}

func TestPasswordGeneratorProposesDistinctStrongPasswords(t *testing.T) {
	generator := provisioning.NewPasswordGenerator(passwords.RandomDrafter{})
	seen := map[string]bool{}
	for range 200 {
		proposed, err := generator.GenerateStrongPassword()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if proposed.Weak() || len(proposed.Reveal()) != 16 {
			t.Fatalf("proposed %q is not a strong 16 character password", proposed.Reveal())
		}
		seen[proposed.Reveal()] = true
	}
	if len(seen) < 200 {
		t.Fatalf("200 proposals repeat: %d distinct", len(seen))
	}
}
