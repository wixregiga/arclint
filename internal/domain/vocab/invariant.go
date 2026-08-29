package vocab

// Invariant is a must-always/must-never rule owned by exactly one term
// inside a bounded context. Invariants are keyed by Statement within a
// context. business_rule and assertion classifications always resolve
// to an Invariant entry (statement + owner). Line is where the
// invariant is written down in the recorded Ubiquitous Language file,
// 0 for an invariant that is not written down yet.
type Invariant struct {
	Statement string
	Owner     string
	Line      int
}
