package cmlvocab

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	"github.com/wixregiga/arclint/internal/infrastructure/vocab/cml/syntax"
)

const (
	questionDirective     = "question"
	factoryRole           = "factory"
	nameDirective         = "name"
	invariantPrefix       = "ensure"
	separateWaysDirective = "separate-ways"
	teamContext           = "TEAM"
	satisfactionOperation = "satisfiedBy"
	assertionPrefix       = "assert"
	nodeField             = "Node"
	// verifiedByHint names the hint entries that carry the tests an
	// invariant or assertion names under verified_by, one entry a test.
	verifiedByHint = "verified_by"
)

var (
	identifier     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	contextPattern = regexp.MustCompile(vocab.ContextNamePattern)
	keyPattern     = regexp.MustCompile(vocab.KeyPattern)
)

func keyCase(key string) string {
	parts := strings.Split(key, "-")
	for i, part := range parts {
		if part[0] >= '0' && part[0] <= '9' {
			parts[i] = "_" + part
		} else {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

func decodeKey(s string) (string, bool) {
	var b strings.Builder
	for i, ch := range s {
		switch {
		case ch == '_':
			if i == 0 || i+1 == len(s) || s[i+1] < '0' || s[i+1] > '9' {
				return "", false
			}
			b.WriteByte('-')
		case ch >= 'A' && ch <= 'Z':
			if i > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(unicode.ToLower(ch))
		default:
			b.WriteRune(ch)
		}
	}
	key := b.String()
	return key, keyPattern.MatchString(key) && keyCase(key) == s
}

func quote(s string) string {
	// Go's ASCII control escapes outside Xtext STRING are rewritten to Unicode escapes.
	q := strconv.Quote(s)
	var b strings.Builder
	for i := 0; i < len(q); i++ {
		if q[i] != '\\' || i+1 == len(q) {
			b.WriteByte(q[i])
			continue
		}
		i++
		switch q[i] {
		case 'a':
			b.WriteString(`\u0007`)
		case 'v':
			b.WriteString(`\u000b`)
		case 'x':
			b.WriteString(`\u00`)
			b.WriteString(q[i+1 : i+3])
			i += 2
		case 'U':
			value, _ := strconv.ParseUint(q[i+1:i+9], 16, 32)
			value -= 0x10000
			fmt.Fprintf(&b, `\u%04x\u%04x`, 0xd800+(value>>10), 0xdc00+(value&0x3ff))
			i += 8
		default:
			b.WriteByte('\\')
			b.WriteByte(q[i])
		}
	}
	return b.String()
}

func directive(kind string, args ...string) syntax.Comment {
	text := "// @arclint " + kind
	for _, arg := range args {
		text += " " + quote(arg)
	}
	return syntax.Comment{Text: text}
}

type instruction struct {
	kind    string
	args    []string
	comment syntax.Comment
}

// Directive arguments go through the CML parser's STRING converter. It is the
// same lexer used for declarations; accepting Go-only escapes here would drift.
func readDirective(c syntax.Comment) (instruction, bool, error) {
	text := strings.TrimSpace(c.Text)
	if !strings.HasPrefix(text, "//") {
		return instruction{}, false, nil
	}
	text = strings.TrimSpace(strings.TrimPrefix(text, "//"))
	if !strings.HasPrefix(text, "@arclint") {
		return instruction{}, false, nil
	}
	rest := strings.TrimSpace(strings.TrimPrefix(text, "@arclint"))
	end := strings.IndexAny(rest, " \t")
	if end < 0 {
		end = len(rest)
	}
	ins := instruction{kind: rest[:end], comment: c}
	rest = strings.TrimSpace(rest[end:])
	for rest != "" {
		if rest[0] != '"' {
			return ins, true, fmt.Errorf("directive arguments must be CML double-quoted strings")
		}
		i := 1
		for i < len(rest) {
			if rest[i] == '\\' {
				i += 2
				continue
			}
			if rest[i] == '"' {
				break
			}
			i++
		}
		if i >= len(rest) {
			return ins, true, fmt.Errorf("unterminated directive string")
		}
		ast, err := syntax.Parse([]byte("Domain CmlDirectiveStringCarrier { domainVisionStatement = " + rest[:i+1] + " }"))
		if err != nil {
			return ins, true, fmt.Errorf("invalid directive string: %w", err)
		}
		ins.args = append(ins.args, ast.Domains[0].DomainVisionStatement)
		rest = strings.TrimSpace(rest[i+1:])
	}
	arity := map[string]int{nameDirective: 1, questionDirective: 2, "description": 1, separateWaysDirective: 2}
	count, ok := arity[ins.kind]
	if !ok {
		return ins, true, fmt.Errorf("unknown arclint directive %q", ins.kind)
	}
	if len(ins.args) != count && (ins.kind != separateWaysDirective || len(ins.args) != 3) {
		return ins, true, fmt.Errorf("%s directive requires %d arguments", ins.kind, count)
	}
	return ins, true, nil
}

func hintEscape(s string) string {
	return strings.NewReplacer("%", "%25", ",", "%2C", "=", "%3D").Replace(s)
}

func hintUnescape(s string) string {
	// Exactly these three escapes belong to the carrier; decoding is one pass.
	return strings.NewReplacer("%25", "%", "%2C", ",", "%2c", ",", "%3D", "=", "%3d", "=").Replace(s)
}

func hints(s, key string) []string {
	var out []string
	for _, entry := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(entry, "=")
		if ok && strings.TrimSpace(k) == "arclint."+key {
			out = append(out, hintUnescape(v))
		}
	}
	return out
}

func hintOne(s, key string) string {
	values := hints(s, key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// hintRank orders the entries the adapter writes: aliases, then the repository,
// then the rest. A key the hint does not hold yet is written at its rank among
// the arclint entries it holds, which is where a fresh recording writes it;
// entries already there, authored or foreign, keep their places.
func hintRank(key string) int {
	switch key {
	case "arclint.alias":
		return 0
	case "arclint.repository":
		return 1
	}
	return 2
}

func replaceHint(s, key string, values []string) string {
	entries := strings.Split(s, ",")
	if s == "" {
		entries = nil
	}
	var out []string
	inserted := false
	add := func() {
		for _, v := range values {
			out = append(out, "arclint."+key+"="+hintEscape(v))
		}
		inserted = true
	}
	for _, entry := range entries {
		k, _, _ := strings.Cut(strings.TrimSpace(entry), "=")
		if k == "arclint."+key {
			if !inserted {
				add()
			}
			continue
		}
		out = append(out, entry)
	}
	if !inserted {
		// Before the first arclint entry of a later rank; at the end when none is.
		at := len(out)
		for i, entry := range out {
			k, _, _ := strings.Cut(strings.TrimSpace(entry), "=")
			if strings.HasPrefix(k, "arclint.") && hintRank(k) > hintRank("arclint."+key) {
				at = i
				break
			}
		}
		tail := append([]string(nil), out[at:]...)
		out = out[:at]
		add()
		out = append(out, tail...)
	}
	return strings.Join(out, ",")
}

func scalarHint(s, key, value string) string {
	var values []string
	if value != "" {
		values = []string{value}
	}
	return replaceHint(s, key, values)
}

func exactName(name string, n *syntax.Node) string {
	for _, c := range n.LeadingComments {
		ins, ok, err := readDirective(c)
		if err == nil && ok && ins.kind == nameDirective {
			return ins.args[0]
		}
	}
	return name
}

func snakeCase(s string) string {
	r := []rune(s)
	var b strings.Builder
	for i, ch := range r {
		if unicode.IsUpper(ch) && i > 0 && (unicode.IsLower(r[i-1]) || unicode.IsDigit(r[i-1]) || (unicode.IsUpper(r[i-1]) && i+1 < len(r) && unicode.IsLower(r[i+1]))) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(ch))
	}
	return b.String()
}

func definition(doc, comment string) string {
	if doc != "" {
		return doc
	}
	comment = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(comment), "/*"), "*/"))
	var lines []string
	for _, line := range strings.Split(comment, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "//"))
		line = strings.TrimSpace(strings.TrimPrefix(line, "*"))
		// An ArcLint directive in the leading comment carries metadata the
		// adapter reads separately; it is never part of the definition.
		if strings.HasPrefix(line, "@arclint ") {
			continue
		}
		lines = append(lines, line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// visitNodes traverses only exported grammar fields. Private parser derivations
// are deliberately excluded, so positions and original syntax remain intact.
func visitNodes(value any, fn func(*syntax.Node)) {
	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		if v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
			if !v.IsNil() {
				walk(v.Elem())
			}
			return
		}
		switch v.Kind() {
		default:
			return
		case reflect.Struct:
			if v.Type() == reflect.TypeOf(syntax.Node{}) {
				if v.CanAddr() {
					fn(v.Addr().Interface().(*syntax.Node))
				}
				return
			}
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() {
					walk(v.Field(i))
				}
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i))
			}
		}
	}
	walk(reflect.ValueOf(value))
}

