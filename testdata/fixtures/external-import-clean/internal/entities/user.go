package entities

import (
	"fmt"
	"strings"
)

// User is a domain entity using stdlib only.
type User struct {
	Name string
}

// Validate reports whether the user is well-formed.
func (u User) Validate() error {
	if strings.TrimSpace(u.Name) == "" {
		return fmt.Errorf("empty name")
	}
	return nil
}
