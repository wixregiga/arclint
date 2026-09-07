package vocab

import "fmt"

// ValueObject is one recorded value object of a context: a term with
// no identity, equal to any other with the same values, with the
// invariants every value of its kind satisfies. An identity an
// aggregate or a member names is a value object of the context without
// an entry here; a ValueObject entry exists when there is more to say.
type ValueObject struct {
	Name       string
	Definition string
	Aliases    []string
	Invariants []Invariant
	Line       int
}

// Invariant finds one invariant of the value object by key.
func (v ValueObject) Invariant(key string) (Invariant, bool) {
	for _, inv := range v.Invariants {
		if inv.Key == key {
			return inv, true
		}
	}
	return Invariant{}, false
}

// validate applies the value object block's loader invariants and those
// of the invariants it owns.
func (v ValueObject) validate(c BoundedContext) error {
	where := fmt.Sprintf("context %q: value object %q", c.Name, v.Name)
	if err := requireDefinition(v.Line, "value object", v.Name, v.Definition); err != nil {
		return err
	}
	if err := validateAliases(v.Line, where, v.Aliases); err != nil {
		return err
	}
	return validateInvariants(where, v.Invariants)
}

func (v ValueObject) clone() ValueObject {
	out := v
	out.Aliases = cloneStrings(v.Aliases)
	out.Invariants = cloneSlice(v.Invariants)
	return out
}
