package typescript

import (
	"regexp"
	"sort"
	"strings"
)

// The extractor is lexer-grade by design (multi-language-rule-engines.md
// §4): comments are blanked, string and template literal bodies are masked
// so no keyword can fire inside them, then the static import forms are
// matched structurally. Computed specifiers (import(expr), require(v),
// template-literal specifiers) are the documented false-negative class:
// they never match a masked literal and are intentionally not extracted.

// maskedSource is the scan buffer: comments blanked with spaces, string
// bodies replaced by NUL runs (delimiters kept), plus the original literal
// values keyed by the offset of their opening quote.
type maskedSource struct {
	src      string
	masked   []byte
	literals map[int]string
}

// jsKeywordBeforeRegex lists tokens after which a `/` starts a regex
// literal, not division (the standard lexer heuristic).
var jsKeywordBeforeRegex = map[string]bool{
	"return": true, "typeof": true, "case": true, "in": true, "of": true,
	"new": true, "delete": true, "void": true, "instanceof": true,
	"do": true, "else": true, "yield": true, "await": true, "throw": true,
}

func mask(src string) *maskedSource {
	m := &maskedSource{src: src, masked: []byte(src), literals: map[int]string{}}
	n := len(src)
	i := 0

	// lastSig tracks the previous significant character to disambiguate
	// regex literals from division; lastWord the previous identifier.
	var lastSig byte
	var lastWord string

	// Template interpolation nests arbitrarily: a stack of brace depths,
	// one entry per open template literal.
	var templateStack []int

	blank := func(from, to int) {
		for k := from; k < to && k < n; k++ {
			if m.masked[k] != '\n' {
				m.masked[k] = ' '
			}
		}
	}
	maskBody := func(from, to int) {
		for k := from; k < to && k < n; k++ {
			if m.masked[k] != '\n' {
				m.masked[k] = 0
			}
		}
	}

	// Shebang.
	if strings.HasPrefix(src, "#!") {
		end := strings.IndexByte(src, '\n')
		if end < 0 {
			end = n
		}
		blank(0, end)
		i = end
	}

	for i < n {
		c := src[i]
		switch {
		case c == '/' && i+1 < n && src[i+1] == '/':
			end := strings.IndexByte(src[i:], '\n')
			if end < 0 {
				end = n - i
			}
			blank(i, i+end)
			i += end
			continue
		case c == '/' && i+1 < n && src[i+1] == '*':
			end := strings.Index(src[i+2:], "*/")
			stop := n
			if end >= 0 {
				stop = i + 2 + end + 2
			}
			blank(i, stop)
			i = stop
			continue
		case c == '\'' || c == '"':
			start := i
			i++
			for i < n && src[i] != c && src[i] != '\n' {
				if src[i] == '\\' {
					i++
				}
				i++
			}
			var value string
			if i < n && src[i] == c {
				value = src[start+1 : i]
				i++
			} else {
				value = src[start+1 : min(i, n)]
			}
			m.literals[start] = unescapeJS(value)
			maskBody(start+1, i-1)
			lastSig = c
			lastWord = ""
			continue
		case c == '`':
			start := i
			i++
			for i < n {
				if src[i] == '\\' {
					i += 2
					continue
				}
				if src[i] == '`' {
					break
				}
				if src[i] == '$' && i+1 < n && src[i+1] == '{' {
					// Interpolation: mask up to here, push template state,
					// and return to code scanning.
					maskBody(start+1, i)
					templateStack = append(templateStack, 0)
					i += 2
					lastSig = '{'
					lastWord = ""
					break
				}
				i++
			}
			if len(templateStack) > 0 && templateStack[len(templateStack)-1] == 0 && i < n && src[i-1] == '{' {
				continue // scanning interpolation code now
			}
			if i < n && src[i] == '`' {
				maskBody(start+1, i)
				i++
			} else if i >= n {
				maskBody(start+1, n)
			}
			lastSig = '`'
			lastWord = ""
			continue
		case c == '{' && len(templateStack) > 0:
			templateStack[len(templateStack)-1]++
			lastSig = c
			lastWord = ""
			i++
			continue
		case c == '}' && len(templateStack) > 0:
			depth := templateStack[len(templateStack)-1]
			if depth == 0 {
				// End of interpolation: resume the template literal scan.
				templateStack = templateStack[:len(templateStack)-1]
				i++
				start := i
				for i < n {
					if src[i] == '\\' {
						i += 2
						continue
					}
					if src[i] == '`' {
						break
					}
					if src[i] == '$' && i+1 < n && src[i+1] == '{' {
						maskBody(start, i)
						templateStack = append(templateStack, 0)
						i += 2
						break
					}
					i++
				}
				if i < n && src[i] == '`' {
					maskBody(start, i)
					i++
					lastSig = '`'
				}
				continue
			}
			templateStack[len(templateStack)-1]--
			lastSig = c
			lastWord = ""
			i++
			continue
		case c == '/':
			// Regex literal vs division.
			if lastSig == 0 || strings.ContainsRune("(=:[!&|?{};,+-*%~^<>", rune(lastSig)) || jsKeywordBeforeRegex[lastWord] {
				start := i
				i = scanRegexLiteral(src, i)
				maskBody(start+1, i)
				lastSig = '/'
				lastWord = ""
				continue
			}
			lastSig = c
			lastWord = ""
			i++
			continue
		default:
			if isIdentChar(c) {
				start := i
				for i < n && isIdentChar(src[i]) {
					i++
				}
				lastWord = src[start:i]
				lastSig = src[i-1]
				continue
			}
			if !isSpace(c) {
				lastSig = c
				lastWord = ""
			}
			i++
		}
	}
	return m
}

