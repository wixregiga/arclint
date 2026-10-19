package passwords_test

import (
	"strings"
	"testing"

	"example.com/saasovation/identityaccess/adapters/passwords"
	"example.com/saasovation/identityaccess/provisioning"
	"example.com/saasovation/identityaccess/user"
)

func TestPBKDF2RoundTripThroughTheEncrypter(t *testing.T) {
	encrypter := user.NewEncrypter(passwords.PBKDF2KeyDerivation{})
	p, err := user.NewPassword("s3cret-Pass")
	if err != nil {
		t.Fatal(err)
	}
	first, err := encrypter.Encrypt(p)
	if err != nil {
		t.Fatal(err)
	}
	second, err := encrypter.Encrypt(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first.String(), "pbkdf2-sha256$600000$") {
		t.Fatalf("stored form %q", first.String())
	}
	if first.String() == second.String() {
		t.Fatal("two encryptions share a salt")
	}
	if !encrypter.Matches(p, first) || !encrypter.Matches(p, second) {
		t.Fatal("the password does not match its stored forms")
	}
	other, err := user.NewPassword("an0ther-Pass")
	if err != nil {
		t.Fatal(err)
	}
	if encrypter.Matches(other, first) {
		t.Fatal("another password matches")
	}
	if (passwords.PBKDF2KeyDerivation{}).Verify("s3cret-Pass", "md5$abc") {
		t.Fatal("a stored form of another scheme matches")
	}
}

func TestRandomCandidatesAreStrongAndDistinct(t *testing.T) {
	generator := provisioning.NewPasswordGenerator(passwords.RandomCandidates{})
	seen := map[string]bool{}
	for range 200 {
		p, err := generator.GenerateStrongPassword()
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Reveal()) != 16 {
			t.Fatalf("candidate %q is not 16 characters", p.Reveal())
		}
		if seen[p.Reveal()] {
			t.Fatalf("candidate %q proposed twice", p.Reveal())
		}
		seen[p.Reveal()] = true
	}
}
