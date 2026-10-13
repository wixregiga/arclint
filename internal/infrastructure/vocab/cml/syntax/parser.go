package syntax

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

type derivation struct {
	expr        *expression
	children    []*derivation
	value       any
	start, end  int
	alternative int
	rule        string
}
type (
	continuation func(int, *derivation) bool
	parser       struct {
		grammar  *grammar
		tokens   []token
		file     string
		farthest int
		expected map[string]bool
		internal error
		depth    int
	}
)

// Parse parses a CML context mapping model. Errors use <input> as the filename.
func Parse(source []byte) (*File, error) { return ParseFile("<input>", source) }

// ParseFile parses bytes, using filename in precise lexical and syntax diagnostics.
func ParseFile(filename string, source []byte) (*File, error) {
	g, err := cmlGrammar()
	if err != nil {
		return nil, err
	}
	ts, err := lex(filename, source, g)
	if err != nil {
		return nil, err
	}
	p := parser{grammar: g, tokens: ts, file: filename, expected: map[string]bool{}}
	var file *File
	ok := p.rule(modelRule, 0, func(end int, d *derivation) bool {
		if end != len(ts)-1 {
			p.expect(end, "end of file")
			return false
		}
		var yes bool
		file, yes = d.value.(*File)
		return yes
	})
	if !ok {
		return nil, p.syntaxError()
	}
	attachComments(file, ts)
	return file, nil
}

func (p *parser) syntaxError() error {
	if p.internal != nil {
		return p.internal
	}
	var expected []string
	for e := range p.expected {
		expected = append(expected, e)
	}
	sort.Strings(expected)
	t := p.tokens[p.farthest]
	found := "end of file"
	if t.text != "" {
		kind := t.kind
		switch kind {
		case stringLiteralKind:
			kind = tokenString
		case tokenString:
			kind = "keyword STRING"
		}
		found = fmt.Sprintf("%q (%s)", t.text, kind)
	}
	return &Error{p.file, t.pos, "expected " + strings.Join(expected, " or ") + "; found " + found}
}

func (p *parser) expect(at int, want string) {
	if at > p.farthest {
		p.farthest = at
		p.expected = map[string]bool{}
	}
	if at == p.farthest {
		p.expected[want] = true
	}
}

func (p *parser) rule(name string, at int, k continuation) bool {
	r := p.grammar.rules[name]
	if r == nil {
		return p.terminal(name, at, k)
	}
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > 2048 {
		p.internal = &Error{p.file, p.tokens[at].pos, "CML nesting exceeds 2048 grammar calls"}
		return false
	}
	return p.expression(r.body, at, func(end int, d *derivation) bool {
		result := &derivation{children: []*derivation{d}, start: at, end: end, rule: name}
		value, err := p.materialize(r, result)
		if err != nil {
			p.internal = err
			return false
		}
		result.value = value
		// A continuation parses the caller's remaining input, not a nested rule.
		p.depth--
		accepted := k(end, result)
		p.depth++
		return accepted
	})
}

func (p *parser) terminal(name string, at int, k continuation) bool {
	t := p.tokens[at]
	match := t.kind == name
	switch name {
	case tokenString:
		match = t.kind == stringLiteralKind
	// ANTLR's combined grammar aliases a single-literal terminal to its literal.
	case tokenOpen:
		match = t.text == "{"
	case tokenClose:
		match = t.text == "}"
	}
	if !match {
		p.expect(at, name)
		return false
	}
	var value any = t.value
	if name == tokenINT {
		value, _ = parseInteger(t.value)
	}
	return k(at+1, &derivation{value: value, start: at, end: at + 1, rule: name})
}

func (p *parser) expression(e *expression, at int, k continuation) bool {
	if e.cardinality == 0 {
		return p.once(e, at, k)
	}
	switch e.cardinality {
	case '?':
		if p.once(e, at, func(end int, d *derivation) bool {
			return k(end, &derivation{expr: e, children: []*derivation{d}, start: at, end: end})
		}) {
			return true
		}
		return k(at, &derivation{expr: e, start: at, end: at})
	case '*', '+':
		var repeat func(int, []*derivation) bool
		repeat = func(cur int, children []*derivation) bool {
			if p.once(e, cur, func(end int, d *derivation) bool {
				if end == cur {
					return false
				}
				next := append(children, d)
				return repeat(end, next)
			}) {
				return true
			}
			if e.cardinality == '+' && len(children) == 0 {
				return false
			}
			return k(cur, &derivation{expr: e, children: append([]*derivation(nil), children...), start: at, end: cur})
		}
		return repeat(at, nil)
	}
	return false
}

