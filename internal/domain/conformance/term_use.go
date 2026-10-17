package conformance

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

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

// TermUse is one place where a file's code reaches a recorded term,
// wherever the term is declared: the file and line, the term with its
// bounded context and role, and how the code reaches it. Two TermUses
// with the same values are the same use.
type TermUse struct {
	path      string
	line      int
	context   string
	term      string
	role      UsedRole
	aggregate string
	// identifies lists the aggregates whose identity a value object is.
	identifies []string
	way        UseWay
	via        string
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
		role: term.role, aggregate: term.aggregate, identifies: append([]string(nil), term.identifies...), way: site.way, via: site.via,
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

// Aggregate names the aggregate an entity is a member of, or a
// repository or factory serves; "" otherwise.
func (u TermUse) Aggregate() string { return u.aggregate }

// Identifies lists, for a value object, every aggregate whose identity
// it is, in recording order; empty otherwise.
func (u TermUse) Identifies() []string { return append([]string(nil), u.identifies...) }

// Way says whether a call or a field reaches the term.
func (u TermUse) Way() UseWay { return u.way }

// Via is what the code spells at the site: the callee as written, or
// the field.
func (u TermUse) Via() string { return u.via }

// UndeterminedUse is one reference of a worksite file that may be a use
// resolution cannot confirm: one resolution could not follow, one that
// reaches a declaration two candidates of a term spell, or a call to a
// function whose written result names a term. Nothing says it is not a
// use. Two UndeterminedUses with the same values are the same.
type UndeterminedUse struct {
	path, what, reason string
	line               int
	context, term      string
}

// UndeterminedUseSpec is the input to the validated UndeterminedUse.
// Context and Term name the term the reference may reach, when one is
// known; Line is 0 when the whole file could not be read.
type UndeterminedUseSpec struct {
	Path    string
	Line    int
	What    string
	Reason  string
	Context string
	Term    string
}

// NewUndeterminedUse is the one door an UndeterminedUse is built
// through: it states why resolution cannot confirm whether the
// reference is a use, and where the reference is.
func NewUndeterminedUse(s UndeterminedUseSpec) (UndeterminedUse, error) {
	if strings.TrimSpace(s.Reason) == "" {
		return UndeterminedUse{}, fmt.Errorf("undetermined use at %s:%d: a reason is required", s.Path, s.Line)
	}
	if s.Path == "" || s.Line < 0 || strings.TrimSpace(s.What) == "" {
		return UndeterminedUse{}, fmt.Errorf("undetermined use: a path, a nonnegative line, and what is written there are required")
	}
	if s.Term != "" && s.Context == "" {
		return UndeterminedUse{}, fmt.Errorf("undetermined use of %s at %s:%d: the term needs its bounded context", s.Term, s.Path, s.Line)
	}
	return UndeterminedUse{path: s.Path, line: s.Line, what: s.What, reason: s.Reason, context: s.Context, term: s.Term}, nil
}

// Path is the worksite file holding the reference.
func (u UndeterminedUse) Path() string { return u.path }

// Line is the line of the reference, 0 for the whole file.
func (u UndeterminedUse) Line() int { return u.line }

// What is the reference as the code spells it: the call or field, or
// the file.
func (u UndeterminedUse) What() string { return u.what }

// Reason says why resolution cannot confirm whether it is a use.
func (u UndeterminedUse) Reason() string { return u.reason }

// Context names the bounded context of the term the reference may
// reach, "" when no term is known.
func (u UndeterminedUse) Context() string { return u.context }

// Term names the term the reference may reach, "" when none is known.
func (u UndeterminedUse) Term() string { return u.term }

// TermUses is what the code of a worksite reaches of the recorded
// domain: every use of a term, and every reference that cannot be told
// to be one or not.
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
	identifies []string
	candidates []locDecl
}

// reachedThrough reports whether a reached declaration is the term's
// located declaration or a member of it.
func (t usedTerm) reachedThrough(reached locDecl) bool {
	decl := t.candidates[0]
	switch {
	case reached.isDeclaration(decl):
		return true
	case reached.decl.Owner != "" && decl.owns(reached.file, reached.decl.Owner):
		return true
	}
	return false
}