func normalized(lang vocab.UbiquitousLanguage) vocab.UbiquitousLanguage {
	lang.Recording = vocab.DomainRecording{}
	// Clone through the constructor, so clearing provenance never mutates callers.
	copyLang, err := vocab.NewUbiquitousLanguage(lang.Project, lang.Description, lang.Contexts, lang.Relations)
	if err != nil {
		return lang
	}
	var clearLines func(reflect.Value)
	clearLines = func(v reflect.Value) {
		switch v.Kind() {
		default:
			return
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).Name == "Line" {
					v.Field(i).SetInt(0)
				} else {
					clearLines(v.Field(i))
				}
			}
		case reflect.Slice:
			if v.Len() == 0 {
				v.Set(reflect.Zero(v.Type()))
				return
			}
			for i := 0; i < v.Len(); i++ {
				clearLines(v.Index(i))
			}
		}
	}
	clearLines(reflect.ValueOf(&copyLang).Elem())
	return copyLang
}

func sameLanguage(a, b vocab.UbiquitousLanguage) bool {
	return reflect.DeepEqual(normalized(a), normalized(b))
}

type nameAllocator struct {
	used  map[string]bool
	names map[string]string
}

func newNames() *nameAllocator {
	return &nameAllocator{used: map[string]bool{}, names: map[string]string{}}
}

