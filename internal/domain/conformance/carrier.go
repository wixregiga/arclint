package conformance

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// Carrier is the declaration carrying one recorded contract: the file
// and line a listing points at.
type Carrier struct {
	Path string
	Line int
}

// Carriers locates the recorded language in the observed declarations
// the way the built-in domain rules do, so a listing of the language
// and a check of it never disagree about which declaration carries a
// contract: the type spelling a term, the constructor a value object's
// invariants are enforced at, the root method enforcing an invariant
// or checking an assertion, and the satisfaction method of a
// specification. Every lookup is within one bounded context, because a
// name means one thing per context.
type Carriers struct {
	contexts map[string]contextCode
}

// NewCarriers locates every context of the recorded language in the
// observed files, narrowing a context to the Zone named for it when
// the ruleset declares one.
func NewCarriers(obs Observations, knowledge vocab.UbiquitousLanguage, zones []rule.Zone) (Carriers, error) {
	mem, err := newMembership(zones, obs)
	if err != nil {
		return Carriers{}, err
	}
	code, err := resolveDomain(func(string) bool { return false }, mem, obs, knowledge)
	if err != nil {
		return Carriers{}, err
	}
	c := Carriers{contexts: map[string]contextCode{}}
	for _, cc := range code.contexts {
		c.contexts[cc.ctx.Name] = cc
	}
	return c, nil
}

// TypeFiles lists every file declaring a candidate for the term in the
// context: one when the term is located, several when nothing tells
// which is the model's, none when nothing spells it.
func (c Carriers) TypeFiles(ctx, term string) []string {
	cc, ok := c.contexts[ctx]
	if !ok {
		return nil
	}
	candidates := cc.terms[term].candidates
	if ac, isAggregate := cc.aggregates[term]; isAggregate {
		candidates = ac.candidates
	}
	var out []string
	seen := map[string]bool{}
	for _, d := range candidates {
		if !seen[d.file.path] {
			seen[d.file.path] = true
			out = append(out, d.file.path)
		}
	}
	return out
}

// Constructor locates the door a value object's invariants are
// enforced at: the first constructor of the type spelling the term in
// its unit. A term no type spells has no constructor.
func (c Carriers) Constructor(ctx, term string) (Carrier, bool) {
	cc, ok := c.contexts[ctx]
	if !ok {
		return Carrier{}, false
	}
	loc := cc.terms[term]
	if !loc.located() {
		return Carrier{}, false
	}
	decl := loc.decl()
	doors := cc.unitIdx(decl).constructors(decl.decl.Name)
	if len(doors) == 0 {
		return Carrier{}, false
	}
	return carrierOf(doors[0]), true
}

// Invariant locates the root method enforcing an aggregate's
// invariant: ensure followed by the key, in the method case of the
// root's language.
func (c Carriers) Invariant(ctx, owner, key string) (Carrier, bool, error) {
	return c.rootMethod(ctx, owner, key, ensureKey)
}

// Assertion locates the root method checking an aggregate's
// assertion: assert followed by the key, in the method case of the
// root's language.
func (c Carriers) Assertion(ctx, owner, key string) (Carrier, bool, error) {
	return c.rootMethod(ctx, owner, key, assertKey)
}

// rootMethod finds the method a contract key names on an aggregate's
// root. A key no case can spell is an error before the prefix hides
// it, never a silent miss.
func (c Carriers) rootMethod(ctx, owner, key string, prefixed func(string) string) (Carrier, bool, error) {
	if _, err := rule.CaseTerm(key, "flatcase"); err != nil {
		return Carrier{}, false, fmt.Errorf("method name for %q: %w", key, err)
	}
	cc, ok := c.contexts[ctx]
	if !ok {
		return Carrier{}, false, nil
	}
	ac, ok := cc.aggregates[owner]
	if !ok || !ac.located() {
		return Carrier{}, false, nil
	}
	m, found, err := ac.idx.methodNamed(ac.root().decl.Name, prefixed(key))
	if err != nil || !found {
		return Carrier{}, false, err
	}
	return carrierOf(m), true, nil
}

// Satisfaction locates the satisfaction method of the specification
// the term names.
func (c Carriers) Satisfaction(ctx, term string) (Carrier, bool) {
	cc, ok := c.contexts[ctx]
	if !ok {
		return Carrier{}, false
	}
	loc := cc.terms[term]
	if !loc.located() {
		return Carrier{}, false
	}
	decl := loc.decl()
	for _, m := range cc.unitIdx(decl).methodsOn(decl.decl.Name) {
		if isSatisfaction(m.decl.Name) {
			return carrierOf(m), true
		}
	}
	return Carrier{}, false
}

// EnsureKey is the recorded key of the root method enforcing an
// invariant: ensure followed by the invariant's key.
func EnsureKey(key string) string { return ensureKey(key) }

// AssertKey is the recorded key of the root method checking an
// assertion: assert followed by the assertion's key.
func AssertKey(key string) string { return assertKey(key) }

// MethodName spells a recorded key as the method of one language.
func MethodName(key string, lang rule.Language) (string, error) {
	return methodNameFor(key, lang)
}

func carrierOf(d locDecl) Carrier {
	return Carrier{Path: d.file.path, Line: d.decl.StartLine}
}
