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
	if len([]rune(text)) < 8 {
		return Password{}, ErrPasswordWeak
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
	if !letter || !digit || !symbol {
		return Password{}, ErrPasswordWeak
	}
	return Password{value: text}, nil
}

// Reveal hands the plain text to an Encrypter; it is the only reader.
func (p Password) Reveal() string { return p.value }

// EncryptedPassword is the stored form of a password. Only an Encrypter
// produces one, and only an Encrypter can tell whether a plain password
// matches it.
type EncryptedPassword struct {
	value string
}

// ErrEncryptedPasswordEmpty is returned when an encrypter yields nothing.
var ErrEncryptedPasswordEmpty = errors.New("encrypted password must not be empty")

// NewEncryptedPassword is the one door through which a derived stored form
// becomes an EncryptedPassword. Invariant not-empty.
func NewEncryptedPassword(text string) (EncryptedPassword, error) {
	if text == "" {
		return EncryptedPassword{}, ErrEncryptedPasswordEmpty
	}
	return EncryptedPassword{value: text}, nil
}

// String renders the stored form.
func (p EncryptedPassword) String() string { return p.value }

// KeyDerivation derives the stored form of a password's text and tells
// whether a text matches a stored form. It is the port through which the
// Encrypter reaches a key derivation function; an adapter implements it.
type KeyDerivation interface {
	Derive(plain string) (string, error)
	Verify(plain, stored string) bool
}

// ErrEncrypterWithoutKeyDerivation is returned by an Encrypter that was not
// built through NewEncrypter.
var ErrEncrypterWithoutKeyDerivation = errors.New("encrypter has no key derivation")

// Encrypter is the domain service that turns a Password into its stored
// form and verifies a Password against one, through its KeyDerivation.
type Encrypter struct {
	kdf KeyDerivation
}

// NewEncrypter returns the Encrypter that derives stored forms through kdf.
func NewEncrypter(kdf KeyDerivation) Encrypter {
	return Encrypter{kdf: kdf}
}

// Encrypt turns a Password into its stored form.
//
// Contract (assertion on Encrypt): not-empty.
func (e Encrypter) Encrypt(p Password) (EncryptedPassword, error) {
	if e.kdf == nil {
		return EncryptedPassword{}, ErrEncrypterWithoutKeyDerivation
	}
	derived, err := e.kdf.Derive(p.value)
	if err != nil {
		return EncryptedPassword{}, fmt.Errorf("encrypt password: %w", err)
	}
	encrypted, err := NewEncryptedPassword(derived)
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
	if e.kdf == nil || stored.value == "" {
		return false
	}
	return e.kdf.Verify(p.value, stored.value)
}

// AssertNotEmpty checks the contract assertion not-empty on Encrypt: an
// encrypted password is never empty.
func (Encrypter) AssertNotEmpty(encrypted EncryptedPassword) error {
	if encrypted.value == "" {
		return ErrEncryptedPasswordEmpty
	}
	return nil
}