func (p *parser) once(e *expression, at int, k continuation) bool {
	d := func(end int, children []*derivation) *derivation {
		return &derivation{expr: e, children: append([]*derivation(nil), children...), start: at, end: end}
	}
	switch e.kind {
	case exprLiteral:
		if p.tokens[at].kind == e.text || ((e.text == "{" || e.text == "}") && p.tokens[at].text == e.text) {
			v := d(at+1, nil)
			v.value = e.text
			return k(at+1, v)
		}
		p.expect(at, strconvLiteral(e.text))
		return false
	case exprAction:
		return k(at, d(at, nil))
	case exprCall:
		return p.rule(e.text, at, func(end int, child *derivation) bool {
			v := d(end, []*derivation{child})
			v.value = child.value
			return k(end, v)
		})
	case exprReference:
		if p.tokens[at].kind != tokenID {
			p.expect(at, "ID for "+e.text)
			return false
		}
		v := d(at+1, nil)
		v.value = Ref{p.tokens[at].value, p.tokens[at].pos}
		return k(at+1, v)
	case exprAssignment:
		return p.expression(e.children[0], at, func(end int, child *derivation) bool {
			v := d(end, []*derivation{child})
			v.value = semanticValue(child)
			if e.operator == "?=" {
				v.value = true
			}
			return k(end, v)
		})
	case exprAlternative:
		for i, c := range e.children {
			if p.expression(c, at, func(end int, child *derivation) bool {
				v := d(end, []*derivation{child})
				v.alternative = i
				v.value = semanticValue(child)
				return k(end, v)
			}) {
				return true
			}
		}
		return false
	case exprSequence:
		var next func(int, int, []*derivation) bool
		next = func(i, cur int, children []*derivation) bool {
			if i == len(e.children) {
				return k(cur, d(cur, children))
			}
			return p.expression(e.children[i], cur, func(end int, child *derivation) bool {
				cs := append(children, child)
				return next(i+1, end, cs)
			})
		}
		return next(0, at, nil)
	case exprUnordered:
		selected := make([]bool, len(e.children))
		var next func(int, []*derivation) bool
		next = func(cur int, children []*derivation) bool {
			for i, c := range e.children {
				if selected[i] {
					continue
				}
				selected[i] = true
				ok := p.unorderedMember(c, cur, func(end int, child *derivation) bool {
					if end == cur {
						return false
					}
					cs := append(children, child)
					return next(end, cs)
				})
				selected[i] = false
				if ok {
					return true
				}
			}
			for i, c := range e.children {
				if !selected[i] && !p.nullable(c) {
					p.expect(cur, "remaining unordered-group member")
					return false
				}
			}
			return k(cur, d(cur, children))
		}
		return next(at, nil)
	}
	p.internal = fmt.Errorf("unknown parser expression %s", e.kind)
	return false
}

// Xtext emits a selected multiple-cardinality unordered member as a contiguous
// ({true}?=> member)+ loop (AbstractAntlrGrammarWithActionsGenerator). Selection
// distinguishes the members, but continuation of that loop uses its FIRST set,
// not the caller's remaining input. A shared doc STRING therefore commits to
// another declaration of the selected kind, even if its keyword does not match.
func (p *parser) unorderedMember(e *expression, at int, k continuation) bool {
	if e.cardinality != '*' && e.cardinality != '+' {
		return p.expression(e, at, k)
	}
	cur := at
	var children []*derivation
	for p.starts(e, cur) {
		var child *derivation
		end := cur
		if !p.once(e, cur, func(next int, d *derivation) bool {
			end, child = next, d
			return next > cur
		}) {
			return false
		}
		children = append(children, child)
		cur = end
	}
	if len(children) == 0 && e.cardinality == '+' {
		return false
	}
	return k(cur, &derivation{expr: e, children: children, start: at, end: cur})
}

