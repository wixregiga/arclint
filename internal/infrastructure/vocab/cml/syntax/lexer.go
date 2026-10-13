package syntax

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Error is a lexical or syntactic error at the first offending token.
type Error struct {
	File    string
	Pos     Pos
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", e.File, e.Pos.Line, e.Pos.Column, e.Message)
}

type token struct {
	kind, text, value string
	pos               Pos
	comments          []Comment
}

func isIDStart(c byte) bool { return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func isIDPart(c byte) bool  { return isIDStart(c) || c >= '0' && c <= '9' }

type lexer struct {
	file, src            string
	offset, line, column int
}

func (l *lexer) pos() Pos { return Pos{l.offset, l.line, l.column} }
func (l *lexer) advance(n int) {
	end := l.offset + n
	for l.offset < end {
		c := l.src[l.offset]
		switch c {
		case '\r':
			l.line++
			l.column = 1
			l.offset++

		case '\n':
			if l.offset == 0 || l.src[l.offset-1] != '\r' {
				l.line++
			}
			l.column = 1
			l.offset++
		default:
			_, width := utf8.DecodeRuneInString(l.src[l.offset:])
			l.offset += width
			l.column++
		}
	}
}

var literalTerminals = []struct {
	kind  string
	forms []string
}{
	{tokenOpen, []string{"{"}},
	{tokenClose, []string{"}"}},
	{tokenMap, []string{mapSpelling}},
	{tokenNot, []string{"!", "not"}},
	{tokenDelegate, []string{"=>", "delegates to"}},
	{tokenOpposite, []string{symmetricArrow, oppositeSpelling}},
	{tokenReference, []string{"-", exprReference}},
	{tokenAssociation, []string{"--", "association"}},
}

func lex(file string, source []byte, g *grammar) ([]token, error) {
	l := lexer{file: file, src: string(source), line: 1, column: 1}
	if !utf8.Valid(source) {
		for i := 0; i < len(source); {
			r, width := utf8.DecodeRune(source[i:])
			if r == utf8.RuneError && width == 1 {
				l.advance(i)
				return nil, &Error{file, l.pos(), fmt.Sprintf("expected UTF-8 source; found invalid byte 0x%02x", source[i])}
			}
			i += width
		}
	}
	var ts []token
	var comments []Comment
	keywords := make([]string, 0, len(g.keywords))
	for kw := range g.keywords {
		keywords = append(keywords, kw)
	}
	sort.Strings(keywords)

	for l.offset < len(l.src) {
		rest := l.src[l.offset:]
		pos := l.pos()
		if strings.ContainsRune(" \t\r\n", rune(rest[0])) {
			l.advance(1)
			continue
		}
		if strings.HasPrefix(rest, "//") {
			n := strings.IndexAny(rest, "\r\n")
			if n < 0 {
				n = len(rest)
			}
			comments = append(comments, Comment{rest[:n], pos})
			l.advance(n)
			continue
		}
		if strings.HasPrefix(rest, "/*") {
			n := strings.Index(rest[2:], "*/")
			if n < 0 {
				return nil, &Error{file, pos, "expected */ to close block comment; found end of file"}
			}
			n += 4
			comments = append(comments, Comment{rest[:n], pos})
			l.advance(n)
			continue
		}
		if rest[0] == '\'' || rest[0] == '"' {
			n, value, bad, message := scanString(rest)
			if message != "" {
				l.advance(bad)
				return nil, &Error{file, l.pos(), message}
			}
			ts = append(ts, token{stringLiteralKind, rest[:n], value, pos, comments})
			comments = nil
			l.advance(n)
			continue
		}

		kind, best := "", 0
		for _, kw := range keywords {
			if len(kw) > best && strings.HasPrefix(rest, kw) {
				kind = kw
				best = len(kw)
			}
		}
		for _, term := range literalTerminals {
			for _, s := range term.forms {
				if len(s) > best && strings.HasPrefix(rest, s) {
					kind = term.kind
					best = len(s)
				}
			}
		}
		n := 0
		if rest[0] == '^' {
			n++
		}
		if n < len(rest) && isIDStart(rest[n]) {
			n++
			for n < len(rest) && isIDPart(rest[n]) {
				n++
			}
			if n > best {
				kind = tokenID
				best = n
			}
		}
		if rest[0] >= '0' && rest[0] <= '9' {
			n = 1
			for n < len(rest) && rest[n] >= '0' && rest[n] <= '9' {
				n++
			}
			if n > best {
				kind = tokenINT
				best = n
			}
		}
		if best == 0 {
			_, n = utf8.DecodeRuneInString(rest)
			return nil, &Error{file, pos, fmt.Sprintf("expected CML token; found %q (ANY_OTHER)", rest[:n])}
		}
		value := rest[:best]
		if kind == tokenID {
			value = strings.TrimPrefix(value, "^")
		}
		if kind == tokenINT {
			if _, err := parseInteger(value); err != nil {
				return nil, &Error{file, pos, "expected 32-bit INT; found " + value}
			}
		}
		ts = append(ts, token{kind, rest[:best], value, pos, comments})
		comments = nil
		l.advance(best)
	}
	ts = append(ts, token{tokenEOF, "", "", l.pos(), comments})
	return ts, nil
}

func unescapeString(s string) (string, int, string) {
	var b strings.Builder
	for i := 1; i < len(s)-1; i++ {
		if s[i] != '\\' {
			b.WriteByte(s[i])
			continue
		}
		start := i
		i++
		switch s[i] {
		case 'b':
			b.WriteByte('\b')
		case 't':
			b.WriteByte('\t')
		case 'n':
			b.WriteByte('\n')
		case 'f':
			b.WriteByte('\f')
		case 'r':
			b.WriteByte('\r')
		case '\\', '\'', '"':
			b.WriteByte(s[i])
		case 'u':
			if i+4 >= len(s)-1 {
				return "", start, "expected four hexadecimal digits after \\u"
			}
			v, err := strconv.ParseUint(s[i+1:i+5], 16, 16)
			if err != nil {
				return "", start, "expected four hexadecimal digits after \\u"
			}
			i += 4
			if v >= 0xd800 && v <= 0xdbff && i+6 < len(s)-1 && s[i+1:i+3] == "\\u" {
				w, err := strconv.ParseUint(s[i+3:i+7], 16, 16)
				if err == nil && w >= 0xdc00 && w <= 0xdfff {
					b.WriteRune(rune(0x10000 + (v-0xd800)*0x400 + w - 0xdc00))
					i += 6
					continue
				}
			}
			if v >= 0xd800 && v <= 0xdfff {
				b.Write([]byte{0xe0 | byte((v>>12)&0x0f), 0x80 | byte((v>>6)&0x3f), 0x80 | byte(v&0x3f)})
			} else {
				b.WriteRune(rune(v))
			}
		default:
			return "", start, fmt.Sprintf("expected valid STRING escape; found \\%c", s[i])
		}
	}
	return b.String(), 0, ""
}

func scanString(rest string) (int, string, int, string) {
	n := 1
	for n < len(rest) && rest[n] != rest[0] {
		if rest[n] == '\\' {
			n++
			if n == len(rest) {
				break
			}
		}
		n++
	}
	if n == len(rest) {
		return 0, "", 0, "expected closing quote for string; found end of file"
	}
	n++
	value, bad, message := unescapeString(rest[:n])
	return n, value, bad, message
}

const stringLiteralKind = "STRING_LITERAL"
