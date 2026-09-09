package rule

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// Constraint is the checkable architectural proposition a Rule carries.
// Each concrete value owns its configuration and validates its own restrictions.
type Constraint interface {
	Key() string
	Kind() Type
	Scope() Scope
	AcceptsFiles() bool
	Proposition() string
	Validate() error
}

// AllowList is a declared Zone allow-list. The empty list is
// meaningful: it permits no other declared Zone. The owning Zone
// itself is always permitted implicitly.
type AllowList struct {
	zones []ZoneName
}

// NewAllowList validates the listed Zone names and rejects
// duplicates.
func NewAllowList(zones ...ZoneName) (AllowList, error) {
	seen := map[ZoneName]bool{}
	out := make([]ZoneName, 0, len(zones))
	for _, m := range zones {
		if err := m.validate(); err != nil {
			return AllowList{}, err
		}
		if seen[m] {
			return AllowList{}, fmt.Errorf("allow-list: duplicate zone %q", m)
		}
		seen[m] = true
		out = append(out, m)
	}
	return AllowList{zones: out}, nil
}

// Zones returns the allowed Zone names.
func (l AllowList) Zones() []ZoneName {
	return append([]ZoneName(nil), l.zones...)
}

// Permits reports whether the named Zone is on the list.
func (l AllowList) Permits(m ZoneName) bool {
	for _, a := range l.zones {
		if a == m {
			return true
		}
	}
	return false
}

// ImportPolicy allows or forbids one class of imports. The zero value
// resolves to allow, the declared default.
type ImportPolicy string

// The two import policies.
const (
	ImportAllow  ImportPolicy = "allow"
	ImportForbid ImportPolicy = "forbid"
)

// ParseImportPolicy accepts allow or forbid; the empty string resolves
// to the declared default, allow.
func ParseImportPolicy(s string) (ImportPolicy, error) {
	switch ImportPolicy(s) {
	case ImportAllow, ImportForbid:
		return ImportPolicy(s), nil
	}
	if s == "" {
		return ImportAllow, nil
	}
	return "", fmt.Errorf("import policy %q: not allow or forbid", s)
}

// Forbids reports whether the policy forbids its import class.
func (p ImportPolicy) Forbids() bool { return p == ImportForbid }

func (p ImportPolicy) valid() bool {
	return p == "" || p == ImportAllow || p == ImportForbid
}

// ConsumesConstraint state what the Rule's Zone may import. Internal nil
// means other declared Zones are unrestricted.
type ConsumesConstraint struct {
	Internal *AllowList
	External ImportPolicy
	Stdlib   ImportPolicy
}

// Kind returns the compatibility discriminator TypeConsumes.
func (p ConsumesConstraint) Kind() Type { return TypeConsumes }

// Key returns the authored constraint key; built-in domain constraints have none.
func (p ConsumesConstraint) Key() string { return "imports" }

// Scope returns the demanded applicability shape.
func (p ConsumesConstraint) Scope() Scope { return ScopeZones }

// AcceptsFiles reports whether file globs may narrow applicability.
func (p ConsumesConstraint) AcceptsFiles() bool { return false }

// Validate rejects an invalid constraint configuration.
func (p ConsumesConstraint) Validate() error {
	if !p.External.valid() {
		return fmt.Errorf("consumes: external policy %q invalid", p.External)
	}
	if !p.Stdlib.valid() {
		return fmt.Errorf("consumes: stdlib policy %q invalid", p.Stdlib)
	}
	if p.Internal == nil && !p.External.Forbids() && !p.Stdlib.Forbids() {
		return fmt.Errorf("consumes: no restriction declared; the Rule would state no proposition")
	}
	return nil
}

// Proposition states the configured architectural proposition.
func (p ConsumesConstraint) Proposition() string {
	var parts []string
	if p.Internal != nil {
		if len(p.Internal.zones) == 0 {
			parts = append(parts, "imports no other declared Zone")
		} else {
			parts = append(parts, fmt.Sprintf("imports only the declared Zones %s", zoneList(p.Internal.zones)))
		}
	}
	if p.External.Forbids() {
		parts = append(parts, "uses no external imports")
	}
	if p.Stdlib.Forbids() {
		parts = append(parts, "uses no standard-library imports")
	}
	return strings.Join(parts, " and ")
}

// StructureConstraint require or forbid member files matching globs.
type StructureConstraint struct {
	Require []Glob
	Forbid  []Glob
}