func (p *parser) starts(e *expression, at int) bool {
	switch e.kind {
	case exprLiteral:
		return p.tokens[at].kind == e.text || p.tokens[at].text == e.text
	case exprReference:
		return p.tokens[at].kind == tokenID
	case exprCall:
		if r := p.grammar.rules[e.text]; r != nil {
			return p.starts(r.body, at)
		}
		switch e.text {
		case tokenString:
			return p.tokens[at].kind == stringLiteralKind
		case tokenOpen:
			return p.tokens[at].text == "{"
		case tokenClose:
			return p.tokens[at].text == "}"
		}
		return p.tokens[at].kind == e.text
	case exprAssignment:
		return p.starts(e.children[0], at)
	case exprSequence:
		for _, c := range e.children {
			if p.starts(c, at) {
				return true
			}
			if !p.nullable(c) {
				break
			}
		}
	case exprUnordered, exprAlternative:
		for _, c := range e.children {
			if p.starts(c, at) {
				return true
			}
		}
	}
	return false
}

func (p *parser) nullable(e *expression) bool {
	if e.cardinality == '?' || e.cardinality == '*' {
		return true
	}
	switch e.kind {
	case exprAction:
		return true
	case exprSequence, exprUnordered:
		for _, c := range e.children {
			if !p.nullable(c) {
				return false
			}
		}
		return true
	case exprAlternative:
		for _, c := range e.children {
			if p.nullable(c) {
				return true
			}
		}
		return false
	case exprAssignment:
		return p.nullable(e.children[0])
	case exprCall:
		if r := p.grammar.rules[e.text]; r != nil {
			return p.nullable(r.body)
		}
	}
	return false
}
func strconvLiteral(s string) string { return fmt.Sprintf("%q", s) }
func semanticValue(d *derivation) any {
	if d.value != nil {
		return d.value
	}
	// Unassigned parser calls forward their object through keyword prefixes.
	for _, c := range d.children {
		if v := semanticValue(c); nodeOf(v) != nil {
			return v
		}
	}
	for _, c := range d.children {
		if v := semanticValue(c); v != nil {
			return v
		}
	}
	return nil
}

// braceBodyNeedsFlag reports whether the braces of an optional body need
// ExplicitBody to survive an edit: the author wrote them around nothing, or
// around optional syntax that no assigned field represents (a keyword or
// negation at its default, such as "immutable" or "!cache"). Braces around
// assigned fields alone print while any of those fields has a value, and go
// with them when an edit clears the last one.
func (p *parser) braceBodyNeedsFlag(body *derivation) bool {
	empty := true
	for _, t := range p.tokens[body.start:body.end] {
		if t.value != "{" && t.value != "}" {
			empty = false
			break
		}
	}
	if empty {
		return true
	}
	var unassigned func(*derivation) bool
	unassigned = func(d *derivation) bool {
		// A nested declaration owns its own braces, and the optional tokens
		// inside an optional assigned feature, such as the "=" of a property,
		// belong to that feature.
		if d != body && d.rule != "" {
			return false
		}
		if d != body && d.expr != nil && d.expr.cardinality != 0 {
			if derivationHasAssignment(d) {
				return false
			}
			if len(d.children) > 0 {
				return true
			}
		}
		for _, child := range d.children {
			if unassigned(child) {
				return true
			}
		}
		return false
	}
	return unassigned(body)
}

