package provisioning

import (
	"errors"
	"fmt"

	"example.com/saasovation/identityaccess/user"
)

// Drafter drafts the text of a password to propose. It is the port
// through which the PasswordGenerator reaches unpredictable choices,
// which the model cannot make; an adapter implements it.
type Drafter interface {
	Draft() (string, error)
}

// ErrPasswordGeneratorWithoutDrafter is returned when a PasswordGenerator
// was not built through NewPasswordGenerator.
var ErrPasswordGeneratorWithoutDrafter = errors.New("password generator has no drafter")

// PasswordGenerator is the domain service that proposes a strong password
// for a user who has not chosen one yet, from what its Drafter drafts.
type PasswordGenerator struct {
	drafter Drafter
}

// NewPasswordGenerator returns the PasswordGenerator that proposes what
// drafter drafts.
func NewPasswordGenerator(drafter Drafter) PasswordGenerator {
	return PasswordGenerator{drafter: drafter}
}

// GenerateStrongPassword proposes a strong password.
//
// Contract (assertion on GenerateStrongPassword): not-weak.
func (g PasswordGenerator) GenerateStrongPassword() (user.Password, error) {
	if g.drafter == nil {
		return user.Password{}, ErrPasswordGeneratorWithoutDrafter
	}
	draft, err := g.drafter.Draft()
	if err != nil {
		return user.Password{}, fmt.Errorf("draft password: %w", err)
	}
	proposed, err := user.NewPassword(draft)
	if err != nil {
		return user.Password{}, fmt.Errorf("proposed password: %w", err)
	}
	if err := g.AssertNotWeak(proposed); err != nil {
		return user.Password{}, err
	}
	return proposed, nil
}

// AssertNotWeak checks the contract assertion not-weak on
// GenerateStrongPassword: a proposed password is never weak.
func (PasswordGenerator) AssertNotWeak(proposed user.Password) error {
	if proposed.Weak() {
		return fmt.Errorf("proposed password: %w", user.ErrPasswordWeak)
	}
	return nil
}