// isDeclaration reports whether a reached declaration is the located one
// itself, by its whole identity: file, kind, owner, name, and line. A
// function, or a type nested elsewhere in the file, that only shares the
// name is another declaration.
func (d locDecl) isDeclaration(located locDecl) bool {
	return d.file.path == located.file.path && d.decl.Kind == located.decl.Kind &&
		d.decl.Owner == located.decl.Owner && d.decl.Name == located.decl.Name &&
		d.decl.StartLine == located.decl.StartLine
}

// writtenResult finds the result of a function that is written with
// the name of the type a declaration of the term declares, when the
// function lies beside it in its own language: in Go in its package,
// elsewhere in its file.
// The function may build the term, but its result types are not
// resolved, so the text can raise the question and never settle it: a
// type parameter or an import can spell the same name.
func writtenResult(fn, decl locDecl) (string, bool) {
	if fn.decl.Kind != declKindFunc || fn.decl.Owner != "" || !isTypeKind(decl.decl.Kind) || fn.file.lang != decl.file.lang {
		return "", false
	}
	beside := fn.file.path == decl.file.path
	if fn.file.lang == rule.LanguageGo {
		beside = samePackage(fn.file, decl.file)
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
		identifies := identityOwners(cc.ctx)
		for _, t := range cc.ctx.Terms() {
			role, ok := termRoles[t.Concept]
			if !ok {
				continue
			}
			term := usedTerm{context: cc.ctx.Name, name: t.Name, role: role, candidates: cc.terms[t.Name].candidates}
			switch role {
			case UsedValueObject:
				term.identifies = identifies[t.Name]
			case UsedEntity:
				term.aggregate = t.Aggregate
			case UsedAggregate, UsedDomainEvent, UsedDomainService, UsedSpecification, UsedRepository, UsedFactory:
				// Belongs to no aggregate as a term of Terms: an aggregate is
				// its own root, and repositories and factories are listed
				// with their aggregate below.
			}
			out = append(out, term)
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

// identityOwners maps each identity the context's aggregates and their
// members name to every aggregate it identifies, in recording order,
// whether or not the identity is also recorded as a value object.
func identityOwners(ctx vocab.BoundedContext) map[string][]string {
	owners := map[string][]string{}
	claim := func(identity, aggregate string) {
		if identity != "" && !slices.Contains(owners[identity], aggregate) {
			owners[identity] = append(owners[identity], aggregate)
		}
	}
	for _, agg := range ctx.Aggregates {
		claim(agg.Identity, agg.Name)
		for _, e := range agg.Entities {
			claim(e.Identity, agg.Name)
		}
	}
	return owners
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
// a located term or a member of it, wherever the term is declared;
// which of them a listing shows as declared here is the listing's
// decision. A reference resolution could not follow, one that reaches
// a term two declarations spell, a call to a function beside a term
// whose written result names it, and a file of one of the languages
// whose calls or field types could not be read are undetermined, with
// the reason.
func (c Carriers) Uses(inWorksite func(path string) bool, languages []rule.Language) (TermUses, error) {
	var out TermUses
	type useKey struct {
		path, context, term, via string
		line                     int
		way                      UseWay
	}
	seenUse := map[useKey]bool{}
	seenUndetermined := map[UndeterminedUse]bool{}
	var invalid error
	undetermined := func(spec UndeterminedUseSpec) {
		u, err := NewUndeterminedUse(spec)
		if err != nil {
			invalid = errors.Join(invalid, err)
			return
		}
		if !seenUndetermined[u] {
			seenUndetermined[u] = true
			out.Undetermined = append(out.Undetermined, u)
		}
	}
	for _, gap := range c.all.failed {
		if inWorksite(gap.path) {
			undetermined(UndeterminedUseSpec{Path: gap.path, What: "file", Reason: gap.reason})
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
		undetermined(UndeterminedUseSpec{Path: p, What: "file", Reason: "its language supplies no declarations or calls for this file, so its references are not known"})
	}
	reach := func(ref Reference, site useSite) error {
		switch ref.Resolution {
		case ResolutionUndetermined:
			undetermined(UndeterminedUseSpec{Path: site.path, Line: site.line, What: string(site.way) + " " + site.via, Reason: ref.Reason})
			return nil
		case ResolutionDeclared:
		default:
			return nil
		}
		spelled := files[ref.File].declarations(ref)
		if len(spelled) == 0 {
			undetermined(UndeterminedUseSpec{
				Path: site.path, Line: site.line, What: string(site.way) + " " + site.via,
				Reason: fmt.Sprintf("it reaches %s in %s, whose declarations were not read", referenceName(ref), ref.File),
			})
			return nil
		}
		for _, term := range c.terms {
			var reaching []locDecl
			for _, d := range spelled {
				if term.mayBeReachedThrough(d) {
					reaching = append(reaching, d)
				}
			}
			switch {
			case len(reaching) == 0:
				continue
			case len(reaching) < len(spelled):
				// The file declares the name twice and the reference does
				// not say which one it reaches.
				undetermined(UndeterminedUseSpec{
					Path: site.path, Line: site.line, What: string(site.way) + " " + site.via,
					Reason:  fmt.Sprintf("it reaches %s in %s, which %d declarations spell, and not all of them are %s", referenceName(ref), ref.File, len(spelled), term.name),
					Context: term.context, Term: term.name,
				})
				continue
			case len(term.candidates) > 1:
				undetermined(UndeterminedUseSpec{
					Path: site.path, Line: site.line, What: string(site.way) + " " + site.via,
					Reason:  fmt.Sprintf("it reaches %s in %s, one of %s spelling %s, so which is the model's cannot be told", referenceName(ref), ref.File, candidateList(term.candidates), term.name),
					Context: term.context, Term: term.name,
				})
				continue
			}
			use, err := newTermUse(ref, reaching[0], term, site)
			if err != nil {
				return err
			}
			if k := (useKey{use.path, use.context, use.term, use.via, use.line, use.way}); !seenUse[k] {
				seenUse[k] = true
				out.Uses = append(out.Uses, use)
			}
		}
		for _, term := range c.terms {
		written:
			for _, d := range term.candidates {
				for _, reached := range spelled {
					result, ok := writtenResult(reached, d)
					if !ok {
						continue
					}
					undetermined(UndeterminedUseSpec{
						Path: site.path, Line: site.line, What: string(site.way) + " " + site.via,
						Reason:  fmt.Sprintf("%s returns %s as written; result types are not resolved", referenceName(ref), result),
						Context: term.context, Term: term.name,
					})
					break written
				}
			}
		}
		return nil
	}
	for _, f := range c.all.files {
		if !inWorksite(f.path) {
			continue
		}
		if !f.resolvedCalls {
			undetermined(UndeterminedUseSpec{Path: f.path, What: "calls", Reason: "the file's resolved calls are not available"})
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
			undetermined(UndeterminedUseSpec{Path: f.path, What: "fields", Reason: "the file's field types are not available"})
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
	if invalid != nil {
		return TermUses{}, invalid
	}
	sort.SliceStable(out.Undetermined, func(i, j int) bool {
		a, b := out.Undetermined[i], out.Undetermined[j]
		if a.path != b.path {
			return a.path < b.path
		}
		return a.line < b.line
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

// declarations lists the declarations of the file a resolved reference
// may reach: every one spelled with its name and owner, since a file
// may declare a name twice and the reference names no line.
func (f fileFacts) declarations(ref Reference) []locDecl {
	var out []locDecl
	for _, d := range f.decls {
		if d.Name == ref.Name && d.Owner == ref.Owner {
			out = append(out, locDecl{file: f, decl: d})
		}
	}
	return out
}

// referenceName spells a reached declaration as its owner and name.
func referenceName(ref Reference) string {
	if ref.Owner == "" {
		return ref.Name
	}
	return ref.Owner + "." + ref.Name
}
