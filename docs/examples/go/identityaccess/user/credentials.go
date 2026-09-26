package user

import "errors"

// Credentials pairs a username with the plain password chosen for it.
type Credentials struct {
	username Username
	password Password
}

// ErrPasswordEqualsUsername is returned when the password repeats the username.
var ErrPasswordEqualsUsername = errors.New("password must not equal the username")

// NewCredentials is the one door through which a username and password
// become Credentials. Invariant password-differs-from-username.
func NewCredentials(username Username, password Password) (Credentials, error) {
	if password.Reveal() == username.String() {
		return Credentials{}, ErrPasswordEqualsUsername
	}
	return Credentials{username: username, password: password}, nil
}

// Username is the name half of the credentials.
func (c Credentials) Username() Username { return c.username }

// Password is the plain password half of the credentials.
func (c Credentials) Password() Password { return c.password }
