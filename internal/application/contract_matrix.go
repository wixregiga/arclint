package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// ContractAnchor states how a recorded contract relates to the observed
// declarations. Every recorded contract names the declaration that
// carries it (an aggregate invariant the root method its key names, a
// value object invariant the constructor, an assertion the root method
// its key names, a specification its satisfaction method), so a
// contract is either found in source or missing from it.
type ContractAnchor string

const (
	// AnchorFound means Source names the declaration carrying the
	// contract.
	AnchorFound ContractAnchor = "found"
	// AnchorMissing means no observed file declares the carrier the
	// recording names.
	AnchorMissing ContractAnchor = "missing"
)

// ContractKind names which recorded contract a listing entry is.
type ContractKind string

const (
	// ContractInvariant is a recorded invariant.
	ContractInvariant ContractKind = "invariant"
	// ContractAssertion is a recorded assertion.
	ContractAssertion ContractKind = "assertion"
	// ContractSpecification is a recorded specification.
	ContractSpecification ContractKind = "specification"
)

// locateDomainContracts fills Source and Anchor on every contract of
// the projected model from the observed declarations, reading them
// exactly as the built-in domain rules do.
func locateDomainContracts(dk *DomainKnowledge, carriers conformance.Carriers) error {
	if dk == nil {
		return nil
	}
	dk.Located = true
	for i := range dk.Contexts {
		ctx := dk.Contexts[i].Name
		for j, inv := range dk.Contexts[i].Invariants {
			src, found, err := locateInvariant(carriers, ctx, inv)
			if err != nil {
				return err
			}
			dk.Contexts[i].Invariants[j].Source = src
			dk.Contexts[i].Invariants[j].Anchor = anchorOf(found)
		}
		for j, a := range dk.Contexts[i].Assertions {
			c, found, err := carriers.Assertion(ctx, a.Owner, a.Key)
			if err != nil {
				return fmt.Errorf("assertion %s of %s: %w", a.Key, a.Owner, err)
			}
			dk.Contexts[i].Assertions[j].Source = sourceOf(c, found)
			dk.Contexts[i].Assertions[j].Anchor = anchorOf(found)
		}
		for j, s := range dk.Contexts[i].Specifications {
			c, found := carriers.Satisfaction(ctx, s.Name)
			dk.Contexts[i].Specifications[j].Source = sourceOf(c, found)
			dk.Contexts[i].Specifications[j].Anchor = anchorOf(found)
		}
	}
	return nil
}

func anchorOf(found bool) ContractAnchor {
	if found {
		return AnchorFound
	}
	return AnchorMissing
}

// sourceOf spells a located carrier as path:line, or "" when none.
func sourceOf(c conformance.Carrier, found bool) string {
	if !found {
		return ""
	}
	return fmt.Sprintf("%s:%d", c.Path, c.Line)
}

// locateInvariant resolves one invariant against the declarations: a
// value object's invariant is enforced in the constructor, an
// aggregate's in the ensure method of the root.
func locateInvariant(carriers conformance.Carriers, ctx string, inv DomainInvariantRef) (string, bool, error) {
	if inv.OwnerConcept == vocab.ConceptValueObject {
		c, found := carriers.Constructor(ctx, inv.Owner)
		return sourceOf(c, found), found, nil
	}
	c, found, err := carriers.Invariant(ctx, inv.Owner, inv.Key)
	if err != nil {
		return "", false, fmt.Errorf("invariant %s of %s: %w", inv.Key, inv.Owner, err)
	}
	return sourceOf(c, found), found, nil
}
