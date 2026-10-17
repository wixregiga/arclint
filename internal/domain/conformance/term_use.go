package conformance

import (
	"fmt"
	"path"
	"slices"
	"sort"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// UsedRole is the part a used term plays in its bounded context: one
// of the building blocks recorded as a type, or the repository or
// factory an aggregate names.
type UsedRole string

// The roles a used term can play.
const (
	UsedAggregate     UsedRole = "aggregate"
	UsedEntity        UsedRole = "entity"
	UsedValueObject   UsedRole = "value_object"
	UsedDomainEvent   UsedRole = "domain_event"
	UsedDomainService UsedRole = "domain_service"
	UsedSpecification UsedRole = "specification"
	UsedRepository    UsedRole = "repository"
	UsedFactory       UsedRole = "factory"
)

// UseWay says how the code reaches a used term.
type UseWay string

const (
	// UseByCall is a call whose target or receiver resolves to the
	// term's declaration or a member of it.
	UseByCall UseWay = "call"
	// UseByField is a field whose type holds the term's declaration.
	UseByField UseWay = "field"
)

// TermUse is one place where a file's code reaches a recorded term the
// file does not declare: the file and line, the term with its bounded
// context and role, and how the code reaches it. Two TermUses with the
// same values are the same use.
type TermUse struct {
	path      string
	line      int
	context   string
	term      string
	role      UsedRole
	aggregate string
	way       UseWay
	via       string
}

// useSite is where a reference is written: the file and line, how the
// code reaches through it, and what the code spells there, the callee
// as written or the field.
type useSite struct {
	path string
	line int
	way  UseWay
	via  string
}

// newTermUse is the one door a TermUse is built through. The
// reference must be resolved by the language to the reached
// declaration, and the reached declaration must be the term's one
// located declaration or a member of it; a reference that only spells
// the term's name is rejected.
func newTermUse(ref Reference, reached locDecl, term usedTerm, site useSite) (TermUse, error) {
	if ref.Resolution != ResolutionDeclared {
		return TermUse{}, fmt.Errorf("term use of %s at %s:%d: the reference is %s, not resolved to a declaration", term.name, site.path, site.line, ref.Resolution)
	}
	if reached.file.path != ref.File || reached.decl.Name != ref.Name || reached.decl.Owner != ref.Owner {
		return TermUse{}, fmt.Errorf("term use of %s at %s:%d: the reference does not reach %s", term.name, site.path, site.line, reached.decl.Name)
	}
	if len(term.candidates) != 1 {
		return TermUse{}, fmt.Errorf("term use of %s at %s:%d: the term is not located in one declaration", term.name, site.path, site.line)
	}
	if !term.reachedThrough(reached) {
		return TermUse{}, fmt.Errorf("term use of %s at %s:%d: %s is neither its declaration nor a member of it", term.name, site.path, site.line, reached.decl.Name)
	}
	if site.path == "" || site.line < 1 || (site.way != UseByCall && site.way != UseByField) {
		return TermUse{}, fmt.Errorf("term use of %s: a site needs a path, a line, and a way of reaching", term.name)
	}
	return TermUse{
		path: site.path, line: site.line, context: term.context, term: term.name,
		role: term.role, aggregate: term.aggregate, way: site.way, via: site.via,
	}, nil
}

// Path is the file whose code reaches the term.
func (u TermUse) Path() string { return u.path }

// Line is the line of the call or field that reaches the term.
func (u TermUse) Line() int { return u.line }

// Context names the bounded context the term is recorded in.
func (u TermUse) Context() string { return u.context }

// Term is the recorded name of the term reached.
func (u TermUse) Term() string { return u.term }

// Role is the part the term plays in its context.
func (u TermUse) Role() UsedRole { return u.role }

// Aggregate names the aggregate an entity is a member of, an identity
// identifies, or a repository or factory serves; "" otherwise.
func (u TermUse) Aggregate() string { return u.aggregate }

// Way says whether a call or a field reaches the term.
func (u TermUse) Way() UseWay { return u.way }

// Via is what the code spells at the site: the callee as written, or
// the field.
func (u TermUse) Via() string { return u.via }

// UndeterminedUse is one reference of a worksite file that resolution
// could not follow, or that reaches a declaration two candidates of a
// term spell: it may be a use, and nothing says it is not. Line is 0
// when the whole file could not be read.
type UndeterminedUse struct {
	Path   string
	Line   int
	What   string
	Reason string
}

// TermUses is what the code of a worksite reaches of the recorded
// domain: every use of a term declared outside the worksite, and every
// reference that cannot be told to be one or not.
type TermUses struct {
	Uses         []TermUse
	Undetermined []UndeterminedUse
}

// usedTerm is one recorded term as located for use: its context, its
// name and role, the aggregate it belongs to, and the declarations
// spelling it, one when it is located.
type usedTerm struct {
	context    string
	name       string
	role       UsedRole
	aggregate  string
	candidates []locDecl
}

// reachedThrough reports whether a reached declaration is the term's
// located declaration or a member of it.
func (t usedTerm) reachedThrough(reached locDecl) bool {
	decl := t.candidates[0]
	switch {
	case reached.decl.Owner == "" && reached.file.path == decl.file.path && reached.decl.Name == decl.decl.Name:
		return true
	case reached.decl.Owner != "" && decl.owns(reached.file, reached.decl.Owner):
		return true
	}
	return false
}

// writtenResult finds the result of a function that is written with
// the name of the type a declaration of the term declares, when the
// function lies beside it: in Go in its package, elsewhere in its file.
// The function may build the term, but its result types are not
// resolved, so the text can raise the question and never settle it: a
// type parameter or an import can spell the same name.
func writtenResult(fn, decl locDecl) (string, bool) {
	if fn.decl.Kind != declKindFunc || fn.decl.Owner != "" || !isTypeKind(decl.decl.Kind) {
		return "", false
	}
	beside := fn.file.path == decl.file.path
	if fn.file.lang == rule.LanguageGo {
		beside = path.Dir(fn.file.path) == path.Dir(decl.file.path)
	}
	if !beside {
		return "", false
	}
	for _, r := range fn.decl.Results {
		if mentionsType(r, "", decl.decl.Name) {
			return r, true
		}
	}
	return "", false
}

// usedTerms lists every term of the recorded language as located,
// context by context in recording order: aggregate roots, the terms
// located as types, and the factories aggregates name.
func usedTerms(contexts []contextCode) []usedTerm {
	var out []usedTerm
	for _, cc := range contexts {
		for _, agg := range cc.ctx.Aggregates {
			out = append(out, usedTerm{context: cc.ctx.Name, name: agg.Name, role: UsedAggregate, candidates: cc.aggregates[agg.Name].candidates})
		}
		for _, t := range cc.ctx.Terms() {
			role, ok := termRoles[t.Concept]
			if !ok {
				continue
			}
			out = append(out, usedTerm{context: cc.ctx.Name, name: t.Name, role: role, aggregate: t.Aggregate, candidates: cc.terms[t.Name].candidates})
		}
		for _, agg := range cc.ctx.Aggregates {
			if agg.Repository != "" {
				out = append(out, usedTerm{context: cc.ctx.Name, name: agg.Repository, role: UsedRepository, aggregate: agg.Name, candidates: cc.terms[agg.Repository].candidates})
			}
			if agg.Factory != "" {
				out = append(out, usedTerm{context: cc.ctx.Name, name: agg.Factory, role: UsedFactory, aggregate: agg.Name, candidates: cc.terms[agg.Factory].candidates})
			}
		}
	}
	return out
}

// termRoles maps the concepts recorded as types to their roles.
var termRoles = map[vocab.Concept]UsedRole{
	vocab.ConceptEntity:        UsedEntity,
	vocab.ConceptValueObject:   UsedValueObject,
	vocab.ConceptDomainEvent:   UsedDomainEvent,
	vocab.ConceptDomainService: UsedDomainService,
	vocab.ConceptSpecification: UsedSpecification,
}

// Uses finds what the code of the worksite reaches of the recorded
// domain: for every file the worksite holds, each call whose target or
// receiver, and each field whose held types, the language resolved to
// a term located outside the worksite or a member of it. A term
// declared inside the worksite is declared here, not used. A reference
// resolution could not follow, one that reaches a term two
// declarations spell, a call to a function beside a term whose written
// result names it, and a file of one of the languages whose calls or
// field types could not be read are undetermined, with the reason.
func (c Carriers) Uses(inWorksite func(path string) bool, languages []rule.Language) (TermUses, error) {
	var out TermUses
	seenUse := map[TermUse]bool{}
	seenUndetermined := map[UndeterminedUse]bool{}
	undetermined := func(u UndeterminedUse) {
		if !seenUndetermined[u] {
			seenUndetermined[u] = true
			out.Undetermined = append(out.Undetermined, u)
		}
	}
	for _, gap := range c.all.failed {
		if inWorksite(gap.path) {
			undetermined(UndeterminedUse{Path: gap.path, What: "file", Reason: gap.reason})
		}
	}
	files := map[string]fileFacts{}
	for _, f := range c.all.files {
		files[f.path] = f
	}
	failed := map[string]bool{}
	for _, gap := range c.all.failed {
		failed[gap.path] = true
	}
	for _, p := range c.observed {
		if _, read := files[p]; read || failed[p] || !inWorksite(p) || !slices.Contains(languages, rule.LanguageOf(p)) {
			continue
		}
		undetermined(UndeterminedUse{Path: p, What: "file", Reason: "its language supplies no declarations or calls for this file, so its references are not known"})
	}
	reach := func(ref Reference, site useSite) error {
		switch ref.Resolution {
		case ResolutionUndetermined:
			undetermined(UndeterminedUse{Path: site.path, Line: site.line, What: string(site.way) + " " + site.via, Reason: ref.Reason})
			return nil
		case ResolutionDeclared:
		default:
			return nil
		}
		reached, found := files[ref.File].declaration(ref)
		if !found {
			undetermined(UndeterminedUse{
				Path: site.path, Line: site.line, What: string(site.way) + " " + site.via,
				Reason: fmt.Sprintf("it reaches %s in %s, whose declarations were not read", referenceName(ref), ref.File),
			})
			return nil
		}
		for _, term := range c.terms {
			if !term.mayBeReachedThrough(reached) {
				continue
			}
			if len(term.candidates) > 1 {
				undetermined(UndeterminedUse{
					Path: site.path, Line: site.line, What: string(site.way) + " " + site.via,
					Reason: fmt.Sprintf("it reaches %s in %s, one of %s spelling %s, so which is the model's cannot be told", referenceName(ref), ref.File, candidateList(term.candidates), term.name),
				})
				continue
			}
			if inWorksite(term.candidates[0].file.path) {
				continue
			}
			use, err := newTermUse(ref, reached, term, site)
			if err != nil {
				return err
			}
			if !seenUse[use] {
				seenUse[use] = true
				out.Uses = append(out.Uses, use)
			}
		}
		for _, term := range c.terms {
			for _, d := range term.candidates {
				result, written := writtenResult(reached, d)
				if !written || inWorksite(d.file.path) {
					continue
				}
				undetermined(UndeterminedUse{
					Path: site.path, Line: site.line, What: string(site.way) + " " + site.via,
					Reason: fmt.Sprintf("%s returns %s as written; result types are not resolved", referenceName(ref), result),
				})
				break
			}
		}
		return nil
	}
	for _, f := range c.all.files {
		if !inWorksite(f.path) {
			continue
		}
		if !f.resolvedCalls {
			undetermined(UndeterminedUse{Path: f.path, What: "calls", Reason: "the file's resolved calls are not available"})
		} else {
			for _, call := range f.calls {
				site := useSite{path: f.path, line: call.Line, way: UseByCall, via: call.Callee}
				if err := reach(call.Target, site); err != nil {
					return TermUses{}, err
				}
				if err := reach(call.Receiver, site); err != nil {
					return TermUses{}, err
				}
			}
		}
		if !f.fieldTypes {
			undetermined(UndeterminedUse{Path: f.path, What: "fields", Reason: "the file's field types are not available"})
			continue
		}
		for _, d := range f.decls {
			if d.Kind != declKindField {
				continue
			}
			via := d.Name
			if d.Owner != "" {
				via = d.Owner + "." + d.Name
			}
			site := useSite{path: f.path, line: d.StartLine, way: UseByField, via: via}
			for _, ref := range d.Holds {
				if err := reach(ref, site); err != nil {
					return TermUses{}, err
				}
			}
		}
	}
	sort.SliceStable(out.Uses, func(i, j int) bool {
		a, b := out.Uses[i], out.Uses[j]
		if a.path != b.path {
			return a.path < b.path
		}
		return a.line < b.line
	})
	sort.SliceStable(out.Undetermined, func(i, j int) bool {
		a, b := out.Undetermined[i], out.Undetermined[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Line < b.Line
	})
	return out, nil
}

// mayBeReachedThrough reports whether a reached declaration is, for
// any declaration spelling the term, that declaration or a member of
// it. A located term then makes a use; a term two declarations spell
// leaves the reference undetermined.
func (t usedTerm) mayBeReachedThrough(reached locDecl) bool {
	for _, d := range t.candidates {
		if (usedTerm{candidates: []locDecl{d}}).reachedThrough(reached) {
			return true
		}
	}
	return false
}

// declaration finds the declaration a resolved reference reaches in
// the file.
func (f fileFacts) declaration(ref Reference) (locDecl, bool) {
	for _, d := range f.decls {
		if d.Name == ref.Name && d.Owner == ref.Owner {
			return locDecl{file: f, decl: d}, true
		}
	}
	return locDecl{}, false
}

// referenceName spells a reached declaration as its owner and name.
func referenceName(ref Reference) string {
	if ref.Owner == "" {
		return ref.Name
	}
	return ref.Owner + "." + ref.Name
}
