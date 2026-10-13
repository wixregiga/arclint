package syntax

import (
	"embed"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// The pinned specifications are data, not generated Go or a runtime toolchain.
// The recursive descent engine below interprets only their parser expressions.
//
//go:embed grammar/*.xtext
var specifications embed.FS

type expression struct {
	kind        string
	text        string
	feature     string
	operator    string
	cardinality byte
	children    []*expression
}
type grammarRule struct {
	name     string
	body     *expression
	enum     bool
	datatype bool
}
type grammar struct {
	rules    map[string]*grammarRule
	keywords map[string]bool
}

var (
	grammarOnce  sync.Once
	language     *grammar
	grammarError error
)

func cmlGrammar() (*grammar, error) {
	grammarOnce.Do(func() {
		language = &grammar{rules: map[string]*grammarRule{}, keywords: map[string]bool{}}
		for _, file := range []string{"ContextMappingDSL.xtext", "TacticDDDLanguage.xtext"} {
			data, err := specifications.ReadFile("grammar/" + file)
			if err != nil {
				grammarError = err
				return
			}
			if err = language.read(string(data)); err != nil {
				grammarError = err
				return
			}
		}
		language.keywords = make(map[string]bool, len(reservedKeywords))
		for _, word := range reservedKeywords {
			language.keywords[word] = true
		}
	})
	return language, grammarError
}

type grammarToken struct {
	text    string
	literal bool
}

func grammarTokens(src string) ([]grammarToken, error) {
	var result []grammarToken
	for i := 0; i < len(src); {
		if strings.ContainsRune(" \t\r\n", rune(src[i])) {
			i++
			continue
		}
		if strings.HasPrefix(src[i:], "//") {
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		}
		if strings.HasPrefix(src[i:], "/*") {
			n := strings.Index(src[i+2:], "*/")
			if n < 0 {
				return nil, fmt.Errorf("unclosed grammar comment")
			}
			i += n + 4
			continue
		}
		if src[i] == '\'' || src[i] == '"' {
			quote := src[i]
			i++
			var b strings.Builder
			for i < len(src) && src[i] != quote {
				if src[i] == '\\' {
					i++
					if i >= len(src) {
						return nil, fmt.Errorf("unclosed grammar string")
					}
				}
				b.WriteByte(src[i])
				i++
			}
			if i == len(src) {
				return nil, fmt.Errorf("unclosed grammar string")
			}
			i++
			result = append(result, grammarToken{b.String(), true})
			continue
		}
		start := i
		if isIDStart(src[i]) {
			i++
			for i < len(src) && isIDPart(src[i]) {
				i++
			}
		} else if i+1 < len(src) && (src[i:i+2] == "+=" || src[i:i+2] == "?=" || src[i:i+2] == "::") {
			i += 2
		} else {
			i++
		}
		result = append(result, grammarToken{text: src[start:i]})
	}
	return result, nil
}

func (g *grammar) read(src string) error {
	ts, err := grammarTokens(src)
	if err != nil {
		return err
	}
	for i := 0; i < len(ts)-1; {
		enum, terminal := false, false
		if ts[i].text == enumKeyword || ts[i].text == "terminal" {
			enum = ts[i].text == enumKeyword
			terminal = !enum
			i++
		}
		if i+1 >= len(ts) {
			break
		}
		if ts[i].literal || !isIDStart(ts[i].text[0]) || ts[i+1].text != ":" {
			i++
			continue
		}
		name := ts[i].text
		i += 2
		start := i
		for i < len(ts) && (ts[i].text != ";" || ts[i].literal) {
			i++
		}
		if i == len(ts) {
			return fmt.Errorf("grammar rule %s has no terminator", name)
		}
		if !terminal {
			if err = g.addRule(name, ts[start:i], enum); err != nil {
				return err
			}
		}
		i++
	}
	return nil
}

func (g *grammar) addRule(name string, tokens []grammarToken, enum bool) error {
	p := grammarReader{tokens: tokens, enum: enum}
	e := p.alternatives()
	if p.err != nil {
		return fmt.Errorf("grammar rule %s: %w", name, p.err)
	}
	if p.i != len(p.tokens) {
		return fmt.Errorf("grammar rule %s: unexpected %q", name, p.tokens[p.i].text)
	}
	datatype := name == typeRule || name == javaIdentifierRule || name == channelIdentifierRule || name == throwsIdentifierRule || name == verbRule
	g.rules[name] = &grammarRule{name: name, body: e, enum: enum, datatype: datatype}
	var visit func(*expression)
	visit = func(e *expression) {
		if e.kind == exprLiteral {
			g.keywords[e.text] = true
		}
		for _, c := range e.children {
			visit(c)
		}
	}
	visit(e)
	return nil
}

type grammarReader struct {
	tokens []grammarToken
	i      int
	enum   bool
	err    error
}

func (p *grammarReader) peek(s string) bool {
	return p.i < len(p.tokens) && !p.tokens[p.i].literal && p.tokens[p.i].text == s
}

func (p *grammarReader) take(s string) bool {
	if p.peek(s) {
		p.i++
		return true
	}
	return false
}

func (p *grammarReader) expect(s string) {
	if !p.take(s) && p.err == nil {
		p.err = fmt.Errorf("expected %s at grammar token %d", s, p.i)
	}
}

func collapse(kind string, children []*expression) *expression {
	if len(children) == 1 {
		return children[0]
	}
	return &expression{kind: kind, children: children}
}

func (p *grammarReader) alternatives() *expression {
	cs := []*expression{p.unordered()}
	for p.take("|") {
		cs = append(cs, p.unordered())
	}
	return collapse(exprAlternative, cs)
}

func (p *grammarReader) unordered() *expression {
	cs := []*expression{p.sequence()}
	for p.take("&") {
		cs = append(cs, p.sequence())
	}
	return collapse(exprUnordered, cs)
}

func (p *grammarReader) sequence() *expression {
	var cs []*expression
	for p.i < len(p.tokens) && !p.peek(")") && !p.peek("|") && !p.peek("&") && p.err == nil {
		cs = append(cs, p.element())
	}
	return collapse(exprSequence, cs)
}

func (p *grammarReader) element() *expression {
	var e *expression
	if p.i >= len(p.tokens) {
		p.err = fmt.Errorf("expected grammar element")
		return &expression{kind: exprSequence}
	}
	t := p.tokens[p.i]
	p.i++
	switch {
	case t.literal:
		e = &expression{kind: exprLiteral, text: t.text}
	case t.text == "(":
		e = p.alternatives()
		p.expect(")")
	case t.text == "[":
		// A cross-reference uses ID unless the grammar explicitly chooses a terminal.
		var parts []string
		for p.i < len(p.tokens) && !p.peek("]") {
			parts = append(parts, p.tokens[p.i].text)
			p.i++
		}
		p.expect("]")
		e = &expression{kind: exprReference, text: strings.Join(parts, "")}
	case t.text == "{":
		for p.i < len(p.tokens) && !p.peek("}") {
			p.i++
		}
		p.expect("}")
		e = &expression{kind: exprAction}
	default:
		if p.peek("=") || p.peek("+=") || p.peek("?=") {
			op := p.tokens[p.i].text
			p.i++
			rhs := p.element()
			// Cardinality belongs to the assignment, not its assigned terminal.
			q := rhs.cardinality
			rhs.cardinality = 0
			if p.enum {
				e = rhs
			} else {
				e = &expression{kind: exprAssignment, feature: t.text, operator: op, children: []*expression{rhs}}
			}
			e.cardinality = q
		} else if p.enum {
			e = &expression{kind: exprLiteral, text: t.text}
		} else {
			e = &expression{kind: exprCall, text: t.text}
		}
	}
	if p.peek("?") || p.peek("*") || p.peek("+") {
		e.cardinality = p.tokens[p.i].text[0]
		p.i++
	}
	return e
}

func goFeature(name string) string {
	if name == "importURI" {
		return "ImportURI"
	}
	if name == urlFeature {
		return "URL"
	}
	if name == "httpMethod" {
		return "HTTPMethod"
	}
	return string(unicode.ToUpper(rune(name[0]))) + name[1:]
}

func escapeName(name string, g *grammar) string {
	if isKeyword(name, g) {
		return "^" + name
	}
	return name
}

func quoteString(value string) (string, bool) {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(value); {
		// Java STRING accepts individual UTF-16 surrogates. Retain them in WTF-8
		// internally and emit their original Unicode escape rather than replacement.
		if i+2 < len(value) && value[i] == 0xed && value[i+1] >= 0xa0 && value[i+1] <= 0xbf && value[i+2] >= 0x80 && value[i+2] <= 0xbf {
			code := int(value[i]&0x0f)<<12 | int(value[i+1]&0x3f)<<6 | int(value[i+2]&0x3f)
			fmt.Fprintf(&b, "\\u%04x", code)
			i += 3
			continue
		}
		r, width := utf8.DecodeRuneInString(value[i:])
		if r == utf8.RuneError && width == 1 {
			return "", false
		}
		i += width
		switch r {
		case '\b':
			b.WriteString("\\b")
		case '\t':
			b.WriteString("\\t")
		case '\n':
			b.WriteString("\\n")
		case '\f':
			b.WriteString("\\f")
		case '\r':
			b.WriteString("\\r")
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		default:
			if r < 32 {
				fmt.Fprintf(&b, "\\u%04x", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String(), true
}

func parseInteger(s string) (int, error) { v, err := strconv.ParseInt(s, 10, 32); return int(v), err }

const (
	exprLiteral           = "literal"
	exprSequence          = "sequence"
	exprReference         = "reference"
	exprAction            = "action"
	exprAssignment        = "assignment"
	exprCall              = "call"
	exprAlternative       = "alternative"
	exprUnordered         = "unordered"
	tokenOpen             = "OPEN"
	tokenClose            = "CLOSE"
	tokenDelegate         = "DELEGATE"
	tokenOpposite         = "OPPOSITE"
	tokenReference        = "REF"
	tokenAssociation      = "ASC"
	tokenMap              = "MAP_COLLECTION_TYPE"
	tokenID               = "ID"
	tokenINT              = "INT"
	tokenString           = "STRING"
	tokenEOF              = "EOF"
	tokenNot              = "NOT"
	mapSpelling           = "Map"
	oppositeSpelling      = "opposite"
	modelRule             = "ContextMappingModel"
	typeRule              = "Type"
	javaIdentifierRule    = "JavaIdentifier"
	channelIdentifierRule = "ChannelIdentifier"
	throwsIdentifierRule  = "ThrowsIdentifier"
	verbRule              = "UserActivityDefaultVerb"
	complexTypeRule       = "ComplexType"
	commentFeature        = "comment"
	topCommentFeature     = "topComment"
	exposedCommentFeature = "exposedAggregatesComment"
	notationField         = "Notation"
	nodeField             = "Node"
)

const (
	enumKeyword = "enum"
	urlFeature  = "url"
)

const directionalRule = "UpstreamDownstreamRelationship"

func validIdentifier(name string) bool {
	if len(name) == 0 || !isIDStart(name[0]) {
		return false
	}
	for i := 1; i < len(name); i++ {
		if !isIDPart(name[i]) {
			return false
		}
	}
	return true
}

const (
	boundedContextRule   = "BoundedContext"
	customerSupplierRule = "CustomerSupplierRelationship"
	symmetricArrow       = "<->"
)
