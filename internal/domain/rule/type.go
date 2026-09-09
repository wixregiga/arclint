package rule

import (
	"fmt"
)

// Type is the legacy discriminator. Constraint owns domain behavior.
type Type string

// ConstraintKind is a backward-compatible alias for Type.
type ConstraintKind = Type

const (
	// ConstraintKindConsumes states what a Zone may import: other declared
	// Zones by allow-list, external and standard-library imports by
	// policy.
	ConstraintKindConsumes ConstraintKind = "consumes"
	// ConstraintKindStructure requires or forbids files matching globs inside a
	// Zone.
	ConstraintKindStructure ConstraintKind = "structure"
	// ConstraintKindNaming constrains file names within a Zone to a finite case
	// vocabulary.
	ConstraintKindNaming ConstraintKind = "naming"
	// ConstraintKindLayers orders Zones highest first; a Zone may import same
	// or lower layers, never higher.
	ConstraintKindLayers ConstraintKind = "layers"
	// ConstraintKindProtected restricts which Zones may import one Zone.
	ConstraintKindProtected ConstraintKind = "protected"
	// ConstraintKindIndependence forbids imports between sibling Folders.
	ConstraintKindIndependence ConstraintKind = "independence"
	// ConstraintKindAcyclic forbids dependency cycles among declared Zones.
	ConstraintKindAcyclic ConstraintKind = "acyclic"
	// ConstraintKindDomain evaluates one invariant of a Domain-Driven Design
	// building block against every instance the recorded domain holds.
	// Its Rules are built in: arclint composes one per check-level block
	// invariant under the invariant's own id the moment a domain is
	// recorded (see BuiltIn), and a ruleset adopts them through
	// Overrides only.
	ConstraintKindDomain ConstraintKind = "domain"
	// ConstraintKindContent forbids lines matching a regular expression in the
	// Rule's Subjects: the built-in evaluator over file bytes.
	ConstraintKindContent ConstraintKind = "content"
	// ConstraintKindExtension delegates enforcement to a named Extension through
	// the sandboxed SDK; parameters are validated host-side against the
	// extension's published schema before any extension code runs.
	ConstraintKindExtension ConstraintKind = "extension"
)

// Backward-compatibility aliases for legacy Type* identifiers.
const (
	// TypeConsumes is an alias for ConstraintKindConsumes.
	TypeConsumes = ConstraintKindConsumes
	// TypeStructure is an alias for ConstraintKindStructure.
	TypeStructure = ConstraintKindStructure
	// TypeNaming is an alias for ConstraintKindNaming.
	TypeNaming = ConstraintKindNaming
	// TypeLayers is an alias for ConstraintKindLayers.
	TypeLayers = ConstraintKindLayers
	// TypeProtected is an alias for ConstraintKindProtected.
	TypeProtected = ConstraintKindProtected
	// TypeIndependence is an alias for ConstraintKindIndependence.
	TypeIndependence = ConstraintKindIndependence
	// TypeAcyclic is an alias for ConstraintKindAcyclic.
	TypeAcyclic = ConstraintKindAcyclic
	// TypeDomain is an alias for ConstraintKindDomain.
	TypeDomain = ConstraintKindDomain
	// TypeContent is an alias for ConstraintKindContent.
	TypeContent = ConstraintKindContent
	// TypeExtension is an alias for ConstraintKindExtension.
	TypeExtension = ConstraintKindExtension
)

// ConstraintKinds returns the published enum in stable order.
func ConstraintKinds() []ConstraintKind {
	return []ConstraintKind{
		ConstraintKindConsumes, ConstraintKindStructure, ConstraintKindNaming,
		ConstraintKindLayers, ConstraintKindProtected, ConstraintKindIndependence, ConstraintKindAcyclic, ConstraintKindDomain,
		ConstraintKindContent, ConstraintKindExtension,
	}
}

// Types returns the published enum in stable order.
func Types() []Type {
	return ConstraintKinds()
}

// Params is the legacy name for a complete Constraint.
type Params = Constraint

// ConsumesParams is the legacy name for ConsumesConstraint.
type ConsumesParams = ConsumesConstraint

// StructureParams is the legacy name for StructureConstraint.
type StructureParams = StructureConstraint

// NamingParams is the legacy name for NamingConstraint.
type NamingParams = NamingConstraint

// LayersParams is the legacy name for LayersConstraint.
type LayersParams = LayersConstraint

// ProtectedParams is the legacy name for ProtectedConstraint.
type ProtectedParams = ProtectedConstraint

// IndependenceParams is the legacy name for IndependenceConstraint.
type IndependenceParams = IndependenceConstraint

// AcyclicParams is the legacy name for AcyclicConstraint.
type AcyclicParams = AcyclicConstraint

// DomainParams is the legacy name for DomainConstraint.
type DomainParams = DomainConstraint

// ContentParams is the legacy name for ContentConstraint.
type ContentParams = ContentConstraint

// ExtensionParams is the legacy name for ExtensionConstraint.
type ExtensionParams = ExtensionConstraint

