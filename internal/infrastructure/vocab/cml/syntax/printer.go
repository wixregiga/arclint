package syntax

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

type printToken struct {
	startRule   string
	endRule     bool
	propertyEnd bool
	text        string
	newline     bool
	blankline   bool
	comment     bool
	inline      bool
}
type printScope struct {
	value   any
	fields  map[string][]any
	used    map[string]int
	present map[string]bool
	choices map[choiceKey]int
	rule    string
}
type printer struct{ grammar *grammar }

// FormatChecked emits deterministic CML with two-space indentation and reports
// an invalid constructed or edited AST: the AST must obey its grammar
// cardinalities and mandatory fields, and the output must parse again.
func FormatChecked(file *File) ([]byte, error) {
	if file == nil {
		return nil, fmt.Errorf("cannot format nil CML file")
	}
	g, err := cmlGrammar()
	if err != nil {
		return nil, err
	}
	p := printer{grammar: g}
	tokens, ok := p.node(modelRule, file)
	if !ok {
		return nil, fmt.Errorf("CML AST does not satisfy the pinned grammar")
	}
	data := layout(tokens)
	if _, err := Parse(data); err != nil {
		return nil, fmt.Errorf("formatted CML AST is invalid: %w", err)
	}
	return data, nil
}

func newPrintScope(rule string, value any) *printScope {
	s := &printScope{value: value, fields: map[string][]any{}, used: map[string]int{}, present: map[string]bool{}, choices: map[choiceKey]int{}, rule: rule}
	if n := nodeOf(value); n != nil {
		for name, present := range n.Present {
			s.present[name] = present
		}
	}

	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		for i := 0; i < v.NumField(); i++ {
			s.addField(v.Type().Field(i).Name, v.Field(i))
		}
	}

	// Explicit comment features are exposed for readers but are printed with their
	// owning comment runs, rather than appearing twice through assignments.
	delete(s.fields, commentFeature)
	delete(s.fields, topCommentFeature)
	delete(s.fields, exposedCommentFeature)
	return s
}

func (s *printScope) addField(name string, f reflect.Value) {
	switch name {
	case nodeField, notationField, "UpstreamBracket", "DownstreamBracket", "UpstreamRoleMarker", "DownstreamRoleMarker":
		return
	}
	key := strings.ToLower(name[:1]) + name[1:]
	switch name {
	case "ImportURI":
		key = "importURI"
	case "URL":
		key = urlFeature
	case "HTTPMethod":
		key = "httpMethod"
	}
	if f.Kind() == reflect.Slice {
		for j := 0; j < f.Len(); j++ {
			s.fields[key] = append(s.fields[key], f.Index(j).Interface())
		}
		return
	}
	if !f.IsZero() || s.present[key] && (f.Kind() == reflect.String || f.Kind() == reflect.Int) {
		s.fields[key] = []any{f.Interface()}
	}
}

type printState struct {
	used    map[string]int
	choices map[choiceKey]int
}

func (s *printScope) snapshot() printState {
	r := printState{used: map[string]int{}, choices: map[choiceKey]int{}}
	for k, v := range s.used {
		r.used[k] = v
	}
	for k, v := range s.choices {
		r.choices[k] = v
	}
	return r
}

func (s *printScope) restore(state printState) {
	s.used = make(map[string]int, len(state.used))
	for key, value := range state.used {
		s.used[key] = value
	}
	s.choices = make(map[choiceKey]int, len(state.choices))
	for key, value := range state.choices {
		s.choices[key] = value
	}
}

func (s *printScope) hasChoice(e *expression, kind string) bool {
	n := nodeOf(s.value)
	if n == nil {
		return false
	}
	key := choiceKey{e, kind}
	return s.choices[key] < len(n.choices[key])
}

func (s *printScope) choice(e *expression, kind string) (syntaxChoice, bool) {
	n := nodeOf(s.value)
	if n == nil {
		return syntaxChoice{}, false
	}
	key := choiceKey{e, kind}
	i := s.choices[key]
	choices := n.choices[key]
	if i >= len(choices) {
		return syntaxChoice{}, false
	}
	s.choices[key]++
	return choices[i], true
}

