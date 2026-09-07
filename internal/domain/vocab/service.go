package vocab

// DomainService is one recorded domain service of a context: a
// standalone operation for a process or transformation that is not the
// natural responsibility of an aggregate or a value object.
type DomainService struct {
	Name       string
	Definition string
	Line       int
}

// validate applies the domain service block's loader invariants.
func (s DomainService) validate() error {
	return requireDefinition(s.Line, "service", s.Name, s.Definition)
}
