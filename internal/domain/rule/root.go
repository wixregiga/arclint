// Package rule holds the Rule aggregate and the domain values ArcLint
// uses to evaluate repository conformance. Rule is the sole aggregate
// root: one independently identifiable lint rule carrying a Constraint,
// optional Rationale, and where and how ArcLint evaluates it. Invalid
// Rules cannot be constructed, and no code outside the aggregate
// enforces its invariants.
package rule

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// Rule is the aggregate root. Values are immutable: configuration
// methods return a new Rule with the same identity.
type Rule struct {
	id            ID
	rationale     Rationale
	severity      Severity
	constraint    Constraint
	applicability Applicability
	enforcement   Enforcement
	suppressions  []Suppression
	disablement   *Disablement
	tests         []Test
	provenance    *PatternReference
	expansion     *Expansion
}

// Spec is the input to validated Rule construction.
type Spec struct {
	// Constraint is the complete checkable proposition.
	// Supply it alone; Type and Params remain legacy construction inputs.
	Constraint Constraint
	// ID is the explicit stable identity, qualified by namespace/name
	// when a Pattern distributes the Rule.
	ID string
	// Type is one published Rule Type.
	Type Type
	// Rationale optionally explains why the Constraint is needed.
	Rationale string
	// Claim accepts the legacy authored description. Supply either it or Rationale.
	// Deprecated: use Rationale.
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
	// Expansion, on a structure Rule, records that Params derive from
	// a recorded vocabulary collection; Params must then hold exactly
	// the Expansion's resolution against the recorded language.
	Expansion *Expansion
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
	c, err := spec.constraintValue()
	if err != nil {
		return fail(err)
	}
	if spec.Expansion != nil {
		if spec.Expansion.IsZero() {
			return fail(fmt.Errorf("unconstructed expansion"))
		}
		if c.Kind() != TypeStructure {
			return fail(fmt.Errorf("expansion: only structure rules expand over the recorded vocabulary"))
		}
		if _, ok := c.(StructureConstraint); !ok {
			return fail(fmt.Errorf("expansion: structure rule with %T params", c))
		}
	}
	if err := validateConstraint(c, spec.Expansion); err != nil {
		return fail(err)
	}
	if err := respectsBuiltIn(id, c); err != nil {
		return fail(err)
	}
	severity, err := ParseSeverity(spec.Severity)
	if err != nil {
		return fail(err)
	}
	if err := validateScope(c, spec.Applicability); err != nil {
		return fail(err)
	}
	enforcement := BuiltinEnforcement(c.Kind())
	if inv, ok := builtInInvariant(id); ok {
		enforcement, err = builtInEnforcement(inv)
		if err != nil {
			return fail(err)
		}
	}
	if spec.Enforcement != nil {
		enforcement = *spec.Enforcement
	}
	if enforcement.IsZero() {
		return fail(fmt.Errorf("missing enforcement"))
	}
	if spec.Rationale != "" && spec.Claim != "" {
		return fail(fmt.Errorf("rationale: cannot combine with legacy claim"))
	}
	explanation := spec.Rationale
	if explanation == "" {
		explanation = strings.TrimSpace(spec.Claim)
	}
	var rationale Rationale
	if explanation != "" {
		rationale, err = NewRationale(explanation)
		if err != nil {
			return fail(err)
		}
	}
	for _, t := range spec.Tests {
		if t.IsZero() {
			return fail(fmt.Errorf("unconstructed rule test"))
		}
		if t.RuleID() != id.Qualified() {
			return fail(fmt.Errorf("rule test %q identifies %q, not this rule", t.Name(), t.RuleID()))
		}
	}
	var provenance *PatternReference
	if spec.Provenance != nil {
		if spec.Provenance.IsZero() {
			return fail(fmt.Errorf("unconstructed pattern provenance"))
		}
		ref := *spec.Provenance
		if id.Qualifier() != ref.Qualifier() {
			return fail(fmt.Errorf("provenance %s does not qualify rule id %s", ref, id))
		}
		provenance = &ref
	}
	var expansion *Expansion
	if spec.Expansion != nil {
		e := *spec.Expansion
		expansion = &e
	}
	return Rule{
		id:            id,
		rationale:     rationale,
		severity:      severity,
		constraint:    c,
		applicability: spec.Applicability,
		enforcement:   enforcement,
		tests:         append([]Test(nil), spec.Tests...),
		provenance:    provenance,
		expansion:     expansion,
	}, nil
}

