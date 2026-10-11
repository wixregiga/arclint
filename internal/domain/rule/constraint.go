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
	AcceptsScope(Scope) bool
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

// ImportPolicy states what an imports Constraint lets a Zone do with
// one class of imports: allow them, forbid them, or, for imports from
// outside the repository, allow only the import paths it lists, each
// matched exactly as the code spells it. The zero value resolves to
// allow, the declared default.
type ImportPolicy struct {
	mode  string
	paths []string
}

// The policy modes; a listed policy is spelled as its paths, never as
// a mode word.
const (
	importModeAllow  = "allow"
	importModeForbid = "forbid"
	importModeOnly   = "only"
)

// The two policies spelled with one word.
var (
	ImportAllow  = ImportPolicy{mode: importModeAllow}
	ImportForbid = ImportPolicy{mode: importModeForbid}
)

// ParseImportPolicy accepts allow or forbid; the empty string resolves
// to the declared default, allow.
func ParseImportPolicy(s string) (ImportPolicy, error) {
	switch s {
	case importModeAllow, "":
		return ImportAllow, nil
	case importModeForbid:
		return ImportForbid, nil
	}
	return ImportPolicy{}, fmt.Errorf("import policy %q: not allow, forbid, or a list of import paths", s)
}

// NewImportPathList returns the policy that allows only the listed
// import paths. It enforces ImportPolicy's invariant: the list holds at
// least one path, because forbid is how a Zone says it may import none;
// a blank or repeated path is rejected as well.
func NewImportPathList(paths []string) (ImportPolicy, error) {
	if len(paths) == 0 {
		return ImportPolicy{}, fmt.Errorf("import policy lists no import path; write forbid to allow none")
	}
	seen := map[string]bool{}
	for _, p := range paths {
		if strings.TrimSpace(p) == "" || p != strings.TrimSpace(p) {
			return ImportPolicy{}, fmt.Errorf("import policy lists the blank or padded path %q", p)
		}
		if seen[p] {
			return ImportPolicy{}, fmt.Errorf("import policy lists %q twice", p)
		}
		seen[p] = true
	}
	return ImportPolicy{mode: importModeOnly, paths: append([]string(nil), paths...)}, nil
}

// Forbids reports whether the policy forbids its whole import class.
func (p ImportPolicy) Forbids() bool { return p.mode == importModeForbid }

// Lists reports whether the policy allows only listed import paths.
func (p ImportPolicy) Lists() bool { return p.mode == importModeOnly }

// Paths returns the listed import paths, in the order written.
func (p ImportPolicy) Paths() []string { return append([]string(nil), p.paths...) }

// Permits reports whether an import spelled path is allowed: every path
// under allow, none under forbid, and under a list only a path written
// exactly as listed.
func (p ImportPolicy) Permits(path string) bool {
	switch p.mode {
	case importModeForbid:
		return false
	case importModeOnly:
		for _, listed := range p.paths {
			if listed == path {
				return true
			}
		}
		return false
	}
	return true
}

// Restricts reports whether the policy narrows its import class.
func (p ImportPolicy) Restricts() bool { return p.Forbids() || p.Lists() }

// String renders the policy as written: allow, forbid, or its paths.
func (p ImportPolicy) String() string {
	switch p.mode {
	case importModeForbid:
		return importModeForbid
	case importModeOnly:
		return "[" + strings.Join(p.paths, ", ") + "]"
	}
	return importModeAllow
}

func (p ImportPolicy) valid() bool {
	switch p.mode {
	case "", importModeAllow, importModeForbid:
		return len(p.paths) == 0
	case importModeOnly:
		return len(p.paths) > 0
	}
	return false
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

// AcceptsScope reports whether this Constraint can evaluate the selected code.
func (p ConsumesConstraint) AcceptsScope(s Scope) bool { return len(s.zones) > 0 && len(s.files) == 0 }

// AcceptsFiles reports whether file globs may narrow scope.
func (p ConsumesConstraint) AcceptsFiles() bool { return false }

// Validate rejects an invalid constraint configuration.
func (p ConsumesConstraint) Validate() error {
	if !p.External.valid() {
		return fmt.Errorf("consumes: external policy %s invalid", p.External)
	}
	if !p.Stdlib.valid() {
		return fmt.Errorf("consumes: stdlib policy %s invalid", p.Stdlib)
	}
	if p.Stdlib.Lists() {
		return fmt.Errorf("consumes: stdlib takes allow or forbid; only external imports take a list of import paths")
	}
	if p.Internal == nil && !p.External.Restricts() && !p.Stdlib.Forbids() {
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
	switch {
	case p.External.Forbids():
		parts = append(parts, "uses no external imports")
	case p.External.Lists():
		parts = append(parts, fmt.Sprintf("uses only the external imports %s", p.External))
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

// AcceptsScope reports whether this Constraint can evaluate the selected code.
func (p StructureConstraint) AcceptsScope(s Scope) bool { return len(s.zones) > 0 && len(s.files) == 0 }

// AcceptsFiles reports whether file globs may narrow scope.
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
// Narrowing to a subset of member files is Scope's file
// dimension, not a parameter.
type NamingConstraint struct {
	Case CaseSpec
}

// Kind returns the compatibility discriminator TypeNaming.
func (p NamingConstraint) Kind() Type { return TypeNaming }

// Key returns the authored constraint key; built-in domain constraints have none.
func (p NamingConstraint) Key() string { return "naming" }

// AcceptsScope reports whether this Constraint can evaluate the selected code.
func (p NamingConstraint) AcceptsScope(s Scope) bool { return len(s.zones) > 0 }

// AcceptsFiles reports whether file globs may narrow scope.
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

// AcceptsScope reports whether this Constraint can evaluate the selected code.
func (p LayersConstraint) AcceptsScope(s Scope) bool { return s.entireRepository && len(s.files) == 0 }

// AcceptsFiles reports whether file globs may narrow scope.
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

// AcceptsScope reports whether this Constraint can evaluate the selected code.
func (p ProtectedConstraint) AcceptsScope(s Scope) bool {
	return s.entireRepository && len(s.files) == 0
}

// AcceptsFiles reports whether file globs may narrow scope.
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

// AcceptsScope reports whether this Constraint can evaluate the selected code.
func (p ExtensionConstraint) AcceptsScope(s Scope) bool { return !s.IsZero() }

// AcceptsFiles reports whether file globs may narrow scope.
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

// AcceptsScope reports whether this Constraint can evaluate the selected code.
func (p AcyclicConstraint) AcceptsScope(s Scope) bool { return s.entireRepository && len(s.files) == 0 }

// AcceptsFiles reports whether file globs may narrow scope.
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

// AcceptsScope reports whether this Constraint can evaluate the selected code.
func (p IndependenceConstraint) AcceptsScope(s Scope) bool {
	return s.entireRepository && len(s.files) == 0
}

// AcceptsFiles reports whether file globs may narrow scope.
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

// AcceptsScope reports whether this Constraint can evaluate the selected code.
func (p DomainConstraint) AcceptsScope(s Scope) bool { return s.entireRepository && len(s.files) == 0 }

// AcceptsFiles reports whether file globs may narrow scope.
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

// AcceptsScope reports whether this Constraint can evaluate the selected code.
func (p ContentConstraint) AcceptsScope(s Scope) bool { return !s.IsZero() }

// AcceptsFiles reports whether file globs may narrow scope.
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
