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

// KeyDerivation is the port through which the Encrypter derives the
// stored text of a password and compares a password with stored text.
// Infrastructure implements it with a key derivation function.
type KeyDerivation interface {
	Derive(plain string) (string, error)
	Matches(plain, stored string) bool
}

// Encrypter is the domain service that turns a Password into its stored
// form and verifies a Password against one. It decides what a stored
// password is; the KeyDerivation it is given does the cryptography.
type Encrypter struct {
	derivation KeyDerivation
}

// NewEncrypter builds the Encrypter over a key derivation.
func NewEncrypter(derivation KeyDerivation) Encrypter {
	return Encrypter{derivation: derivation}
}

// Encrypt derives the stored form of the password. Assertion not-empty:
// the stored form it returns is never empty.
func (e Encrypter) Encrypt(p Password) (EncryptedPassword, error) {
	text, err := e.derivation.Derive(p.Reveal())
	if err != nil {
		return EncryptedPassword{}, err
	}
	encrypted, err := NewEncryptedPassword(text)
	if err != nil {
		return EncryptedPassword{}, err
	}
	if err := e.AssertNotEmpty(encrypted); err != nil {
		return EncryptedPassword{}, err
	}
	return encrypted, nil
}

// AssertNotEmpty checks not-empty on Encrypt. NewEncryptedPassword refuses
// empty text, so the zero EncryptedPassword is the only empty one.
func (Encrypter) AssertNotEmpty(encrypted EncryptedPassword) error {
	if encrypted == (EncryptedPassword{}) {
		return ErrEncryptedPasswordEmpty
	}
	return nil
}

// Matches tells whether the password is the one the stored form protects.
func (e Encrypter) Matches(p Password, stored EncryptedPassword) bool {
	return e.derivation.Matches(p.Reveal(), stored.String())
}
