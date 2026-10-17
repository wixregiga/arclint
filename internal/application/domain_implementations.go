package application

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// DomainImplementationRef is one implementation of a domain service
// declared as an interface or an abstract class, with what its
// operation must do to reach the check of one of the service's
// guarantees.
type DomainImplementationRef struct {
	Context   string `json:"context"`
	Service   string `json:"service"`
	Assertion string `json:"assertion"`
	Type      string `json:"type"`
	Operation string `json:"operation"`
	// Evidence is exact, heuristic, or undetermined: how the language
	// established that the type implements the service.
	Evidence string `json:"evidence"`
	// Must says what the operation does to reach the check: call the
	// context's function, return the value object, or either.
	Must   string `json:"must"`
	Source string `json:"source"`
}

// implementedHere lists the implementations of every declared domain
// service of the recorded model whose operation lies in the worksite,
// located the way assertion/checked-by-its-operation locates them.
func implementedHere(dk *DomainKnowledge, carriers conformance.Carriers, scope domainScope) ([]DomainImplementationRef, error) {
	var out []DomainImplementationRef
	for _, ctx := range dk.Contexts {
		for _, a := range ctx.Assertions {
			if a.OwnerConcept != vocab.ConceptDomainService {
				continue
			}
			impls, err := carriers.Implementations(ctx.Name, a.Owner, a.Key)
			if err != nil {
				return nil, fmt.Errorf("assertion %s of %s: %w", a.Key, a.Owner, err)
			}
			for _, impl := range impls {
				if !scope.contains(impl.Path) {
					continue
				}
				out = append(out, DomainImplementationRef{
					Context: ctx.Name, Service: a.Owner, Assertion: a.Key, Type: impl.Type, Operation: impl.Operation,
					Evidence: string(impl.Evidence), Must: impl.Must, Source: fmt.Sprintf("%s:%d", impl.Path, impl.Line),
				})
			}
		}
	}
	return out, nil
}

// String spells the implementation the way a listing names it.
func (r DomainImplementationRef) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "implements %s: %s.%s must %s (assertion %s)", r.Service, r.Type, r.Operation, r.Must, r.Assertion)
	if r.Evidence != string(conformance.ImplementationExact) {
		fmt.Fprintf(&b, ", %s evidence", r.Evidence)
	}
	return b.String()
}
