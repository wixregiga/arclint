package passwords

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// SystemRandom implements provisioning.RandomSource with the operating
// system's cryptographic random source.
type SystemRandom struct{}

// Intn returns a uniformly random integer in [0, n).
func (SystemRandom) Intn(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("random source: bound %d is not positive", n)
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, fmt.Errorf("random source: %w", err)
	}
	return int(v.Int64()), nil
}
