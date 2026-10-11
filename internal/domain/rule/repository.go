package rule

import "fmt"

// RulesetFileName is the repository ruleset file: the one document
// arclint discovers upward from the working directory, drafts on
// init, and evaluates on check.
const RulesetFileName = "rules.arclint.yaml"

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

// ConfiguredExtension is one Extension source an extended Pattern
// distributes, attributed to that Pattern.
type ConfiguredExtension struct {
	Pattern   PatternReference
	Extension PatternExtension
}

// Repository is the domain-owned persistence port for complete Rule
// aggregates. Implementations return the repository's Ruleset, built
// through NewRuleset, never representation structs; a representation
// that cannot become a valid Ruleset is an error, not a partial value.
//
//nolint:iface // Domain-owned persistence port is consumed by application use cases.
type Repository interface {
	ConfiguredRules() (Ruleset, error)
}