func (s *printScope) complete() bool {
	for k, v := range s.fields {
		if s.used[k] != len(v) {
			return false
		}
	}
	return true
}

func (p *printer) node(rule string, value any) ([]printToken, bool) {
	r := p.grammar.rules[rule]
	if r == nil {
		return nil, false
	}
	concrete := reflect.TypeOf(value)
	if concrete == nil {
		return nil, false
	}
	if concrete.Kind() == reflect.Pointer {
		if reflect.ValueOf(value).IsNil() {
			return nil, false
		}
		concrete = concrete.Elem()
	}
	if concrete.Name() != rule {
		if _, concreteRule := nodeTypes[rule]; concreteRule && (rule != directionalRule || concrete.Name() != customerSupplierRule) {
			return nil, false
		}
		// Union rules retain their keyword prefixes and forward to a concrete member.
		return p.union(r.body, value)
	}
	comments, valid := p.prepareComments(rule, value)
	if !valid {
		return nil, false
	}
	s := newPrintScope(rule, value)
	var result []printToken
	ok := p.expression(r.body, s, func(tokens []printToken) bool {
		if !s.complete() {
			return false
		}
		result = append([]printToken(nil), tokens...)
		return true
	})
	if !ok {
		return nil, false
	}
	result = compactEmptyBodies(decorateComments(rule, comments, result))

	// Each object declaration or operation is a layout boundary. Parameters, type
	// expressions, features, targets, and workflow operands stay inline.
	if isStatement(rule) {
		result = append(result, printToken{newline: true})
	}
	result = append([]printToken{{startRule: rule}}, result...)
	result = append(result, printToken{endRule: true})
	return result, true
}

func (p *printer) prepareComments(rule string, value any) (*Node, bool) {
	original := nodeOf(value)
	if original == nil {
		return nil, true
	}
	n := *original
	if len(n.LeadingComments) == 0 {
		key := commentFeature
		if rule == modelRule {
			key = topCommentFeature
		}
		field := reflect.ValueOf(value).Elem().FieldByName(goFeature(key))
		if field.IsValid() && field.String() != "" {
			tokens, err := lex("comments", []byte(field.String()), p.grammar)
			if err != nil || len(tokens) != 1 {
				return nil, false
			}
			n.LeadingComments = tokens[0].comments
		}
	}
	for _, run := range [][]Comment{n.LeadingComments, n.TrailingComments} {
		for _, comment := range run {
			tokens, err := lex("comment", []byte(comment.Text), p.grammar)
			if err != nil || len(tokens) != 1 || len(tokens[0].comments) != 1 {
				return nil, false
			}
		}
	}
	field := reflect.ValueOf(value).Elem().FieldByName(goFeature(exposedCommentFeature))
	if field.IsValid() && field.String() != "" {
		tokens, err := lex("comment", []byte(field.String()), p.grammar)
		if err != nil || len(tokens) != 1 || len(tokens[0].comments) != 1 || !strings.HasPrefix(tokens[0].comments[0].Text, "//") {
			return nil, false
		}
	}
	return &n, true
}

func decorateComments(rule string, n *Node, result []printToken) []printToken {
	if n == nil {
		return result
	}
	leading := make([]printToken, 0, len(n.LeadingComments)+1)
	for _, c := range n.LeadingComments {
		leading = append(leading, printToken{text: c.Text, comment: true})
	}
	if rule == modelRule && len(leading) > 0 {
		leading = append(leading, printToken{blankline: true})
	}
	result = append(leading, result...)
	if len(n.TrailingComments) == 0 {
		return result
	}
	comments := make([]printToken, 0, len(n.TrailingComments))
	for _, c := range n.TrailingComments {
		comments = append(comments, printToken{text: c.Text, comment: true})
	}
	for i := len(result) - 1; rule != modelRule && i >= 0; i-- {
		if result[i].text != "}" || result[i].comment {
			continue
		}
		end := append([]printToken(nil), result[i:]...)
		result = append(result[:i], comments...)
		return append(result, end...)
	}
	return append(result, comments...)
}

