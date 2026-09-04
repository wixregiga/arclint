package rule

import "fmt"

// UnknownImportPolicy is the repository policy for imports that
// classify neither stdlib, internal, nor external.
type UnknownImportPolicy string

// The defined unknown-import policies.
const (
	UnknownImportsError  UnknownImportPolicy = "error"
	UnknownImportsWarn   UnknownImportPolicy = "warn"
	UnknownImportsIgnore UnknownImportPolicy = "ignore"
)

// ParseUnknownImportPolicy accepts a defined value; the empty string
// resolves to the declared default, warn.
func ParseUnknownImportPolicy(s string) (UnknownImportPolicy, error) {
	switch UnknownImportPolicy(s) {
	case UnknownImportsError, UnknownImportsWarn, UnknownImportsIgnore:
		return UnknownImportPolicy(s), nil
	}
	if s == "" {
		return UnknownImportsWarn, nil
	}
	return "", fmt.Errorf("unknown_imports policy %q: not one of error, warn, ignore", s)
}

// Scan is the repository-owned observation policy: what the walk
// excludes and how unknown imports are treated.
type Scan struct {
	Exclude         []Glob
	IncludeTestdata bool
	UnknownImports  UnknownImportPolicy
}

// Configured bundles what a repository has configured: complete Rule
// aggregates (its own and those of every Pattern it extends, Overrides
// applied), its Modules (declared and bound), the language targets,
// the scan policy, and the Extension sources the extended Patterns
// carry. It is a plain result value, not a domain object with identity.
type Configured struct {
	Rules   []Rule
	Modules []Module
	// Languages are the configured language targets (rules.yaml
	// runtime) whose facts observation should produce.
	Languages []Language
	Scan      Scan
	// Extensions are the Extension sources distributed by the extended
	// Patterns, keyed by the distributing Pattern so their registered
	// names stay attributable.
	Extensions []ConfiguredExtension
}

// ConfiguredExtension is one Extension source an extended Pattern
// distributes, attributed to that Pattern.
type ConfiguredExtension struct {
	Pattern   PatternReference
	Extension PatternExtension
}

// Repository is the domain-owned persistence port for complete Rule
// aggregates. Implementations return complete valid Rules, never
// representation structs; a representation that cannot become a valid
// Rule is an error, not a partial value.
//
//nolint:iface // Domain-owned persistence port is consumed by application use cases.
type Repository interface {
	ConfiguredRules() (Configured, error)
}
