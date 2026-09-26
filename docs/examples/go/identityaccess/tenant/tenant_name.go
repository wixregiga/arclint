package tenant

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// TenantName is the name a subscribing organisation registers under.
type TenantName struct {
	value string
}

// ErrTenantNameBlank is returned when a tenant name carries no text.
var ErrTenantNameBlank = errors.New("tenant name must not be blank")

// ErrTenantNameTooLong is returned when a tenant name exceeds 100 characters.
var ErrTenantNameTooLong = errors.New("tenant name must be at most 100 characters")

// ANCHOR: value-door

// NewTenantName is the one door through which text becomes a TenantName.
// Invariant length-within-bounds: the trimmed name is 1 to 100 characters.
func NewTenantName(text string) (TenantName, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return TenantName{}, ErrTenantNameBlank
	}
	if utf8.RuneCountInString(trimmed) > 100 {
		return TenantName{}, ErrTenantNameTooLong
	}
	return TenantName{value: trimmed}, nil
}

// ANCHOR_END: value-door

// String renders the name.
func (n TenantName) String() string { return n.value }