// Authored empty bodies remain explicit and use the compact brace spelling.
func compactEmptyBodies(tokens []printToken) []printToken {
	for i := 0; i < len(tokens); i++ {
		if tokens[i].text != "{" {
			continue
		}
		j := i + 1
		for j < len(tokens) && (tokens[j].newline || tokens[j].propertyEnd) {
			j++
		}
		if j < len(tokens) && tokens[j].text == "}" {
			tokens[i].text = "{}"
			tokens = append(tokens[:i+1], tokens[j+1:]...)
		}
	}
	return tokens
}

func isStatement(rule string) bool {
	switch rule {
	case modelRule, complexTypeRule, "Parameter", "TargetState", "SingleStateTransitionTarget", "ExclusiveAlternativeStateTransitionTarget", "StateTransition", "EitherCommandOrOperation", "NormalFeature", "StoryFeature", "StoryValuation", "SingleCommandInvokation", "ConcurrentCommandInvokation", "ExclusiveAlternativeCommandInvokation", "InclusiveAlternativeCommandInvokation", "SingleOperationInvokation", "ConcurrentOperationInvokation", "ExclusiveAlternativeOperationInvokation", "InclusiveAlternativeOperationInvokation", "SingleEventProduction", "MultipleEventProduction", "ExclusiveAlternativeEventProduction", "InclusiveAlternativeEventProduction", "Subscribe", "Publish", "ServiceOperationDelegate", "ResourceOperationDelegate", "OppositeHolder", "EnumValue", "EnumParameter", "Action":
		return false
	}
	return true
}

func (p *printer) union(e *expression, value any) ([]printToken, bool) {
	switch e.kind {
	case exprCall:
		return p.node(e.text, value)
	case exprAlternative:
		for _, c := range e.children {
			if ts, ok := p.union(c, value); ok {
				return ts, true
			}
		}
		return nil, false
	case exprSequence:
		var ts []printToken
		found := false
		for _, c := range e.children {
			if c.kind == exprLiteral {
				ts = append(ts, printToken{text: c.text})
			} else {
				v, ok := p.union(c, value)
				if !ok {
					return nil, false
				}
				ts = append(ts, v...)
				found = true
			}
		}
		return ts, found
	}
	return nil, false
}

func (p *printer) expression(e *expression, s *printScope, k func([]printToken) bool) bool {
	state := s.snapshot()
	restore := func() {
		s.restore(state)
	}
	if e.cardinality == 0 {
		ok := p.once(e, s, k)
		if !ok {
			restore()
		}
		return ok
	}
	// Optional unassigned syntax, such as void and empty declaration blocks, is
	// retained from the parse derivation. Assigned fields always control edits.
	hasFeatures := expressionHasAssignment(e)
	want := hasFeatures && hasAvailable(e, s)
	if role := headerRole(e); role != "" && containsLiteral(e, "[") {
		want = want || notationFlag(s, role+"Bracket")
	}
	if !hasFeatures {
		if containsLiteral(e, "U") {
			want = notationFlag(s, "UpstreamRoleMarker")
		}
		if containsLiteral(e, "D") {
			want = notationFlag(s, "DownstreamRoleMarker")
		}
	}
	if (e.kind == exprSequence || e.kind == exprUnordered) && containsBrace(e) {
		if n := nodeOf(s.value); n != nil && (n.ExplicitBody || len(n.TrailingComments) > 0) {
			want = true
		}
	}

	choice, authored := s.choice(e, "cardinality")
	if authored && !containsBrace(e) {
		if !hasFeatures {
			want = choice.count > 0
		} else if choice.unassigned && choice.count > 0 {
			want = true
		}
	}
	if !authored {
		want = want || !hasFeatures && containsLiteral(e, "=") || containsLiteral(e, "void") || containsLiteral(e, "(")
	}
	if e.cardinality == '?' {
		if want {
			if p.once(e, s, k) {
				return true
			}
			restore()
		}
		if k(nil) {
			return true
		}
		restore()
		return false
	}
	var loop func([]printToken, int) bool
	loop = func(ts []printToken, n int) bool {
		before := s.snapshot()
		if authored && n == choice.count {
			if k(ts) {
				return true
			}
			s.restore(before)
		}
		available := hasAvailable(e, s)
		if available && p.once(e, s, func(next []printToken) bool {
			changed := false
			for key, v := range s.used {
				if before.used[key] != v {
					changed = true
				}
			}
			if !changed {
				return false
			}
			return loop(append(ts, next...), n+1)
		}) {
			return true
		}
		s.restore(before)
		if e.cardinality == '+' && n == 0 {
			return false
		}
		return k(ts)
	}
	ok := loop(nil, 0)
	if !ok {
		restore()
	}
	return ok
}

