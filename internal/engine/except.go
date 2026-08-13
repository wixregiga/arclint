package engine

import (
	"fmt"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/report"
)

// exceptIndex maps rule ids to their exception globs. Clauses sharing
// one explicit id contribute to one entry: suppression is id-scoped,
// matching how findings report. Id derivation mirrors Instances().
func exceptIndex(rs *config.RuleSet) map[string][]config.ExceptRule {
	idx := map[string][]config.ExceptRule{}
	add := func(id string, excepts []config.ExceptRule) {
		idx[id] = append(idx[id], excepts...)
	}
	for m, c := range rs.Contracts {
		if c.Consumes != nil && len(c.Consumes.Except) > 0 {
			id := c.Consumes.ID
			if id == "" {
				id = m + ".consumes"
			}
			add(id, c.Consumes.Except)
		}
		for i, r := range c.Provides {
			if len(r.Except) == 0 {
				continue
			}
			id := r.ID
			if id == "" {
				id = fmt.Sprintf("%s.provides.%s[%d]", m, r.Kind, i)
			}
			add(id, r.Except)
		}
		for i, r := range c.Invariants {
			if len(r.Except) == 0 {
				continue
			}
			id := r.ID
			if id == "" {
				id = fmt.Sprintf("%s.invariants.%s[%d]", m, r.Kind, i)
			}
			add(id, r.Except)
		}
	}
	for i, r := range rs.Dependencies {
		if len(r.Except) == 0 {
			continue
		}
		id := r.ID
		if id == "" {
			id = fmt.Sprintf("dependencies.%s[%d]", r.Kind, i)
		}
		add(id, r.Except)
	}
	for i, r := range rs.Rules {
		if len(r.Except) == 0 {
			continue
		}
		id := r.ID
		if id == "" {
			id = fmt.Sprintf("rules.%s[%d]", r.Type, i)
		}
		add(id, r.Except)
	}
	return idx
}

// applyExcepts splits findings whose anchor path matches an except glob
// declared for their rule id away from the kept set. Suppressed findings
// are returned marked with the matching entry's reason, so output can
// show what was omitted and why; the rule keeps firing for every other
// anchor.
func applyExcepts(rs *config.RuleSet, vs []report.Violation) (kept, suppressed []report.Violation) {
	idx := exceptIndex(rs)
	if len(idx) == 0 {
		return vs, nil
	}
	entriesFor := func(id string) []config.ExceptRule {
		if entries, ok := idx[id]; ok {
			return entries
		}
		// Derived consumes ids carry a per-aspect suffix; the clause
		// (and its except list) is <module>.consumes.
		for _, aspect := range []string{".internal", ".external", ".stdlib"} {
			if trimmed, ok := strings.CutSuffix(id, aspect); ok {
				return idx[trimmed]
			}
		}
		return nil
	}
	kept = make([]report.Violation, 0, len(vs))
	for _, v := range vs {
		reason, matched := "", false
		for _, e := range entriesFor(v.RuleID) {
			for _, g := range e.Paths {
				if ok, _ := doublestar.Match(g, v.Path); ok {
					reason, matched = e.Reason, true
					break
				}
			}
			if matched {
				break
			}
		}
		if matched {
			v.Suppressed = true
			v.SuppressedReason = reason
			suppressed = append(suppressed, v)
			continue
		}
		kept = append(kept, v)
	}
	return kept, suppressed
}
