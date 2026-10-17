package application

import (
	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// DomainUseRef is one recorded entry a worksite's code uses that the
// declared listing does not show: the entry, the sites that reach it,
// and the contracts that bind the code calling it, each with its anchor.
type DomainUseRef struct {
	Context string `json:"context"`
	Name    string `json:"name"`
	// Role is aggregate, entity, value_object, domain_event,
	// domain_service, specification, repository, or factory.
	Role conformance.UsedRole `json:"role"`
	// Aggregate names the aggregate an entity is a member of, an
	// identity identifies, or a repository or factory serves.
	Aggregate string          `json:"aggregate,omitempty"`
	Sites     []DomainUseSite `json:"sites"`
	// Invariants and Assertions are the contracts of the entry, or of
	// its aggregate for a member entity; Specification is a used
	// specification's satisfaction method.
	Invariants    []DomainInvariantRef    `json:"invariants,omitempty"`
	Assertions    []DomainAssertionRef    `json:"assertions,omitempty"`
	Specification *DomainSpecificationRef `json:"specification,omitempty"`
}

// DomainUseSite is one line of the worksite that reaches a used entry:
// a call, with the callee as written, or a field.
type DomainUseSite struct {
	Path string             `json:"path"`
	Line int                `json:"line"`
	Way  conformance.UseWay `json:"way"`
	Via  string             `json:"via"`
}

// UndeterminedUseRef is one reference of the worksite that may be a use
// resolution cannot confirm. Context and Term name the recorded entry
// it may reach, when one is known; Line is 0 when the whole file could
// not be read.
type UndeterminedUseRef struct {
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	What    string `json:"what"`
	Reason  string `json:"reason"`
	Context string `json:"context,omitempty"`
	Term    string `json:"term,omitempty"`
}

// usedKnowledge groups the worksite's uses into one entry per recorded
// term, in order of the first site reaching each, with the contracts
// the located listing carries for it. A term the declared listing
// shows is declared here: neither its uses nor the references that may
// reach it are listed again. Every other term, a repository or factory
// the listing has no place for included, is listed wherever the
// worksite reaches it.
func usedKnowledge(dk, declaredHere *DomainKnowledge, uses conformance.TermUses) ([]DomainUseRef, []UndeterminedUseRef) {
	type key struct{ context, name string }
	declared := map[key]bool{}
	for _, ctx := range declaredHere.Contexts {
		names := append(append(append([]string(nil), ctx.ValueObjects...), ctx.Events...), ctx.Services...)
		for _, a := range ctx.Aggregates {
			names = append(append(names, a.Name), a.Entities...)
		}
		for _, s := range ctx.Specifications {
			names = append(names, s.Name)
		}
		for _, name := range names {
			declared[key{ctx.Name, name}] = true
		}
	}
	var used []DomainUseRef
	index := map[key]int{}
	for _, u := range uses.Uses {
		k := key{u.Context(), u.Term()}
		if declared[k] {
			continue
		}
		i, ok := index[k]
		if !ok {
			i = len(used)
			index[k] = i
			used = append(used, usedEntry(dk, u))
		}
		used[i].Sites = append(used[i].Sites, DomainUseSite{Path: u.Path(), Line: u.Line(), Way: u.Way(), Via: u.Via()})
	}
	// A member entity carries its aggregate's contracts only when the
	// aggregate itself is not used, so no contract is listed twice.
	for i, u := range used {
		if _, rootUsed := index[key{u.Context, u.Aggregate}]; u.Role == conformance.UsedEntity && rootUsed {
			used[i].Invariants, used[i].Assertions = nil, nil
		}
	}
	undetermined := make([]UndeterminedUseRef, 0, len(uses.Undetermined))
	for _, u := range uses.Undetermined {
		if u.Term() != "" && declared[key{u.Context(), u.Term()}] {
			continue
		}
		undetermined = append(undetermined, UndeterminedUseRef{Path: u.Path(), Line: u.Line(), What: u.What(), Reason: u.Reason(), Context: u.Context(), Term: u.Term()})
	}
	return used, undetermined
}

// usedEntry builds the entry of one used term with its contracts.
func usedEntry(dk *DomainKnowledge, u conformance.TermUse) DomainUseRef {
	entry := DomainUseRef{Context: u.Context(), Name: u.Term(), Role: u.Role(), Aggregate: u.Aggregate()}
	owner := ""
	switch u.Role() {
	case conformance.UsedAggregate, conformance.UsedValueObject, conformance.UsedDomainService:
		owner = u.Term()
	case conformance.UsedEntity:
		owner = u.Aggregate()
	case conformance.UsedDomainEvent, conformance.UsedSpecification, conformance.UsedRepository, conformance.UsedFactory:
	}
	for _, ctx := range dk.Contexts {
		if ctx.Name != u.Context() {
			continue
		}
		for _, inv := range ctx.Invariants {
			if owner != "" && inv.Owner == owner {
				entry.Invariants = append(entry.Invariants, inv)
			}
		}
		for _, a := range ctx.Assertions {
			if owner != "" && a.Owner == owner {
				entry.Assertions = append(entry.Assertions, a)
			}
		}
		if u.Role() == conformance.UsedSpecification {
			for _, s := range ctx.Specifications {
				if s.Name == u.Term() {
					spec := s
					entry.Specification = &spec
				}
			}
		}
	}
	return entry
}