// Kind returns the compatibility discriminator TypeStructure.
func (p StructureConstraint) Kind() Type { return TypeStructure }

// Key returns the authored constraint key; built-in domain constraints have none.
func (p StructureConstraint) Key() string { return "structure" }

// Scope returns the demanded applicability shape.
func (p StructureConstraint) Scope() Scope { return ScopeZones }

// AcceptsFiles reports whether file globs may narrow applicability.
func (p StructureConstraint) AcceptsFiles() bool { return false }

// Validate rejects an invalid constraint configuration.
func (p StructureConstraint) Validate() error {
	if len(p.Require)+len(p.Forbid) == 0 {
		return fmt.Errorf("structure: neither require nor forbid declared")
	}
	for _, g := range append(append([]Glob(nil), p.Require...), p.Forbid...) {
		if g.IsZero() {
			return fmt.Errorf("structure: unconstructed glob")
		}
	}
	return nil
}

// Proposition states the configured architectural proposition.
func (p StructureConstraint) Proposition() string {
	var parts []string
	if len(p.Require) > 0 {
		parts = append(parts, fmt.Sprintf("contains files matching %s", globList(p.Require)))
	}
	if len(p.Forbid) > 0 {
		parts = append(parts, fmt.Sprintf("contains no files matching %s", globList(p.Forbid)))
	}
	return strings.Join(parts, " and ")
}

// NamingConstraint constrain the file-name case of the Rule's Subjects.
// Narrowing to a subset of member files is Applicability's file
// dimension, not a parameter.
type NamingConstraint struct {
	Case CaseSpec
}

// Kind returns the compatibility discriminator TypeNaming.
func (p NamingConstraint) Kind() Type { return TypeNaming }

// Key returns the authored constraint key; built-in domain constraints have none.
func (p NamingConstraint) Key() string { return "naming" }

// Scope returns the demanded applicability shape.
func (p NamingConstraint) Scope() Scope { return ScopeZones }

// AcceptsFiles reports whether file globs may narrow applicability.
func (p NamingConstraint) AcceptsFiles() bool { return true }

// Validate rejects an invalid constraint configuration.
func (p NamingConstraint) Validate() error {
	if p.Case.IsZero() {
		return fmt.Errorf("naming: missing case specification")
	}
	return nil
}

// Proposition states the configured architectural proposition.
func (p NamingConstraint) Proposition() string {
	return fmt.Sprintf("file names use %s", p.Case)
}

// LayersConstraint order Zones highest first.
type LayersConstraint struct {
	Layers []ZoneName
}

// Kind returns the compatibility discriminator TypeLayers.
func (p LayersConstraint) Kind() Type { return TypeLayers }

// Key returns the authored constraint key; built-in domain constraints have none.
func (p LayersConstraint) Key() string { return "layers" }

// Scope returns the demanded applicability shape.
func (p LayersConstraint) Scope() Scope { return ScopeRepository }

// AcceptsFiles reports whether file globs may narrow applicability.
func (p LayersConstraint) AcceptsFiles() bool { return false }

// Validate rejects an invalid constraint configuration.
func (p LayersConstraint) Validate() error {
	if len(p.Layers) < 2 {
		return fmt.Errorf("layers: fewer than two layers")
	}
	return uniqueValidZones("layers", p.Layers)
}

// Proposition states the configured architectural proposition.
func (p LayersConstraint) Proposition() string {
	return fmt.Sprintf("Zones layer highest first as %s; a Zone never imports a higher layer", zoneList(p.Layers))
}

// ProtectedConstraint restrict who may import one Zone.
type ProtectedConstraint struct {
	Zone  ZoneName
	Allow []ZoneName
}

// Kind returns the compatibility discriminator TypeProtected.
func (p ProtectedConstraint) Kind() Type { return TypeProtected }

// Key returns the authored constraint key; built-in domain constraints have none.
func (p ProtectedConstraint) Key() string { return "imported_by" }

// Scope returns the demanded applicability shape.
func (p ProtectedConstraint) Scope() Scope { return ScopeOneZone }

// AcceptsFiles reports whether file globs may narrow applicability.
func (p ProtectedConstraint) AcceptsFiles() bool { return false }

// Validate rejects an invalid constraint configuration.
func (p ProtectedConstraint) Validate() error {
	if err := p.Zone.validate(); err != nil {
		return fmt.Errorf("protected: %v", err)
	}
	return uniqueValidZones("protected allow", p.Allow)
}

