// Package passwords holds the adapters behind the password ports of the
// identityaccess context: a PBKDF2 key derivation and the operating
// system's random source.
package passwords

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

const (
	pbkdf2Iterations = 600_000
	pbkdf2KeyLength  = 32
	saltLength       = 16
)

// PBKDF2KeyDerivation implements user.KeyDerivation with
// PBKDF2-HMAC-SHA256. The stored text is
// "pbkdf2-sha256$<iterations>$<salt>$<key>".
type PBKDF2KeyDerivation struct{}

// Derive derives a salted key from the plain text.
func (PBKDF2KeyDerivation) Derive(plain string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("pbkdf2: read salt: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, plain, salt, pbkdf2Iterations, pbkdf2KeyLength)
	if err != nil {
		return "", fmt.Errorf("pbkdf2: derive key: %w", err)
	}
	return strings.Join([]string{
		"pbkdf2-sha256",
		strconv.Itoa(pbkdf2Iterations),
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	}, "$"), nil
}

// Matches re-derives the key with the stored salt and compares in constant time.
func (PBKDF2KeyDerivation) Matches(plain, stored string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	key, err := pbkdf2.Key(sha256.New, plain, salt, iterations, len(expected))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(key, expected) == 1
}
