package vocab

import "fmt"

// Aggregate is one cluster of entities and value objects with a single
// root and a boundary; the entry is its root, spelled with the
// aggregate's name and carrying the aggregate's identity. It owns its
// member entities, its invariants, and the assertions on its root's
// operations; the owner is the nesting, never a reference. Repository
// and Factory name the aggregate's repository and factory types when
// it records them.
type Aggregate struct {
	Name       string
	Definition string
	Identity   string
	Aliases    []string
	Entities   []Entity
	Invariants []Invariant
	Assertions []Assertion
	Repository string
	Factory    string
	Line       int
}

// Entity is one member entity of an aggregate other than the root; its
// identity, when it names one, is local, distinct only inside the
// aggregate.
type Entity struct {
	Name       string
	Definition string
	Identity   string
	Aliases    []string
	Line       int
}

// Entity finds one member by name.
func (a Aggregate) Entity(name string) (Entity, bool) {
	for _, e := range a.Entities {
		if e.Name == name {
			return e, true
		}
	}
	return Entity{}, false
}

// Invariant finds one invariant of the aggregate by key.
func (a Aggregate) Invariant(key string) (Invariant, bool) {
	for _, inv := range a.Invariants {
		if inv.Key == key {
			return inv, true
		}
	}
	return Invariant{}, false
}

// Assertion finds one assertion of the aggregate by key.
func (a Aggregate) Assertion(key string) (Assertion, bool) {
	for _, as := range a.Assertions {
		if as.Key == key {
			return as, true
		}
	}
	return Assertion{}, false
}

// validate applies the aggregate block's loader invariants and those of
// the invariants, assertions, and entities it owns.
func (a Aggregate) validate(c BoundedContext) error {
	where := fmt.Sprintf("context %q: aggregate %q", c.Name, a.Name)
	if err := requireDefinition(a.Line, "aggregate", a.Name, a.Definition); err != nil {
		return err
	}
	if a.Identity == "" {
		return fmt.Errorf("aggregate/identity-recorded: %s%s names no identity; record the value object that identifies its root", at(a.Line), where)
	}
	if err := identityIsValueObject(c, a.Line, where, a.Identity); err != nil {
		return err
	}
	if err := validateAliases(a.Line, where, a.Aliases); err != nil {
		return err
	}
	for _, e := range a.Entities {
		if err := requireDefinition(e.Line, "entity", e.Name, e.Definition); err != nil {
			return err
		}
		if e.Identity != "" {
			if err := identityIsValueObject(c, e.Line, fmt.Sprintf("%s: entity %q", where, e.Name), e.Identity); err != nil {
				return err
			}
		}
		if err := validateAliases(e.Line, fmt.Sprintf("%s: entity %q", where, e.Name), e.Aliases); err != nil {
			return err
		}
	}
	if err := validateInvariants(where, a.Invariants); err != nil {
		return err
	}
	keys := map[string]bool{}
	for _, inv := range a.Invariants {
		keys[inv.Key] = true
	}
	for _, as := range a.Assertions {
		if err := as.validate(where); err != nil {
			return err
		}
		if keys[as.Key] {
			return fmt.Errorf("invariant/keyed-within-owner: %s%s: %q is both an invariant and an assertion", at(as.Line), where, as.Key)
		}
		keys[as.Key] = true
	}
	return nil
}

// identityIsValueObject applies aggregate/identity-recorded: an
// identity is a value object of the same context, recorded there or
// implied by the naming.
func identityIsValueObject(c BoundedContext, line int, where, identity string) error {
	if concept, recorded := c.recordedAs(identity); recorded && concept != ConceptValueObject {
		return fmt.Errorf("aggregate/identity-recorded: %s%s: identity %q is recorded as %s; an identity is a value object",
			at(line), where, identity, concept.Doc().Title)
	}
	return nil
}

func validateAliases(line int, where string, aliases []string) error {
	seen := map[string]bool{}
	for _, alias := range aliases {
		if alias == "" {
			return fmt.Errorf("%s%s: alias is empty", at(line), where)
		}
		if seen[alias] {
			return fmt.Errorf("%s%s: alias %q is listed twice", at(line), where, alias)
		}
		seen[alias] = true
	}
	return nil
}

func (a Aggregate) clone() Aggregate {
	out := a
	out.Aliases = cloneStrings(a.Aliases)
	out.Entities = nil
	for _, e := range a.Entities {
		e.Aliases = cloneStrings(e.Aliases)
		out.Entities = append(out.Entities, e)
	}
	out.Invariants = cloneSlice(a.Invariants)
	out.Assertions = cloneSlice(a.Assertions)
	return out
}
