package rule

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// Type is one value from the finite ArcLint-owned set of supported Rule
// shapes: consumes, structure, naming, layers, protected, independence,
// acyclic, domain, content, and extension. Pattern and Extension
// authors configure existing values; they do not add new ones; custom
// logic plugs into the extension kind through the SDK, it never grows
// this enum. In rules.arclint.yaml a Type is never spelled: the one Assertion
// key a Rule carries decides it (see AssertionKey); the domain Type has
// no key because its Rules are built in, never authored.
type Type string

const (
	// TypeConsumes states what a Zone may import: other declared
	// Zones by allow-list, external and standard-library imports by
	// policy.
	TypeConsumes Type = "consumes"
	// TypeStructure requires or forbids files matching globs inside a
	// Zone.
	TypeStructure Type = "structure"
	// TypeNaming constrains file names within a Zone to a finite case
	// vocabulary.
	TypeNaming Type = "naming"
	// TypeLayers orders Zones highest first; a Zone may import same
	// or lower layers, never higher.
	TypeLayers Type = "layers"
	// TypeProtected restricts which Zones may import one Zone.
	TypeProtected Type = "protected"
	// TypeIndependence forbids imports between sibling Folders.
	TypeIndependence Type = "independence"
	// TypeAcyclic forbids dependency cycles among declared Zones.
	TypeAcyclic Type = "acyclic"
	// TypeDomain evaluates one invariant of a Domain-Driven Design
	// building block against every instance the recorded domain holds.
	// Its Rules are built in: arclint composes one per check-level block
	// invariant under the invariant's own id the moment a domain is
	// recorded (see BuiltIn), and a ruleset adopts them through
	// Overrides only.
	TypeDomain Type = "domain"
	// TypeContent forbids lines matching a regular expression in the
	// Rule's Subjects: the built-in evaluator over file bytes.
	TypeContent Type = "content"
	// TypeExtension delegates enforcement to a named Extension through
	// the sandboxed SDK; parameters are validated host-side against the
	// extension's published schema before any extension code runs.
	TypeExtension Type = "extension"
)

// Types returns the published enum in stable order.
func Types() []Type {
	return []Type{
		TypeConsumes, TypeStructure, TypeNaming,
		TypeLayers, TypeProtected, TypeIndependence, TypeAcyclic, TypeDomain,
		TypeContent, TypeExtension,
	}
}

// assertionKeys maps each authored Type to the one rules.arclint.yaml
// key that spells its Assertion. The key is the Type's whole public
// spelling: a Rule written with that key is a Rule of that Type, and
// no rule carries two keys. The domain Type has no key: its Rules are
// never written.
var assertionKeys = map[Type]string{
	TypeConsumes:     "imports",
	TypeStructure:    "structure",
	TypeNaming:       "naming",
	TypeLayers:       "layers",
	TypeProtected:    "imported_by",
	TypeIndependence: "independent",
	TypeAcyclic:      "acyclic",
	TypeContent:      "content",
	TypeExtension:    "uses",
}

// AssertionKey returns the rules.arclint.yaml key that spells this Type's
// Assertion, or "" for the domain Type, which is never authored.
func (t Type) AssertionKey() string { return assertionKeys[t] }

// Authored reports whether a ruleset can spell a Rule of this Type:
// every Type but domain, whose Rules are built in.
func (t Type) Authored() bool { return assertionKeys[t] != "" }

// AuthoredTypes returns the Types a ruleset can spell, in published
// order.
func AuthoredTypes() []Type {
	out := make([]Type, 0, len(assertionKeys))
	for _, t := range Types() {
		if t.Authored() {
			out = append(out, t)
		}
	}
	return out
}

// AssertionKeys returns every Assertion key in published Type order.
func AssertionKeys() []string {
	out := make([]string, 0, len(assertionKeys))
	for _, t := range Types() {
		if t.Authored() {
			out = append(out, assertionKeys[t])
		}
	}
	return out
}

// TypeOfAssertionKey resolves a rules.arclint.yaml Assertion key to its Type.
func TypeOfAssertionKey(key string) (Type, bool) {
	for _, t := range Types() {
		if assertionKeys[t] == key {
			return t, true
		}
	}
	return "", false
}

