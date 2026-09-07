package vocab

// Specification is one recorded specification of a context: a named
// predicate experts pass around as a thing, answered in the code by a
// satisfaction method.
type Specification struct {
	Name       string
	Definition string
	Line       int
}

// validate applies the specification block's loader invariants.
func (s Specification) validate() error {
	return requireDefinition(s.Line, "specification", s.Name, s.Definition)
}
