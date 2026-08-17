// Package rule holds the Rule aggregate and the domain values ArcLint
// uses to evaluate repository conformance. Rule is the sole aggregate
// root: one independently identifiable lint rule stating a Claim and
// defining where and how ArcLint attempts to evaluate it. Invalid
// Rules cannot be constructed, and no code outside the aggregate
// enforces its invariants.
package rule

import (
	"fmt"
	"strings"
)

// Rule is the aggregate root. Values are immutable: configuration
// methods return a new Rule with the same identity.
type Rule struct {
	id            ID
	typ           Type
	claim         Claim
	severity      Severity
	params        Params
	applicability Applicability
	enforcement   Enforcement
	suppressions  []Suppression
	disablement   *Disablement
	tests         []Test
	provenance    *PatternReference
}

// Spec is the input to validated Rule construction.
type Spec struct {
	// ID is the explicit stable identity, optionally namespace-qualified.
	ID string
	// Type is one published Rule Type.
	Type Type
	// Claim optionally states the proposition; when empty the canonical
	// Claim is derived from Type, parameters, and Applicability.
	Claim string
	// Severity defaults to error.
	Severity string
	// Params are the Type-specific parameters.
	Params Params
	// Applicability selects the Rule Subjects.
	Applicability Applicability
	// Enforcement defaults to the built-in Enforcement for the Type.
	Enforcement *Enforcement
	// Tests carry the Rule's deterministic scenarios.
	Tests []Test
	// Provenance records the distributing Pattern, when any.
	Provenance *PatternReference
}

// New constructs a valid Rule or rejects the complete Spec. Every owned
// value is validated; a Rule that reaches the caller satisfies its
// invariants.
func New(spec Spec) (Rule, error) {
	id, err := NewID(spec.ID)
	if err != nil {
		return Rule{}, err
	}
	fail := func(err error) (Rule, error) {
		return Rule{}, fmt.Errorf("rule %s: %v", id, err)
	}
	if !spec.Type.Valid() {
		return fail(fmt.Errorf("type %q: not a published ArcLint Rule Type", spec.Type))
	}
	if err := spec.Type.Accepts(spec.Params); err != nil {
		return fail(err)
	}
	severity, err := ParseSeverity(spec.Severity)
	if err != nil {
		return fail(err)
	}
	if err := validateScope(spec.Type, spec.Applicability); err != nil {
		return fail(err)
	}
	enforcement := BuiltinEnforcement(spec.Type)
	if spec.Enforcement != nil {
		enforcement = *spec.Enforcement
	}
	if enforcement.IsZero() {
		return fail(fmt.Errorf("missing enforcement"))
	}
	statement := spec.Claim
	if strings.TrimSpace(statement) == "" {
		statement = deriveClaim(spec.Applicability, spec.Params)
	}
	claim, err := NewClaim(statement)
	if err != nil {
		return fail(err)
	}
	for _, t := range spec.Tests {
		if t.IsZero() {
			return fail(fmt.Errorf("unconstructed rule test"))
		}
	}
	var provenance *PatternReference
	if spec.Provenance != nil {
		if spec.Provenance.IsZero() {
			return fail(fmt.Errorf("unconstructed pattern provenance"))
		}
		ref := *spec.Provenance
		provenance = &ref
	}
	return Rule{
		id:            id,
		typ:           spec.Type,
		claim:         claim,
		severity:      severity,
		params:        spec.Params,
		applicability: spec.Applicability,
		enforcement:   enforcement,
		tests:         append([]Test(nil), spec.Tests...),
		provenance:    provenance,
	}, nil
}

// validateScope keeps Applicability coherent with the Rule Type:
// module-scoped Types bind to at least one Module; graph Types range
// over the repository's Module graph.
func validateScope(t Type, a Applicability) error {
	switch t {
	case TypeConsumes, TypeStructure, TypeNaming:
		if len(a.Modules()) == 0 {
			return fmt.Errorf("%s rule requires module applicability", t)
		}
	case TypeLayers, TypeProtected, TypeAcyclic:
		if !a.EntireRepository() {
			return fmt.Errorf("%s rule requires repository applicability", t)
		}
	case TypeExtension:
		if a.IsZero() {
			return fmt.Errorf("extension rule requires applicability")
		}
	}
	return nil
}