// Proposition states the configured architectural proposition.
func (p ProtectedConstraint) Proposition() string {
	if len(p.Allow) == 0 {
		return fmt.Sprintf("Zone %q is imported by no other Zone", p.Zone)
	}
	return fmt.Sprintf("Zone %q is imported only by %s", p.Zone, zoneList(p.Allow))
}

// ExtensionConstraint bind a Rule to Extension-supplied enforcement: the
// registered extension rule name and the parameters its published
// schema validates host-side before any extension code runs.
type ExtensionConstraint struct {
	Uses string
	With map[string]any
}

// Kind returns the compatibility discriminator TypeExtension.
func (p ExtensionConstraint) Kind() Type { return TypeExtension }

// Key returns the authored constraint key; built-in domain constraints have none.
func (p ExtensionConstraint) Key() string { return "uses" }

// Scope returns the demanded applicability shape.
func (p ExtensionConstraint) Scope() Scope { return ScopeZonesOrRepository }

// AcceptsFiles reports whether file globs may narrow applicability.
func (p ExtensionConstraint) AcceptsFiles() bool { return true }

// Validate rejects an invalid constraint configuration.
func (p ExtensionConstraint) Validate() error {
	if strings.TrimSpace(p.Uses) == "" {
		return fmt.Errorf("extension: missing the extension rule name (uses)")
	}
	return nil
}

// Proposition states the configured architectural proposition.
func (p ExtensionConstraint) Proposition() string {
	return fmt.Sprintf("satisfies extension rule %q", p.Uses)
}

// AcyclicConstraint scope the no-cycles proposition; an empty scope means
// every Zone the repository declares. A Pattern never distributes an
// empty scope: its loader resolves {} to the Pattern's own Zones.
type AcyclicConstraint struct {
	Zones []ZoneName
}

// Kind returns the compatibility discriminator TypeAcyclic.
func (p AcyclicConstraint) Kind() Type { return TypeAcyclic }

// Key returns the authored constraint key; built-in domain constraints have none.
func (p AcyclicConstraint) Key() string { return "acyclic" }

// Scope returns the demanded applicability shape.
func (p AcyclicConstraint) Scope() Scope { return ScopeRepository }

// AcceptsFiles reports whether file globs may narrow applicability.
func (p AcyclicConstraint) AcceptsFiles() bool { return false }

// Validate rejects an invalid constraint configuration.
func (p AcyclicConstraint) Validate() error {
	return uniqueValidZones("acyclic", p.Zones)
}

// Proposition states the configured architectural proposition.
func (p AcyclicConstraint) Proposition() string {
	if len(p.Zones) == 0 {
		return "declared Zone dependencies contain no cycle"
	}
	return fmt.Sprintf("dependencies among %s contain no cycle", zoneList(p.Zones))
}

// IndependenceConstraint configures an independence Rule: sibling Folders
// selected by the globs may not import each other.
type IndependenceConstraint struct {
	Folders []Glob
}

// Kind returns the compatibility discriminator TypeIndependence.
func (p IndependenceConstraint) Kind() Type { return TypeIndependence }

// Key returns the authored constraint key; built-in domain constraints have none.
func (p IndependenceConstraint) Key() string { return "independent" }

// Scope returns the demanded applicability shape.
func (p IndependenceConstraint) Scope() Scope { return ScopeRepository }

// AcceptsFiles reports whether file globs may narrow applicability.
func (p IndependenceConstraint) AcceptsFiles() bool { return false }

// Validate rejects an invalid constraint configuration.
func (p IndependenceConstraint) Validate() error {
	if len(p.Folders) == 0 {
		return fmt.Errorf("independence folders: none declared")
	}
	seen := map[string]bool{}
	for _, g := range p.Folders {
		s := g.String()
		if s == "" {
			return fmt.Errorf("independence folders: empty glob")
		}
		if seen[s] {
			return fmt.Errorf("independence folders: duplicate %q", s)
		}
		seen[s] = true
	}
	return nil
}

// Proposition states the configured architectural proposition.
func (p IndependenceConstraint) Proposition() string {
	return fmt.Sprintf("sibling Folders matching %s may not import each other", globList(p.Folders))
}

// DomainConstraint name the one block invariant of the Domain-Driven
// Design meta-model a domain Rule evaluates: an invariant the domain
// evaluator enforces, by its id.
type DomainConstraint struct {
	Invariant string
}

// Kind returns the compatibility discriminator TypeDomain.
func (p DomainConstraint) Kind() Type { return TypeDomain }

