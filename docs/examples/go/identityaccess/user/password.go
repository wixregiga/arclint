package user

import (
	"errors"
	"fmt"
	"unicode"
)

// Password is a plain-text password on its way into the model. It exists
// only long enough to be encrypted; nothing stores it.
type Password struct {
	value string
}

// ErrPasswordWeak is returned when a password fails the strength rule.
var ErrPasswordWeak = errors.New("password must be at least 8 characters and mix letters, digits, and symbols")

// NewPassword is the one door through which text becomes a Password.
// Invariant not-weak: at least 8 characters holding a letter, a digit, and
// a symbol.
func NewPassword(text string) (Password, error) {
	if !strong(text) {
		return Password{}, ErrPasswordWeak
	}
	return Password{value: text}, nil
}

// strong reports whether text satisfies the strength rule: at least 8
// characters holding a letter, a digit, and a symbol.
func strong(text string) bool {
	if len([]rune(text)) < 8 {
		return false
	}
	var letter, digit, symbol bool
	for _, r := range text {
		switch {
		case unicode.IsLetter(r):
			letter = true
		case unicode.IsDigit(r):
			digit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			symbol = true
		}
	}
	return letter && digit && symbol
}

// Weak reports whether the password fails the strength rule, as a
// Password that never passed through NewPassword does.
func (p Password) Weak() bool { return !strong(p.value) }

// Reveal hands the plain text to the ProtectionScheme; it is the only
// reader.
func (p Password) Reveal() string { return p.value }

// EncryptedPassword is the stored form of a password. Only the Encrypter
// produces one, and only the Encrypter can tell whether a plain password
// matches it.
type EncryptedPassword struct {
	value string
}

// ErrEncryptedPasswordEmpty is returned when an encrypter yields nothing.
var ErrEncryptedPasswordEmpty = errors.New("encrypted password must not be empty")

// NewEncryptedPassword is the one door through which a protected form
// becomes an EncryptedPassword. Invariant not-empty.
func NewEncryptedPassword(text string) (EncryptedPassword, error) {
	if text == "" {
		return EncryptedPassword{}, ErrEncryptedPasswordEmpty
	}
	return EncryptedPassword{value: text}, nil
}

// String renders the stored form.
func (p EncryptedPassword) String() string { return p.value }

// ProtectionScheme derives the protected form of a Password and tells
// whether a Password matches a protected form. It is the port through
// which the Encrypter reaches the derivation, which needs randomness and
// a key derivation function the model does not hold; an adapter
// implements it.
type ProtectionScheme interface {
	Protect(p Password) (string, error)
	Matches(p Password, protected string) bool
}

// ErrEncrypterWithoutScheme is returned when an Encrypter was not built
// through NewEncrypter.
var ErrEncrypterWithoutScheme = errors.New("encrypter has no protection scheme")

// Encrypter is the domain service that turns a Password into its stored
// form and verifies a Password against one, through its
// ProtectionScheme.
type Encrypter struct {
	scheme ProtectionScheme
}

// NewEncrypter returns the Encrypter that protects passwords through
// scheme.
func NewEncrypter(scheme ProtectionScheme) Encrypter {
	return Encrypter{scheme: scheme}
}

// Encrypt turns a Password into its stored form.
//
// Contract (assertion on Encrypt): not-empty.
func (e Encrypter) Encrypt(p Password) (EncryptedPassword, error) {
	if e.scheme == nil {
		return EncryptedPassword{}, ErrEncrypterWithoutScheme
	}
	protected, err := e.scheme.Protect(p)
	if err != nil {
		return EncryptedPassword{}, fmt.Errorf("encrypt password: %w", err)
	}
	encrypted, err := NewEncryptedPassword(protected)
	if err != nil {
		return EncryptedPassword{}, err
	}
	if err := e.AssertNotEmpty(encrypted); err != nil {
		return EncryptedPassword{}, err
	}
	return encrypted, nil
}

// Matches tells whether p is the password stored as stored.
func (e Encrypter) Matches(p Password, stored EncryptedPassword) bool {
	if e.scheme == nil || stored.value == "" {
		return false
	}
	return e.scheme.Matches(p, stored.value)
}

// AssertNotEmpty checks the contract assertion not-empty on Encrypt: an
// encrypted password is never empty.
func (Encrypter) AssertNotEmpty(encrypted EncryptedPassword) error {
	if encrypted.value == "" {
		return ErrEncryptedPasswordEmpty
	}
	return nil
}
