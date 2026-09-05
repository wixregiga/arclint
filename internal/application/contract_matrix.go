package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

const declKindMethod = "method"

// ContractAnchor states how a recorded contract relates to the observed
// declarations. The three outcomes stay apart because each calls for
// different work: found needs none, missing needs source, and
// unanchorable needs the recording changed before any source could
// carry it.
type ContractAnchor string

const (
	// AnchorFound means Source names the declaration carrying the
	// contract.
	AnchorFound ContractAnchor = "found"
	// AnchorMissing means the recording names a declaration to look for
	// and no observed file declares it.
	AnchorMissing ContractAnchor = "missing"
	// AnchorUnanchorable means the recording shape names no declaration
	// at all; Reason says which shape.
	AnchorUnanchorable ContractAnchor = "unanchorable"
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
// the projected model from the observed declarations.
func locateDomainContracts(dk *DomainKnowledge, lang vocab.UbiquitousLanguage, idx []declHit) {
	if dk == nil {
		return
	}
	dk.Located = true
	for i := range dk.Contexts {
		ctx := lang.Contexts[i]
		for j, inv := range dk.Contexts[i].Invariants {
			src, anchor, reason := locateInvariant(idx, ctx, inv)
			dk.Contexts[i].Invariants[j].Source = src
			dk.Contexts[i].Invariants[j].Anchor = anchor
			dk.Contexts[i].Invariants[j].Reason = reason
		}
		for j, a := range dk.Contexts[i].Assertions {
			src, found := locateNamedMethod(idx, a.Owner, a.ID)
			dk.Contexts[i].Assertions[j].Source = src
			dk.Contexts[i].Assertions[j].Anchor = anchorOf(found)
		}
		for j, s := range dk.Contexts[i].Specifications {
			src, found := locateSpecification(idx, s.Name)
			dk.Contexts[i].Specifications[j].Source = src
			dk.Contexts[i].Specifications[j].Anchor = anchorOf(found)
		}
	}
}

func anchorOf(found bool) ContractAnchor {
	if found {
		return AnchorFound
	}
	return AnchorMissing
}

type declHit struct {
	path string
	decl conformance.Declaration
	lang rule.Language
}

func indexDeclarations(obs conformance.Observations) []declHit {
	var out []declHit
	for _, f := range obs.Files() {
		facts, ok := obs.FactsFor(f.Path)
		if !ok || facts.ParseFailure != "" || !facts.DeclarationsAvailable {
			continue
		}
		for _, d := range facts.Declarations {
			out = append(out, declHit{path: f.Path, decl: d, lang: facts.Language})
		}
	}
	return out
}

// locateInvariant resolves one invariant against the declarations. Only
// two recorded shapes name a declaration: a value object owner without
// an id names that type's constructor, and an aggregate owner with an
// id names the method the id renders to. Every other shape is
// unanchorable, and the reason says which shape it is.
func locateInvariant(idx []declHit, ctx vocab.BoundedContext, inv DomainInvariantRef) (string, ContractAnchor, string) {
	agg, vo := ownerKind(ctx, inv.Owner)
	switch {
	case vo && inv.ID == "":
		src, found := locateConstructor(idx, inv.Owner)
		return src, anchorOf(found), ""
	case agg && inv.ID != "":
		src, found := locateNamedMethod(idx, inv.Owner, inv.ID)
		return src, anchorOf(found), ""
	}
	return "", AnchorUnanchorable, unanchorableReason(ctx, inv.Owner, inv.ID, agg, vo)
}

// unanchorableReason names the recorded shape that leaves an invariant
// without a declaration to look for.
func unanchorableReason(ctx vocab.BoundedContext, owner, id string, agg, vo bool) string {
	switch {
	case agg:
		return fmt.Sprintf("owner %s is an aggregate and the invariant has no id, so no method is named to carry it", owner)
	case vo:
		return fmt.Sprintf("owner %s is a value object and the invariant carries id %s, but value integrity names the constructor, not a method", owner, id)
	case isRecordedEntity(ctx, owner):
		return fmt.Sprintf("owner %s is an entity that is not an aggregate; only an aggregate or a value object names a declaration", owner)
	}
	return fmt.Sprintf("owner %s is not a recorded entity or value object of context %s", owner, ctx.Name)
}

func isRecordedEntity(ctx vocab.BoundedContext, owner string) bool {
	for _, e := range ctx.Entities {
		if e.Name == owner {
			return true
		}
	}
	return false
}

func ownerKind(ctx vocab.BoundedContext, owner string) (aggregate, valueObject bool) {
	for _, e := range ctx.Entities {
		if e.Name == owner {
			return e.Aggregate, false
		}
	}
	for _, v := range ctx.ValueObjects {
		if v.Name == owner {
			return false, true
		}
	}
	return false, false
}

func locateSpecification(idx []declHit, name string) (string, bool) {
	for _, h := range idx {
		if h.decl.Kind != declKindMethod || h.decl.Owner != name {
			continue
		}
		switch h.decl.Name {
		case "SatisfiedBy", "satisfiedBy", "satisfied_by":
			return fmt.Sprintf("%s:%d", h.path, h.decl.StartLine), true
		}
	}
	return "", false
}

func locateNamedMethod(idx []declHit, owner, id string) (string, bool) {
	for _, h := range idx {
		name, err := rule.CaseTerm(id, methodCase(h.lang))
		if err != nil {
			continue
		}
		if h.decl.Kind == declKindMethod && h.decl.Owner == owner && h.decl.Name == name {
			return fmt.Sprintf("%s:%d", h.path, h.decl.StartLine), true
		}
	}
	return "", false
}

func locateConstructor(idx []declHit, typeName string) (string, bool) {
	for _, h := range idx {
		d := h.decl
		if d.Kind == declKindMethod && d.Owner == typeName && (d.Name == "constructor" || d.Name == "__init__") {
			return fmt.Sprintf("%s:%d", h.path, d.StartLine), true
		}
		if d.Kind != "func" && d.Kind != declKindMethod {
			continue
		}
		if d.Name != "New" && d.Name != "create" && d.Name != "New"+typeName {
			continue
		}
		for _, r := range d.Results {
			if r == typeName || r == "*"+typeName || containsType(r, typeName) {
				return fmt.Sprintf("%s:%d", h.path, d.StartLine), true
			}
		}
	}
	return "", false
}

// typeDeclarationPaths lists every observed file declaring a type of
// the given name: the places a recorded term anchors into source.
func typeDeclarationPaths(idx []declHit, name string) []string {
	var out []string
	for _, h := range idx {
		if h.decl.Name != name {
			continue
		}
		switch h.decl.Kind {
		case "struct", "interface", "type", "class", "enum":
			out = appendUnique(out, h.path)
		}
	}
	return out
}

func containsType(text, typeName string) bool {
	return len(text) >= len(typeName) && (text == typeName || text == "*"+typeName)
}

func methodCase(lang rule.Language) string {
	switch lang {
	case rule.LanguageGo:
		return "PascalCase"
	case rule.LanguageTypeScript:
		return "camelCase"
	case rule.LanguagePython:
		return "snake_case"
	default:
		return "PascalCase"
	}
}
