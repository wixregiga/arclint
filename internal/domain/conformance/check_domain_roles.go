package conformance

import (
	"fmt"
	"path"

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

// key identifies the located declaration by its file, line, and name.
func (d locDecl) key() string {
	return fmt.Sprintf("%s:%d:%s", d.file.path, d.decl.StartLine, d.decl.Name)
}

// owns reports whether a member declared in file f under owner belongs
// to the located type: in Go a method or field belongs to the type of
// its name in the same package, elsewhere to the class of its name in
// the same file.
func (d locDecl) owns(f fileFacts, owner string) bool {
	if owner != d.decl.Name {
		return false
	}
	if f.lang == rule.LanguageGo {
		return path.Dir(f.path) == path.Dir(d.file.path)
	}
	return f.path == d.file.path
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
// field types, outside test files.
func (idx contractIndex) resolvedFields() []locDecl {
	var out []locDecl
	for _, f := range idx.files {
		if !f.fieldTypes || isTestFile(f.path) {
			continue
		}
		for _, d := range f.decls {
			if d.Kind == declKindField {
				out = append(out, locDecl{file: f, decl: d})
			}
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

// fieldRootReferences finds a field of one aggregate's root or member
// that holds another aggregate's root.
func fieldRootReferences(r rule.Rule, subject rule.Subject, cc contextCode, code domainCode) ([]Violation, []string, error) {
	var vs []Violation
	var unresolved []string
	fields := cc.idx.resolvedFields()
	for _, agg := range cc.ctx.Aggregates {
		types := cc.aggregateTypes(agg)
		if len(types) == 0 {
			continue
		}
		var others []locDecl
		byDecl := map[string]aggregateRoot{}
		for _, other := range code.roots {
			if other.root.key() == types[0].key() {
				continue
			}
			others = append(others, other.root)
			byDecl[other.root.key()] = other
		}
		for _, field := range fields {
			holder, ok := ownerOf(types, field.file, field.decl.Owner)
			if !ok {
				continue
			}
			held, found, open := heldBy(field, others)
			if found {
				other := byDecl[held.key()]
				v, err := newViolation(r, subject, field.file.path, field.decl.StartLine,
					fmt.Sprintf("aggregate %s: %s of %s holds %s, the root of aggregate %s; what one aggregate needs of another it receives as an identity or a value", agg.Name, fieldWhat(field), holder.decl.Name, held.decl.Name, other.agg.Name),
					fmt.Sprintf("hold %s's identity (%s) instead of its root", other.agg.Name, other.agg.Identity))
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

// checkRootOnlyReferences judges that nothing outside an aggregate
// holds one of its members: a field declared on any type that is not
// the aggregate's root or one of its members, anywhere in the
// supplied code, never holds a member.
func checkRootOnlyReferences(r rule.Rule, subject rule.Subject, cc contextCode, code domainCode) ([]Violation, []string, error) {
	var vs []Violation
	var unresolved []string
	fields := code.all.resolvedFields()
	for _, agg := range cc.ctx.Aggregates {
		types := cc.aggregateTypes(agg)
		if len(types) < 2 {
			continue
		}
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
	for _, c := range code.contexts {
		for _, svc := range c.ctx.Services {
			if loc := c.terms[svc.Name]; loc.located() {
				services = append(services, loc.decl())
			}
		}
	}
	var vs []Violation
	var unresolved []string
	if len(services) == 0 {
		return nil, nil, nil
	}
	fields := cc.idx.resolvedFields()
	for _, agg := range cc.ctx.Aggregates {
		types := cc.aggregateTypes(agg)
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

// checkEntityIdentity judges that every entity with a recorded
// identity, an aggregate's root or a member, declares a field holding
// that identity.
func checkEntityIdentity(r rule.Rule, subject rule.Subject, cc contextCode, _ domainCode) ([]Violation, []string, error) {
	var vs []Violation
	var unresolved []string
	fields := cc.idx.resolvedFields()
	judge := func(agg vocab.Aggregate, entity, identity string, line int, decl locDecl) error {
		idLoc := cc.terms[identity]
		if !idLoc.located() {
			anchor, at := cc.recordAnchor(line)
			unresolved = append(unresolved, fmt.Sprintf("%s:%d: identity %s of %s is not located in the code, so the field holding it cannot be recognized", anchor, at, identity, entity))
			return nil
		}
		id := idLoc.decl()
		var open []string
		for _, field := range fields {
			if !decl.owns(field.file, field.decl.Owner) {
				continue
			}
			_, found, refs := heldBy(field, []locDecl{id})
			if found {
				return nil
			}
			for _, ref := range refs {
				open = append(open, unresolvedAt(field.file.path, field.decl.StartLine, fieldWhat(field), ref))
			}
		}
		if len(open) > 0 {
			unresolved = append(unresolved, open...)
			return nil
		}
		v, err := newViolation(r, subject, decl.file.path, decl.decl.StartLine,
			fmt.Sprintf("aggregate %s: %s declares no field holding its identity %s", agg.Name, entity, identity),
			fmt.Sprintf("declare a field of type %s on %s; its identity is what tells one %s from another", id.decl.Name, decl.decl.Name, entity))
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
		for _, e := range agg.Entities {
			loc := cc.terms[e.Name]
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
	for _, c := range code.contexts {
		for _, agg := range c.ctx.Aggregates {
			if agg.Repository == "" {
				continue
			}
			if loc := c.terms[agg.Repository]; loc.located() {
				repos = append(repos, loc.decl())
			}
		}
	}
	if len(repos) == 0 {
		return nil, nil, nil
	}
	var vs []Violation
	var unresolved []string
	for _, agg := range cc.ctx.Aggregates {
		types := cc.aggregateTypes(agg)
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
// method it calls or the value it calls it on.
func calledRepository(c Call, repos []locDecl) (locDecl, bool) {
	for _, repo := range repos {
		if repo.declaredBy(c.Target) || repo.declaredBy(c.Receiver) {
			return repo, true
		}
	}
	return locDecl{}, false
}
