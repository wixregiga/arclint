package rule

import (
	"fmt"
	"strings"
)

// Ruleset is everything one repository has configured, assembled in
// load order: the built-in Rules, every extended Pattern's Rules with its
// Zones bound, and the repository's own Zones and Rules, with every
// Override applied. It is built only through NewRuleset, which enforces
// unique-qualified-id and one-import-rule-per-zone, so a Ruleset never
// holds two Rules with one RuleID or two enabled imports Rules on one
// Zone.
type Ruleset struct {
	rules      []Rule
	zones      []Zone
	languages  []Language
	scan       Scan
	extensions []ConfiguredExtension
}

// RulesetSpec is what a repository assembled, before it becomes a
// Ruleset.
type RulesetSpec struct {
	Rules []Rule
	Zones []Zone
	// Languages are the configured language targets (rules.arclint.yaml
	// runtime) whose facts observation should produce.
	Languages []Language
	Scan      Scan
	// Extensions are the Extension sources distributed by the extended
	// Patterns, keyed by the distributing Pattern so their registered
	// names stay attributable.
	Extensions []ConfiguredExtension
}

// NewRuleset is the one door to a Ruleset.
func NewRuleset(spec RulesetSpec) (Ruleset, error) {
	if err := ensureUniqueQualifiedID(spec.Rules); err != nil {
		return Ruleset{}, err
	}
	if err := ensureOneImportRulePerZone(spec.Rules); err != nil {
		return Ruleset{}, err
	}
	return Ruleset{
		rules:      append([]Rule(nil), spec.Rules...),
		zones:      append([]Zone(nil), spec.Zones...),
		languages:  append([]Language(nil), spec.Languages...),
		scan:       spec.Scan,
		extensions: append([]ConfiguredExtension(nil), spec.Extensions...),
	}, nil
}

// Rules returns every Rule in load order.
func (s Ruleset) Rules() []Rule { return append([]Rule(nil), s.rules...) }

// Zones returns every Zone, declared and bound.
func (s Ruleset) Zones() []Zone { return append([]Zone(nil), s.zones...) }

// Languages returns the configured language targets.
func (s Ruleset) Languages() []Language { return append([]Language(nil), s.languages...) }

// Scan returns the repository's observation policy.
func (s Ruleset) Scan() Scan { return s.scan }

// Extensions returns the Extension sources the extended Patterns carry.
func (s Ruleset) Extensions() []ConfiguredExtension {
	return append([]ConfiguredExtension(nil), s.extensions...)
}

// ImportsRuleFor returns the one enabled imports Rule that constrains the
// Zone, if any.
func (s Ruleset) ImportsRuleFor(zone ZoneName) (Rule, bool) {
	for _, r := range s.rules {
		if constrainsZone(r, zone) && !r.Disabled() {
			return r, true
		}
	}
	return Rule{}, false
}

// DisabledImportsRulesFor returns, in load order, the disabled imports
// Rules that select the Zone, so what was disabled or replaced stays
// visible.
func (s Ruleset) DisabledImportsRulesFor(zone ZoneName) []Rule {
	var out []Rule
	for _, r := range s.rules {
		if constrainsZone(r, zone) && r.Disabled() {
			out = append(out, r)
		}
	}
	return out
}

// constrainsZone reports whether r is an imports Rule whose Scope selects
// the Zone after its Exclusions.
func constrainsZone(r Rule, zone ZoneName) bool {
	return r.Type() == TypeConsumes && r.Scope().SelectsZone(zone)
}

func ensureUniqueQualifiedID(rules []Rule) error {
	seen := map[string]bool{}
	for _, r := range rules {
		q := r.ID().Qualified()
		if seen[q] {
			return fmt.Errorf("ruleset: duplicate rule id %q", q)
		}
		seen[q] = true
	}
	return nil
}

func ensureOneImportRulePerZone(rules []Rule) error {
	if conflicts := importsRuleConflicts(rules); len(conflicts) > 0 {
		return ImportsRuleConflictError{Conflicts: conflicts}
	}
	return nil
}

