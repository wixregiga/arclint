package vocab

// Specification is a named predicate experts pass around as a thing: a
// type of this Name with an Evans satisfaction method (SatisfiedBy,
// satisfiedBy, or satisfied_by). A specification is not an invariant
// and is not a flag on a value object. Specifications are keyed by
// Name within a context. Line is where the specification is written
// down in the recorded Ubiquitous Language file, 0 for a specification
// that is not written down yet.
type Specification struct {
	Name       string
	Definition string
	Line       int
}
