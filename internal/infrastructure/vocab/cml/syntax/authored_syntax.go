package syntax

import (
	"fmt"
	"reflect"
)

// Choices retain grammar decisions, not the parse tree. Each expression can
// occur repeatedly within a node (for example each optional @ in a traits list).
// The printer consumes occurrences transactionally while it matches edited fields.
type choiceKey struct {
	expression *expression
	kind       string
}

type syntaxChoice struct {
	spelling   string
	unassigned bool
	count      int
	order      []*expression
	tokens     []printToken
	value      any
}

func (p *parser) syntaxChoices(root *derivation) map[choiceKey][]syntaxChoice {
	choices := map[choiceKey][]syntaxChoice{}
	var visit func(*derivation, bool)
	visit = func(d *derivation, once bool) {
		if d.rule != "" && d != root {
			return
		}
		e := d.expr
		if e != nil {
			add := func(kind string, c syntaxChoice) { key := choiceKey{e, kind}; choices[key] = append(choices[key], c) }
			if e.cardinality != 0 && !once {
				add("cardinality", syntaxChoice{count: len(d.children), unassigned: !derivationHasAssignment(d)})
				for _, child := range d.children {
					visit(child, true)
				}
				return
			}
			switch e.kind {
			case exprAlternative:
				add("alternative", syntaxChoice{count: d.alternative})
			case exprUnordered:
				var order []*expression
				for _, child := range d.children {
					order = append(order, child.expr)
				}
				add("order", syntaxChoice{order: order})
			case exprCall:
				if p.grammar.rules[e.text] == nil {
					var ts []printToken
					for _, t := range p.tokens[d.start:d.end] {
						ts = append(ts, printToken{text: t.text})
					}
					add("text", syntaxChoice{tokens: ts})
				}
				return
			case exprAssignment:
				// Object fields print their current nodes recursively. Scalar spellings,
				// including escaped identifiers and single-quoted strings, stay authored
				// until their value changes.
				if nodeOf(d.value) == nil && d.value != nil {
					add("assigned", p.assignedChoice(d))
				}
				return
			}
		}
		for _, child := range d.children {
			visit(child, false)
		}
	}
	visit(root, false)
	return choices
}

func sameAssignedValue(a, b any) bool {
	if ref, ok := a.(Ref); ok {
		other, yes := b.(Ref)
		return yes && ref.Name == other.Name
	}
	return reflect.DeepEqual(a, b)
}

func startsPropertyClause(e *expression) bool {
	if e.kind != exprSequence || len(e.children) == 0 {
		return false
	}
	first := e.children[0]
	return first.kind == exprLiteral && first.text != "," && first.text != "(" && first.text != ":" || first.kind == exprAlternative && !expressionHasAssignment(first)
}

func canonicalFeatureOrder(rule string, e *expression) int {
	feature := e.feature
	if feature == "" {
		for _, child := range e.children {
			if n := canonicalFeatureOrder(rule, child); n != 100 {
				return n
			}
		}
		return 100
	}
	switch rule {
	case "Entity":
		if feature == "aggregateRoot" {
			return 0
		}
	case modelRule:
		switch feature {
		case "imports":
			return 0
		case "domains":
			return 1
		case "map":
			return 2
		case "boundedContexts":
			return 3
		}
	case boundedContextRule:
		switch feature {
		case "domainServices":
			return 0
		case "aggregates":
			return 1
		case "modules":
			return 2
		}
	}
	return 100
}

func derivationHasAssignment(d *derivation) bool {
	if d.expr != nil && d.expr.kind == exprAssignment && d.value != nil {
		return true
	}
	if d.rule != "" {
		return false
	}
	for _, child := range d.children {
		if derivationHasAssignment(child) {
			return true
		}
	}
	return false
}

func authoredOrder(e *expression, s *printScope) []*expression {
	choice, ok := s.choice(e, "order")
	if !ok {
		return orderedExpressions(e.children, s)
	}
	children := append([]*expression(nil), choice.order...)
	seen := make(map[*expression]bool, len(children))
	for _, child := range children {
		seen[child] = true
	}
	for _, child := range e.children {
		if !seen[child] {
			children = append(children, child)
		}
	}
	// Primitive clauses keep their slots. Sort only declaration slots: moving a
	// Service must not move a UseCase's interactions ahead of its actor clause.
	var declarations []*expression
	var slots []int
	for i, child := range children {
		if hasNodeValue(child, s) {
			slots = append(slots, i)
			declarations = append(declarations, child)
		}
	}
	declarations = orderedExpressions(declarations, s)
	for i, slot := range slots {
		children[slot] = declarations[i]
	}
	return children
}

func hasNodeValue(e *expression, s *printScope) bool {
	if e.kind == exprAssignment {
		values := s.fields[e.feature]
		i := s.used[e.feature]
		return i < len(values) && nodeOf(values[i]) != nil
	}
	for _, child := range e.children {
		if hasNodeValue(child, s) {
			return true
		}
	}
	return false
}

func (p *parser) assignedChoice(d *derivation) syntaxChoice {
	choice := syntaxChoice{value: d.value}
	for _, t := range p.tokens[d.start:d.end] {
		choice.tokens = append(choice.tokens, printToken{text: t.text})
	}
	if raw, ok := p.datatypeSpelling(d.children[0]); ok && raw != fmt.Sprint(d.value) {
		choice.spelling = raw
	}
	return choice
}
