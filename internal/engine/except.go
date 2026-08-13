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
func exceptIndex(rs *config.RuleSet) map[string][]string {
	idx := map[string][]string{}
	add := func(id string, excepts []config.ExceptRule) {
		for _, e := range excepts {
			idx[id] = append(idx[id], e.Paths...)
		}
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

// applyExcepts drops findings whose anchor path matches an except glob
// declared for their rule id, and returns the kept findings plus the
// suppressed count. The rule keeps firing for every other anchor.
func applyExcepts(rs *config.RuleSet, vs []report.Violation) ([]report.Violation, int) {
	idx := exceptIndex(rs)
	if len(idx) == 0 {
		return vs, 0
	}
	globsFor := func(id string) []string {
		if globs, ok := idx[id]; ok {
			return globs
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
	kept := make([]report.Violation, 0, len(vs))
	suppressed := 0
	for _, v := range vs {
		matched := false
		for _, g := range globsFor(v.RuleID) {
			if ok, _ := doublestar.Match(g, v.Path); ok {
				matched = true
				break
			}
		}
		if matched {
			suppressed++
			continue
		}
		kept = append(kept, v)
	}
	return kept, suppressed
}