func (p *parser) materialize(r *grammarRule, d *derivation) (any, error) {
	if r.enum || r.datatype {
		var b strings.Builder
		for _, t := range p.tokens[d.start:d.end] {
			b.WriteString(t.value)
		}
		return b.String(), nil
	}
	if r.name == directionalRule {
		if v, ok := semanticValue(d).(*CustomerSupplierRelationship); ok {
			return v, nil
		}
	}
	t, concrete := nodeTypes[r.name]
	if !concrete {
		v := semanticValue(d)
		if v == nil {
			return nil, fmt.Errorf("grammar union %s has no value", r.name)
		}
		return unionValue(r.name, v)
	}
	value := reflect.New(t)
	node := nodeOf(value.Interface())
	node.Pos = p.tokens[d.start].pos
	node.form = d
	node.choices = p.syntaxChoices(d)
	var collect func(*derivation) error
	collect = func(tree *derivation) error {
		if tree.rule != "" && tree != d {
			return nil
		}
		if tree.expr != nil && tree.expr.cardinality == '?' && (tree.expr.kind == exprSequence || tree.expr.kind == exprUnordered) && containsBrace(tree.expr) && tree.end > tree.start && p.braceBodyNeedsFlag(tree) {
			node.ExplicitBody = true
		}
		if tree.expr != nil && tree.expr.kind == exprAssignment && tree.value != nil {
			return p.recordAssignment(value.Elem(), node, tree)
		}
		for _, c := range tree.children {
			if err := collect(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := collect(d); err != nil {
		return nil, err
	}
	if field := value.Elem().FieldByName(notationField); field.IsValid() {
		var find func(*derivation) (int, bool)
		find = func(tree *derivation) (int, bool) {
			if tree.expr != nil && tree.expr.kind == exprAlternative && len(tree.expr.children) >= 4 {
				return tree.alternative, true
			}
			for _, c := range tree.children {
				if n, ok := find(c); ok {
					return n, true
				}
			}
			return 0, false
		}
		n, _ := find(d)
		field.SetInt(int64(n))
		recordRoleNotation(r.name, value.Elem(), n, p.tokens[d.start:d.end])
	}
	return value.Interface(), nil
}

func (p *parser) recordAssignment(value reflect.Value, node *Node, tree *derivation) error {
	if node.Present == nil {
		node.Present = map[string]bool{}
	}
	node.Present[tree.expr.feature] = true
	if raw, ok := p.datatypeSpelling(tree.children[0]); ok && raw != fmt.Sprint(tree.value) {
		if node.Spellings == nil {
			node.Spellings = map[string]string{}
		}
		node.Spellings[tree.expr.feature] = raw
	}
	if tree.expr.feature == "name" {
		node.NamePos = p.tokens[tree.start].pos
	}
	return assignFeature(value, tree.expr.feature, tree.value, tree.expr.operator == "+=")
}

func (p *parser) datatypeSpelling(d *derivation) (string, bool) {
	if d.rule != "" {
		r := p.grammar.rules[d.rule]
		if r == nil || !r.datatype {
			return "", false
		}
		var b strings.Builder
		for _, t := range p.tokens[d.start:d.end] {
			b.WriteString(t.text)
		}
		return b.String(), true
	}
	for _, child := range d.children {
		if raw, ok := p.datatypeSpelling(child); ok {
			return raw, true
		}
	}
	return "", false
}

func recordRoleNotation(rule string, value reflect.Value, notation int, ts []token) {
	if rule != directionalRule && rule != customerSupplierRule {
		return
	}
	forward := notation == 0 || rule == directionalRule && notation == 2 || rule == customerSupplierRule && notation == 3
	afterOperator := false
	for i := 0; i < len(ts); i++ {
		switch ts[i].text {
		case "->", "<-", "Upstream-Downstream", "Downstream-Upstream", "Customer-Supplier", "Supplier-Customer":
			afterOperator = true
		case "{", ":":
			return
		}
		if ts[i].text != "[" {
			continue
		}
		prefix := "Downstream"
		if forward != afterOperator {
			prefix = "Upstream"
		}
		value.FieldByName(prefix + "Bracket").SetBool(true)
		for i++; i < len(ts) && ts[i].text != "]"; i++ {
			if ts[i].kind == "U" || ts[i].kind == "D" {
				value.FieldByName(prefix + "RoleMarker").SetBool(true)
			}
		}
	}
}

// Comments are attached after successful parsing, so speculative lookahead never
// steals comments. At a shared start offset, the outer declaration owns the run.
func attachComments(file *File, ts []token) {
	var nodes []any
	var visit func(reflect.Value)
	visit = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		if v.Kind() == reflect.Interface {
			if !v.IsNil() {
				visit(v.Elem())
			}
			return
		}
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return
			}
			if n := nodeOf(v.Interface()); n != nil {
				nodes = append(nodes, v.Interface())
			}
			visit(v.Elem())
			return
		}
		if v.Kind() == reflect.Struct {
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).Name != nodeField {
					visit(v.Field(i))
				}
			}
		}
		if v.Kind() == reflect.Slice {
			for i := 0; i < v.Len(); i++ {
				visit(v.Index(i))
			}
		}
	}
	visit(reflect.ValueOf(file))
	defer func() {
		for _, v := range nodes {
			nodeOf(v).form = nil
		}
	}()
	index := makeCommentIndex(nodes, ts, file)
	for i, t := range ts {
		comments := t.comments
		if len(comments) == 0 {
			continue
		}
		if i == len(ts)-1 {
			file.TrailingComments = append(file.TrailingComments, comments...)
			continue
		}
		if i == 0 {
			split := len(comments)
			for j := 1; j < len(comments); j++ {
				previous := comments[j-1]
				lastLine := previous.Pos.Line + strings.Count(previous.Text, "\n")
				if comments[j].Pos.Line > lastLine+1 {
					split = j
					break
				}
			}
			file.LeadingComments = append(file.LeadingComments, comments[:split]...)
			file.TopComment = commentText(comments[:split])
			comments = comments[split:]
			if len(comments) == 0 {
				continue
			}
		}
		if exposed, ok := index.exposed[i]; ok && strings.HasPrefix(comments[0].Text, "//") && comments[0].Pos.Line == exposed.line {
			value := reflect.ValueOf(exposed.owner).Elem()
			value.FieldByName(goFeature(exposedCommentFeature)).SetString(comments[0].Text)
			n := nodeOf(exposed.owner)
			if n.Present == nil {
				n.Present = map[string]bool{}
			}
			n.Present[exposedCommentFeature] = true
			comments = comments[1:]
			if len(comments) == 0 {
				continue
			}
		}
		if owner := index.closing[i]; owner != nil {
			nodeOf(owner).TrailingComments = append(nodeOf(owner).TrailingComments, comments...)
			continue
		}
		block := index.blockAt[i]
		limit := len(ts)
		if block != nil {
			limit = nodeOf(block).form.end - 1
		}
		following := sort.Search(len(index.declarations), func(j int) bool { return nodeOf(index.declarations[j]).form.start >= i })
		if following < len(index.declarations) && nodeOf(index.declarations[following]).form.start < limit {
			attachLeading(index.declarations[following], comments)
			continue
		}
		if block != nil {
			nodeOf(block).TrailingComments = append(nodeOf(block).TrailingComments, comments...)
		} else {
			file.LeadingComments = append(file.LeadingComments, comments...)
		}
	}
}

