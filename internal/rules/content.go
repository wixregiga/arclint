package rules

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/jofyi/arclint/internal/config"
)

// compiledMatcher is one content pattern with its optional message
// override, compiled once up front.
type compiledMatcher struct {
	re      *regexp.Regexp
	pattern string
	message string
}

func compileMatchers(id, kind string, ms []config.ContentMatcher) ([]compiledMatcher, error) {
	var out []compiledMatcher
	var errs []error
	for _, m := range ms {
		re, err := regexp.Compile(m.Pattern)
		if err != nil {
			errs = append(errs, fmt.Errorf("rule %q: %s pattern %q does not compile as RE2 — %v", id, kind, m.Pattern, err))
			continue
		}
		out = append(out, compiledMatcher{re: re, pattern: m.Pattern, message: m.Message})
	}
	return out, errors.Join(errs...)
}

// compileContent builds the content evaluator (rules.md §5.4):
// mustNotContain — no targeted file may match, one line-anchored violation
// per matching line; mustContain — every targeted file must match at least
// once, violation carries no line.
func compileContent(id string, r config.Rule) (ruleFunc, error) {
	must, err1 := compileMatchers(id, "mustContain", r.Content.MustContain)
	mustNot, err2 := compileMatchers(id, "mustNotContain", r.Content.MustNotContain)
	if err := errors.Join(err1, err2); err != nil {
		return nil, err
	}

	return func(c *evalCtx) ([]Violation, error) {
		scope := targeted(c.paths, r.Files)
		return forFiles(scope, func(rel string) ([]Violation, error) {
			data, err := c.read(rel)
			if err != nil {
				return nil, fmt.Errorf("rule %q: cannot read %s — %v", id, rel, err)
			}
			content := string(data)
			var vs []Violation

			if len(mustNot) > 0 {
				lines := splitLines(content)
				for _, m := range mustNot {
					for i, line := range lines {
						if !m.re.MatchString(line) {
							continue
						}
						msg := m.message
						if msg == "" {
							msg = fmt.Sprintf("line matches forbidden pattern %s", m.pattern)
						}
						ln := i + 1
						vs = append(vs, Violation{
							RuleID:   id,
							Category: r.Type,
							Severity: r.Severity,
							Path:     rel,
							Line:     &ln,
							Message:  msg,
							FixHint:  r.FixHint,
						})
					}
				}
			}

			for _, m := range must {
				if m.re.MatchString(content) {
					continue
				}
				msg := m.message
				if msg == "" {
					msg = fmt.Sprintf("file does not contain required pattern %s", m.pattern)
				}
				vs = append(vs, Violation{
					RuleID:   id,
					Category: r.Type,
					Severity: r.Severity,
					Path:     rel,
					Message:  msg,
					FixHint:  r.FixHint,
				})
			}
			return vs, nil
		})
	}, nil
}
