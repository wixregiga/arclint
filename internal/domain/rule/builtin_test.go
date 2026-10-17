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
		if r.Proposition() != inv.Statement {
			t.Errorf("%s: proposition %q, want the invariant's statement", inv.ID, r.Proposition())
		}
		if !r.Scope().EntireRepository() {
			t.Errorf("%s: applies to %v, want the whole repository", inv.ID, r.Scope().Zones())
		}
		p, ok := r.Params().(rule.DomainParams)
		if r.Type() != rule.TypeDomain || !ok || p.Invariant != inv.ID {
			t.Errorf("%s: type %s params %#v", inv.ID, r.Type(), r.Params())
		}
		if got, err := p.BlockInvariant(); err != nil || got.ID != inv.ID {
			t.Errorf("%s: BlockInvariant = %v, %v", inv.ID, got.ID, err)
		}
	}
	if len(rules) != checkLevel+1 {
		t.Errorf("BuiltIn composed %d rules for %d check-level invariants and the word limit", len(rules), checkLevel)
	}
}

func TestBuiltInWordLimitRuleIsTheVocabularysWordLimits(t *testing.T) {
	rules, err := rule.BuiltIn()
	if err != nil {
		t.Fatalf("BuiltIn: %v", err)
	}
	var found []rule.Rule
	for _, r := range rules {
		if r.Type() == rule.ConstraintKindWordLimit {
			found = append(found, r)
		}
	}
	if len(found) != 1 {
		t.Fatalf("BuiltIn composed %d word-limit rules, want 1", len(found))
	}
	r := found[0]
	if r.ID().Qualified() != rule.WordLimitRuleID || r.ID().Qualifier() != "" {
		t.Errorf("id %s, want the unqualified %s", r.ID(), rule.WordLimitRuleID)
	}
	if r.Severity() != rule.SeverityError {
		t.Errorf("severity %s, want error: over-limit text fails the gate", r.Severity())
	}
	if !r.Scope().EntireRepository() {
		t.Errorf("applies to %v, want the whole repository", r.Scope().Zones())
	}
	if r.Proposition() != vocab.WordLimitsStatement() {
		t.Errorf("proposition %q, want the vocabulary's statement of its limits", r.Proposition())
	}
	if !r.BuiltIn() || !rule.BuiltInID(r.ID()) {
		t.Error("the word-limit rule is built in and its id reserved")
	}
	if source, ok := r.BuiltInSource(); !ok || source != rule.SourceWordLimits {
		t.Errorf("BuiltInSource = %q, %v; want %q", source, ok, rule.SourceWordLimits)
	}
	if _, inMetaModel := vocab.DDD().Invariant(rule.WordLimitRuleID); inMetaModel {
		t.Error("the word limit is arclint's own, never a block invariant of the meta-model")
	}
}

func TestWordLimitIsCarriedUnderItsOwnIDAlone(t *testing.T) {
	scope, err := rule.RepositoryScope()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rule.New(rule.Spec{ID: "local/limits", Severity: "error", Scope: scope, Constraint: rule.WordLimitConstraint{}}); err == nil ||
		!strings.Contains(err.Error(), rule.WordLimitRuleID) {
		t.Errorf("a word-limit constraint under another id must be refused naming %s; err = %v", rule.WordLimitRuleID, err)
	}
	if _, err := rule.New(rule.Spec{ID: rule.WordLimitRuleID, Severity: "error", Scope: scope, Constraint: rule.ContentConstraint{Forbid: "x"}}); err == nil {
		t.Error("another constraint under the word limit's id must be refused")
	}
	if _, err := rule.New(rule.Spec{ID: rule.WordLimitRuleID, Severity: "warning", Scope: scope, Constraint: rule.WordLimitConstraint{}}); err != nil {
		t.Errorf("an override may change the word limit's severity: %v", err)
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
		wantLanguages := len(inv.Enforcement.Languages.Names)
		if inv.Enforcement.Languages.All {
			wantLanguages = len(rule.Languages())
		}
		if languageBound && len(e.Languages()) != wantLanguages {
			t.Errorf("%s: languages %v, want the languages the invariant records", r.ID(), e.Languages())
		}
		if !languageBound && len(e.Languages()) != 0 {
			t.Errorf("%s: a file-tree invariant is language-independent; got %v", r.ID(), e.Languages())
		}
		limitations := strings.Join(e.Limitations(), "\n")
		callsRead, callsResolved, fieldsResolved := false, false, false
		for _, f := range facts {
			callsRead = callsRead || f == rule.FactCalls
			callsResolved = callsResolved || f == rule.FactResolvedCalls
			fieldsResolved = fieldsResolved || f == rule.FactFieldTypes
		}
		if (callsRead && !callsResolved) != strings.Contains(limitations, "matched by callee name") {
			t.Errorf("%s: limitations %q do not follow the calls fact", r.ID(), limitations)
		}
		if callsResolved != strings.Contains(limitations, "calls are resolved to declarations") {
			t.Errorf("%s: limitations %q do not follow the resolved calls fact", r.ID(), limitations)
		}
		if fieldsResolved != strings.Contains(limitations, "field types are resolved") {
			t.Errorf("%s: limitations %q do not follow the field types fact", r.ID(), limitations)
		}
	}
}