func (p *printer) once(e *expression, s *printScope, k func([]printToken) bool) bool {
	switch e.kind {
	case exprLiteral:
		return k([]printToken{{text: e.text}})
	case exprAction:
		return k(nil)
	case exprCall:
		if choice, ok := s.choice(e, "text"); ok {
			return k(choice.tokens)
		}
		terminalForms := map[string]string{tokenReference: "-", tokenAssociation: "--", tokenNot: "!", tokenDelegate: "=>", tokenOpposite: oppositeSpelling, tokenMap: mapSpelling}
		if text, ok := terminalForms[e.text]; ok {
			return k([]printToken{{text: text}})
		}
		if e.text == tokenOpen {
			return k([]printToken{{text: "{"}})
		}
		if e.text == tokenClose {
			return k([]printToken{{text: "}"}})
		}
		return false
	case exprAssignment:
		feature := e.feature
		if feature == commentFeature || feature == topCommentFeature || feature == exposedCommentFeature {
			return false
		}
		i := s.used[feature]
		values := s.fields[feature]
		if i >= len(values) {
			if e.operator != "=" {
				return false
			}
			v := reflect.ValueOf(s.value).Elem().FieldByName(goFeature(feature))
			if !v.IsValid() || v.Kind() != reflect.String && v.Kind() != reflect.Int {
				return false
			}
			ts, ok := p.assigned(e.children[0], v.Interface(), false)
			return ok && k(ts)
		}
		s.used[feature]++
		value := values[i]
		if n := nodeOf(s.value); n != nil {
			if spelling := n.Spellings[feature]; spelling != "" && p.sameDatatypeValue(spelling, value) {
				value = spelling
			}
		}
		choice, authored := s.choice(e, "assigned")
		ts, ok := p.assigned(e.children[0], value, e.operator == "?=")
		if authored && sameAssignedValue(choice.value, values[i]) && (choice.spelling == "" || nodeOf(s.value).Spellings[feature] == choice.spelling) {
			ts, ok = choice.tokens, true
		}
		if ok && feature == "doc" {
			ts = append(ts, printToken{newline: true})
		}
		if e.operator == "?=" && e.children[0].kind == exprLiteral {
			ts = append(ts, printToken{propertyEnd: true})
		}
		if ok && feature == "upstreamExposedAggregates" && i == len(values)-1 {
			value := reflect.ValueOf(s.value).Elem().FieldByName(goFeature(exposedCommentFeature))
			if value.IsValid() && value.String() != "" {
				ts = append(ts, printToken{text: value.String(), comment: true, inline: true})
			}
		}
		if ok && k(ts) {
			return true
		}
		s.used[feature]--
		return false
	case exprAlternative:
		if field := reflect.ValueOf(s.value).Elem().FieldByName(notationField); field.IsValid() && len(e.children) >= 4 {
			i := int(field.Int())
			if i < 0 || i >= len(e.children) {
				return false
			}
			return p.expression(e.children[i], s, k)
		}
		children := orderedExpressions(e.children, s)
		if choice, ok := s.choice(e, "alternative"); ok {
			children = append([]*expression{e.children[choice.count]}, children...)
		}
		for _, c := range children {
			if p.expression(c, s, k) {
				return true
			}
		}
		return false
	case exprSequence, exprUnordered:
		children := e.children
		if e.kind == exprUnordered && (!containsBrace(e) || s.hasChoice(e, "order")) {
			children = authoredOrder(e, s)
		}
		var next func(int, []printToken) bool
		next = func(i int, ts []printToken) bool {
			if i == len(children) {
				if p.propertyClause(e) && startsPropertyClause(e) {
					ts = append(ts, printToken{propertyEnd: true})
				}
				return k(ts)
			}
			return p.expression(children[i], s, func(child []printToken) bool { return next(i+1, append(ts, child...)) })
		}
		return next(0, nil)
	}
	return false
}