// Scope is the Applicability shape a Type demands: which Zones a
// Rule of the Type judges and how rules.arclint.yaml spells that.
type Scope int

const (
	// ScopeZones judges the members of the Zones named under on;
	// on is required.
	ScopeZones Scope = iota
	// ScopeOneZone judges exactly one Zone named under on.
	ScopeOneZone
	// ScopeRepository ranges over the repository's Zone graph or
	// its files; on is not accepted.
	ScopeRepository
	// ScopeZonesOrRepository judges the Zones named under on, or
	// the whole repository when on is omitted.
	ScopeZonesOrRepository
)

// Scope returns the Applicability shape the Type demands.
func (t Type) Scope() Scope {
	switch t {
	case TypeConsumes, TypeStructure, TypeNaming:
		return ScopeZones
	case TypeProtected:
		return ScopeOneZone
	case TypeLayers, TypeIndependence, TypeAcyclic, TypeDomain:
		return ScopeRepository
	case TypeContent, TypeExtension:
		return ScopeZonesOrRepository
	}
	return ScopeRepository
}

// AcceptsFiles reports whether the Type's Applicability may be narrowed
// by file globs (the files key).
func (t Type) AcceptsFiles() bool {
	switch t {
	case TypeNaming, TypeContent, TypeExtension:
		return true
	default:
		return false
	}
}

// ParseType accepts only a published enum value.
func ParseType(s string) (Type, error) {
	for _, t := range Types() {
		if Type(s) == t {
			return t, nil
		}
	}
	return "", fmt.Errorf("rule type %q: not a published ArcLint Rule Type %v", s, Types())
}

// Valid reports whether the value is a published enum member.
func (t Type) Valid() bool {
	for _, known := range Types() {
		if t == known {
			return true
		}
	}
	return false
}

// Meaning states the Rule Type's proposition in one line for
// human-facing surfaces; the Rule Schema carries the field-level
// contract.
func (t Type) Meaning() string {
	switch t {
	case TypeConsumes:
		return "states what a Zone may import: declared Zones by allow-list, external and standard-library imports by policy"
	case TypeStructure:
		return "requires or forbids files matching globs inside a Zone"
	case TypeNaming:
		return "constrains file names within a Zone to a finite case vocabulary"
	case TypeLayers:
		return "orders Zones highest first; a Zone may import same or lower layers, never higher"
	case TypeProtected:
		return "restricts which Zones may import one Zone"
	case TypeIndependence:
		return "forbids imports between sibling Folders selected by globs"
	case TypeAcyclic:
		return "forbids dependency cycles among declared Zones"
	case TypeDomain:
		return "evaluates one invariant of a Domain-Driven Design building block against every instance the recorded domain holds"
	case TypeContent:
		return "forbids lines matching a regular expression in the selected files"
	case TypeExtension:
		return "delegates enforcement to a named Extension through the sandboxed SDK"
	}
	return ""
}

// Accepts decides whether parameters are valid for this Rule Type.
func (t Type) Accepts(p Params) error {
	if p == nil {
		return fmt.Errorf("rule type %q: missing parameters", t)
	}
	if p.Type() != t {
		return fmt.Errorf("rule type %q: got %q parameters", t, p.Type())
	}
	return p.validate()
}

