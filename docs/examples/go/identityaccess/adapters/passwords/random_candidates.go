package passwords

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const (
	letters       = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ"
	digits        = "23456789"
	symbols       = "!@#$%^&*-_=+?"
	generatedSize = 16
)

// RandomCandidates implements provisioning.Candidates with the operating
// system's random source. Every candidate it supplies holds a letter, a
// digit, and a symbol: one character of each class is placed at a random
// position and the rest is drawn from all.
type RandomCandidates struct{}

// Next supplies a 16 character candidate.
func (RandomCandidates) Next() (string, error) {
	all := letters + digits + symbols
	buf := make([]byte, generatedSize)
	for i := range buf {
		c, err := pick(all)
		if err != nil {
			return "", err
		}
		buf[i] = c
	}
	positions, err := distinctPositions(3, generatedSize)
	if err != nil {
		return "", err
	}
	for i, class := range []string{letters, digits, symbols} {
		c, err := pick(class)
		if err != nil {
			return "", err
		}
		buf[positions[i]] = c
	}
	return string(buf), nil
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