// A primitive grammar clause can end a property line inside its own node's
// body. Object assignments and brace-bearing groups are composition, not clauses.
func (p *printer) propertyClause(e *expression) bool {
	if containsBrace(e) || !expressionHasAssignment(e) {
		return false
	}
	var primitive func(*expression) bool
	primitive = func(part *expression) bool {
		if part.kind == exprAssignment {
			var scalar func(*expression) bool
			scalar = func(rhs *expression) bool {
				if rhs.kind == exprCall {
					r := p.grammar.rules[rhs.text]
					return r == nil || r.enum || r.datatype
				}
				for _, child := range rhs.children {
					if !scalar(child) {
						return false
					}
				}
				return true
			}
			return scalar(part.children[0])
		}
		for _, child := range part.children {
			if !primitive(child) {
				return false
			}
		}
		return true
	}
	return primitive(e)
}

// Node positions order declarations across separate feature slices. This keeps
// a caller's source-defined order while still allowing zero-position new nodes.
// Grammar order breaks ties, and a parsed fixed point retains the same order.
func orderedExpressions(children []*expression, s *printScope) []*expression {
	result := append([]*expression(nil), children...)
	sort.SliceStable(result, func(i, j int) bool {
		a, aOK := firstNodeOffset(result[i], s)
		b, bOK := firstNodeOffset(result[j], s)
		if aOK != bOK {
			return aOK
		}
		if aOK {
			return a < b
		}
		if n := nodeOf(s.value); n != nil && n.choices != nil {
			return false
		}
		return canonicalFeatureOrder(s.rule, result[i]) < canonicalFeatureOrder(s.rule, result[j])
	})
	return result
}

func firstNodeOffset(e *expression, s *printScope) (int, bool) {
	if e.kind == exprAssignment {
		values := s.fields[e.feature]
		i := s.used[e.feature]
		if i < len(values) {
			if n := nodeOf(values[i]); n != nil && n.Pos.Line > 0 {
				return n.Pos.Offset, true
			}
		}
		return 0, false
	}
	best, found := 0, false
	for _, c := range e.children {
		if offset, ok := firstNodeOffset(c, s); ok && (!found || offset < best) {
			best, found = offset, true
		}
	}
	return best, found
}

func notationFlag(s *printScope, name string) bool {
	if n := nodeOf(s.value); n != nil && n.choices == nil {
		v := reflect.ValueOf(s.value).Elem()
		notation := v.FieldByName(notationField)
		if notation.IsValid() && notation.Int() < 2 && (s.rule == directionalRule || s.rule == customerSupplierRule) {
			return true
		}
	}
	v := reflect.ValueOf(s.value).Elem().FieldByName(name)
	return v.IsValid() && v.Kind() == reflect.Bool && v.Bool()
}

func containsLiteral(e *expression, literal string) bool {
	if e.kind == exprLiteral && e.text == literal {
		return true
	}
	for _, c := range e.children {
		if containsLiteral(c, literal) {
			return true
		}
	}
	return false
}

