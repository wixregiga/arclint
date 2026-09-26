package user

import (
	"errors"
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

// NewEncryptedPassword is the one door through which an encrypter's output
// becomes an EncryptedPassword. Invariant not-empty.
func NewEncryptedPassword(text string) (EncryptedPassword, error) {
	if text == "" {
		return EncryptedPassword{}, ErrEncryptedPasswordEmpty
	}
	return EncryptedPassword{value: text}, nil
}

// String renders the stored form.
func (p EncryptedPassword) String() string { return p.value }

// Encrypter is the domain service that turns a Password into its stored
// form and verifies a Password against one. Infrastructure implements it.
type Encrypter interface {
	Encrypt(p Password) (EncryptedPassword, error)
	Matches(p Password, stored EncryptedPassword) bool
}