func TestBuiltInIDsAreReserved(t *testing.T) {
	repo := mustRepoScope(t)
	cases := []struct {
		name string
		spec rule.Spec
	}{
		{"an authored rule under a built-in id", rule.Spec{
			ID:     "aggregate/root-declared",
			Type:   rule.TypeContent,
			Params: rule.ContentParams{Forbid: "x"},
			Scope:  repo,
		}},
		{"a domain rule under a foreign id", rule.Spec{
			ID:     "local/root-declared",
			Type:   rule.TypeDomain,
			Params: rule.DomainParams{Invariant: "aggregate/root-declared"},
			Scope:  repo,
		}},
		{"a domain rule evaluating another invariant", rule.Spec{
			ID:     "aggregate/root-declared",
			Type:   rule.TypeDomain,
			Params: rule.DomainParams{Invariant: "repository/declared"},
			Scope:  repo,
		}},
		{"a domain rule for a loader-level invariant", rule.Spec{
			ID:     "aggregate/identity-recorded",
			Type:   rule.TypeDomain,
			Params: rule.DomainParams{Invariant: "aggregate/identity-recorded"},
			Scope:  repo,
		}},
		{"an acyclic rule under a built-in id", rule.Spec{
			ID:     "aggregate/root-declared",
			Type:   rule.TypeAcyclic,
			Params: rule.AcyclicParams{},
			Scope:  repo,
		}},
	}
	for _, c := range cases {
		if _, err := rule.New(c.spec); err == nil {
			t.Errorf("%s: accepted", c.name)
		}
	}
	local, err := rule.New(rule.Spec{
		ID:     "dependencies/acyclic",
		Type:   rule.TypeAcyclic,
		Params: rule.AcyclicParams{},
		Scope:  repo,
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

func TestBuiltInForComposesWhatTheRecordedLanguageCallsFor(t *testing.T) {
	ids := func(lang vocab.UbiquitousLanguage) (int, bool) {
		t.Helper()
		rules, err := rule.BuiltInFor(lang)
		if err != nil {
			t.Fatalf("BuiltInFor: %v", err)
		}
		wordLimit := false
		for _, r := range rules {
			if r.ID().Qualified() == rule.WordLimitRuleID {
				wordLimit = true
			}
		}
		return len(rules), wordLimit
	}
	all, err := rule.BuiltIn()
	if err != nil {
		t.Fatal(err)
	}
	if n, wordLimit := ids(vocab.UbiquitousLanguage{Project: "shop"}); n != 0 || wordLimit {
		t.Errorf("a language that records nothing composes %d rules", n)
	}
	described, err := vocab.NewUbiquitousLanguage("shop", "A shop's catalog and orders.", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n, wordLimit := ids(described); n != 1 || !wordLimit {
		t.Errorf("a language holding only its description composes %d rules, word limit %v; want the word limit alone", n, wordLimit)
	}
	recorded, err := vocab.NewUbiquitousLanguage("shop", "", []vocab.BoundedContext{{Name: "ordering", Definition: "Taking orders."}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n, wordLimit := ids(recorded); n != len(all) || !wordLimit {
		t.Errorf("a recorded context composes %d rules, want every built-in Rule (%d)", n, len(all))
	}
}