func headerRole(e *expression) string {
	if e.kind == exprAssignment {
		switch e.feature {
		case "upstreamRoles":
			return "Upstream"
		case "downstreamRoles":
			return "Downstream"
		}
	}
	for _, c := range e.children {
		if role := headerRole(c); role != "" {
			return role
		}
	}
	return ""
}

func containsBrace(e *expression) bool {
	if e.kind == exprCall && e.text == tokenOpen || e.kind == exprLiteral && e.text == "{" {
		return true
	}
	for _, c := range e.children {
		if containsBrace(c) {
			return true
		}
	}
	return false
}

func expressionHasAssignment(e *expression) bool {
	if e.kind == exprAssignment {
		return true
	}
	for _, c := range e.children {
		if expressionHasAssignment(c) {
			return true
		}
	}
	return false
}

func hasAvailable(e *expression, s *printScope) bool {
	if e.kind == exprAssignment {
		return s.used[e.feature] < len(s.fields[e.feature])
	}
	for _, c := range e.children {
		if hasAvailable(c, s) {
			return true
		}
	}
	return false
}

func (p *printer) assigned(e *expression, value any, boolean bool) ([]printToken, bool) {
	if boolean {
		b, ok := value.(bool)
		if !ok || !b {
			return nil, false
		}
	}
	switch e.kind {
	case exprReference:
		r, ok := value.(Ref)
		if !ok || !validIdentifier(r.Name) {
			return nil, false
		}
		return []printToken{{text: escapeName(r.Name, p.grammar)}}, true
	case exprLiteral:
		if boolean || fmt.Sprint(value) == e.text {
			return []printToken{{text: e.text}}, true
		}
		return nil, false
	case exprAlternative:
		for _, c := range e.children {
			if ts, ok := p.assigned(c, value, boolean); ok {
				return ts, true
			}
		}
		return nil, false
	case exprCall:
		switch e.text {
		case tokenID:
			name := fmt.Sprint(value)
			if !validIdentifier(name) {
				return nil, false
			}
			return []printToken{{text: escapeName(name, p.grammar)}}, true
		case tokenString:
			quoted, ok := quoteString(fmt.Sprint(value))
			return []printToken{{text: quoted}}, ok
		case tokenINT:
			return []printToken{{text: strconv.Itoa(value.(int))}}, true
		case tokenNot:
			return []printToken{{text: "!"}}, boolean
		case tokenDelegate:
			return []printToken{{text: "=>"}}, boolean
		case tokenMap:
			return []printToken{{text: mapSpelling}}, fmt.Sprint(value) == mapSpelling
		}
		r := p.grammar.rules[e.text]
		if r == nil {
			return nil, false
		}
		if r.enum || r.datatype {
			return p.scalar(e.text, fmt.Sprint(value))
		}
		return p.node(e.text, value)
	}
	return nil, false
}

func (p *printer) sameDatatypeValue(spelling string, value any) bool {
	tokens, err := lex("datatype", []byte(spelling), p.grammar)
	if err != nil {
		return false
	}
	var normalized strings.Builder
	for _, token := range tokens {
		normalized.WriteString(token.value)
	}
	return normalized.String() == fmt.Sprint(value)
}

func (p *printer) scalar(rule, value string) ([]printToken, bool) {
	r := p.grammar.rules[rule]
	if r.enum || rule == verbRule {
		var allowed func(*expression) bool
		allowed = func(e *expression) bool {
			if e.kind == exprLiteral && e.text == value {
				return true
			}
			for _, c := range e.children {
				if allowed(c) {
					return true
				}
			}
			return false
		}
		if allowed(r.body) {
			return []printToken{{text: value}}, true
		}
		return nil, false
	}
	if rule == typeRule && p.grammar.keywords[value] {
		var allowed func(*expression) bool
		allowed = func(e *expression) bool {
			if e.kind == exprLiteral && e.text == value {
				return true
			}
			for _, c := range e.children {
				if allowed(c) {
					return true
				}
			}
			return false
		}
		if allowed(r.body) {
			return []printToken{{text: value}}, true
		}
	}
	var ts []printToken
	start := 0
	for i := 0; i <= len(value); i++ {
		if i == len(value) || strings.ContainsRune("./:,", rune(value[i])) {
			name := strings.TrimSpace(value[start:i])
			if !validIdentifier(strings.TrimPrefix(name, "^")) {
				return nil, false
			}
			ts = append(ts, printToken{text: escapeName(name, p.grammar)})
			if i < len(value) {
				ts = append(ts, printToken{text: value[i : i+1]})
			}
			start = i + 1
		}
	}
	return ts, len(ts) > 0
}

