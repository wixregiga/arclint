// Package distribution is the distribution bounded context: how a
// published Pattern version travels between the binary, a Registry,
// and a repository's own .arclint/patterns directory without ever
// being redefined. It speaks the rule context's Pattern and
// PatternReference unchanged and adds only what movement needs: the
// files a Pattern ships, the Manifest that names them, the Digest that
// proves them, and the sources a reference resolves through.
package distribution

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// digestScheme is the one published Digest spelling prefix.
const digestScheme = "sha256:"

// Digest is the sha256 content hash of one PatternFile, or of a whole
// Pattern through the sorted list of its file paths and file Digests.
// Two copies of one published version share one Digest.
type Digest struct {
	hex string
}

// DigestOf hashes exact bytes.
func DigestOf(data []byte) Digest {
	sum := sha256.Sum256(data)
	return Digest{hex: hex.EncodeToString(sum[:])}
}

// ParseDigest reads the published spelling sha256:<64 lowercase hex>.
func ParseDigest(s string) (Digest, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, digestScheme) {
		return Digest{}, fmt.Errorf("digest %q: expected sha256:<hex>", s)
	}
	raw := strings.TrimPrefix(s, digestScheme)
	if len(raw) != sha256.Size*2 {
		return Digest{}, fmt.Errorf("digest %q: expected %d hex digits", s, sha256.Size*2)
	}
	if _, err := hex.DecodeString(raw); err != nil || raw != strings.ToLower(raw) {
		return Digest{}, fmt.Errorf("digest %q: not lowercase hex", s)
	}
	return Digest{hex: raw}, nil
}

// IsZero reports an unconstructed Digest.
func (d Digest) IsZero() bool { return d.hex == "" }

// Equals compares two Digests by value.
func (d Digest) Equals(o Digest) bool { return d.hex == o.hex }

// String is the published spelling.
func (d Digest) String() string {
	if d.hex == "" {
		return ""
	}
	return digestScheme + d.hex
}

// Short is the leading twelve hex digits, for listings.
func (d Digest) Short() string {
	if len(d.hex) < 12 {
		return d.hex
	}
	return d.hex[:12]
}
