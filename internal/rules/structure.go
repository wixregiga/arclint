package rules

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/walk"
)

// compileStructure builds the structure evaluator (rules.md §5.1):
// every `require` glob must match at least one file in the whole tree
// (minus global excludes — rule file targeting does not apply to require);
// `forbid` globs must match no file within the rule's file targeting.
func compileStructure(id string, r config.Rule) ruleFunc {
	p := r.Structure
	return func(c *evalCtx) ([]Violation, error) {
		var vs []Violation

		for _, pat := range p.Require {
			found := false
			for _, f := range c.paths {
				if walk.Match(pat, f) {
					found = true
					break
				}
			}
			if !found {
				vs = append(vs, Violation{
					RuleID:   id,
					Category: r.Type,
					Severity: r.Severity,
					Path:     pat,
					Message:  fmt.Sprintf("missing required file: no file matches %s", pat),
					FixHint:  r.FixHint,
				})
			}
		}

		scope := targeted(c.paths, r.Files)
		for _, pat := range p.Forbid {
			for _, f := range scope {
				if walk.Match(pat, f) {
					vs = append(vs, Violation{
						RuleID:   id,
						Category: r.Type,
						Severity: r.Severity,
						Path:     f,
						Message:  fmt.Sprintf("file matches forbidden pattern %s", pat),
						FixHint:  r.FixHint,
					})
				}
			}
		}
		return vs, nil
	}
}