func separatesDeclarations(rule string) bool {
	switch rule {
	case modelRule, "ContextMap", boundedContextRule, "Domain", "Subdomain", "Aggregate", "SculptorModule", "Application":
		return true
	}
	return false
}

func layout(ts []printToken) []byte {
	var b strings.Builder
	indent := 0
	lineStart := true
	bracketDepth := 0
	previous := ""
	newline := func() {
		if !lineStart {
			b.WriteByte('\n')
			lineStart = true
			previous = ""
		}
	}
	write := func(s string) {
		if lineStart {
			b.WriteString(strings.Repeat("  ", indent))
			lineStart = false
		}
		b.WriteString(s)
	}
	type layoutScope struct {
		rule          string
		previousChild string
		entryIndent   int
	}
	var scopes []layoutScope
	for _, t := range ts {
		if t.startRule != "" {
			if len(scopes) > 0 && separatesDeclarations(scopes[len(scopes)-1].rule) {
				parent := &scopes[len(scopes)-1]
				newline()
				if parent.previousChild != "" && (parent.previousChild != "Import" || t.startRule != "Import") {
					b.WriteByte('\n')
				}
				parent.previousChild = t.startRule
			}
			scopes = append(scopes, layoutScope{rule: t.startRule, entryIndent: indent})
			continue
		}
		if t.endRule {
			scopes = scopes[:len(scopes)-1]
			continue
		}
		if t.propertyEnd {
			if len(scopes) > 0 && indent > scopes[len(scopes)-1].entryIndent {
				newline()
			}
			continue
		}
		if t.blankline {
			newline()
			b.WriteByte('\n')
			continue
		}
		if t.newline {
			newline()
			continue
		}
		if t.comment {
			if t.inline && !lineStart {
				write(" ")
			} else {
				newline()
			}
			for i, line := range strings.Split(strings.TrimRight(t.text, "\r\n"), "\n") {
				if i > 0 {
					newline()
					b.WriteString(strings.TrimSuffix(line, "\r"))
					lineStart = false
				} else {
					write(strings.TrimSuffix(line, "\r"))
				}
			}
			newline()
			continue
		}
		s := t.text
		if s == "}" {
			newline()
			if indent > 0 {
				indent--
			}
			write(s)
			previous = s
			continue
		}
		space := !lineStart && previous != "" && s != "," && s != ";" && s != ")" && s != "]" && s != ">" && s != "." && s != "::" && s != "/" && previous != "(" && previous != "[" && previous != "<" && previous != "@" && previous != "." && previous != "::" && previous != "/"
		relationship := false
		for _, scope := range scopes {
			switch scope.rule {
			case "Partnership", "SharedKernel", directionalRule, customerSupplierRule:
				relationship = true
			}
		}
		if relationship && ((s == "->" || s == "<-" || s == symmetricArrow) && previous == "]" || (previous == "->" || previous == "<-" || previous == symmetricArrow) && s == "[" || bracketDepth > 0 && previous == ",") {
			space = false
		}
		if s == "(" {
			space = false
		}
		if space {
			write(" ")
		}
		write(s)
		if s == "[" {
			bracketDepth++
		}
		if s == "]" {
			bracketDepth--
		}
		previous = s
		if s == "{" {
			indent++
			newline()
		}
		if s == ";" {
			newline()
		}
	}
	newline()
	return []byte(b.String())
}
