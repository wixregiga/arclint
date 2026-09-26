package vocab

import "fmt"

// DomainService is one recorded domain service of a context: a
// stateless operation that makes a business decision no single
// aggregate or value object can make alone. It may read through
// repositories, but it never commits and never publishes; the
// application service that calls it does. Its assertions are its
// contract: the guarantees its operations give, each checked by the
// operation it constrains.
type DomainService struct {
	Name       string
	Definition string
	Assertions []Assertion
	Line       int
}

// Assertion finds one assertion of the service by key.
func (s DomainService) Assertion(key string) (Assertion, bool) {
	for _, as := range s.Assertions {
		if as.Key == key {
			return as, true
		}
	}
	return Assertion{}, false
}

// validate applies the domain service block's loader invariants and
// those of the assertions it owns.
func (s DomainService) validate(c BoundedContext) error {
	if err := requireDefinition(s.Line, "service", s.Name, s.Definition); err != nil {
		return err
	}
	where := fmt.Sprintf("context %q: service %q", c.Name, s.Name)
	keys := map[string]bool{}
	for _, as := range s.Assertions {
		if err := as.validate(where); err != nil {
			return err
		}
		if keys[as.Key] {
			return fmt.Errorf("invariant/keyed-within-owner: %s%s: assertion %q is recorded twice", at(as.Line), where, as.Key)
		}
		keys[as.Key] = true
	}
	return nil
}

func (s DomainService) clone() DomainService {
	out := s
	out.Assertions = cloneSlice(s.Assertions)
	return out
}
