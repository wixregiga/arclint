package passwords

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"example.com/saasovation/identityaccess/user"
)

const (
	letters       = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ"
	digits        = "23456789"
	symbols       = "!@#$%^&*-_=+?"
	generatedSize = 16
)

// RandomGenerator implements provisioning.PasswordGenerator with the
// operating system's random source. Every password it proposes satisfies
// the user.Password strength rule by construction: one character of each
// class is placed at a random position and the rest is drawn from all.
type RandomGenerator struct{}

// GenerateStrongPassword proposes a 16 character password.
func (RandomGenerator) GenerateStrongPassword() (user.Password, error) {
	all := letters + digits + symbols
	buf := make([]byte, generatedSize)
	for i := range buf {
		c, err := pick(all)
		if err != nil {
			return user.Password{}, err
		}
		buf[i] = c
	}
	positions, err := distinctPositions(3, generatedSize)
	if err != nil {
		return user.Password{}, err
	}
	for i, class := range []string{letters, digits, symbols} {
		c, err := pick(class)
		if err != nil {
			return user.Password{}, err
		}
		buf[positions[i]] = c
	}
	return user.NewPassword(string(buf))
}

func pick(alphabet string) (byte, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
	if err != nil {
		return 0, fmt.Errorf("random password: %w", err)
	}
	return alphabet[n.Int64()], nil
}

func distinctPositions(count, size int) ([]int, error) {
	out := make([]int, 0, count)
	for len(out) < count {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(size)))
		if err != nil {
			return nil, fmt.Errorf("random password: %w", err)
		}
		p := int(n.Int64())
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
