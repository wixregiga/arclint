package python

import (
	"regexp"
	"strings"
)

// Lexer-grade Python import extraction (multi-language-rule-engines.md
// §4): `import` and `from ... import` are simple statements legal at any
// indentation, including inside functions and conditionals, with
// parenthesized multi-line and backslash-continued forms. The documented
// false-negative class — importlib.import_module(name), __import__(name)
// — is textually a call, never matches the statement grammar, and is
// intentionally not extracted.

type rawImport struct {
	module string // dotted module; relative imports keep leading dots
	line   int
}

var (
	importStmtRe = regexp.MustCompile(`^\s*import\s+(.+)$`)
	fromStmtRe   = regexp.MustCompile(`^\s*from\s+([.]*[\w.]*)\s+import\b`)
	importItemRe = regexp.MustCompile(`^([\w.]+)(?:\s+as\s+\w+)?$`)
)

// stripLine blanks #-comments and the bodies of single-line string
// literals so neither can fake or hide a statement, and reports whether
// the line ends inside a triple-quoted string (with the active delimiter).
func stripLine(line string, inTriple string) (clean string, tripleAfter string) {
	var b strings.Builder
	i := 0
	n := len(line)
	for i < n {
		if inTriple != "" {
			end := strings.Index(line[i:], inTriple)
			if end < 0 {
				for ; i < n; i++ {
					b.WriteByte(' ')
				}
				return b.String(), inTriple
			}
			for k := 0; k < end+3; k++ {
				b.WriteByte(' ')
			}
			i += end + 3
			inTriple = ""
			continue
		}
		c := line[i]
		switch c {
		case '#':
			return b.String(), ""
		case '\'', '"':
			// String prefixes (r, b, f, u and combos) were already written
			// through; they are inert identifier characters here.
			if i+2 < n && line[i+1] == c && line[i+2] == c {
				delim := string(c) + string(c) + string(c)
				b.WriteString("   ")
				i += 3
				end := strings.Index(line[i:], delim)
				if end < 0 {
					for ; i < n; i++ {
						b.WriteByte(' ')
					}
					return b.String(), delim
				}
				for k := 0; k < end+3; k++ {
					b.WriteByte(' ')
				}
				i += end + 3
				continue
			}
			quote := c
			b.WriteByte(' ')
			i++
			for i < n {
				if line[i] == '\\' {
					b.WriteString("  ")
					i += 2
					continue
				}
				if line[i] == quote {
					b.WriteByte(' ')
					i++
					break
				}
				b.WriteByte(' ')
				i++
			}
			continue
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String(), ""
}

// extract scans one Python source for import statements.
func extract(src string) []rawImport {
	var out []rawImport
	lines := strings.Split(src, "\n")
	inTriple := ""
	i := 0
	for i < len(lines) {
		startLine := i + 1
		// stripLine blanks docstring/string bodies, so clean holds only
		// real code regardless of triple-quote state.
		clean, tripleAfter := stripLine(lines[i], inTriple)
		inTriple = tripleAfter
		i++

		// Join backslash continuations into one logical line.
		logical := clean
		for strings.HasSuffix(strings.TrimRight(logical, " \t"), "\\") && i < len(lines) {
			logical = strings.TrimSuffix(strings.TrimRight(logical, " \t"), "\\")
			next, nextTriple := stripLine(lines[i], inTriple)
			inTriple = nextTriple
			i++
			logical += " " + strings.TrimSpace(next)
		}
		// `from x import (` may span lines, but the module name always
		// sits before `import` on the first logical segment, so no
		// paren-joining is needed for extraction.

		for _, stmt := range strings.Split(logical, ";") {
			if m := fromStmtRe.FindStringSubmatch(stmt); m != nil {
				if m[1] != "" {
					out = append(out, rawImport{module: m[1], line: startLine})
				}
				continue
			}
			if m := importStmtRe.FindStringSubmatch(stmt); m != nil {
				for _, item := range strings.Split(m[1], ",") {
					item = strings.TrimSpace(item)
					if im := importItemRe.FindStringSubmatch(item); im != nil {
						out = append(out, rawImport{module: im[1], line: startLine})
					}
				}
			}
		}
	}
	return out
}