func (a *nameAllocator) reserve(s string) {
	if identifier.MatchString(s) {
		a.used[s] = true
	}
}

func (a *nameAllocator) name(scope, name string) string {
	key := scope + "\x00" + name
	if h, ok := a.names[key]; ok {
		return h
	}
	if identifier.MatchString(name) {
		a.names[key] = name
		a.used[name] = true
		return name
	}
	var b strings.Builder
	bad := false
	for _, r := range name {
		valid := r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
		if valid {
			b.WriteRune(r)
			bad = false
		} else if !bad {
			b.WriteByte('_')
			bad = true
		}
	}
	base := b.String()
	if base == "" {
		base = "_"
	}
	if base[0] >= '0' && base[0] <= '9' {
		base = "_" + base
	}
	h := base
	for i := 2; a.used[h]; i++ {
		h = fmt.Sprintf("%s_%d", base, i)
	}
	a.used[h] = true
	a.names[key] = h
	return h
}

func namedNode(handle, exact string) syntax.Node {
	var n syntax.Node
	if handle != exact {
		n.LeadingComments = []syntax.Comment{directive(nameDirective, exact)}
	}
	return n
}

func ordered[T any](items []T, pos func(T) int) []T {
	sort.SliceStable(items, func(i, j int) bool { return pos(items[i]) < pos(items[j]) })
	return items
}

func (a *nameAllocator) unique(scope, base string) string {
	key := scope + "\x00" + base
	if handle, ok := a.names[key]; ok {
		return handle
	}
	// Reuse the normal handle spelling, then allocate independently of a real
	// declaration that happens to have the same valid identifier.
	allocator := newNames()
	candidate := allocator.name(scope, base)
	handle := candidate
	for i := 2; a.used[handle]; i++ {
		handle = fmt.Sprintf("%s_%d", candidate, i)
	}
	a.used[handle] = true
	a.names[key] = handle
	return handle
}

func declarationName(file *syntax.File, name string, n *syntax.Node) string {
	exact := exactName(name, n)
	if exact != name {
		return exact
	}
	for _, c := range file.LeadingComments {
		ins, ok, err := readDirective(c)
		if ok && err == nil && ins.kind == nameDirective && c.Pos.Line+1 == n.Pos.Line {
			return ins.args[0]
		}
	}
	return exact
}

func bindInitialNameComment(file *syntax.File, n *syntax.Node) {
	var remaining []syntax.Comment
	for _, c := range file.LeadingComments {
		ins, ok, err := readDirective(c)
		if ok && err == nil && ins.kind == nameDirective && c.Pos.Line+1 == n.Pos.Line {
			n.LeadingComments = append(n.LeadingComments, c)
		} else {
			remaining = append(remaining, c)
		}
	}
	file.LeadingComments = remaining
	texts := make([]string, 0, len(remaining))
	for _, c := range remaining {
		texts = append(texts, c.Text)
	}
	file.TopComment = strings.Join(texts, "\n")
}

func hasNameDirective(file *syntax.File, n *syntax.Node) bool {
	for _, c := range n.LeadingComments {
		ins, ok, err := readDirective(c)
		if ok && err == nil && ins.kind == nameDirective {
			return true
		}
	}
	for _, c := range file.LeadingComments {
		ins, ok, err := readDirective(c)
		if ok && err == nil && ins.kind == nameDirective && c.Pos.Line+1 == n.Pos.Line {
			return true
		}
	}
	return false
}
