package vocab

import (
	"fmt"
	"strings"
)

// Assertion is one guarantee of an operation: a command of an
// aggregate's root, or an operation of a domain service. The key names
// the method that checks it, On names the operation it constrains, and
// Statement is the guarantee as the domain expert states it, never
// rewritten into a post-condition. VerifiedBy lists the tests that
// verify the guarantee, each spelled as a VerifyingTest is. The owner is
// the aggregate or domain service the assertion is recorded under.
type Assertion struct {
	Key        string
	On         string
	Statement  string
	VerifiedBy []string
	Line       int
}

// OwnedAssertion is an Assertion with the aggregate or domain service
// it is recorded under; OwnerConcept says which.
type OwnedAssertion struct {
	Context      string
	Owner        string
	OwnerConcept Concept
	Assertion    Assertion
}

// cloneAssertions copies assertions with the tests each names, so an
// edit of the copy never reaches the original.
func cloneAssertions(in []Assertion) []Assertion {
	out := cloneSlice(in)
	for i := range out {
		out[i].VerifiedBy = cloneStrings(out[i].VerifiedBy)
	}
	return out
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
	return requireVerifyingTests("assertion/verified-by-names-tests", a.Line, fmt.Sprintf("%s: assertion %q", where, a.Key), a.VerifiedBy)
}
