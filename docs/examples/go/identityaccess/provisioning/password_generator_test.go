package provisioning_test

import (
	"errors"
	"testing"

	"example.com/saasovation/identityaccess/provisioning"
	"example.com/saasovation/identityaccess/user"
)

// fixed supplies one candidate, or fails.
type fixed struct {
	text string
	err  error
}

func (f fixed) Next() (string, error) { return f.text, f.err }

func TestGenerateStrongPasswordProposesAStrongCandidate(t *testing.T) {
	p, err := provisioning.NewPasswordGenerator(fixed{text: "s3cret-Pass"}).GenerateStrongPassword()
	if err != nil {
		t.Fatal(err)
	}
	if p.Reveal() != "s3cret-Pass" {
		t.Fatalf("proposed %q", p.Reveal())
	}
}

func TestGenerateStrongPasswordRefusesAWeakCandidate(t *testing.T) {
	for _, weak := range []string{"", "short1!", "nodigits!!", "n0symbols1"} {
		if _, err := provisioning.NewPasswordGenerator(fixed{text: weak}).GenerateStrongPassword(); !errors.Is(err, user.ErrPasswordWeak) {
			t.Errorf("%q: %v", weak, err)
		}
	}
	if err := (provisioning.PasswordGenerator{}).AssertNotWeak(user.Password{}); !errors.Is(err, user.ErrPasswordWeak) {
		t.Fatalf("AssertNotWeak of a password never built: %v", err)
	}
}

func TestGenerateStrongPasswordReportsFailingCandidates(t *testing.T) {
	failure := errors.New("no entropy")
	if _, err := provisioning.NewPasswordGenerator(fixed{err: failure}).GenerateStrongPassword(); !errors.Is(err, failure) {
		t.Fatalf("failing candidates: %v", err)
	}
	if _, err := (provisioning.PasswordGenerator{}).GenerateStrongPassword(); !errors.Is(err, provisioning.ErrPasswordGeneratorWithoutCandidates) {
		t.Fatalf("no candidates: %v", err)
	}
}
