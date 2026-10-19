package provisioning

import (
	"fmt"

	"example.com/saasovation/identityaccess/user"
)

// The characters a proposed password is drawn from, by class, and its
// length.
const (
	letters       = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ"
	digits        = "23456789"
	symbols       = "!@#$%^&*-_=+?"
	generatedSize = 16
)

// RandomSource is the port through which the PasswordGenerator draws its
// choices: Intn returns a uniformly random integer in [0, n).
// Infrastructure implements it with the operating system's random source.
type RandomSource interface {
	Intn(n int) (int, error)
}

// PasswordGenerator is the domain service that proposes a strong password
// for a user who has not chosen one yet. It decides what the password is
// made of; the RandomSource it is given makes the choices.
type PasswordGenerator struct {
	random RandomSource
}

// NewPasswordGenerator builds the PasswordGenerator over a random source.
func NewPasswordGenerator(random RandomSource) PasswordGenerator {
	return PasswordGenerator{random: random}
}

// GenerateStrongPassword proposes a 16 character password: one letter,
// one digit, and one symbol at distinct random positions, the rest drawn
// from all three classes. Assertion not-weak: the password it returns is
// never weak.
func (g PasswordGenerator) GenerateStrongPassword() (user.Password, error) {
	all := letters + digits + symbols
	buf := make([]byte, generatedSize)
	for i := range buf {
		c, err := g.pick(all)
		if err != nil {
			return user.Password{}, err
		}
		buf[i] = c
	}
	positions, err := g.distinctPositions(3, generatedSize)
	if err != nil {
		return user.Password{}, err
	}
	for i, class := range []string{letters, digits, symbols} {
		c, err := g.pick(class)
		if err != nil {
			return user.Password{}, err
		}
		buf[positions[i]] = c
	}
	p, err := user.NewPassword(string(buf))
	if err != nil {
		return user.Password{}, err
	}
	if err := g.AssertNotWeak(p); err != nil {
		return user.Password{}, err
	}
	return p, nil
}

// AssertNotWeak checks not-weak on GenerateStrongPassword. NewPassword
// refuses a weak text, so the zero Password is the only weak one.
func (PasswordGenerator) AssertNotWeak(p user.Password) error {
	if p == (user.Password{}) {
		return user.ErrPasswordWeak
	}
	return nil
}

func (g PasswordGenerator) pick(alphabet string) (byte, error) {
	n, err := g.random.Intn(len(alphabet))
	if err != nil {
		return 0, fmt.Errorf("generate password: %w", err)
	}
	if n < 0 || n >= len(alphabet) {
		return 0, fmt.Errorf("generate password: random source returned %d outside [0, %d)", n, len(alphabet))
	}
	return alphabet[n], nil
}

func (g PasswordGenerator) distinctPositions(count, size int) ([]int, error) {
	out := make([]int, 0, count)
	for len(out) < count {
		p, err := g.random.Intn(size)
		if err != nil {
			return nil, fmt.Errorf("generate password: %w", err)
		}
		if p < 0 || p >= size {
			return nil, fmt.Errorf("generate password: random source returned %d outside [0, %d)", p, size)
		}
		taken := false
		for _, q := range out {
			if q == p {
				taken = true
			}
		}
		if !taken {
			out = append(out, p)
		}
	}
	return out, nil
}
