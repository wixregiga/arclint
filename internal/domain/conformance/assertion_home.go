package conformance

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// CheckHome is where the guarantee of an assertion is checked, chosen
// by how its owner is declared in the code.
type CheckHome string

const (
	// CheckHomeRoot is an aggregate's assertion: the root declares
	// assert followed by the key, and the command calls it.
	CheckHomeRoot CheckHome = "root"
	// CheckHomeService is the assertion of a domain service declared as
	// a concrete type: the service declares assert followed by the key,
	// and the operation calls it.
	CheckHomeService CheckHome = "service"
	// CheckHomeModel is the assertion of a domain service declared as an
	// abstraction: the check is a function of the context spelled assert
	// followed by the key, or the invariant of the same key on the value
	// object the operation returns.
	CheckHomeModel CheckHome = "model"
	// CheckHomeUndeclared is the assertion of a domain service that no
	// single type declares yet, so no declared kind chooses its home.
	CheckHomeUndeclared CheckHome = "undeclared"
	// CheckHomeTests is the assertion of an aggregate whose root is
	// declared as an abstract class: no body calls a check, so the tests
	// its verified_by names verify it.
	CheckHomeTests CheckHome = "tests"
)

// The kinds a domain service is declared as when its check is in the
// model.
const (
	DeclaredInterface     = "interface"
	DeclaredAbstractClass = "abstract class"
)

// AssertionHome says where one recorded assertion is checked.
type AssertionHome struct {
	Home CheckHome
	// Declared is the kind the owner is declared as when Home is
	// CheckHomeModel or CheckHomeTests: DeclaredInterface or
	// DeclaredAbstractClass.
	Declared string
	// Check spells assert followed by the key: in the owner's language
	// when its type is located, otherwise in each language the ruleset
	// names, the language beside each name when there are several, and
	// empty when the ruleset names none.
	Check string
	// Operation is the operation as the owner declares it, or as
	// recorded when the owner does not declare it.
	Operation string
}

// AssertionHome says where the assertion recorded under the key on an
// aggregate or a domain service of the context is checked; false when
// the context records no such assertion. languages are the ruleset's,
// spelling the check of an owner no type declares yet.
func (c Carriers) AssertionHome(ctx, owner, key string, languages []rule.Language) (AssertionHome, bool, error) {
	cc, ok := c.contexts[ctx]
	if !ok {
		return AssertionHome{}, false, nil
	}
	as, aggregate, recorded := cc.recordedAssertion(owner, key)
	if !recorded {
		return AssertionHome{}, false, nil
	}
	spelled, err := spellingsIn(assertKey(key), languages)
	if err != nil {
		return AssertionHome{}, false, err
	}
	home := AssertionHome{Check: spelled, Operation: as.On}
	owned, idx, located := cc.assertionOwner(owner, aggregate)
	switch {
	case aggregate && located && bodiless(owned.decl) != "":
		home.Home, home.Declared = CheckHomeTests, DeclaredAbstractClass
		if owned.decl.Kind == declKindInterface {
			home.Declared = DeclaredInterface
		}
	case aggregate:
		home.Home = CheckHomeRoot
	case !located:
		home.Home = CheckHomeUndeclared
	case owned.decl.Kind == declKindInterface:
		home.Home, home.Declared = CheckHomeModel, DeclaredInterface
	case bodiless(owned.decl) != "":
		home.Home, home.Declared = CheckHomeModel, DeclaredAbstractClass
	default:
		home.Home = CheckHomeService
	}
	if !located {
		return home, true, nil
	}
	check, err := methodNameFor(assertKey(key), owned.file.lang)
	if err != nil {
		return AssertionHome{}, false, err
	}
	home.Check = check
	op, found, err := idx.methodNamed(owned.decl.Name, as.On)
	if err != nil {
		return AssertionHome{}, false, err
	}
	if found {
		home.Operation = op.decl.Name
	}
	return home, true, nil
}

// assertionOwner locates the type owning an assertion, the aggregate's
// root or the domain service, with the facts of its unit; false when
// no single type declares it.
func (cc contextCode) assertionOwner(owner string, aggregate bool) (locDecl, contractIndex, bool) {
	if aggregate {
		ac := cc.aggregates[owner]
		if !ac.located() {
			return locDecl{}, contractIndex{}, false
		}
		return ac.root(), ac.idx, true
	}
	loc := cc.terms[owner]
	if !loc.located() {
		return locDecl{}, contractIndex{}, false
	}
	return loc.decl(), cc.unitIdx(loc.decl()), true
}

// recordedAssertion finds the assertion recorded under the key on the
// aggregate or domain service named owner, and whether the owner is an
// aggregate.
func (cc contextCode) recordedAssertion(owner, key string) (vocab.Assertion, bool, bool) {
	for _, agg := range cc.ctx.Aggregates {
		if agg.Name != owner {
			continue
		}
		for _, as := range agg.Assertions {
			if as.Key == key {
				return as, true, true
			}
		}
	}
	for _, s := range cc.ctx.Services {
		if s.Name != owner {
			continue
		}
		for _, as := range s.Assertions {
			if as.Key == key {
				return as, false, true
			}
		}
	}
	return vocab.Assertion{}, false, false
}

// spellingsIn spells a key as the method of each language: the name
// alone for one language, each name with its language for several, and
// nothing for none.
func spellingsIn(key string, languages []rule.Language) (string, error) {
	if len(languages) == 0 {
		return "", nil
	}
	if len(languages) == 1 {
		return methodNameFor(key, languages[0])
	}
	parts := make([]string, 0, len(languages))
	for _, lang := range languages {
		name, err := methodNameFor(key, lang)
		if err != nil {
			return "", err
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", name, lang))
	}
	return strings.Join(parts, ", "), nil
}
