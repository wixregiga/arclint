package vocab

import (
	"fmt"
	"strings"
)

// Assertion is one guarantee of an operation: a command of an
// aggregate's root, or an operation of a domain service. The key names
// the method that checks it, On names the operation it constrains, and
// Statement is the guarantee as the domain expert states it, never
// rewritten into a post-condition. The owner is the aggregate or domain
// service the assertion is recorded under.
type Assertion struct {
	Key       string
	On        string
	Statement string
	Line      int
}

// OwnedAssertion is an Assertion with the aggregate or domain service
// it is recorded under; OwnerConcept says which.
type OwnedAssertion struct {
	Context      string
	Owner        string
	OwnerConcept Concept
	Assertion    Assertion
}

// validate applies assertion/names-owner-operation-and-check: the key,
// the operation, and the statement are all present.
func (a Assertion) validate(where string) error {
	if err := requireKey(a.Line, where+": assertion", a.Key); err != nil {
		return err
	}
	if strings.TrimSpace(a.On) == "" {
		return fmt.Errorf("assertion/names-owner-operation-and-check: %s%s: assertion %q names no operation; record under on the command or service operation it constrains",
			at(a.Line), where, a.Key)
	}
	if strings.TrimSpace(a.Statement) == "" {
		return fmt.Errorf("assertion/names-owner-operation-and-check: %s%s: assertion %q has an empty statement", at(a.Line), where, a.Key)
	}
	return nil
}
