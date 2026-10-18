package provisioning_test

import (
	"errors"
	"testing"

	"example.com/saasovation/identityaccess/provisioning"
	"example.com/saasovation/identityaccess/user"
)

// fixedDrafter drafts the same text every time.
type fixedDrafter string

func (d fixedDrafter) Draft() (string, error) { return string(d), nil }

func TestPasswordGeneratorRefusesAWeakDraft(t *testing.T) {
	_, err := provisioning.NewPasswordGenerator(fixedDrafter("short")).GenerateStrongPassword()
	if !errors.Is(err, user.ErrPasswordWeak) {
		t.Fatalf("err = %v, want %v", err, user.ErrPasswordWeak)
	}
}

func TestPasswordGeneratorProposesAStrongDraft(t *testing.T) {
	proposed, err := provisioning.NewPasswordGenerator(fixedDrafter("k7#Rw9!qPz2$")).GenerateStrongPassword()
	if err != nil || proposed.Reveal() != "k7#Rw9!qPz2$" {
		t.Fatalf("proposed %q, %v", proposed.Reveal(), err)
	}
}

func TestPasswordGeneratorWithoutADrafterProposesNothing(t *testing.T) {
	var generator provisioning.PasswordGenerator
	if _, err := generator.GenerateStrongPassword(); !errors.Is(err, provisioning.ErrPasswordGeneratorWithoutDrafter) {
		t.Fatalf("err = %v, want %v", err, provisioning.ErrPasswordGeneratorWithoutDrafter)
	}
}

func TestAssertNotWeakRefusesAPasswordNeverBuilt(t *testing.T) {
	var never user.Password
	if err := (provisioning.PasswordGenerator{}).AssertNotWeak(never); !errors.Is(err, user.ErrPasswordWeak) {
		t.Fatalf("err = %v, want %v", err, user.ErrPasswordWeak)
	}
}
