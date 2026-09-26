// Package passwords holds the adapters behind the password ports of the
// identityaccess context: a PBKDF2 encrypter and a random generator.
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

	"example.com/saasovation/identityaccess/user"
)

const (
	pbkdf2Iterations = 600_000
	pbkdf2KeyLength  = 32
	saltLength       = 16
)

// PBKDF2Encrypter implements user.Encrypter with PBKDF2-HMAC-SHA256.
// The stored form is "pbkdf2-sha256$<iterations>$<salt>$<key>".
type PBKDF2Encrypter struct{}

// Encrypt derives a salted key from the password.
func (PBKDF2Encrypter) Encrypt(p user.Password) (user.EncryptedPassword, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return user.EncryptedPassword{}, fmt.Errorf("pbkdf2: read salt: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, p.Reveal(), salt, pbkdf2Iterations, pbkdf2KeyLength)
	if err != nil {
		return user.EncryptedPassword{}, fmt.Errorf("pbkdf2: derive key: %w", err)
	}
	encoded := strings.Join([]string{
		"pbkdf2-sha256",
		strconv.Itoa(pbkdf2Iterations),
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	}, "$")
	return user.NewEncryptedPassword(encoded)
}

// Matches re-derives the key with the stored salt and compares in constant time.
func (PBKDF2Encrypter) Matches(p user.Password, stored user.EncryptedPassword) bool {
	parts := strings.Split(stored.String(), "$")
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
	key, err := pbkdf2.Key(sha256.New, p.Reveal(), salt, iterations, len(expected))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(key, expected) == 1
}
