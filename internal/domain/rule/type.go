package rule

import (
	"fmt"
	"regexp"
	"strings"
)

// Type is one value from the finite ArcLint-owned set of supported Rule
// shapes: consumes, structure, naming, layers, protected, independence,
// acyclic, invariants, content, and extension. Pattern and Extension
// authors configure existing values; they do not add new ones — custom
// logic plugs into the extension kind through the SDK, it never grows
// this enum. In rules.yaml a Type is never spelled: the one Assertion
// key a Rule carries decides it (see AssertionKey).
type Type string

const (
	// TypeConsumes states what a Module may import: other declared
	// Modules by allow-list, external and standard-library imports by
	// policy.
	TypeConsumes Type = "consumes"
	// TypeStructure requires or forbids files matching globs inside a
	// Module.
	TypeStructure Type = "structure"
	// TypeNaming constrains file names within a Module to a finite case
	// vocabulary.
	TypeNaming Type = "naming"
	// TypeLayers orders Modules highest first; a Module may import same
	// or lower layers, never higher.
	TypeLayers Type = "layers"
	// TypeProtected restricts which Modules may import one Module.
	TypeProtected Type = "protected"
	// TypeIndependence forbids imports between sibling Folders.
	TypeIndependence Type = "independence"
	// TypeAcyclic forbids dependency cycles among declared Modules.
	TypeAcyclic Type = "acyclic"
	// TypeInvariants requires recorded domain contracts to be visible
	// in source as named methods called from their join points.
	TypeInvariants Type = "invariants"
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
		TypeLayers, TypeProtected, TypeIndependence, TypeAcyclic, TypeInvariants,
		TypeContent, TypeExtension,
	}
}

// assertionKeys maps each Type to the one rules.yaml key that spells
// its Assertion. The key is the Type's whole public spelling: a Rule
// written with that key is a Rule of that Type, and no rule carries
// two keys.
var assertionKeys = map[Type]string{
	TypeConsumes:     "imports",
	TypeStructure:    "structure",
	TypeNaming:       "naming",
	TypeLayers:       "layers",
	TypeProtected:    "imported_by",
	TypeIndependence: "independent",
	TypeAcyclic:      "acyclic",
	TypeInvariants:   "invariants",
	TypeContent:      "content",
	TypeExtension:    "uses",
}

// AssertionKey returns the rules.yaml key that spells this Type's
// Assertion.
func (t Type) AssertionKey() string { return assertionKeys[t] }

// AssertionKeys returns every Assertion key in published Type order.
func AssertionKeys() []string {
	out := make([]string, 0, len(assertionKeys))
	for _, t := range Types() {
		out = append(out, assertionKeys[t])
	}
	return out
}

// TypeOfAssertionKey resolves a rules.yaml Assertion key to its Type.
func TypeOfAssertionKey(key string) (Type, bool) {
	for _, t := range Types() {
		if assertionKeys[t] == key {
			return t, true
		}
	}
	return "", false
}

// Scope is the Applicability shape a Type demands: which Modules a
// Rule of the Type judges and how rules.yaml spells that.
type Scope int

const (
	// ScopeModules judges the members of the Modules named under on;
	// on is required.
	ScopeModules Scope = iota
	// ScopeOneModule judges exactly one Module named under on.
	ScopeOneModule
	// ScopeRepository ranges over the repository's Module graph or
	// its files; on is not accepted.
	ScopeRepository
	// ScopeModulesOrRepository judges the Modules named under on, or
	// the whole repository when on is omitted.
	ScopeModulesOrRepository
)