// Key returns the authored constraint key; built-in domain constraints have none.
func (p DomainConstraint) Key() string { return "" }

// Scope returns the demanded applicability shape.
func (p DomainConstraint) Scope() Scope { return ScopeRepository }

// AcceptsFiles reports whether file globs may narrow applicability.
func (p DomainConstraint) AcceptsFiles() bool { return false }

// Validate rejects an invalid constraint configuration.
func (p DomainConstraint) Validate() error {
	inv, ok := vocab.DDD().Invariant(p.Invariant)
	if !ok {
		return fmt.Errorf("domain: %q is not a block invariant of the %s meta-model", p.Invariant, vocab.DDD().Name)
	}
	if inv.Enforcement.By != vocab.EvaluatorDomain {
		return fmt.Errorf("domain: %s is not evaluated by the domain evaluator (enforcement: %s)", inv.ID, describeEnforcement(inv))
	}
	return nil
}

// Proposition states the configured architectural proposition.
func (p DomainConstraint) Proposition() string {
	inv, _ := vocab.DDD().Invariant(p.Invariant)
	return inv.Statement
}

// BlockInvariant returns the meta-model invariant the parameters name.
// Construction validated the name, so a miss means the value was built
// outside New.
func (p DomainConstraint) BlockInvariant() (vocab.BlockInvariant, error) {
	inv, ok := vocab.DDD().Invariant(p.Invariant)
	if !ok {
		return vocab.BlockInvariant{}, fmt.Errorf("domain: %q is not a block invariant of the %s meta-model", p.Invariant, vocab.DDD().Name)
	}
	return inv, nil
}

// describeEnforcement spells how a meta-model invariant is enforced,
// for diagnostics about Rules that name one.
func describeEnforcement(inv vocab.BlockInvariant) string {
	if inv.Enforcement.Needs != "" {
		return "milestone " + inv.Enforcement.Needs
	}
	return string(inv.Enforcement.By)
}

// ContentConstraint configure a content Rule: no line of any selected file
// may match the forbidden regular expression (Go RE2 syntax).
type ContentConstraint struct {
	Forbid string
}

// Kind returns the compatibility discriminator TypeContent.
func (p ContentConstraint) Kind() Type { return TypeContent }

// Key returns the authored constraint key; built-in domain constraints have none.
func (p ContentConstraint) Key() string { return "content" }

// Scope returns the demanded applicability shape.
func (p ContentConstraint) Scope() Scope { return ScopeZonesOrRepository }

// AcceptsFiles reports whether file globs may narrow applicability.
func (p ContentConstraint) AcceptsFiles() bool { return true }

// Validate rejects an invalid constraint configuration.
func (p ContentConstraint) Validate() error {
	if strings.TrimSpace(p.Forbid) == "" {
		return fmt.Errorf("content: missing the forbidden pattern (forbid)")
	}
	if _, err := regexp.Compile(p.Forbid); err != nil {
		return fmt.Errorf("content: forbid %q: %v", p.Forbid, err)
	}
	return nil
}

// Proposition states the configured architectural proposition.
func (p ContentConstraint) Proposition() string {
	return fmt.Sprintf("contains no line matching /%s/", p.Forbid)
}

// Regexp compiles the forbidden pattern. Construction validated it, so
// a compile failure here means the value was built outside New.
func (p ContentConstraint) Regexp() (*regexp.Regexp, error) {
	re, err := regexp.Compile(p.Forbid)
	if err != nil {
		return nil, fmt.Errorf("content: forbid %q: %v", p.Forbid, err)
	}
	return re, nil
}

func uniqueValidZones(what string, zones []ZoneName) error {
	seen := map[ZoneName]bool{}
	for _, m := range zones {
		if err := m.validate(); err != nil {
			return fmt.Errorf("%s: %v", what, err)
		}
		if seen[m] {
			return fmt.Errorf("%s: duplicate zone %q", what, m)
		}
		seen[m] = true
	}
	return nil
}

func zoneList(zones []ZoneName) string {
	names := make([]string, len(zones))
	for i, m := range zones {
		names[i] = fmt.Sprintf("%q", string(m))
	}
	return "[" + strings.Join(names, ", ") + "]"
}

func globList(globs []Glob) string {
	names := make([]string, len(globs))
	for i, g := range globs {
		names[i] = fmt.Sprintf("%q", g.String())
	}
	return "[" + strings.Join(names, ", ") + "]"
}