// deriveClaim composes the canonical Claim from the Rule's scope and
// its parameters' proposition.
func deriveClaim(a Applicability, p Params) string {
	proposition := p.proposition()
	switch p.Type() {
	case TypeLayers, TypeProtected, TypeAcyclic:
		return proposition
	case TypeConsumes, TypeStructure, TypeNaming, TypeExtension:
		// Module-scoped Types: the Claim carries the Modules below.
	}
	modules := a.Modules()
	if len(modules) == 1 {
		return fmt.Sprintf("Module %q: %s", modules[0], proposition)
	}
	return fmt.Sprintf("Modules %s: %s", moduleList(modules), proposition)
}

// ID returns the stable Rule identity.
func (r Rule) ID() ID { return r.id }

// Type returns the Rule's published Type.
func (r Rule) Type() Type { return r.typ }

// Claim returns the architectural proposition.
func (r Rule) Claim() Claim { return r.claim }

// Severity returns the configured gate importance.
func (r Rule) Severity() Severity { return r.severity }

// Params returns the Type-specific parameters.
func (r Rule) Params() Params { return r.params }

// Applicability returns the Subject selection, Exclusions applied.
func (r Rule) Applicability() Applicability { return r.applicability }

// Enforcement describes how the Rule is evaluated.
func (r Rule) Enforcement() Enforcement { return r.enforcement }

// Suppressions returns the attached Diagnostic Suppressions.
func (r Rule) Suppressions() []Suppression {
	return append([]Suppression(nil), r.suppressions...)
}

// Tests returns the Rule's deterministic scenarios.
func (r Rule) Tests() []Test { return append([]Test(nil), r.tests...) }

// Provenance returns the distributing Pattern reference, when any.
func (r Rule) Provenance() (PatternReference, bool) {
	if r.provenance == nil {
		return PatternReference{}, false
	}
	return *r.provenance, true
}

// Disabled reports whether evaluation is prevented for the repository.
func (r Rule) Disabled() bool { return r.disablement != nil }

// Disablement returns the disabling decision, when any.
func (r Rule) Disablement() (Disablement, bool) {
	if r.disablement == nil {
		return Disablement{}, false
	}
	return *r.disablement, true
}

// AppliesToFile decides whether a File is a Rule Subject, given its
// resolved Module membership.
func (r Rule) AppliesToFile(path string, memberOf []ModuleName) bool {
	return r.applicability.SelectsFile(path, memberOf)
}

// Validate proves that the complete Rule satisfies its invariants.
// Construction guarantees this; Validate re-proves it for callers
// holding a Rule of unknown lineage.
func (r Rule) Validate() error {
	if r.id.IsZero() {
		return fmt.Errorf("rule: unconstructed (zero value)")
	}
	if err := r.typ.Accepts(r.params); err != nil {
		return fmt.Errorf("rule %s: %v", r.id, err)
	}
	if !r.severity.Valid() {
		return fmt.Errorf("rule %s: severity %q invalid", r.id, r.severity)
	}
	if r.claim.IsZero() {
		return fmt.Errorf("rule %s: missing claim", r.id)
	}
	if r.enforcement.IsZero() {
		return fmt.Errorf("rule %s: missing enforcement", r.id)
	}
	return validateScope(r.typ, r.applicability)
}

// WithSeverity produces a valid repository-specific Rule with the same
// identity and a different Severity — the configure operation for the
// one common field the Rule Schema marks configurable.
func (r Rule) WithSeverity(s Severity) (Rule, error) {
	if !s.Valid() {
		return Rule{}, fmt.Errorf("rule %s: severity %q invalid", r.id, s)
	}
	r.severity = s
	return r, nil
}

// Exclude removes the Exclusion's subjects from Applicability. The
// identity is unchanged; excluded subjects evaluate not-applicable.
func (r Rule) Exclude(e Exclusion) Rule {
	r.applicability = r.applicability.Excluding(e)
	return r
}

// Suppress retains matching Violations while changing their reporting
// and gate effect.
func (r Rule) Suppress(s Suppression) Rule {
	r.suppressions = append(append([]Suppression(nil), r.suppressions...), s)
	return r
}

// Disable prevents evaluation of this Rule for the repository while
// keeping the Rule and its provenance inspectable.
func (r Rule) Disable(d Disablement) Rule {
	r.disablement = &d
	return r
}

// SuppressionFor returns the reason of the first Suppression matching
// a Violation anchored at path.
func (r Rule) SuppressionFor(path string) (string, bool) {
	for _, s := range r.suppressions {
		if s.MatchesPath(path) {
			return s.Reason(), true
		}
	}
	return "", false
}
