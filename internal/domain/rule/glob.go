package rule

import (
	"errors"
	"fmt"
	"strings"
)

// Glob is a validated repo-relative path pattern used by Applicability,
// Module membership, Rule Exclusions, and structure Rules. Matching is
// segment-wise: `*` and `?` never cross `/`, a segment consisting solely
// of `**` matches any number of whole segments including none, and
// `[...]` matches one character by set or range (negate with `^` or
// `!`). A backslash escapes the following character. Brace alternation
// is rejected at construction: an unsupported pattern must fail loudly
// rather than silently select nothing.
type Glob struct {
	pattern  string
	segments []string
}

// NewGlob validates and compiles one pattern.
func NewGlob(pattern string) (Glob, error) {
	if pattern == "" {
		return Glob{}, errors.New("glob: empty pattern")
	}
	if strings.ContainsAny(pattern, "{}") {
		return Glob{}, fmt.Errorf("glob %q: brace alternation is not supported", pattern)
	}
	segments := strings.Split(pattern, "/")
	for _, seg := range segments {
		if seg == "" {
			return Glob{}, fmt.Errorf("glob %q: empty path segment", pattern)
		}
		if err := validateSegment(seg); err != nil {
			return Glob{}, fmt.Errorf("glob %q: %v", pattern, err)
		}
	}
	return Glob{pattern: pattern, segments: segments}, nil
}

// NewGlobs validates a list of patterns.
func NewGlobs(patterns []string) ([]Glob, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	out := make([]Glob, 0, len(patterns))
	for _, p := range patterns {
		g, err := NewGlob(p)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

// IsZero reports an unconstructed Glob, which matches nothing.
func (g Glob) IsZero() bool { return g.pattern == "" }

func (g Glob) String() string { return g.pattern }

// Match reports whether a repo-relative path (forward slashes, no
// leading "./") satisfies the pattern.
func (g Glob) Match(path string) bool {
	if g.IsZero() {
		return false
	}
	return matchSegments(g.segments, strings.Split(path, "/"))
}

// MatchesSubtree reports whether the path satisfies the pattern
// directly, or lies inside a directory the pattern names. This is the
// Module-membership convenience: a glob that names a directory claims
// the directory's whole subtree.
func (g Glob) MatchesSubtree(path string) bool {
	if g.IsZero() {
		return false
	}
	parts := strings.Split(path, "/")
	if matchSegments(g.segments, parts) {
		return true
	}
	sub := make([]string, len(g.segments)+1)
	copy(sub, g.segments)
	sub[len(g.segments)] = "**"
	return matchSegments(sub, parts)
}

// matchSegments matches a segment-split pattern against a segment-split
// path. `**` consumes zero or more whole segments.
func matchSegments(pat, parts []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			if matchSegments(pat[1:], parts) {
				return true
			}
			if len(parts) == 0 {
				return false
			}
			parts = parts[1:]
			continue
		}
		if len(parts) == 0 {
			return false
		}
		if !matchSegment(pat[0], parts[0]) {
			return false
		}
		pat = pat[1:]
		parts = parts[1:]
	}
	return len(parts) == 0
}

// matchSegment matches one pattern segment against one path segment.
// A run of stars inside a segment (including a mid-segment `**`)
// collapses to one `*`.
func matchSegment(pattern, name string) bool {
	p := []rune(pattern)
	n := []rune(name)
	i, j := 0, 0
	starI, starJ := -1, -1
	for j < len(n) {
		matched := false
		if i < len(p) {
			switch p[i] {
			case '*':
				for i < len(p) && p[i] == '*' {
					i++
				}
				starI, starJ = i, j
				continue
			case '?':
				i++
				j++
				matched = true
			case '[':
				ok, next := matchClass(p, i, n[j])
				if ok {
					i = next
					j++
					matched = true
				}
			case '\\':
				if i+1 < len(p) && p[i+1] == n[j] {
					i += 2
					j++
					matched = true
				}
			default:
				if p[i] == n[j] {
					i++
					j++
					matched = true
				}
			}
		}
		if matched {
			continue
		}
		if starI >= 0 {
			starJ++
			i, j = starI, starJ
			continue
		}
		return false
	}
	for i < len(p) && p[i] == '*' {
		i++
	}
	return i == len(p)
}

// matchClass matches one rune against the character class starting at
// p[start] (which is '['). It returns whether the rune matched and the
// index just past the closing ']'. Construction validation guarantees
// the class is well formed.
func matchClass(p []rune, start int, r rune) (bool, int) {
	j := start + 1
	negate := false
	if j < len(p) && (p[j] == '^' || p[j] == '!') {
		negate = true
		j++
	}
	matched := false
	first := true
	for j < len(p) && (p[j] != ']' || first) {
		first = false
		lo := p[j]
		if lo == '\\' && j+1 < len(p) {
			j++
			lo = p[j]
		}
		hi := lo
		if j+2 < len(p) && p[j+1] == '-' && p[j+2] != ']' {
			j += 2
			hi = p[j]
			if hi == '\\' && j+1 < len(p) {
				j++
				hi = p[j]
			}
		}
		if r >= lo && r <= hi {
			matched = true
		}
		j++
	}
	if j >= len(p) {
		return false, start + 1
	}
	return matched != negate, j + 1
}

// validateSegment rejects dangling escapes and unclosed or empty
// character classes.
func validateSegment(seg string) error {
	r := []rune(seg)
	for i := 0; i < len(r); i++ {
		switch r[i] {
		case '\\':
			if i+1 >= len(r) {
				return errors.New("dangling escape")
			}
			i++
		case '[':
			j := i + 1
			if j < len(r) && (r[j] == '^' || r[j] == '!') {
				j++
			}
			entries := 0
			for j < len(r) && (r[j] != ']' || entries == 0) {
				if r[j] == '\\' {
					j++
					if j >= len(r) {
						return errors.New("dangling escape in character class")
					}
				}
				j++
				entries++
			}
			if j >= len(r) || entries == 0 {
				return errors.New("unclosed or empty character class")
			}
			i = j
		}
	}
	return nil
}
