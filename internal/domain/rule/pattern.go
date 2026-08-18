package rule

import (
	"fmt"
	"regexp"
)

// patternVersion accepts the semantic-versioning core with optional
// pre-release or build metadata.
var patternVersion = regexp.MustCompile(`^\d+\.\d+\.\d+([\-+][0-9A-Za-z.\-+]+)?$`)

// PatternReference is an exact repository-owned reference to one
// distributed Pattern version. It resolves deterministically.
type PatternReference struct {
	namespace string
	name      string
	version   string
}

// NewPatternReference requires namespace, name, and an exact version.
func NewPatternReference(namespace, name, version string) (PatternReference, error) {
	if namespace == "" || name == "" {
		return PatternReference{}, fmt.Errorf("pattern reference: namespace and name required")
	}
	if !patternVersion.MatchString(version) {
		return PatternReference{}, fmt.Errorf("pattern reference %s/%s: version %q is not exact semver", namespace, name, version)
	}
	return PatternReference{namespace: namespace, name: name, version: version}, nil
}

// Namespace of the distributing Pattern.
func (r PatternReference) Namespace() string { return r.namespace }

// Name of the Pattern within its namespace.
func (r PatternReference) Name() string { return r.name }

// Version is the exact published version.
func (r PatternReference) Version() string { return r.version }

// IsZero reports an unconstructed reference.
func (r PatternReference) IsZero() bool { return r.name == "" }

func (r PatternReference) String() string {
	return r.namespace + "/" + r.name + "@" + r.version
}

// Pattern is a named, versioned, namespaced, tested collection of Rules
// dressed for distribution. A published version is immutable; every
// included Rule retains its own Rule ID; Pattern order creates no
// implicit Rule precedence.
type Pattern struct {
	ref      PatternReference
	rules    []Rule
	coverage []Language
}

// NewPattern requires an exact identity and at least one valid Rule.
// Each carried Rule is stamped with this Pattern's provenance.
func NewPattern(namespace, name, version string, rules []Rule, coverage []Language) (Pattern, error) {
	ref, err := NewPatternReference(namespace, name, version)
	if err != nil {
		return Pattern{}, err
	}
	if len(rules) == 0 {
		return Pattern{}, fmt.Errorf("pattern %s: no rules", ref)
	}
	seen := map[string]bool{}
	stamped := make([]Rule, 0, len(rules))
	for _, r := range rules {
		if r.id.IsZero() {
			return Pattern{}, fmt.Errorf("pattern %s: unconstructed rule", ref)
		}
		qualified := r.id.Qualified()
		if seen[qualified] {
			return Pattern{}, fmt.Errorf("pattern %s: duplicate rule id %q", ref, qualified)
		}
		seen[qualified] = true
		r.provenance = &ref
		stamped = append(stamped, r)
	}
	for _, l := range coverage {
		if !l.Valid() {
			return Pattern{}, fmt.Errorf("pattern %s: coverage language %q invalid", ref, l)
		}
	}
	return Pattern{
		ref: ref, rules: stamped,
		coverage: append([]Language(nil), coverage...),
	}, nil
}

// Reference identifies the exact Pattern.
func (p Pattern) Reference() PatternReference { return p.ref }

// Rules returns the Rules carried by the Pattern, each with Pattern
// provenance.
func (p Pattern) Rules() []Rule { return append([]Rule(nil), p.rules...) }

// Coverage returns the declared language coverage.
func (p Pattern) Coverage() []Language { return append([]Language(nil), p.coverage...) }
