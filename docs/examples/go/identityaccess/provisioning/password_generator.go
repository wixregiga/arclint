package provisioning

import (
	"errors"
	"fmt"

	"example.com/saasovation/identityaccess/user"
)

// Candidates supplies the text of the passwords a PasswordGenerator
// proposes. It is the port through which the generator reaches
// unpredictable choices, which the model cannot make; an adapter
// implements it.
type Candidates interface {
	Next() (string, error)
}

// ErrPasswordGeneratorWithoutCandidates is returned by a PasswordGenerator
// that was not built through NewPasswordGenerator.
var ErrPasswordGeneratorWithoutCandidates = errors.New("password generator has no candidates")

// PasswordGenerator is the domain service that proposes a strong password
// for a user who has not chosen one yet, from its Candidates.
type PasswordGenerator struct {
	candidates Candidates
}

// NewPasswordGenerator returns the PasswordGenerator that proposes what
// candidates supplies.
func NewPasswordGenerator(candidates Candidates) PasswordGenerator {
	return PasswordGenerator{candidates: candidates}
}

// GenerateStrongPassword proposes a strong password.
//
// Contract (assertion on GenerateStrongPassword): not-weak.
func (g PasswordGenerator) GenerateStrongPassword() (user.Password, error) {
	if g.candidates == nil {
		return user.Password{}, ErrPasswordGeneratorWithoutCandidates
	}
	text, err := g.candidates.Next()
	if err != nil {
		return user.Password{}, fmt.Errorf("generate password: %w", err)
	}
	proposed, err := user.NewPassword(text)
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
	if _, err := user.NewPassword(proposed.Reveal()); err != nil {
		return fmt.Errorf("proposed password: %w", err)
	}
	return nil
}
