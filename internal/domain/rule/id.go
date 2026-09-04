package rule

import (
	"errors"
	"fmt"
	"strings"
)

// ID is the stable identity of one Rule: an explicit local identity,
// qualified by the namespace/name of the Pattern that distributes it
// ("namespace/name:local") or unqualified for a repository Rule. Two
// Patterns may distribute the same local identity; the qualifier keeps
// them distinct. Identity is never derived from array position and
// never includes a Pattern version.
type ID struct {
	namespace string
	pattern   string
	local     string
}

// namespacePart and namePart label the two parts of a Pattern
// qualifier in diagnostics: the parts of a Pattern reference or of a
// Rule ID qualifier before and after "/".
const (
	namespacePart = "namespace"
	namePart      = "name"
)

// NewID parses and validates "local" or "namespace/name:local".
func NewID(s string) (ID, error) {
	if s == "" {
		return ID{}, errors.New("rule id: empty")
	}
	qualifier, local, qualified := strings.Cut(s, ":")
	if !qualified {
		qualifier, local = "", s
	}
	if qualified && qualifier == "" {
		return ID{}, fmt.Errorf("rule id %q: empty qualifier; a distributed rule id is spelled namespace/name:local", s)
	}
	if local == "" {
		return ID{}, fmt.Errorf("rule id %q: empty local identity", s)
	}
	if strings.Contains(local, ":") {
		return ID{}, fmt.Errorf("rule id %q: more than one qualifier separator", s)
	}
	if err := validateIDPart(local); err != nil {
		return ID{}, fmt.Errorf("rule id %q: local identity %v", s, err)
	}
	if !qualified {
		return ID{local: local}, nil
	}
	namespace, name, ok := strings.Cut(qualifier, "/")
	if !ok || namespace == "" || name == "" {
		return ID{}, fmt.Errorf("rule id %q: qualifier %q must be namespace/name", s, qualifier)
	}
	if err := validateQualifierParts(namespace, name); err != nil {
		return ID{}, fmt.Errorf("rule id %q: %v", s, err)
	}
	return ID{namespace: namespace, pattern: name, local: local}, nil
}

// QualifiedID spells the identity of a Rule a Pattern distributes.
func QualifiedID(ref PatternReference, local string) (ID, error) {
	if ref.IsZero() {
		return ID{}, fmt.Errorf("rule id %q: unconstructed pattern reference", local)
	}
	return NewID(ref.Qualifier() + ":" + local)
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

// PatternName is the distributing Pattern's name, or "" for a
// repository-local Rule.
func (id ID) PatternName() string { return id.pattern }

// Qualifier is the distributing Pattern's namespace/name, or "" for a
// repository-local Rule.
func (id ID) Qualifier() string {
	if id.namespace == "" {
		return ""
	}
	return id.namespace + "/" + id.pattern
}

// Local is the identity within the qualifier.
func (id ID) Local() string { return id.local }

// Qualified returns the Pattern-qualified identity when provenance
// requires it, or the local identity otherwise.
func (id ID) Qualified() string {
	if id.namespace == "" {
		return id.local
	}
	return id.Qualifier() + ":" + id.local
}

func (id ID) String() string { return id.Qualified() }

// Equals compares Rule identity without comparing mutable
// configuration.
func (id ID) Equals(other ID) bool {
	return id.namespace == other.namespace && id.pattern == other.pattern && id.local == other.local
}
