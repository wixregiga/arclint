package tenant

import (
	"crypto/rand"
	"errors"
	"strings"
)

// TenantID is the globally unique identity of a Tenant. Every other
// aggregate of the context refers to its tenant through this value.
type TenantID struct {
	value string
}

// ErrTenantIDMalformed is returned when a text is not a TenantID.
var ErrTenantIDMalformed = errors.New("tenant id must be 26 base32 characters")

const identityAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"

// NewTenantID mints a fresh identity from the operating system's random source.
func NewTenantID() TenantID {
	return TenantID{value: rand.Text()}
}

// ParseTenantID is the one door through which a text becomes a TenantID.
func ParseTenantID(text string) (TenantID, error) {
	if len(text) != 26 || strings.Trim(text, identityAlphabet) != "" {
		return TenantID{}, ErrTenantIDMalformed
	}
	return TenantID{value: text}, nil
}

// String renders the identity as the text it was parsed from.
func (id TenantID) String() string { return id.value }

// IsZero reports whether the identity was never assigned.
func (id TenantID) IsZero() bool { return id.value == "" }
