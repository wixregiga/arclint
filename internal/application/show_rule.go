package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// PolicyNote is one Pattern Consumer decision attached to a Rule: the
// subjects it selects and why.
type PolicyNote struct {
	Selectors []string
	Reason    string
}

// RuleDetail is the plain result value showing one Rule completely:
// identity, claim, applicability, enforcement, attached policy, and
// the accepted configuration of its Type.
type RuleDetail struct {
	Summary          RuleSummary
	Evidence         string
	Languages        []string // empty = language-independent
	Facts            []string
	Limitations      []string
	EntireRepository bool
	Modules          []string
	Files            []string
	Exclusions       []PolicyNote
	Suppressions     []PolicyNote
	Schema           string
}

// ShowRule shows one configured Rule, its schema, provenance, and
// enforcement limitations.
type ShowRule struct {
	rules rule.Repository
}

// NewShowRule requires the Rule repository port.
func NewShowRule(rules rule.Repository) (ShowRule, error) {
	if rules == nil {
		return ShowRule{}, fmt.Errorf("show rule: missing rule repository")
	}
	return ShowRule{rules: rules}, nil
}

// Execute resolves one Rule by its qualified identity.
func (uc ShowRule) Execute(id string) (RuleDetail, error) {
	r, err := findRule(uc.rules, id)
	if err != nil {
		return RuleDetail{}, err
	}
	detail := RuleDetail{
		Summary:          summarize(r),
		Evidence:         r.Enforcement().Evidence().Describe(),
		EntireRepository: r.Applicability().EntireRepository(),
		Schema:           r.Type().Schema().Describe(),
	}
	for _, l := range r.Enforcement().Languages() {
		detail.Languages = append(detail.Languages, string(l))
	}
	for _, f := range r.Enforcement().Facts() {
		detail.Facts = append(detail.Facts, string(f))
	}
	detail.Limitations = r.Enforcement().Limitations()
	for _, m := range r.Applicability().Modules() {
		detail.Modules = append(detail.Modules, string(m))
	}
	for _, g := range r.Applicability().Files() {
		detail.Files = append(detail.Files, g.String())
	}
	for _, e := range r.Applicability().Exclusions() {
		note := PolicyNote{Reason: e.Reason()}
		for _, g := range e.Paths() {
			note.Selectors = append(note.Selectors, g.String())
		}
		for _, m := range e.Modules() {
			note.Selectors = append(note.Selectors, string(m))
		}
		detail.Exclusions = append(detail.Exclusions, note)
	}
	for _, s := range r.Suppressions() {
		note := PolicyNote{Reason: s.Reason()}
		for _, g := range s.Paths() {
			note.Selectors = append(note.Selectors, g.String())
		}
		detail.Suppressions = append(detail.Suppressions, note)
	}
	return detail, nil
}

// findRule resolves a qualified Rule identity among the configured
// Rules.
func findRule(rules rule.Repository, id string) (rule.Rule, error) {
	cfg, err := rules.ConfiguredRules()
	if err != nil {
		return rule.Rule{}, fmt.Errorf("load configured rules: %w", err)
	}
	for _, r := range cfg.Rules {
		if r.ID().Qualified() == id {
			return r, nil
		}
	}
	return rule.Rule{}, fmt.Errorf("rule %q is not configured; run rules to list the configured Rules", id)
}

// summarize is the shared Rule-to-summary projection.
func summarize(r rule.Rule) RuleSummary {
	s := RuleSummary{
		ID:        r.ID().Qualified(),
		Type:      string(r.Type()),
		Severity:  string(r.Severity()),
		Claim:     r.Claim().Statement(),
		Assurance: string(r.Enforcement().Assurance()),
	}
	if ref, ok := r.Provenance(); ok {
		s.Provenance = ref.String()
	}
	if d, ok := r.Disablement(); ok {
		s.Disabled = true
		s.DisabledReason = d.Reason()
	}
	return s
}
