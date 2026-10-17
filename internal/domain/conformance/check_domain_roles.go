package conformance

import (
	"fmt"
	"path"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// The checks in this file judge the roles recorded types play through
// resolved facts: the declaration a call reaches and the declarations
// a field holds. A recorded term is matched by the declaration located
// for it, file and name, never by spelling alone; a reference the
// language's resolution could not follow is returned as unresolved,
// never read as a match or as a miss.

// declares reports whether a reference reaches the located declaration.
func (d locDecl) declaredBy(ref Reference) bool {
	return ref.Declares(d.file.path, d.decl.Name)
}

// owns reports whether a member declared in file f under owner belongs
// to the located type: in Go a method or field belongs to the type of
// its name in the same package, elsewhere to the class of its name in
// the same file. A member never belongs to a type of another language,
// even one of the same name in the same directory, and in Go never to
// a type of another package in the same directory, such as an external
// test package.
func (d locDecl) owns(f fileFacts, owner string) bool {
	if owner != d.decl.Name || f.lang != d.file.lang {
		return false
	}
	if f.lang == rule.LanguageGo {
		return samePackage(f, d.file)
	}
	return f.path == d.file.path
}

// samePackage reports whether two Go files are of one package: the same
// directory and the same package clause.
func samePackage(a, b fileFacts) bool {
	return path.Dir(a.path) == path.Dir(b.path) && a.pkg == b.pkg
}

// aggregateTypes lists the located types of one aggregate: its root
// first, then every member entity located for it.
func (cc contextCode) aggregateTypes(agg vocab.Aggregate) []locDecl {
	ac := cc.aggregates[agg.Name]
	if !ac.located() {
		return nil
	}
	types := []locDecl{ac.root()}
	for _, e := range agg.Entities {
		if loc := cc.terms[e.Name]; loc.located() {
			types = append(types, loc.decl())
		}
	}
	return types
}

// ambiguousTypes spells, as unresolved, the aggregate's root and every
// member entity that two declarations spell: their fields and methods
// cannot be told apart from another type's, so no check over them is
// complete.
func (cc contextCode) ambiguousTypes(agg vocab.Aggregate) []string {
	var out []string
	if ac := cc.aggregates[agg.Name]; len(ac.candidates) > 1 {
		out = append(out, fmt.Sprintf("root of aggregate %s is spelled by %s, so its fields and calls cannot be recognized", agg.Name, candidateList(ac.candidates)))
	}
	return append(out, cc.ambiguousMembers(agg)...)
}

// ambiguousMembers spells, as unresolved, every member entity of the
// aggregate that two declarations spell: its fields and methods cannot
// be told apart from another type's, so no check over them is complete.
func (cc contextCode) ambiguousMembers(agg vocab.Aggregate) []string {
	var out []string
	for _, e := range agg.Entities {
		if loc := cc.terms[e.Name]; loc.ambiguous() {
			out = append(out, fmt.Sprintf("member %s of aggregate %s is spelled by %s, so its fields and calls cannot be recognized", e.Name, agg.Name, candidateList(loc.candidates)))
		}
	}
	return out
}

// ownerOf returns the aggregate type owning a member, if any does.
func ownerOf(types []locDecl, f fileFacts, owner string) (locDecl, bool) {
	for _, t := range types {
		if t.owns(f, owner) {
			return t, true
		}
	}
	return locDecl{}, false
}

// unresolvedAt spells one reference resolution could not follow.
func unresolvedAt(file string, line int, what string, ref Reference) string {
	return fmt.Sprintf("%s:%d: %s: %s", file, line, what, ref.Reason)
}

// resolvedFields lists the field declarations of the index with resolved
// field types, outside test files, and the source files whose fields it
// cannot see: one that does not parse, or whose language supplied no
// field types.
func (idx contractIndex) resolvedFields() ([]locDecl, []fileGap) {
	var out []locDecl
	gaps := idx.sourceGaps()
	for _, f := range idx.files {
		if isTestFile(f.path) {
			continue
		}
		if !f.fieldTypes {
			gaps = append(gaps, fileGap{path: f.path, reason: "its field types are not available"})
			continue
		}
		for _, d := range f.decls {
			if d.Kind == declKindField {
				out = append(out, locDecl{file: f, decl: d})
			}
		}
	}
	return out, gaps
}

// sourceGaps lists the files of the index whose declarations and calls
// could not be read, outside test files.
func (idx contractIndex) sourceGaps() []fileGap {
	var gaps []fileGap
	for _, g := range idx.failed {
		if !isTestFile(g.path) {
			gaps = append(gaps, g)
		}
	}
	return gaps
}

// mayHoldMembersOf reports whether a file whose facts a check cannot
// read could declare members of one of the types: the type's own file,
// and in Go any file of its package.
func mayHoldMembersOf(gap fileGap, types []locDecl) bool {
	for _, t := range types {
		if gap.path == t.file.path || t.file.lang == rule.LanguageGo && path.Dir(gap.path) == path.Dir(t.file.path) {
			return true
		}
	}
	return false
}

// gapsAt spells the files a check could not read as unresolved.
func gapsAt(gaps []fileGap, keep func(fileGap) bool) []string {
	var out []string
	for _, g := range gaps {
		if keep(g) {
			out = append(out, g.path+": "+g.reason)
		}
	}
	return out
}

// heldBy finds the first located declaration a field holds, and the
// references of the field resolution could not follow.
func heldBy(field locDecl, candidates []locDecl) (locDecl, bool, []Reference) {
	var unresolved []Reference
	for _, ref := range field.decl.Holds {
		if ref.Resolution == ResolutionUndetermined {
			unresolved = append(unresolved, ref)
			continue
		}
		for _, c := range candidates {
			if c.declaredBy(ref) {
				return c, true, nil
			}
		}
	}
	return locDecl{}, false, unresolved
}

// fieldWhat names a field for a finding or an unresolved reference.
func fieldWhat(field locDecl) string {
	if field.decl.Owner == "" {
		return "field " + field.decl.Name
	}
	return "field " + field.decl.Owner + "." + field.decl.Name
}

// checkRootOnlyReferences judges that nothing outside an aggregate
// holds one of its members: a field declared on any type that is not
// the aggregate's root or one of its members, anywhere in the
// supplied code, never holds a member.
func checkRootOnlyReferences(r rule.Rule, subject rule.Subject, cc contextCode, code domainCode) ([]Violation, []string, error) {
	var vs []Violation
	var unresolved []string
	fields, gaps := code.all.resolvedFields()
	for _, agg := range cc.ctx.Aggregates {
		// Without a recorded member there is nothing outside the
		// aggregate could hold, whichever declaration is the root.
		if len(agg.Entities) == 0 {
			continue
		}
		types := cc.aggregateTypes(agg)
		unresolved = append(unresolved, cc.ambiguousTypes(agg)...)
		if len(types) < 2 {
			continue
		}
		// Any file could hold a member; one that cannot be read leaves
		// the aggregate undetermined.
		unresolved = append(unresolved, gapsAt(gaps, func(fileGap) bool { return true })...)
		root, members := types[0], types[1:]
		for _, field := range fields {
			if _, inside := ownerOf(types, field.file, field.decl.Owner); inside {
				continue
			}
			member, found, open := heldBy(field, members)
			if found {
				v, err := newViolation(r, subject, field.file.path, field.decl.StartLine,
					fmt.Sprintf("aggregate %s: %s holds %s, a member of aggregate %s; outside the aggregate only its root is held", agg.Name, fieldWhat(field), member.decl.Name, agg.Name),
					fmt.Sprintf("hold the root %s or its identity, and reach %s through the root within one operation", root.decl.Name, member.decl.Name))
				if err != nil {
					return nil, nil, err
				}
				vs = append(vs, v)
				continue
			}
			for _, ref := range open {
				unresolved = append(unresolved, unresolvedAt(field.file.path, field.decl.StartLine, fieldWhat(field), ref))
			}
		}
	}
	return vs, unresolved, nil
}

// checkServiceNotHeldByAggregates judges that no field of an
// aggregate's root or member holds a recorded domain service.
func checkServiceNotHeldByAggregates(r rule.Rule, subject rule.Subject, cc contextCode, code domainCode) ([]Violation, []string, error) {
	var services []locDecl
	var vs []Violation
	var unresolved []string
	for _, c := range code.contexts {
		for _, svc := range c.ctx.Services {
			loc := c.terms[svc.Name]
			switch {
			case loc.located():
				services = append(services, loc.decl())
			case loc.ambiguous():
				// Any of the declarations may be the service.
				unresolved = append(unresolved, fmt.Sprintf("domain service %s is spelled by %s, so a field holding it cannot be recognized", svc.Name, candidateList(loc.candidates)))
			}
		}
	}
	if len(services) == 0 {
		return nil, unresolved, nil
	}
	fields, gaps := cc.idx.resolvedFields()
	for _, agg := range cc.ctx.Aggregates {
		types := cc.aggregateTypes(agg)
		unresolved = append(unresolved, cc.ambiguousTypes(agg)...)
		unresolved = append(unresolved, gapsAt(gaps, func(g fileGap) bool { return mayHoldMembersOf(g, types) })...)
		for _, field := range fields {
			holder, ok := ownerOf(types, field.file, field.decl.Owner)
			if !ok {
				continue
			}
			svc, found, open := heldBy(field, services)
			if found {
				v, err := newViolation(r, subject, field.file.path, field.decl.StartLine,
					fmt.Sprintf("aggregate %s: %s of %s holds the domain service %s; a service reaches an aggregate only as a parameter of one command", agg.Name, fieldWhat(field), holder.decl.Name, svc.decl.Name),
					fmt.Sprintf("remove the field and pass %s to the command of %s that needs it", svc.decl.Name, holder.decl.Name))
				if err != nil {
					return nil, nil, err
				}
				vs = append(vs, v)
				continue
			}
			for _, ref := range open {
				unresolved = append(unresolved, unresolvedAt(field.file.path, field.decl.StartLine, fieldWhat(field)+" of aggregate "+agg.Name, ref))
			}
		}
	}
	return vs, unresolved, nil
}

// changeableParts lists the recorded value objects an identity type
// holds in its fields that are no aggregate's or member's identity, in
// field order: a name or a status the identity is composed of. Each is
// a part the recording has to settle, since a part that can change
// while the entity stays the same is not part of its identity.
func (cc contextCode) changeableParts(id locDecl, fields []locDecl) []string {
	identities := map[string]bool{}
	for _, agg := range cc.ctx.Aggregates {
		identities[agg.Identity] = true
		for _, e := range agg.Entities {
			identities[e.Identity] = true
		}
	}
	var parts []string
	seen := map[string]bool{}
	for _, field := range fields {
		if !id.owns(field.file, field.decl.Owner) {
			continue
		}
		for _, vo := range cc.ctx.ValueObjects {
			if identities[vo.Name] || seen[vo.Name] {
				continue
			}
			if loc := cc.terms[vo.Name]; loc.located() && loc.decl().declaredBy(field.decl.Type) {
				seen[vo.Name] = true
				parts = append(parts, vo.Name)
			}
		}
	}
	return parts
}

// identityRemediation says how to repair a missing identity field.
// When the identity holds parts that are not identities themselves,
// adding the field is right only if none of them can change while the
// entity stays the same; the remediation names them and puts that
// question first, because the finding cannot tell which is the case.
func identityRemediation(entity, identity string, parts []string, idType, entityType string) string {
	declare := fmt.Sprintf("declare a field of type %s on %s", idType, entityType)
	if len(parts) == 0 {
		return fmt.Sprintf("%s; its identity is what tells one %s from another", declare, entity)
	}
	if len(parts) == 1 {
		return fmt.Sprintf("%s includes %s, which is not an identity. If %s can change while the %s stays the same, it is not part of the identity; change the identity before adding a field. If it cannot change, %s.",
			identity, parts[0], parts[0], entity, declare)
	}
	list := strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
	return fmt.Sprintf("%s includes %s, which are not identities. If any of them can change while the %s stays the same, it is not part of the identity; change the identity before adding a field. If none can change, %s.",
		identity, list, entity, declare)
}

// checkEntityIdentity judges that every entity with a recorded
// identity, an aggregate's root or a member, declares a field holding
// that identity.
func checkEntityIdentity(r rule.Rule, subject rule.Subject, cc contextCode, _ domainCode) ([]Violation, []string, error) {
	var vs []Violation
	var unresolved []string
	fields, gaps := cc.idx.resolvedFields()
	judge := func(agg vocab.Aggregate, entity, identity string, line int, decl locDecl) error {
		idLoc := cc.terms[identity]
		if idLoc.ambiguous() {
			anchor, at := cc.recordAnchor(line)
			unresolved = append(unresolved, fmt.Sprintf("%s:%d: identity %s of %s is spelled by %s, so the field holding it cannot be recognized", anchor, at, identity, entity, candidateList(idLoc.candidates)))
			return nil
		}
		if !idLoc.located() {
			// A file of the context that could not be read may declare
			// the identity, so no declaration is proven missing.
			if unread := gapsAt(cc.idx.sourceGaps(), func(fileGap) bool { return true }); len(unread) > 0 {
				unresolved = append(unresolved, unread...)
				return nil
			}
			v, err := newViolation(r, subject, decl.file.path, decl.decl.StartLine,
				fmt.Sprintf("aggregate %s: %s declares no field holding its identity %s; no type in the code spells %s", agg.Name, entity, identity, identity),
				fmt.Sprintf("declare the type %s and a field of it on %s; its identity is what tells one %s from another", identity, decl.decl.Name, entity))
			if err != nil {
				return err
			}
			vs = append(vs, v)
			return nil
		}
		id := idLoc.decl()
		var open []string
		for _, field := range fields {
			if !decl.owns(field.file, field.decl.Owner) {
				continue
			}
			// The field's own type is the identity; one that merely
			// holds identities, such as a list of them, is not.
			typ := field.decl.Type
			if id.declaredBy(typ) {
				return nil
			}
			if typ.Resolution == ResolutionUndetermined {
				open = append(open, unresolvedAt(field.file.path, field.decl.StartLine, fieldWhat(field), typ))
			}
		}
		open = append(open, gapsAt(gaps, func(g fileGap) bool { return mayHoldMembersOf(g, []locDecl{decl}) })...)
		if len(open) > 0 {
			unresolved = append(unresolved, open...)
			return nil
		}
		v, err := newViolation(r, subject, decl.file.path, decl.decl.StartLine,
			fmt.Sprintf("aggregate %s: %s declares no field holding its identity %s", agg.Name, entity, identity),
			identityRemediation(entity, identity, cc.changeableParts(id, fields), id.decl.Name, decl.decl.Name))
		if err != nil {
			return err
		}
		vs = append(vs, v)
		return nil
	}
	for _, agg := range cc.ctx.Aggregates {
		ac := cc.aggregates[agg.Name]
		if ac.located() && agg.Identity != "" {
			if err := judge(agg, agg.Name, agg.Identity, agg.Line, ac.root()); err != nil {
				return nil, nil, err
			}
		}
		if ac := cc.aggregates[agg.Name]; len(ac.candidates) > 1 && agg.Identity != "" {
			unresolved = append(unresolved, fmt.Sprintf("root of aggregate %s is spelled by %s, so the field holding its identity cannot be recognized", agg.Name, candidateList(ac.candidates)))
		}
		for _, e := range agg.Entities {
			loc := cc.terms[e.Name]
			if e.Identity != "" && loc.ambiguous() {
				unresolved = append(unresolved, fmt.Sprintf("member %s of aggregate %s is spelled by %s, so the field holding its identity cannot be recognized", e.Name, agg.Name, candidateList(loc.candidates)))
				continue
			}
			if e.Identity == "" || !loc.located() {
				continue
			}
			if err := judge(agg, e.Name, e.Identity, e.Line, loc.decl()); err != nil {
				return nil, nil, err
			}
		}
	}
	return vs, unresolved, nil
}

// checkRepositoryNotCalledFromAggregates judges that no method of an
// aggregate's root or member calls a repository: neither a method a
// recorded repository declares nor any method called on a value whose
// type is a recorded repository.
func checkRepositoryNotCalledFromAggregates(r rule.Rule, subject rule.Subject, cc contextCode, code domainCode) ([]Violation, []string, error) {
	var repos []locDecl
	var unresolved []string
	for _, c := range code.contexts {
		for _, agg := range c.ctx.Aggregates {
			if agg.Repository == "" {
				continue
			}
			loc := c.terms[agg.Repository]
			switch {
			case loc.located():
				repos = append(repos, loc.decl())
			case loc.ambiguous():
				// Any of the declarations may be the repository.
				unresolved = append(unresolved, fmt.Sprintf("repository %s is spelled by %s, so a call reaching it cannot be recognized", agg.Repository, candidateList(loc.candidates)))
			}
		}
	}
	if len(repos) == 0 {
		return nil, unresolved, nil
	}
	var vs []Violation
	gaps := cc.idx.sourceGaps()
	for _, f := range cc.idx.files {
		if !f.resolvedCalls && !isTestFile(f.path) {
			gaps = append(gaps, fileGap{path: f.path, reason: "its resolved calls are not available"})
		}
	}
	for _, agg := range cc.ctx.Aggregates {
		types := cc.aggregateTypes(agg)
		unresolved = append(unresolved, cc.ambiguousTypes(agg)...)
		unresolved = append(unresolved, gapsAt(gaps, func(g fileGap) bool { return mayHoldMembersOf(g, types) })...)
		for _, f := range cc.idx.files {
			if !f.resolvedCalls || isTestFile(f.path) {
				continue
			}
			for _, c := range f.calls {
				caller, ok := ownerOf(types, f, c.EnclosingOwner)
				if !ok {
					continue
				}
				repo, found := calledRepository(c, repos)
				if found {
					v, err := newViolation(r, subject, f.path, c.Line,
						fmt.Sprintf("aggregate %s: %s.%s calls %s on the repository %s; an aggregate never calls a repository", agg.Name, caller.decl.Name, c.Enclosing, c.Callee, repo.decl.Name),
						fmt.Sprintf("load and save %s through %s in the application service that coordinates the operation, and pass %s what it needs", agg.Name, repo.decl.Name, caller.decl.Name))
					if err != nil {
						return nil, nil, err
					}
					vs = append(vs, v)
					continue
				}
				what := fmt.Sprintf("call %s in %s.%s", c.Callee, caller.decl.Name, c.Enclosing)
				if c.Target.Resolution == ResolutionUndetermined {
					unresolved = append(unresolved, unresolvedAt(f.path, c.Line, what, c.Target))
				} else if c.Receiver.Resolution == ResolutionUndetermined {
					unresolved = append(unresolved, unresolvedAt(f.path, c.Line, what, c.Receiver))
				}
			}
		}
	}
	return vs, unresolved, nil
}

// calledRepository finds the repository a call reaches, through the
// method it calls or the value it calls it on. A call whose target is
// the repository type itself, a Go conversion such as
// Repository(value), runs none of its operations.
func calledRepository(c Call, repos []locDecl) (locDecl, bool) {
	for _, repo := range repos {
		if c.Target.Owner != "" && repo.declaredBy(c.Target) || repo.declaredBy(c.Receiver) {
			return repo, true
		}
	}
	return locDecl{}, false
}
