package vocab

import (
	"fmt"
	"strings"
)

// Assertion is one post-condition of an operation of an aggregate's
// root: the key names the root method that checks it, On names the
// operation whose completion it constrains, and Statement is the
// post-condition in the ubiquitous language. The owner is the aggregate
// the assertion is recorded under.
type Assertion struct {
	Key       string
	On        string
	Statement string
	Line      int
}

// OwnedAssertion is an Assertion with the aggregate it is recorded
// under.
type OwnedAssertion struct {
	Context   string
	Owner     string
	Assertion Assertion
}

// validate applies assertion/names-owner-operation-and-check: the key,
// the operation, and the statement are all present.
func (a Assertion) validate(where string) error {
	if err := requireKey(a.Line, where+": assertion", a.Key); err != nil {
		return err
	}
	if strings.TrimSpace(a.On) == "" {
		return fmt.Errorf("assertion/names-owner-operation-and-check: %s%s: assertion %q names no operation; record the root operation it constrains under on",
			at(a.Line), where, a.Key)
	}
	if strings.TrimSpace(a.Statement) == "" {
		return fmt.Errorf("assertion/names-owner-operation-and-check: %s%s: assertion %q has an empty statement", at(a.Line), where, a.Key)
	}
	return nil
}
