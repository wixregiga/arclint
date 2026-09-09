package rule

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// BuiltIn composes the Rules arclint applies to a recorded domain with
// nothing written in rules.arclint.yaml: one Rule per block invariant
// of the Domain-Driven Design meta-model that `arclint check`
// evaluates, under the invariant's own id, at the invariant's severity,
// over the whole repository. A domain evaluator invariant becomes a
// domain Rule. They exist the moment a domain file is recorded; a
// repository adopts them through Overrides under their ids and never
// redefines them, and no ruleset may spell a Rule under one of those
// ids.
func BuiltIn() ([]Rule, error) {
	var out []Rule
	for _, inv := range vocab.DDD().Invariants() {
		spec, ok := builtInSpec(inv)
		if !ok {
			continue
		}
		r, err := New(spec)
		if err != nil {
			return nil, fmt.Errorf("built-in rules: %v", err)
		}
		out = append(out, r)
	}
	return out, nil
}

// builtInSpec is the Spec of the built-in Rule for a check-level block
// invariant, and false for an invariant no built-in Rule evaluates.
func builtInSpec(inv vocab.BlockInvariant) (Spec, bool) {
	applicability, err := RepositoryApplicability()
	if err != nil {
		return Spec{}, false
	}
	spec := Spec{
		ID:            inv.ID,
		Claim:         inv.Statement,
		Severity:      inv.Enforcement.Severity,
		Applicability: applicability,
	}
	if inv.Enforcement.By != vocab.EvaluatorDomain {
		return Spec{}, false
	}
	spec.Constraint = DomainConstraint{Invariant: inv.ID}
	return spec, true
}

// builtInInvariant returns the check-level block invariant an
// unqualified id spells, when it spells one. Such an id is reserved:
// only the built-in Rule carries it.
func builtInInvariant(id ID) (vocab.BlockInvariant, bool) {
	if id.Qualifier() != "" {
		return vocab.BlockInvariant{}, false
	}
	inv, ok := vocab.DDD().Invariant(id.Local())
	if !ok {
		return vocab.BlockInvariant{}, false
	}
	// The loader judges its invariants while the domain file is read and
	// a planned one has no evaluator yet; neither composes a Rule.
	if inv.Enforcement.By != vocab.EvaluatorDomain {
		return vocab.BlockInvariant{}, false
	}
	return inv, true
}

// BuiltInID reports whether an id is reserved for a built-in Rule: a
// ruleset may adopt such a Rule with an Override once a domain is
// recorded and may never spell a Rule of its own under it.
func BuiltInID(id ID) bool {
	_, ok := builtInInvariant(id)
	return ok
}

// respectsBuiltIn keeps the built-in ids and the domain Type together:
// a Spec under a built-in id is exactly the built-in Rule, and a domain
// Rule carries the id of the invariant it evaluates.
func respectsBuiltIn(id ID, c Constraint) error {
	inv, reserved := builtInInvariant(id)
	if !reserved {
		if c.Kind() == TypeDomain {
			p := c.(DomainParams)
			return fmt.Errorf("a domain rule is built in under the id of the invariant it evaluates, %s", p.Invariant)
		}
		return nil
	}
	want, _ := builtInSpec(inv)
	if c.Kind() != want.Constraint.Kind() {
		return fmt.Errorf("the id is built in; arclint composes it from the recorded domain, and a ruleset adopts it with an override (severity, disable, exclude, suppress)")
	}
	if p, ok := c.(DomainParams); ok && p.Invariant != inv.ID {
		return fmt.Errorf("evaluates %s under the id of %s", p.Invariant, inv.ID)
	}
	return nil
}

// builtInEnforcement derives a built-in Rule's Enforcement from the
// invariant's own: the facts it reads and the languages that emit
// them. An invariant reading path facts alone, or none, is
// language-independent.
func builtInEnforcement(inv vocab.BlockInvariant) (Enforcement, error) {
	facts := make([]Fact, 0, len(inv.Enforcement.Facts))
	for _, name := range inv.Enforcement.Facts {
		switch f := Fact(name); f {
		case FactFileTree, FactImports, FactDeclarations, FactCalls:
			facts = append(facts, f)
		default:
			return Enforcement{}, fmt.Errorf("%s: reads unknown fact %q", inv.ID, name)
		}
	}
	if len(facts) == 0 {
		facts = []Fact{FactFileTree}
	}
	var languages []Language
	if languageBound(facts) {
		if inv.Enforcement.Languages.All {
			languages = Languages()
		}
		for _, name := range inv.Enforcement.Languages.Names {
			l := Language(name)
			if !l.Valid() {
				return Enforcement{}, fmt.Errorf("%s: names unknown language %q", inv.ID, name)
			}
			languages = append(languages, l)
		}
	}
	var limitations []string
	if hasFact(facts, FactCalls) {
		limitations = append(limitations,
			"calls are matched by callee name and never resolved to a declaration",
			"a command is an exported method of the root whose result carries an error; a TypeScript or Python command that throws is not seen")
	}
	return NewEnforcement(languages, facts, builtInEvidence(facts), AssuranceExact, limitations, true)
}

// languageBound reports whether any fact beyond the file tree is read,
// which only a language adapter emits.
func languageBound(facts []Fact) bool {
	for _, f := range facts {
		if f != FactFileTree {
			return true
		}
	}
	return false
}

func hasFact(facts []Fact, want Fact) bool {
	for _, f := range facts {
		if f == want {
			return true
		}
	}
	return false
}

// builtInEvidence names the evidence a built-in Rule's facts yield.
func builtInEvidence(facts []Fact) EvidenceMethod {
	var parts []string
	if hasFact(facts, FactImports) {
		parts = append(parts, "static import classification")
	}
	switch {
	case hasFact(facts, FactCalls):
		parts = append(parts, "declaration and call matching")
	case hasFact(facts, FactDeclarations):
		parts = append(parts, "declaration matching")
	}
	if len(parts) == 0 {
		parts = append(parts, "repository file tree matching")
	}
	return EvidenceMethod(strings.Join(parts, " and ") + " against the recorded domain")
}