// constraintForms supplies metadata to legacy callers without holding rule state.
var constraintForms = map[Type]Constraint{
	TypeConsumes:     ConsumesConstraint{},
	TypeStructure:    StructureConstraint{},
	TypeNaming:       NamingConstraint{},
	TypeLayers:       LayersConstraint{},
	TypeProtected:    ProtectedConstraint{},
	TypeIndependence: IndependenceConstraint{},
	TypeAcyclic:      AcyclicConstraint{},
	TypeDomain:       DomainConstraint{},
	TypeContent:      ContentConstraint{},
	TypeExtension:    ExtensionConstraint{},
}

var typeMeanings = map[Type]string{
	TypeConsumes:     "states what a Zone may import: declared Zones by allow-list, external and standard-library imports by policy",
	TypeStructure:    "requires or forbids files matching globs inside a Zone",
	TypeNaming:       "constrains file names within a Zone to a finite case vocabulary",
	TypeLayers:       "orders Zones highest first; a Zone may import same or lower layers, never higher",
	TypeProtected:    "restricts which Zones may import one Zone",
	TypeIndependence: "forbids imports between sibling Folders selected by globs",
	TypeAcyclic:      "forbids dependency cycles among declared Zones",
	TypeDomain:       "evaluates one invariant of a Domain-Driven Design building block against every instance the recorded domain holds",
	TypeContent:      "forbids lines matching a regular expression in the selected files",
	TypeExtension:    "delegates enforcement to a named Extension through the sandboxed SDK",
}

// ConstraintKey returns the authored key of the concrete constraint form.
func (t Type) ConstraintKey() string {
	if c, ok := constraintForms[t]; ok {
		return c.Key()
	}
	return ""
}

// AssertionKey preserves the legacy spelling.
func (t Type) AssertionKey() string { return t.ConstraintKey() }

// Authored reports whether a ruleset can spell this constraint.
func (t Type) Authored() bool { return t.ConstraintKey() != "" }

// AuthoredConstraintKinds returns authored discriminators in published order.
func AuthoredConstraintKinds() []ConstraintKind {
	var out []ConstraintKind
	for _, t := range Types() {
		if t.Authored() {
			out = append(out, t)
		}
	}
	return out
}

// AuthoredTypes preserves the legacy spelling.
func AuthoredTypes() []Type { return AuthoredConstraintKinds() }

// ConstraintKeys returns authored keys in published order.
func ConstraintKeys() []string {
	out := make([]string, 0, len(constraintForms)-1)
	for _, t := range AuthoredTypes() {
		out = append(out, t.ConstraintKey())
	}
	return out
}

// AssertionKeys preserves the legacy spelling.
func AssertionKeys() []string { return ConstraintKeys() }

// KindOfConstraintKey resolves an authored key.
func KindOfConstraintKey(key string) (ConstraintKind, bool) {
	for _, t := range AuthoredTypes() {
		if t.ConstraintKey() == key {
			return t, true
		}
	}
	return "", false
}

// TypeOfAssertionKey preserves the legacy spelling.
func TypeOfAssertionKey(key string) (Type, bool) { return KindOfConstraintKey(key) }

// Scope delegates to the concrete constraint form.
func (t Type) Scope() Scope {
	if c, ok := constraintForms[t]; ok {
		return c.Scope()
	}
	return ScopeRepository
}

// AcceptsFiles delegates to the concrete constraint form.
func (t Type) AcceptsFiles() bool {
	if c, ok := constraintForms[t]; ok {
		return c.AcceptsFiles()
	}
	return false
}

// Meaning preserves the legacy unconfigured help text.
func (t Type) Meaning() string { return typeMeanings[t] }

// ParseConstraintKind accepts only a published discriminator.
func ParseConstraintKind(s string) (ConstraintKind, error) {
	if t := Type(s); t.Valid() {
		return t, nil
	}
	return "", fmt.Errorf("constraint kind %q: not a published ArcLint Constraint Kind %v", s, ConstraintKinds())
}

// ParseType preserves the legacy spelling.
func ParseType(s string) (Type, error) { return ParseConstraintKind(s) }

// Valid reports whether the discriminator has a published constraint form.
func (t Type) Valid() bool { _, ok := constraintForms[t]; return ok }

// Accepts validates legacy paired construction inputs at the compatibility boundary.
func (t Type) Accepts(p Params) error {
	if p == nil {
		return fmt.Errorf("rule type %q: missing parameters", t)
	}
	if p.Kind() != t {
		return fmt.Errorf("rule type %q: got %q parameters", t, p.Kind())
	}
	if err := p.Validate(); err != nil {
		return fmt.Errorf("constraint: %w", err)
	}
	return nil
}

// constraintValue translates legacy construction inputs into one Constraint.
func (s Spec) constraintValue() (Constraint, error) {
	if s.Constraint != nil {
		if s.Type != "" || s.Params != nil {
			return nil, fmt.Errorf("constraint: cannot combine with legacy type or params")
		}
		if !s.Constraint.Kind().Valid() {
			return nil, fmt.Errorf("constraint kind %q: not published", s.Constraint.Kind())
		}
		return s.Constraint, nil
	}
	if !s.Type.Valid() {
		return nil, fmt.Errorf("type %q: not a published ArcLint Rule Type", s.Type)
	}
	if s.Params == nil {
		return nil, fmt.Errorf("rule type %q: missing parameters", s.Type)
	}
	if s.Params.Kind() != s.Type {
		return nil, fmt.Errorf("rule type %q: got %q parameters", s.Type, s.Params.Kind())
	}
	return s.Params, nil
}
