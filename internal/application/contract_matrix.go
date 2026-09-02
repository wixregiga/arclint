package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

const declKindMethod = "method"

const sourceMissing = "missing"

func locateDomainContracts(dk *DomainKnowledge, lang vocab.UbiquitousLanguage, obs conformance.Observations) {
	if dk == nil {
		return
	}
	idx := indexDeclarations(obs)
	for i := range dk.Contexts {
		ctx := lang.Contexts[i]
		for j, inv := range dk.Contexts[i].Invariants {
			dk.Contexts[i].Invariants[j].Source = locateInvariant(idx, ctx, inv)
		}
		for j, a := range dk.Contexts[i].Assertions {
			dk.Contexts[i].Assertions[j].Source = locateAssertion(idx, a)
		}
		for j, s := range dk.Contexts[i].Specifications {
			dk.Contexts[i].Specifications[j].Source = locateSpecification(idx, s.Name)
		}
	}
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

func locateInvariant(idx []declHit, ctx vocab.BoundedContext, inv DomainInvariantRef) string {
	_, vo := ownerKind(ctx, inv.Owner)
	if vo && inv.ID == "" {
		return locateConstructor(idx, inv.Owner)
	}
	agg, _ := ownerKind(ctx, inv.Owner)
	if agg && inv.ID != "" {
		return locateNamedMethod(idx, inv.Owner, inv.ID)
	}
	return ""
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

func locateAssertion(idx []declHit, a DomainAssertionRef) string {
	return locateNamedMethod(idx, a.Owner, a.ID)
}

func locateSpecification(idx []declHit, name string) string {
	for _, h := range idx {
		if h.decl.Kind != declKindMethod || h.decl.Owner != name {
			continue
		}
		switch h.decl.Name {
		case "SatisfiedBy", "satisfiedBy", "satisfied_by":
			return fmt.Sprintf("%s:%d", h.path, h.decl.StartLine)
		}
	}
	return sourceMissing
}

func locateNamedMethod(idx []declHit, owner, id string) string {
	for _, h := range idx {
		name, err := rule.CaseTerm(id, methodCase(h.lang))
		if err != nil {
			continue
		}
		if h.decl.Kind == declKindMethod && h.decl.Owner == owner && h.decl.Name == name {
			return fmt.Sprintf("%s:%d", h.path, h.decl.StartLine)
		}
	}
	return sourceMissing
}

func locateConstructor(idx []declHit, typeName string) string {
	for _, h := range idx {
		d := h.decl
		if d.Kind == declKindMethod && d.Owner == typeName && (d.Name == "constructor" || d.Name == "__init__") {
			return fmt.Sprintf("%s:%d", h.path, d.StartLine)
		}
		if d.Kind != "func" && d.Kind != declKindMethod {
			continue
		}
		if d.Name != "New" && d.Name != "create" && d.Name != "New"+typeName {
			continue
		}
		for _, r := range d.Results {
			if r == typeName || r == "*"+typeName || containsType(r, typeName) {
				return fmt.Sprintf("%s:%d", h.path, d.StartLine)
			}
		}
	}
	return sourceMissing
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