// validateConstraint preserves the one expansion allowance: an
// expanded structure Rule over an empty recorded collection holds
// empty parameters: it exists and asserts nothing yet, which its
// generated proposition states.
func validateConstraint(p Constraint, e *Expansion) error {
	if p == nil {
		return fmt.Errorf("constraint: missing")
	}
	if e != nil {
		sp, ok := p.(StructureConstraint)
		if ok && len(sp.Require)+len(sp.Forbid) == 0 {
			return nil
		}
	}
	if err := p.Validate(); err != nil {
		return fmt.Errorf("constraint: %w", err)
	}
	return nil
}

// deriveExpandedProposition states the universally quantified proposition of an
// expanded structure Rule: the source it ranges over and the globs the
// recorded language currently derives.
func deriveExpandedProposition(a Applicability, p StructureConstraint, e Expansion) string {
	zones := a.Zones()
	scope := fmt.Sprintf("Zones %s", zoneList(zones))
	if len(zones) == 1 {
		scope = fmt.Sprintf("Zone %q", zones[0])
	}
	if len(p.Require)+len(p.Forbid) == 0 {
		return fmt.Sprintf("%s: derives structure obligations from each recorded %s; none recorded yet", scope, e.Source())
	}
	return fmt.Sprintf("%s: %s (derived from each recorded %s)", scope, p.Proposition(), e.Source())
}

// validateScope keeps Applicability coherent with the Rule Type:
// zone-scoped Types bind to at least one Zone; graph Types range
// over the repository's Zone graph.
func validateScope(c Constraint, a Applicability) error {
	t := c.Kind()
	switch c.Scope() {
	case ScopeZones:
		if len(a.Zones()) == 0 {
			return fmt.Errorf("%s rule requires zone applicability", t)
		}
	case ScopeRepository, ScopeOneZone:
		// ProtectedConstraint carries its target Zone; its evaluation spans importers.
		if !a.EntireRepository() {
			return fmt.Errorf("%s rule requires repository applicability", t)
		}
	case ScopeZonesOrRepository:
		if a.IsZero() {
			return fmt.Errorf("%s rule requires applicability", t)
		}
	}
	return nil
}

// deriveProposition composes the canonical proposition from the Rule's scope and
// its parameters' proposition.
func deriveProposition(a Applicability, p Constraint) string {
	proposition := p.Proposition()
	switch p.Kind() {
	case TypeLayers, TypeProtected, TypeIndependence, TypeAcyclic, TypeDomain:
		return proposition
	case TypeConsumes, TypeStructure, TypeNaming, TypeContent, TypeExtension:
		// Zone-scoped Types: the proposition carries the Zones below.
	}
	zones := a.Zones()
	switch len(zones) {
	case 0:
		// Only content and extension Rules reach here without Zones:
		// repository applicability, so the proposition stands alone.
		return proposition
	case 1:
		return fmt.Sprintf("Zone %q: %s", zones[0], proposition)
	}
	return fmt.Sprintf("Zones %s: %s", zoneList(zones), proposition)
}

// ID returns the stable Rule identity.
func (r Rule) ID() ID { return r.id }

// Type returns the Rule's published Type.
func (r Rule) Type() Type {
	if r.constraint == nil {
		return ""
	}
	return r.constraint.Kind()
}

// Constraint returns the checkable proposition the Rule carries.
func (r Rule) Constraint() Constraint {
	return r.constraint
}

// Rationale returns the author's explanation, or the zero value when absent.
func (r Rule) Rationale() Rationale { return r.rationale }

// Claim preserves the legacy display text: the authored description when
// supplied, otherwise the generated proposition. It is not stored by Rule.
// Deprecated: use Rationale and Proposition separately.
func (r Rule) Claim() Claim {
	statement := r.rationale.String()
	if statement == "" {
		statement = r.Proposition()
	}
	return Claim{statement: statement}
}

