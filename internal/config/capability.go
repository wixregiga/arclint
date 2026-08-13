package config

// Capability states HOW a rule type enforces its claim, so a finding
// never carries more confidence than its mechanism earns (M7 ADR):
//
//	exact       from the classified import graph or parsed syntax facts
//	structural  from paths, shapes, and declaration placement
//	heuristic   from names, regexes over text, or complexity signals
//	advisory    guidance; reports without claiming proof
//
// Builtin kinds are labeled here; extension types declare theirs in
// defineRule (default heuristic, the conservative claim). Labels surface
// in `rules ls`, `explain`, and on every finding.
const (
	CapabilityExact      = "exact"
	CapabilityStructural = "structural"
	CapabilityHeuristic  = "heuristic"
	CapabilityAdvisory   = "advisory"
)

// ValidCapability reports whether s names a capability tier.
func ValidCapability(s string) bool {
	switch s {
	case CapabilityExact, CapabilityStructural, CapabilityHeuristic, CapabilityAdvisory:
		return true
	}
	return false
}

// builtinCapabilities maps every builtin rule kind to its enforcement
// tier. Import-graph rules are exact; path/declaration rules are
// structural; text regexes are heuristic.
var builtinCapabilities = map[string]string{
	"policy":         CapabilityExact, // per-module consumes
	"unknown-import": CapabilityExact,
	"layers":         CapabilityExact,
	"forbidden":      CapabilityExact,
	"independence":   CapabilityExact,
	"protected":      CapabilityExact,
	"acyclic":        CapabilityExact,
	"registration":   CapabilityStructural,
	"correspondence": CapabilityStructural,
	"naming":         CapabilityStructural,
	"structure":      CapabilityStructural,
	"expr":           CapabilityStructural,
	"content":        CapabilityHeuristic,
}

// CapabilityOf returns the enforcement tier of a builtin kind, or "".
func CapabilityOf(kind string) string {
	return builtinCapabilities[kind]
}