// importsRuleConflicts finds every Zone that more than one enabled
// imports Rule selects, in the order the Zones first appear, each with
// its Rules in load order.
func importsRuleConflicts(rules []Rule) []ImportsRuleConflict {
	byZone := map[ZoneName][]Rule{}
	var order []ZoneName
	for _, r := range rules {
		if r.Type() != TypeConsumes || r.Disabled() {
			continue
		}
		for _, zone := range r.Scope().Zones() {
			if !r.Scope().SelectsZone(zone) {
				continue
			}
			if _, seen := byZone[zone]; !seen {
				order = append(order, zone)
			}
			byZone[zone] = append(byZone[zone], r)
		}
	}
	var out []ImportsRuleConflict
	for _, zone := range order {
		if len(byZone[zone]) > 1 {
			out = append(out, ImportsRuleConflict{Zone: zone, Rules: byZone[zone]})
		}
	}
	return out
}

// ImportsRuleConflict is one Zone that more than one enabled imports Rule
// constrains, with those Rules in load order.
type ImportsRuleConflict struct {
	Zone  ZoneName
	Rules []Rule
}

// RuleIDs returns the qualified RuleIDs of the conflicting Rules.
func (c ImportsRuleConflict) RuleIDs() []string {
	ids := make([]string, 0, len(c.Rules))
	for _, r := range c.Rules {
		ids = append(ids, r.ID().Qualified())
	}
	return ids
}

// Sentence states the conflict and its remedy for a Ruleset.
func (c ImportsRuleConflict) Sentence() string {
	remedy := "disable one with a reason and let the other state everything the Zone may import"
	if len(c.Rules) > 2 {
		remedy = "disable all but one with a reason and let the one left state everything the Zone may import"
	}
	return fmt.Sprintf("zone %q is constrained by %s imports Rules that are not disabled, %s; a Zone has at most one, so %s",
		c.Zone, countWord(len(c.Rules)), listRules(c.Rules, true), remedy)
}

// ImportsRuleConflictError reports every Zone of one Ruleset that breaks
// one-import-rule-per-zone.
type ImportsRuleConflictError struct {
	Conflicts []ImportsRuleConflict
}

func (e ImportsRuleConflictError) Error() string {
	sentences := make([]string, 0, len(e.Conflicts))
	for _, c := range e.Conflicts {
		sentences = append(sentences, c.Sentence())
	}
	return strings.Join(sentences, "\n")
}

// patternImportsRuleConflicts states, for a Pattern, every Zone its Rules
// constrain more than once; a Pattern carries no Overrides, so the remedy
// is to merge the Rules.
func patternImportsRuleConflicts(ref PatternReference, rules []Rule) error {
	conflicts := importsRuleConflicts(rules)
	if len(conflicts) == 0 {
		return nil
	}
	sentences := make([]string, 0, len(conflicts))
	for _, c := range conflicts {
		sentences = append(sentences, fmt.Sprintf("pattern %s: zone %q is constrained by %s imports Rules, %s; a Zone has at most one, so merge them into one Rule that states everything the Zone may import",
			ref, c.Zone, countWord(len(c.Rules)), listRules(c.Rules, false)))
	}
	return fmt.Errorf("%s", strings.Join(sentences, "\n"))
}

// listRules joins RuleIDs as "a, b and c", each followed by the Pattern
// that distributes it when asked.
func listRules(rules []Rule, provenance bool) string {
	parts := make([]string, 0, len(rules))
	for _, r := range rules {
		part := r.ID().Qualified()
		if ref, ok := r.Provenance(); ok && provenance {
			part += " (from " + ref.String() + ")"
		}
		parts = append(parts, part)
	}
	if len(parts) < 2 {
		return strings.Join(parts, "")
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

func countWord(n int) string {
	words := []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}
	if n < len(words) {
		return words[n]
	}
	return fmt.Sprint(n)
}
