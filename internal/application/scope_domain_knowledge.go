package application

import (
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// domainScope is the worksite as the recorded domain sees it: the
// requested paths and the selected Modules. A recorded term anchors
// into the scope when a declaration named for it, or a declaration
// carrying one of its contracts, lies inside; a whole context anchors
// when a selected Module is named for it, the same match the recorded
// relations are enforced through.
type domainScope struct {
	paths   []string
	modules []rule.Module
}

func worksiteScope(cfg rule.Configured, req ContextRequest) domainScope {
	scope := domainScope{}
	for _, p := range req.Paths {
		scope.paths = append(scope.paths, strings.TrimSuffix(p, "/"))
	}
	for _, name := range req.Modules {
		for _, m := range cfg.Modules {
			if string(m.Name()) == name {
				scope.modules = append(scope.modules, m)
			}
		}
	}
	return scope
}

// contains reports whether an observed file lies inside the scope: at
// or under a requested path, or inside a selected Module.
func (s domainScope) contains(path string) bool {
	for _, p := range s.paths {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	for _, m := range s.modules {
		if m.Contains(path) {
			return true
		}
	}
	return false
}

// containsSource reports whether a file:line contract source lies
// inside the scope.
func (s domainScope) containsSource(source string) bool {
	if source == "" {
		return false
	}
	path := source
	if i := strings.LastIndex(source, ":"); i >= 0 {
		path = source[:i]
	}
	return s.contains(path)
}

// namesContext reports whether a selected Module is named for the
// context: both names rendered to flatcase agree.
func (s domainScope) namesContext(name string) bool {
	want, err := rule.CaseTerm(name, "flatcase")
	if err != nil {
		return false
	}
	for _, m := range s.modules {
		got, err := rule.CaseTerm(string(m.Name()), "flatcase")
		if err == nil && got == want {
			return true
		}
	}
	return false
}

// scopeDomainKnowledge narrows a located projection to the part that
// anchors into the scope. Contexts a selected Module is named for stay
// whole; elsewhere a term stays when a declaration named for it or
// carrying one of its contracts lies inside the scope, an invariant or
// assertion stays with its owner, and a relation stays when it touches
// a kept context. Counts keeps tallying the whole model; Shown tallies
// the narrowed listing. A projection that was never located cannot be
// narrowed and is returned whole.
func scopeDomainKnowledge(dk *DomainKnowledge, idx []declHit, scope domainScope) *DomainKnowledge {
	if dk == nil || !dk.Located {
		return dk
	}
	out := &DomainKnowledge{
		Source:  dk.Source,
		Counts:  dk.Counts,
		Scoped:  true,
		Located: dk.Located,
	}
	kept := map[string]bool{}
	for _, ctx := range dk.Contexts {
		if scope.namesContext(ctx.Name) {
			out.Contexts = append(out.Contexts, ctx)
			kept[ctx.Name] = true
			continue
		}
		narrowed, anchors := scopeContext(ctx, idx, scope)
		if anchors {
			out.Contexts = append(out.Contexts, narrowed)
			kept[ctx.Name] = true
		}
	}
	for _, rel := range dk.Relations {
		if kept[rel.From] || kept[rel.To] {
			out.Relations = append(out.Relations, rel)
		}
	}
	out.Shown = countKnowledge(out)
	return out
}

// scopeContext keeps the terms of one context that anchor into the
// scope, with the contracts their owners carry; the second result is
// false when nothing anchors.
func scopeContext(ctx DomainContextKnowledge, idx []declHit, scope domainScope) (DomainContextKnowledge, bool) {
	out := DomainContextKnowledge{Name: ctx.Name}
	owners := map[string]bool{}
	anchored := func(name string) bool {
		for _, p := range typeDeclarationPaths(idx, name) {
			if scope.contains(p) {
				return true
			}
		}
		for _, inv := range ctx.Invariants {
			if inv.Owner == name && scope.containsSource(inv.Source) {
				return true
			}
		}
		for _, a := range ctx.Assertions {
			if a.Owner == name && scope.containsSource(a.Source) {
				return true
			}
		}
		return false
	}
	for _, e := range ctx.Entities {
		if anchored(e.Name) {
			out.Entities = append(out.Entities, e)
			owners[e.Name] = true
		}
	}
	for _, v := range ctx.ValueObjects {
		if anchored(v) {
			out.ValueObjects = append(out.ValueObjects, v)
			owners[v] = true
		}
	}
	for _, inv := range ctx.Invariants {
		if owners[inv.Owner] {
			out.Invariants = append(out.Invariants, inv)
		}
	}
	for _, a := range ctx.Assertions {
		if owners[a.Owner] {
			out.Assertions = append(out.Assertions, a)
		}
	}
	for _, s := range ctx.Specifications {
		if scope.containsSource(s.Source) || anchored(s.Name) {
			out.Specifications = append(out.Specifications, s)
		}
	}
	for _, e := range ctx.Events {
		if anchored(e) {
			out.Events = append(out.Events, e)
		}
	}
	anchors := len(out.Entities)+len(out.ValueObjects)+len(out.Specifications)+len(out.Events) > 0
	return out, anchors
}

// countKnowledge tallies a projected listing the way the recorded
// model tallies itself.
func countKnowledge(dk *DomainKnowledge) vocab.Counts {
	c := vocab.Counts{Contexts: len(dk.Contexts), Relations: len(dk.Relations)}
	for _, ctx := range dk.Contexts {
		c.Entities += len(ctx.Entities)
		for _, e := range ctx.Entities {
			if e.Aggregate {
				c.Aggregates++
			}
		}
		c.ValueObjects += len(ctx.ValueObjects)
		c.Invariants += len(ctx.Invariants)
		c.Assertions += len(ctx.Assertions)
		c.Specifications += len(ctx.Specifications)
		c.Events += len(ctx.Events)
	}
	return c
}