// Scope returns the Applicability shape the Type demands.
func (t Type) Scope() Scope {
	switch t {
	case TypeConsumes, TypeStructure, TypeNaming, TypeInvariants:
		return ScopeModules
	case TypeProtected:
		return ScopeOneModule
	case TypeLayers, TypeIndependence, TypeAcyclic:
		return ScopeRepository
	case TypeContent, TypeExtension:
		return ScopeModulesOrRepository
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
		return "states what a Module may import: declared Modules by allow-list, external and standard-library imports by policy"
	case TypeStructure:
		return "requires or forbids files matching globs inside a Module"
	case TypeNaming:
		return "constrains file names within a Module to a finite case vocabulary"
	case TypeLayers:
		return "orders Modules highest first; a Module may import same or lower layers, never higher"
	case TypeProtected:
		return "restricts which Modules may import one Module"
	case TypeIndependence:
		return "forbids imports between sibling Folders selected by globs"
	case TypeAcyclic:
		return "forbids dependency cycles among declared Modules"
	case TypeInvariants:
		return "requires recorded domain contracts to be visible in source as named methods called from their join points"
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
	// language, without naming the Modules the Rule applies to; it seeds
	// the canonical Claim when a representation carries none.
	proposition() string
	validate() error
}

// AllowList is a declared Module allow-list. The empty list is
// meaningful: it permits no other declared Module. The owning Module
// itself is always permitted implicitly.
type AllowList struct {
	modules []ModuleName
}

// NewAllowList validates the listed Module names and rejects
// duplicates.
func NewAllowList(modules ...ModuleName) (AllowList, error) {
	seen := map[ModuleName]bool{}
	out := make([]ModuleName, 0, len(modules))
	for _, m := range modules {
		if err := m.validate(); err != nil {
			return AllowList{}, err
		}
		if seen[m] {
			return AllowList{}, fmt.Errorf("allow-list: duplicate module %q", m)
		}
		seen[m] = true
		out = append(out, m)
	}
	return AllowList{modules: out}, nil
}

// Modules returns the allowed Module names.
func (l AllowList) Modules() []ModuleName {
	return append([]ModuleName(nil), l.modules...)
}

// Permits reports whether the named Module is on the list.
func (l AllowList) Permits(m ModuleName) bool {
	for _, a := range l.modules {
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

// ConsumesParams state what the Rule's Module may import. Internal nil
// means other declared Modules are unrestricted.
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
		if len(p.Internal.modules) == 0 {
			parts = append(parts, "imports no other declared Module")
		} else {
			parts = append(parts, fmt.Sprintf("imports only the declared Modules %s", moduleList(p.Internal.modules)))
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

// LayersParams order Modules highest first.
type LayersParams struct {
	Layers []ModuleName
}

// Type returns TypeLayers.
func (p LayersParams) Type() Type { return TypeLayers }

func (p LayersParams) validate() error {
	if len(p.Layers) < 2 {
		return fmt.Errorf("layers: fewer than two layers")
	}
	return uniqueValidModules("layers", p.Layers)
}

func (p LayersParams) proposition() string {
	return fmt.Sprintf("Modules layer highest first as %s; a Module never imports a higher layer", moduleList(p.Layers))
}

// ProtectedParams restrict who may import one Module.
type ProtectedParams struct {
	Module ModuleName
	Allow  []ModuleName
}

// Type returns TypeProtected.
func (p ProtectedParams) Type() Type { return TypeProtected }

func (p ProtectedParams) validate() error {
	if err := p.Module.validate(); err != nil {
		return fmt.Errorf("protected: %v", err)
	}
	return uniqueValidModules("protected allow", p.Allow)
}

func (p ProtectedParams) proposition() string {
	if len(p.Allow) == 0 {
		return fmt.Sprintf("Module %q is imported by no other Module", p.Module)
	}
	return fmt.Sprintf("Module %q is imported only by %s", p.Module, moduleList(p.Allow))
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
// every declared Module.
type AcyclicParams struct {
	Modules []ModuleName
}

// Type returns TypeAcyclic.
func (p AcyclicParams) Type() Type { return TypeAcyclic }

func (p AcyclicParams) validate() error {
	return uniqueValidModules("acyclic", p.Modules)
}

func (p AcyclicParams) proposition() string {
	if len(p.Modules) == 0 {
		return "declared Module dependencies contain no cycle"
	}
	return fmt.Sprintf("dependencies among %s contain no cycle", moduleList(p.Modules))
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

// InvariantsParams configure an invariants Rule. Closed, default false,
// additionally requires every exported error-returning function in the
// owner's files to call the cluster method; child constructors that
// return errors are then extra failures.
type InvariantsParams struct {
	Closed bool
}

// Type returns TypeInvariants.
func (p InvariantsParams) Type() Type { return TypeInvariants }

func (p InvariantsParams) validate() error { return nil }

func (p InvariantsParams) proposition() string {
	if p.Closed {
		return "recorded domain contracts are visible in source as named methods called from their join points (closed)"
	}
	return "recorded domain contracts are visible in source as named methods called from their join points"
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

func uniqueValidModules(what string, modules []ModuleName) error {
	seen := map[ModuleName]bool{}
	for _, m := range modules {
		if err := m.validate(); err != nil {
			return fmt.Errorf("%s: %v", what, err)
		}
		if seen[m] {
			return fmt.Errorf("%s: duplicate module %q", what, m)
		}
		seen[m] = true
	}
	return nil
}

func moduleList(modules []ModuleName) string {
	names := make([]string, len(modules))
	for i, m := range modules {
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
