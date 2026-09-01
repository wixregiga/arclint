package rule

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	exactPatternVersion  = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-((0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(\.(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*))?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)
	patternOriginSegment = regexp.MustCompile(`^[^/@\s]+$`)
)

// PatternOrigin identifies the exact Pattern version from which a Rule
// originated. A Pattern version never becomes part of the RuleID.
type PatternOrigin struct {
	namespace string
	name      string
	version   string
}

// NewPatternOrigin requires a namespace, a name, and an exact semantic
// version. Floating labels and version ranges cannot identify Rule origin.
func NewPatternOrigin(namespace, name, version string) (PatternOrigin, error) {
	if strings.TrimSpace(namespace) == "" || strings.TrimSpace(name) == "" {
		return PatternOrigin{}, fmt.Errorf("pattern origin: namespace and name required")
	}
	if !patternOriginSegment.MatchString(namespace) || !patternOriginSegment.MatchString(name) {
		return PatternOrigin{}, fmt.Errorf("pattern origin %q/%q: namespace and name must be reference segments", namespace, name)
	}
	if !exactPatternVersion.MatchString(version) {
		return PatternOrigin{}, fmt.Errorf("pattern origin %s/%s: version %q is not exact semver", namespace, name, version)
	}
	return PatternOrigin{namespace: namespace, name: name, version: version}, nil
}

// Namespace is the distributing Pattern's namespace.
func (o PatternOrigin) Namespace() string { return o.namespace }

// Name is the Pattern's name within its namespace.
func (o PatternOrigin) Name() string { return o.name }

// Version is the exact distributed version.
func (o PatternOrigin) Version() string { return o.version }

// IsZero reports an unconstructed origin.
func (o PatternOrigin) IsZero() bool {
	return o.namespace == "" || o.name == "" || o.version == ""
}

func (o PatternOrigin) String() string {
	return o.namespace + "/" + o.name + "@" + o.version
}
