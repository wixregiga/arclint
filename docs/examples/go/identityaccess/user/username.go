package user

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// Username is the name a user signs in with, unique within a tenant.
type Username struct {
	value string
}

// ErrUsernameLength is returned when a username is not 3 to 250 characters.
var ErrUsernameLength = errors.New("username must be 3 to 250 characters")

// NewUsername is the one door through which text becomes a Username.
// Invariant length-within-bounds: the trimmed name is 3 to 250 characters.
func NewUsername(text string) (Username, error) {
	trimmed := strings.TrimSpace(text)
	n := utf8.RuneCountInString(trimmed)
	if n < 3 || n > 250 {
		return Username{}, ErrUsernameLength
	}
	return Username{value: trimmed}, nil
}

// String renders the username.
func (u Username) String() string { return u.value }
