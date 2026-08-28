// Identity plumbing: unguessable ids for holds and orders, readable
// slugs for events.

package app

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// newID mints an unguessable id for holds and orders.
func newID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// A platform without crypto/rand is broken; fail loudly.
		panic(err)
	}
	return hex.EncodeToString(b)
}

// slugify turns a title into the event's readable id.
func slugify(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}
