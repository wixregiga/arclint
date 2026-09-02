package vocab

// Assertion is a post-condition of a named operation, owned by exactly
// one term inside a bounded context. Unlike an Invariant, an Assertion
// holds when that operation occurs, not at all times. Assertions are
// keyed by Statement within a context. ID is unique within the
// context and names the method that checks the post-condition. On is
// the operation that must call that method. Line is where the
// assertion is written down in the recorded Ubiquitous Language file,
// 0 for an assertion that is not written down yet.
type Assertion struct {
	Statement string
	Owner     string
	ID        string
	On        string
	Line      int
}