type commentIndex struct {
	declarations []any
	closing      map[int]any
	blockAt      []any
	exposed      map[int]struct {
		owner any
		line  int
	}
}

func makeCommentIndex(nodes []any, ts []token, file *File) *commentIndex {
	index := &commentIndex{closing: map[int]any{}, blockAt: make([]any, len(ts)), exposed: map[int]struct {
		owner any
		line  int
	}{}}
	tokenAt := make(map[int]int, len(ts))
	for i, t := range ts {
		tokenAt[t.pos.Offset] = i
	}
	for _, value := range nodes {
		if value == file {
			continue
		}
		n := nodeOf(value)
		index.declarations = append(index.declarations, value)
		last := n.form.end - 1
		if last >= 0 && ts[last].text == "}" {
			previous := index.closing[last]
			if previous == nil || n.form.start > nodeOf(previous).form.start {
				index.closing[last] = value
			}
		}
		v := reflect.ValueOf(value).Elem()
		field := v.FieldByName("UpstreamExposedAggregates")
		if !field.IsValid() || field.Len() == 0 {
			continue
		}
		ref := field.Index(field.Len() - 1).Interface().(Ref)
		index.exposed[tokenAt[ref.Pos.Offset]+1] = struct {
			owner any
			line  int
		}{value, ref.Pos.Line}
	}
	sort.SliceStable(index.declarations, func(i, j int) bool {
		a, b := nodeOf(index.declarations[i]).form, nodeOf(index.declarations[j]).form
		if a.start != b.start {
			return a.start < b.start
		}
		return a.end > b.end
	})
	var opens []int
	closeFor := map[int]int{}
	for i, t := range ts {
		switch t.text {
		case "{":
			opens = append(opens, i)
		case "}":
			if len(opens) > 0 {
				closeFor[opens[len(opens)-1]] = i
				opens = opens[:len(opens)-1]
			}
		}
	}
	var blocks []any
	for i, t := range ts {
		if t.text == "{" {
			blocks = append(blocks, index.closing[closeFor[i]])
		}
		if len(blocks) > 0 {
			index.blockAt[i] = blocks[len(blocks)-1]
		}
		if t.text == "}" && len(blocks) > 0 {
			blocks = blocks[:len(blocks)-1]
		}
	}
	return index
}

func commentText(comments []Comment) string {
	texts := make([]string, 0, len(comments))
	for _, c := range comments {
		texts = append(texts, c.Text)
	}
	return strings.Join(texts, "\n")
}

func attachLeading(owner any, comments []Comment) {
	n := nodeOf(owner)
	n.LeadingComments = append(n.LeadingComments, comments...)
	v := reflect.ValueOf(owner).Elem()
	if field := v.FieldByName("Comment"); field.IsValid() {
		field.SetString(commentText(n.LeadingComments))
	}
}
