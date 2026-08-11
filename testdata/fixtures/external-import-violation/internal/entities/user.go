package entities

import (
	"fmt"

	"github.com/pkg/errors"
)

// User is a domain entity; the entities contract forbids external imports.
type User struct {
	Name string
}

// Validate reports whether the user is well-formed.
func (u User) Validate() error {
	if u.Name == "" {
		return errors.New("empty name")
	}
	return fmt.Errorf("user %q ok", u.Name)
}
