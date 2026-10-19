package provisioning_test

import (
	"errors"
	"math/rand/v2"
	"testing"

	"example.com/saasovation/identityaccess/provisioning"
	"example.com/saasovation/identityaccess/user"
)

// seeded is a RandomSource drawing from a seeded generator, so a run is
// repeatable.
type seeded struct{ r *rand.Rand }

func (s seeded) Intn(n int) (int, error) { return s.r.IntN(n), nil }

// outOfRange is a RandomSource that answers with a value no bound allows.
type outOfRange struct{}

func (outOfRange) Intn(n int) (int, error) { return n, nil }

// failing is a RandomSource whose source is unavailable.
type failing struct{}

func (failing) Intn(int) (int, error) { return 0, errors.New("no entropy") }

func TestGenerateStrongPasswordProposesStrongPasswords(t *testing.T) {
	generator := provisioning.NewPasswordGenerator(seeded{rand.New(rand.NewPCG(1, 2))})
	for range 5000 {
		p, err := generator.GenerateStrongPassword()
		if err != nil {
			t.Fatalf("GenerateStrongPassword: %v", err)
		}
		if len(p.Reveal()) != 16 {
			t.Fatalf("password %q is not 16 characters", p.Reveal())
		}
		if _, err := user.NewPassword(p.Reveal()); err != nil {
			t.Fatalf("password %q is weak: %v", p.Reveal(), err)
		}
	}
}

func TestGenerateStrongPasswordRefusesABrokenRandomSource(t *testing.T) {
	for name, source := range map[string]provisioning.RandomSource{"out of range": outOfRange{}, "failing": failing{}} {
		if _, err := provisioning.NewPasswordGenerator(source).GenerateStrongPassword(); err == nil {
			t.Errorf("%s: a password was proposed", name)
		}
	}
}

func TestAssertNotWeakRefusesTheZeroPassword(t *testing.T) {
	generator := provisioning.NewPasswordGenerator(seeded{rand.New(rand.NewPCG(1, 2))})
	if err := generator.AssertNotWeak(user.Password{}); !errors.Is(err, user.ErrPasswordWeak) {
		t.Fatalf("err = %v, want ErrPasswordWeak", err)
	}
	p, err := user.NewPassword("c0rrect-Horse")
	if err != nil {
		t.Fatal(err)
	}
	if err := generator.AssertNotWeak(p); err != nil {
		t.Fatalf("a strong password: %v", err)
	}
}
