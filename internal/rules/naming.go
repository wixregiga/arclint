package rules

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/config"
)

// namingTokens are the ls-lint style tokens from rules.md §5.2, compiled
// as anchored RE2 over the checked basename (extension stripped for files).
var namingTokens = map[string]string{
	"camelCase":            `^[a-z][a-z0-9]*(?:[A-Z][a-z0-9]*)*$`,
	"PascalCase":           `^(?:[A-Z][a-z0-9]*)+$`,
	"snake_case":           `^[a-z0-9]+(?:_[a-z0-9]+)*$`,
	"kebab-case":           `^[a-z0-9]+(?:-[a-z0-9]+)*$`,
	"SCREAMING_SNAKE_CASE": `^[A-Z0-9]+(?:_[A-Z0-9]+)*$`,
	"lowercase":            `^[a-z0-9]+$`,
}

// compileStyle compiles a pipe expression ("snake_case | regex:v[0-9]+")
// into a list of anchored alternatives; the name is valid if any matches.
func compileStyle(style string) ([]*regexp.Regexp, error) {
	var alts []*regexp.Regexp
	for _, alt := range strings.Split(style, "|") {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			continue
		}
		if pat, ok := strings.CutPrefix(alt, "regex:"); ok {
			re, err := regexp.Compile("^(?:" + pat + ")$")
			if err != nil {
				return nil, fmt.Errorf("naming regex %q does not compile as RE2 — %v", pat, err)
			}
			alts = append(alts, re)
			continue
		}
		pat, ok := namingTokens[alt]
		if !ok {
			return nil, fmt.Errorf("unknown naming style token %q — supported: camelCase, PascalCase, snake_case, kebab-case, SCREAMING_SNAKE_CASE, lowercase, regex:<pattern>", alt)
		}
		alts = append(alts, regexp.MustCompile(pat))
	}
	if len(alts) == 0 {
		return nil, fmt.Errorf("naming style %q has no alternatives", style)
	}
	return alts, nil
}

func matchesStyle(alts []*regexp.Regexp, name string) bool {
	for _, re := range alts {
		if re.MatchString(name) {
			return true
		}
	}
	return false
}

// stem strips the final extension off a file basename; dotfiles like
// ".gitignore" keep their full name.
func stem(base string) string {
	if e := path.Ext(base); e != "" && e != base {
		return strings.TrimSuffix(base, e)
	}
	return base
}

// compileNaming builds the naming evaluator (rules.md §5.2). Target "file"
// checks the extension-stripped basename of every targeted file; target
// "dir" checks the basename of every directory (derived from the walked
// file list) whose path matches the rule's file targeting.
func compileNaming(id string, r config.Rule) (ruleFunc, error) {
	p := r.Naming
	alts, err := compileStyle(p.Style)
	if err != nil {
		return nil, fmt.Errorf("rule %q: %v", id, err)
	}

	return func(c *evalCtx) ([]Violation, error) {
		var vs []Violation
		if p.Target == "dir" {
			for _, d := range targeted(dirsOf(c.paths), r.Files) {
				name := path.Base(d)
				if !matchesStyle(alts, name) {
					vs = append(vs, Violation{
						RuleID:   id,
						Category: r.Type,
						Severity: r.Severity,
						Path:     d,
						Message:  fmt.Sprintf("directory name %q does not match style %q", name, p.Style),
						FixHint:  r.FixHint,
					})
				}
			}
			return vs, nil
		}

		for _, f := range targeted(c.paths, r.Files) {
			name := stem(path.Base(f))
			if !matchesStyle(alts, name) {
				vs = append(vs, Violation{
					RuleID:   id,
					Category: r.Type,
					Severity: r.Severity,
					Path:     f,
					Message:  fmt.Sprintf("file name %q does not match style %q", name, p.Style),
					FixHint:  r.FixHint,
				})
			}
		}
		return vs, nil
	}, nil
}

// dirsOf derives the sorted set of directories (root-relative, slash
// separated) that contain or lead to any walked file.
func dirsOf(paths []string) []string {
	seen := make(map[string]struct{})
	for _, p := range paths {
		for d := path.Dir(p); d != "." && d != "/"; d = path.Dir(d) {
			seen[d] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}
