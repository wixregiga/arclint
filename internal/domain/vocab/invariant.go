package vocab

// Invariant is a must-always/must-never rule owned by exactly one term
// inside a bounded context. Invariants are keyed by Statement within a
// context. Two recorded shapes share this section:
//
//   - Value integrity: Owner is a value object, ID is empty, and the
//     constructor of that type is the join.
//   - Cluster: Owner is an aggregate, ID names the method on that
//     owner, and the method is called from the constructor and every
//     exported command.
//
// business_rule classifications resolve to an Invariant or an
// Assertion, never to their own section. Line is where the invariant
// is written down in the recorded Ubiquitous Language file, 0 for an
// invariant that is not written down yet.
type Invariant struct {
	Statement string
	Owner     string
	// ID names a cluster invariant, unique within the bounded context.
	// Required when Owner is an aggregate that is recorded as a named
	// cluster contract; forbidden when Owner is a value object.
	ID   string
	Line int
}