// Proposition states the configured Constraint in its applicability and
// expansion context. It never supplies or replaces an author's Rationale.
func (r Rule) Proposition() string {
	if r.constraint == nil {
		return ""
	}
	if r.expansion != nil {
		if sp, ok := r.constraint.(StructureConstraint); ok {
			return deriveExpandedProposition(r.applicability, sp, *r.expansion)
		}
	}
	return deriveProposition(r.applicability, r.constraint)
}

// Assertion preserves the legacy Rule accessor; meta-model assertions are separate.
// Deprecated: use Proposition.
func (r Rule) Assertion() string { return r.Proposition() }

// Severity returns the configured gate importance.
func (r Rule) Severity() Severity { return r.severity }

// Params returns the Type-specific parameters.
func (r Rule) Params() Params { return r.constraint }

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

// BuiltIn reports whether arclint composes this Rule from the
// Domain-Driven Design meta-model rather than a ruleset spelling it:
// its id is a check-level block invariant (see BuiltIn).
func (r Rule) BuiltIn() bool {
	_, ok := builtInInvariant(r.id)
	return ok
}

// Provenance returns the distributing Pattern reference, when any.
func (r Rule) Provenance() (PatternReference, bool) {
	if r.provenance == nil {
		return PatternReference{}, false
	}
	return *r.provenance, true
}

// Expansion returns the vocabulary derivation of an expanded structure
// Rule, when any.
func (r Rule) Expansion() (Expansion, bool) {
	if r.expansion == nil {
		return Expansion{}, false
	}
	return *r.expansion, true
}

// Reexpand re-derives an expanded Rule's Constraint against another
// recorded language, keeping the authored Rationale, identity, and every other
// value. The Rule Test runner uses this to evaluate the Rule against a
// fixture's own vocabulary; a Rule without an Expansion is returned
// unchanged.
func (r Rule) Reexpand(lang vocab.UbiquitousLanguage) (Rule, error) {
	if r.expansion == nil {
		return r, nil
	}
	params, err := r.expansion.Resolve(lang)
	if err != nil {
		return Rule{}, fmt.Errorf("rule %s: %v", r.id, err)
	}
	r.constraint = params
	return r, nil
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
// resolved Zone membership.
func (r Rule) AppliesToFile(path string, memberOf []ZoneName) bool {
	return r.applicability.SelectsFile(path, memberOf)
}

// ReferencedZones returns every ZoneName the Rule speaks about:
// the Zones it applies to plus any its parameters name (an import
// allow-list, a layer order, a protected Zone and its allowed
// importers, an acyclic Zone set). Each name appears once, in first
// mention order. A Rule is well-formed only when every name here is a
// declared Zone.
func (r Rule) ReferencedZones() []ZoneName {
	var out []ZoneName
	seen := map[ZoneName]bool{}
	add := func(names ...ZoneName) {
		for _, n := range names {
			if seen[n] {
				continue
			}
			seen[n] = true
			out = append(out, n)
		}
	}
	add(r.applicability.Zones()...)
	switch p := r.constraint.(type) {
	case ConsumesConstraint:
		if p.Internal != nil {
			add(p.Internal.Zones()...)
		}
	case LayersConstraint:
		add(p.Layers...)
	case ProtectedConstraint:
		add(p.Zone)
		add(p.Allow...)
	case AcyclicConstraint:
		add(p.Zones...)
	}
	return out
}

// Validate proves that the complete Rule satisfies its invariants.
// Construction guarantees this; Validate re-proves it for callers
// holding a Rule of unknown lineage.
func (r Rule) Validate() error {
	if r.id.IsZero() {
		return fmt.Errorf("rule: unconstructed (zero value)")
	}
	if err := validateConstraint(r.constraint, r.expansion); err != nil {
		return fmt.Errorf("rule %s: %v", r.id, err)
	}
	if !r.severity.Valid() {
		return fmt.Errorf("rule %s: severity %q invalid", r.id, r.severity)
	}
	if r.enforcement.IsZero() {
		return fmt.Errorf("rule %s: missing enforcement", r.id)
	}
	return validateScope(r.constraint, r.applicability)
}

// WithSeverity produces a valid repository-specific Rule with the same
// identity and a different Severity, the configure operation for the
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
