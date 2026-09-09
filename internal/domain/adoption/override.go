// Package adoption holds repository decisions about distributed and built-in Rules.
package adoption

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// Override is an adopting repository's decisions for one distributed or
// built-in Rule. It carries neither a Constraint nor applicability of its own.
// Its target identifies the Rule, not an independent identity for the decision.
type Override struct {
	target      rule.ID
	severity    rule.Severity
	exclusion   *rule.Exclusion
	suppression *rule.Suppression
	disablement *rule.Disablement
}

// NewOverride requires an eligible target and at least one adoption decision.
// Empty severity and nil decisions leave the corresponding Rule values alone.
func NewOverride(target rule.ID, severity rule.Severity, exclusion *rule.Exclusion, suppression *rule.Suppression, disablement *rule.Disablement) (Override, error) {
	if target.IsZero() {
		return Override{}, fmt.Errorf("override: missing target RuleID")
	}
	if target.Qualifier() == "" && !rule.BuiltInID(target) {
		return Override{}, fmt.Errorf("override: target %s is neither a distributed nor a built-in Rule", target)
	}
	if severity == "" && exclusion == nil && suppression == nil && disablement == nil {
		return Override{}, fmt.Errorf("an override changes something: severity, disable, exclude, or suppress")
	}
	if severity != "" && !severity.Valid() {
		return Override{}, fmt.Errorf("override: severity %q invalid", severity)
	}
	o := Override{target: target, severity: severity}
	if exclusion != nil {
		e, err := rule.NewExclusion(exclusion.Paths(), exclusion.Zones(), exclusion.Reason())
		if err != nil {
			return Override{}, fmt.Errorf("override: %w", err)
		}
		o.exclusion = &e
	}
	if suppression != nil {
		s, err := rule.NewSuppression(suppression.Paths(), suppression.Reason())
		if err != nil {
			return Override{}, fmt.Errorf("override: %w", err)
		}
		o.suppression = &s
	}
	if disablement != nil {
		d, err := rule.NewDisablement(disablement.Reason())
		if err != nil {
			return Override{}, fmt.Errorf("override: %w", err)
		}
		o.disablement = &d
	}
	return o, nil
}

// Target returns the identity of the Rule this Override adopts.
func (o Override) Target() rule.ID { return o.target }

// Apply returns the targeted Rule with these adoption decisions applied. The
// Rule keeps its identity, proposition, provenance, and original selectors.
func (o Override) Apply(r rule.Rule) (rule.Rule, error) {
	if o.target.IsZero() {
		return rule.Rule{}, fmt.Errorf("override: unconstructed value")
	}
	if !o.target.Equals(r.ID()) {
		return rule.Rule{}, fmt.Errorf("override: target %s does not match Rule %s", o.target, r.ID())
	}
	if _, distributed := r.Provenance(); !distributed && !r.BuiltIn() {
		return rule.Rule{}, fmt.Errorf("override: Rule %s is neither distributed nor built in", r.ID())
	}
	if err := r.Validate(); err != nil {
		return rule.Rule{}, fmt.Errorf("override: %w", err)
	}
	if o.severity != "" {
		var err error
		r, err = r.WithSeverity(o.severity)
		if err != nil {
			return rule.Rule{}, fmt.Errorf("override: %w", err)
		}
	}
	if o.exclusion != nil {
		r = r.Exclude(*o.exclusion)
	}
	if o.suppression != nil {
		r = r.Suppress(*o.suppression)
	}
	if o.disablement != nil {
		r = r.Disable(*o.disablement)
	}
	return r, nil
}
