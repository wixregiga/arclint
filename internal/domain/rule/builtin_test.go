package rule_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

func TestBuiltInRulesComposeOnePerCheckLevelInvariant(t *testing.T) {
	rules, err := rule.BuiltIn()
	if err != nil {
		t.Fatalf("BuiltIn: %v", err)
	}
	byID := map[string]rule.Rule{}
	for _, r := range rules {
		byID[r.ID().Qualified()] = r
	}
	checkLevel := 0
	for _, inv := range vocab.DDD().Invariants() {
		if inv.Enforcement.By != vocab.EvaluatorDomain {
			if _, ok := byID[inv.ID]; ok {
				t.Errorf("%s is evaluated by %s and must not become a built-in Rule", inv.ID, inv.Enforcement.By)
			}
			continue
		}
		checkLevel++
		r, ok := byID[inv.ID]
		if !ok {
			t.Errorf("%s has no built-in Rule", inv.ID)
			continue
		}
		if !r.BuiltIn() {
			t.Errorf("%s: BuiltIn() = false", inv.ID)
		}
		if r.ID().Qualifier() != "" {
			t.Errorf("%s: built-in id is qualified as %q", inv.ID, r.ID().Qualifier())
		}
		if got := string(r.Severity()); got != inv.Enforcement.Severity {
			t.Errorf("%s: severity %s, want the invariant's %s", inv.ID, got, inv.Enforcement.Severity)
		}
		if r.Claim().Statement() != inv.Statement {
			t.Errorf("%s: claim %q, want the invariant's statement", inv.ID, r.Claim().Statement())
		}
		if !r.Applicability().EntireRepository() {
			t.Errorf("%s: applies to %v, want the whole repository", inv.ID, r.Applicability().Zones())
		}
		p, ok := r.Params().(rule.DomainParams)
		if r.Type() != rule.TypeDomain || !ok || p.Invariant != inv.ID {
			t.Errorf("%s: type %s params %#v", inv.ID, r.Type(), r.Params())
		}
		if got, err := p.BlockInvariant(); err != nil || got.ID != inv.ID {
			t.Errorf("%s: BlockInvariant = %v, %v", inv.ID, got.ID, err)
		}
	}
	if len(rules) != checkLevel {
		t.Errorf("BuiltIn composed %d rules for %d check-level invariants", len(rules), checkLevel)
	}
}

func TestBuiltInEnforcementFollowsTheInvariantsFacts(t *testing.T) {
	rules, err := rule.BuiltIn()
	if err != nil {
		t.Fatalf("BuiltIn: %v", err)
	}
	for _, r := range rules {
		if r.Type() != rule.TypeDomain {
			continue
		}
		inv, err := r.Params().(rule.DomainParams).BlockInvariant()
		if err != nil {
			t.Fatalf("%s: %v", r.ID(), err)
		}
		e := r.Enforcement()
		if !e.CanEvaluate() {
			t.Errorf("%s: not evaluable", r.ID())
		}
		if e.Assurance() != rule.AssuranceExact {
			t.Errorf("%s: assurance %s", r.ID(), e.Assurance())
		}
		facts := e.Facts()
		if len(inv.Enforcement.Facts) == 0 {
			if len(facts) != 1 || facts[0] != rule.FactFileTree || len(e.Languages()) != 0 {
				t.Errorf("%s reads no fact and must enforce over the file tree in every language; got facts %v languages %v", r.ID(), facts, e.Languages())
			}
			continue
		}
		if len(facts) != len(inv.Enforcement.Facts) {
			t.Errorf("%s: facts %v, want %v", r.ID(), facts, inv.Enforcement.Facts)
		}
		languageBound := false
		for _, f := range facts {
			languageBound = languageBound || f != rule.FactFileTree
		}
		if languageBound && len(e.Languages()) != len(rule.Languages()) {
			t.Errorf("%s: languages %v, want every language the adapters emit facts for", r.ID(), e.Languages())
		}
		if !languageBound && len(e.Languages()) != 0 {
			t.Errorf("%s: a file-tree invariant is language-independent; got %v", r.ID(), e.Languages())
		}
		limitations := strings.Join(e.Limitations(), "\n")
		callsRead := false
		for _, f := range facts {
			callsRead = callsRead || f == rule.FactCalls
		}
		if callsRead != strings.Contains(limitations, "matched by callee name") {
			t.Errorf("%s: limitations %q do not follow the calls fact", r.ID(), limitations)
		}
	}
}

func TestBuiltInIDsAreReserved(t *testing.T) {
	repo := mustRepoApplicability(t)
	cases := []struct {
		name string
		spec rule.Spec
	}{
		{"an authored rule under a built-in id", rule.Spec{
			ID:            "aggregate/root-declared",
			Type:          rule.TypeContent,
			Params:        rule.ContentParams{Forbid: "x"},
			Applicability: repo,
		}},
		{"a domain rule under a foreign id", rule.Spec{
			ID:            "local/root-declared",
			Type:          rule.TypeDomain,
			Params:        rule.DomainParams{Invariant: "aggregate/root-declared"},
			Applicability: repo,
		}},
		{"a domain rule evaluating another invariant", rule.Spec{
			ID:            "aggregate/root-declared",
			Type:          rule.TypeDomain,
			Params:        rule.DomainParams{Invariant: "repository/declared"},
			Applicability: repo,
		}},
		{"a domain rule for a loader-level invariant", rule.Spec{
			ID:            "aggregate/identity-recorded",
			Type:          rule.TypeDomain,
			Params:        rule.DomainParams{Invariant: "aggregate/identity-recorded"},
			Applicability: repo,
		}},
		{"an acyclic rule under a built-in id", rule.Spec{
			ID:            "aggregate/root-declared",
			Type:          rule.TypeAcyclic,
			Params:        rule.AcyclicParams{},
			Applicability: repo,
		}},
	}
	for _, c := range cases {
		if _, err := rule.New(c.spec); err == nil {
			t.Errorf("%s: accepted", c.name)
		}
	}
	local, err := rule.New(rule.Spec{
		ID:            "dependencies/acyclic",
		Type:          rule.TypeAcyclic,
		Params:        rule.AcyclicParams{},
		Applicability: repo,
	})
	if err != nil {
		t.Fatalf("a local acyclic rule under its own id: %v", err)
	}
	if local.BuiltIn() {
		t.Errorf("a local rule reports itself built in")
	}
}

func TestContextSubject(t *testing.T) {
	s, err := rule.ContextSubject("ordering")
	if err != nil {
		t.Fatalf("ContextSubject: %v", err)
	}
	if s.Kind() != rule.SubjectContext || s.Identity() != "ordering" || s.IsPath() || s.String() != "context:ordering" {
		t.Errorf("subject = %s kind %s path %v", s, s.Kind(), s.IsPath())
	}
	for _, bad := range []string{"", "Ordering", "order ing", "1st"} {
		if _, err := rule.ContextSubject(bad); err == nil {
			t.Errorf("ContextSubject(%q): accepted", bad)
		}
	}
	file, err := rule.FileSubject("a/b.go")
	if err != nil {
		t.Fatalf("FileSubject: %v", err)
	}
	if !file.IsPath() {
		t.Errorf("a file subject is a path")
	}
}