// scanRegexLiteral consumes a regex literal from its opening slash and
// returns the index just past the closing slash and flags.
func scanRegexLiteral(src string, start int) int {
	i := start + 1
	n := len(src)
	inClass := false
	for i < n && src[i] != '\n' {
		if src[i] == '\\' {
			i += 2
			continue
		}
		if src[i] == '[' {
			inClass = true
		} else if src[i] == ']' {
			inClass = false
		} else if src[i] == '/' && !inClass {
			break
		}
		i++
	}
	if i < n && src[i] == '/' {
		i++
		for i < n && isIdentChar(src[i]) { // flags
			i++
		}
	}
	return i
}

func isIdentChar(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\r' || c == '\n' }

func unescapeJS(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			b.WriteByte(s[i])
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// spec matches a masked string literal: quote, NUL-run body, quote.
const specPat = `(["'][` + "\x00" + `]*["'])`

var jstsForms = []*regexp.Regexp{
	// import ... from "spec" (default, named, namespace, `import type`).
	regexp.MustCompile(`\bimport\s+(?:type\s+)?[^;'"` + "`" + `]*?\bfrom\s*` + specPat),
	// import "spec" (side effect).
	regexp.MustCompile(`\bimport\s*` + specPat),
	// export {...} from / export * from / export type {...} from.
	regexp.MustCompile(`\bexport\s+(?:type\s+)?(?:\*(?:\s+as\s+[\w$]+)?|\{[^}'"]*\})\s*from\s*` + specPat),
	// import("spec") with a literal specifier.
	regexp.MustCompile(`\bimport\s*\(\s*` + specPat + `\s*\)`),
	// require("spec") with a literal specifier, not preceded by `.` or an
	// identifier character (foo.require, myrequire).
	regexp.MustCompile(`(?:^|[^.\w$])require\s*\(\s*` + specPat + `\s*\)`),
}

type rawImport struct {
	spec string
	line int
}

// extract returns the statically-declared module specifiers of one source,
// in source order, deduplicated by (offset).
func extract(src string) []rawImport {
	m := mask(src)
	masked := string(m.masked)
	type hit struct {
		offset int
		spec   string
	}
	var hits []hit
	seen := map[int]bool{}
	for _, re := range jstsForms {
		for _, loc := range re.FindAllStringSubmatchIndex(masked, -1) {
			start := loc[2] // opening quote offset of the captured spec
			if start < 0 || seen[start] {
				continue
			}
			value, ok := m.literals[start]
			if !ok {
				continue
			}
			seen[start] = true
			hits = append(hits, hit{offset: start, spec: value})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].offset < hits[j].offset })
	out := make([]rawImport, 0, len(hits))
	for _, h := range hits {
		out = append(out, rawImport{
			spec: h.spec,
			line: 1 + strings.Count(m.src[:h.offset], "\n"),
		})
	}
	return out
}