// Params is the sealed set of Rule-Type-specific parameter values.
type Params interface {
	Type() Type
	// proposition states the parameters' architectural claim in domain
	// language, without naming the Zones the Rule applies to; it seeds
	// the canonical Claim when a representation carries none.
	proposition() string
	validate() error
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

// ConsumesParams state what the Rule's Zone may import. Internal nil
// means other declared Zones are unrestricted.
type ConsumesParams struct {
	Internal *AllowList
	External ImportPolicy
	Stdlib   ImportPolicy
}

// Type returns TypeConsumes.
func (p ConsumesParams) Type() Type { return TypeConsumes }

func (p ConsumesParams) validate() error {
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

func (p ConsumesParams) proposition() string {
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

// StructureParams require or forbid member files matching globs.
type StructureParams struct {
	Require []Glob
	Forbid  []Glob
}

// Type returns TypeStructure.
func (p StructureParams) Type() Type { return TypeStructure }

func (p StructureParams) validate() error {
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

func (p StructureParams) proposition() string {
	var parts []string
	if len(p.Require) > 0 {
		parts = append(parts, fmt.Sprintf("contains files matching %s", globList(p.Require)))
	}
	if len(p.Forbid) > 0 {
		parts = append(parts, fmt.Sprintf("contains no files matching %s", globList(p.Forbid)))
	}
	return strings.Join(parts, " and ")
}

// NamingParams constrain the file-name case of the Rule's Subjects.
// Narrowing to a subset of member files is Applicability's file
// dimension, not a parameter.
type NamingParams struct {
	Case CaseSpec
}

// Type returns TypeNaming.
func (p NamingParams) Type() Type { return TypeNaming }

func (p NamingParams) validate() error {
	if p.Case.IsZero() {
		return fmt.Errorf("naming: missing case specification")
	}
	return nil
}

func (p NamingParams) proposition() string {
	return fmt.Sprintf("file names use %s", p.Case)
}

// LayersParams order Zones highest first.
type LayersParams struct {
	Layers []ZoneName
}

// Type returns TypeLayers.
func (p LayersParams) Type() Type { return TypeLayers }

func (p LayersParams) validate() error {
	if len(p.Layers) < 2 {
		return fmt.Errorf("layers: fewer than two layers")
	}
	return uniqueValidZones("layers", p.Layers)
}

func (p LayersParams) proposition() string {
	return fmt.Sprintf("Zones layer highest first as %s; a Zone never imports a higher layer", zoneList(p.Layers))
}

// ProtectedParams restrict who may import one Zone.
type ProtectedParams struct {
	Zone  ZoneName
	Allow []ZoneName
}

// Type returns TypeProtected.
func (p ProtectedParams) Type() Type { return TypeProtected }

func (p ProtectedParams) validate() error {
	if err := p.Zone.validate(); err != nil {
		return fmt.Errorf("protected: %v", err)
	}
	return uniqueValidZones("protected allow", p.Allow)
}

func (p ProtectedParams) proposition() string {
	if len(p.Allow) == 0 {
		return fmt.Sprintf("Zone %q is imported by no other Zone", p.Zone)
	}
	return fmt.Sprintf("Zone %q is imported only by %s", p.Zone, zoneList(p.Allow))
}

// ExtensionParams bind a Rule to Extension-supplied enforcement: the
// registered extension rule name and the parameters its published
// schema validates host-side before any extension code runs.
type ExtensionParams struct {
	Uses string
	With map[string]any
}

// Type returns TypeExtension.
func (p ExtensionParams) Type() Type { return TypeExtension }

func (p ExtensionParams) validate() error {
	if strings.TrimSpace(p.Uses) == "" {
		return fmt.Errorf("extension: missing the extension rule name (uses)")
	}
	return nil
}

func (p ExtensionParams) proposition() string {
	return fmt.Sprintf("satisfies extension rule %q", p.Uses)
}

// AcyclicParams scope the no-cycles proposition; an empty scope means
// every Zone the repository declares. A Pattern never distributes an
// empty scope: its loader resolves {} to the Pattern's own Zones.
type AcyclicParams struct {
	Zones []ZoneName
}

// Type returns TypeAcyclic.
func (p AcyclicParams) Type() Type { return TypeAcyclic }

func (p AcyclicParams) validate() error {
	return uniqueValidZones("acyclic", p.Zones)
}

func (p AcyclicParams) proposition() string {
	if len(p.Zones) == 0 {
		return "declared Zone dependencies contain no cycle"
	}
	return fmt.Sprintf("dependencies among %s contain no cycle", zoneList(p.Zones))
}

// IndependenceParams configures an independence Rule: sibling Folders
// selected by the globs may not import each other.
type IndependenceParams struct {
	Folders []Glob
}

// Type returns TypeIndependence.
func (p IndependenceParams) Type() Type { return TypeIndependence }

func (p IndependenceParams) validate() error {
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

func (p IndependenceParams) proposition() string {
	return fmt.Sprintf("sibling Folders matching %s may not import each other", globList(p.Folders))
}

// DomainParams name the one block invariant of the Domain-Driven
// Design meta-model a domain Rule evaluates: an invariant the domain
// evaluator enforces, by its id.
type DomainParams struct {
	Invariant string
}

// Type returns TypeDomain.
func (p DomainParams) Type() Type { return TypeDomain }

func (p DomainParams) validate() error {
	inv, ok := vocab.DDD().Invariant(p.Invariant)
	if !ok {
		return fmt.Errorf("domain: %q is not a block invariant of the %s meta-model", p.Invariant, vocab.DDD().Name)
	}
	if inv.Enforcement.By != vocab.EvaluatorDomain {
		return fmt.Errorf("domain: %s is not evaluated by the domain evaluator (enforcement: %s)", inv.ID, describeEnforcement(inv))
	}
	return nil
}

func (p DomainParams) proposition() string {
	inv, _ := vocab.DDD().Invariant(p.Invariant)
	return inv.Statement
}

// BlockInvariant returns the meta-model invariant the parameters name.
// Construction validated the name, so a miss means the value was built
// outside New.
func (p DomainParams) BlockInvariant() (vocab.BlockInvariant, error) {
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

// ContentParams configure a content Rule: no line of any selected file
// may match the forbidden regular expression (Go RE2 syntax).
type ContentParams struct {
	Forbid string
}

// Type returns TypeContent.
func (p ContentParams) Type() Type { return TypeContent }

func (p ContentParams) validate() error {
	if strings.TrimSpace(p.Forbid) == "" {
		return fmt.Errorf("content: missing the forbidden pattern (forbid)")
	}
	if _, err := regexp.Compile(p.Forbid); err != nil {
		return fmt.Errorf("content: forbid %q: %v", p.Forbid, err)
	}
	return nil
}

func (p ContentParams) proposition() string {
	return fmt.Sprintf("contains no line matching /%s/", p.Forbid)
}

// Regexp compiles the forbidden pattern. Construction validated it, so
// a compile failure here means the value was built outside New.
func (p ContentParams) Regexp() (*regexp.Regexp, error) {
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

// CaseSpec is the finite file-naming vocabulary for naming Rules: one
// or more alternatives of kebab-case, snake_case, camelCase,
// PascalCase, or regex:<pattern>, combined with "|" (any-of). A case
// applies to the file stem, extension excluded.
type CaseSpec struct {
	spec string
	alts []caseAlternative
}

type caseAlternative struct {
	label string
	re    *regexp.Regexp
}

var namedCases = map[string]*regexp.Regexp{
	"kebab-case": regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`),
	"snake_case": regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`),
	"camelCase":  regexp.MustCompile(`^[a-z][a-z0-9]*([A-Z][a-z0-9]*)*$`),
	"PascalCase": regexp.MustCompile(`^([A-Z][a-z0-9]*)+$`),
}

// NewCaseSpec validates and compiles a case specification. Unknown case
// names and uncompilable regexes are construction errors, never
// silently skipped alternatives.
func NewCaseSpec(spec string) (CaseSpec, error) {
	if strings.TrimSpace(spec) == "" {
		return CaseSpec{}, fmt.Errorf("case: empty specification")
	}
	var alts []caseAlternative
	for _, alt := range strings.Split(spec, "|") {
		alt = strings.TrimSpace(alt)
		if re, ok := namedCases[alt]; ok {
			alts = append(alts, caseAlternative{alt, re})
			continue
		}
		pat, ok := strings.CutPrefix(alt, "regex:")
		if !ok {
			return CaseSpec{}, fmt.Errorf("case %q: not a named case or regex:<pattern>", alt)
		}
		re, err := regexp.Compile("^(?:" + pat + ")$")
		if err != nil {
			return CaseSpec{}, fmt.Errorf("case %q: %v", alt, err)
		}
		alts = append(alts, caseAlternative{alt, re})
	}
	return CaseSpec{spec: spec, alts: alts}, nil
}

// Matches reports whether a file stem satisfies any alternative.
func (c CaseSpec) Matches(stem string) bool {
	for _, a := range c.alts {
		if a.re.MatchString(stem) {
			return true
		}
	}
	return false
}

// IsZero reports an unconstructed CaseSpec.
func (c CaseSpec) IsZero() bool { return c.spec == "" }

func (c CaseSpec) String() string { return c.spec }
