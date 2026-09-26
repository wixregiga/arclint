package role

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// RoleName names a role within a tenant, unique per tenant.
type RoleName struct {
	value string
}

// ErrRoleNameLength is returned when a role name is not 1 to 250 characters.
var ErrRoleNameLength = errors.New("role name must be 1 to 250 characters")

// NewRoleName is the one door through which text becomes a RoleName.
// Invariant length-within-bounds.
func NewRoleName(text string) (RoleName, error) {
	trimmed := strings.TrimSpace(text)
	n := utf8.RuneCountInString(trimmed)
	if n < 1 || n > 250 {
		return RoleName{}, ErrRoleNameLength
	}
	return RoleName{value: trimmed}, nil
}

// String renders the name.
func (n RoleName) String() string { return n.value }
