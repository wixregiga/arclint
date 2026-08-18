package rule

import (
	"errors"
	"fmt"
	"strings"
)

// ID is the stable identity of one Rule: an explicit local identity,
// optionally qualified by the namespace of the Pattern that distributes
// it ("namespace:local"). Identity is never derived from array position
// and never includes a Pattern version.
type ID struct {
	namespace string
	local     string
}

// NewID parses and validates "local" or "namespace:local".
func NewID(s string) (ID, error) {
	if s == "" {
		return ID{}, errors.New("rule id: empty")
	}
	namespace, local, qualified := strings.Cut(s, ":")
	if !qualified {
		namespace, local = "", s
	}
	if qualified && namespace == "" {
		return ID{}, fmt.Errorf("rule id %q: empty namespace", s)
	}
	if local == "" {
		return ID{}, fmt.Errorf("rule id %q: empty local identity", s)
	}
	if strings.Contains(local, ":") {
		return ID{}, fmt.Errorf("rule id %q: more than one namespace separator", s)
	}
	for part, name := range map[string]string{namespace: "namespace", local: "local identity"} {
		if part == "" {
			continue
		}
		if err := validateIDPart(part); err != nil {
			return ID{}, fmt.Errorf("rule id %q: %s %v", s, name, err)
		}
	}
	return ID{namespace: namespace, local: local}, nil
}

func validateIDPart(part string) error {
	for _, r := range part {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9',
			r == '.', r == '_', r == '-', r == '/':
		default:
			return fmt.Errorf("contains %q (allowed: a-z 0-9 . _ - /)", r)
		}
	}
	if first := part[0]; first == '.' || first == '/' || first == '-' {
		return fmt.Errorf("starts with %q", first)
	}
	if last := part[len(part)-1]; last == '.' || last == '/' {
		return fmt.Errorf("ends with %q", last)
	}
	return nil
}

// IsZero reports an unconstructed ID.
func (id ID) IsZero() bool { return id.local == "" }

// Namespace is the distributing Pattern's namespace, or "" for a
// repository-local Rule.
func (id ID) Namespace() string { return id.namespace }

// Local is the identity within the namespace.
func (id ID) Local() string { return id.local }

// Qualified returns the namespaced identity when provenance requires
// it, or the local identity otherwise.
func (id ID) Qualified() string {
	if id.namespace == "" {
		return id.local
	}
	return id.namespace + ":" + id.local
}

func (id ID) String() string { return id.Qualified() }

// Equals compares Rule identity without comparing mutable
// configuration.
func (id ID) Equals(other ID) bool {
	return id.namespace == other.namespace && id.local == other.local
}
