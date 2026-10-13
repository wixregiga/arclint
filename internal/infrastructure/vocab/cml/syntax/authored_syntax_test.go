package syntax

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// Derive witnesses from both pinned grammars, so a new optional or alternative
// cannot silently escape coverage. Each optional is exercised absent and present;
// each alternative is exercised once for each branch, in its real owning rule.
func TestEveryAuthoredGrammarChoice(t *testing.T) {
	t.Parallel()
	g, err := cmlGrammar()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for name := range g.rules {
		names = append(names, name)
	}
	sort.Strings(names)
	choices, witnesses := 0, 0
	var inventory strings.Builder
	for _, name := range names {
		var visit func(*expression, string)
		visit = func(e *expression, path string) {
			variants := 0
			if e.cardinality == '?' {
				variants = 2
			}
			if e.kind == exprAlternative {
				variants = len(e.children)
				if e.cardinality == '?' {
					variants++
				}
			}
			// Comments are attached by the lexer, independently of the grammar engine.
			if e.kind == exprAssignment && (e.feature == commentFeature || e.feature == topCommentFeature || e.feature == exposedCommentFeature) {
				return
			}
			if variants > 0 {
				choices++
				fmt.Fprintf(&inventory, "| `%s/%s` | `%s` | %d |\n", name, path, grammarChoiceText(e), variants)
				for variant := 0; variant < variants; variant++ {
					t.Run(fmt.Sprintf("%s/%s/%d", name, path, variant), func(t *testing.T) {
						source := strings.Join(grammarWitness(g, g.rules[name].body, e, variant, 0), " ")
						value := parseRule(t, name, source)
						print := func(value any) []byte {
							if g.rules[name].enum || g.rules[name].datatype {
								p := printer{grammar: g}
								tokens, ok := p.scalar(name, value.(string))
								if !ok {
									t.Fatal("cannot print scalar")
								}
								return layout(tokens)
							}
							return printerForTest(t, name, value)
						}
						output := print(value)
						assertAuthoredTokens(t, []byte(source), output)
						again := parseRule(t, name, string(output))
						if string(print(again)) != string(output) {
							t.Fatal("authored layout is not deterministic")
						}
					})
					witnesses++
				}
			}
			for i, child := range e.children {
				visit(child, fmt.Sprintf("%s.%d", path, i))
			}
		}
		visit(g.rules[name].body, "root")
	}
	t.Logf("%d grammar choices, %d absent/present and alternative witnesses", choices, witnesses)
	if path := os.Getenv("CML_CHOICE_INVENTORY"); path != "" {
		if err := os.WriteFile(path, []byte(inventory.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func grammarWitness(g *grammar, e, target *expression, variant, depth int) []string {
	if depth > 100 {
		panic("recursive grammar witness")
	}
	contains := func(e *expression) bool { return containsExpression(e, target) }
	if e.cardinality == '?' && e == target && ((e.kind != exprAlternative && variant == 0) || (e.kind == exprAlternative && variant == len(e.children))) {
		return nil
	}
	if (e.cardinality == '?' || e.cardinality == '*') && !contains(e) {
		return nil
	}
	switch e.kind {
	case exprLiteral:
		return []string{e.text}
	case exprReference:
		return []string{"Sample"}
	case exprAction:
		return nil
	case exprAssignment:
		if e.feature == commentFeature || e.feature == topCommentFeature || e.feature == exposedCommentFeature {
			return nil
		}
		return grammarWitness(g, e.children[0], target, variant, depth+1)
	case exprCall:
		if r := g.rules[e.text]; r != nil {
			return grammarWitness(g, r.body, target, variant, depth+1)
		}
		switch e.text {
		case tokenID:
			return []string{"Sample"}
		case tokenString:
			return []string{`'text'`}
		case tokenINT:
			return []string{"007"}
		}
		for _, terminal := range literalTerminals {
			if terminal.kind == e.text {
				if e.text == tokenOpposite {
					return []string{oppositeSpelling}
				}
				return []string{terminal.forms[0]}
			}
		}
		panic("unhandled witness terminal " + e.text)
	case exprAlternative:
		selected := 0
		if e == target {
			selected = variant
		} else {
			for i, child := range e.children {
				if contains(child) {
					selected = i
					break
				}
			}
		}
		return grammarWitness(g, e.children[selected], target, variant, depth+1)
	case exprSequence, exprUnordered:
		var tokens []string
		for _, child := range e.children {
			tokens = append(tokens, grammarWitness(g, child, target, variant, depth+1)...)
		}
		return tokens
	}
	panic("unhandled grammar witness " + e.kind)
}

func containsExpression(e, target *expression) bool {
	if e == target {
		return true
	}
	for _, child := range e.children {
		if containsExpression(child, target) {
			return true
		}
	}
	return false
}

func TestAuthoredTerminalSpellingsAndRepeatedChoices(t *testing.T) {
	cases := []struct{ rule, source string }{
		{"Entity", `Entity E with @One with Two with @Three {optimisticLocking auditable not cache String fieldA changeable not required not unique not nullable; String fieldB !changeable required !unique nullable - @E one opposite two; reference E two opposite one -- @E; association E}`},
		{"DomainObjectOperation", `* void work()` + `;`},
		{"Service", `Service Svc {inject @R > plain void op() => @R.go;}`},
		{"RepositoryOperation", `void op() => AccessObject;`},
		{"Consumer", `Consumer ConsumerName {unmarshall to E topicName = channel.name}`},
		{"DomainEvent", `Event E {not cache}`},
		{"CommandEvent", `CommandEvent Cmd`},
		{"UserStory", `UserStory Story {As an 'actor' I want to read 'item' so that 'benefit' and that 'a' are promoted, accepting that 'b' are harmed}`},
		{"ContextMap", `ContextMap {contains A,B contains Other,Last A [U,PL] -> [D,CF] B Other -> Last}`},
		{"ValueObject", `ValueObject V {immutable !cache}`},
		{"DtoAttribute", `String> odd;`},
		{"DtoReference", `- E> odd`},
	}
	for _, tt := range cases {
		t.Run(tt.rule+"/"+tt.source, func(t *testing.T) {
			value := parseRule(t, tt.rule, tt.source)
			assertAuthoredTokens(t, []byte(tt.source), printerForTest(t, tt.rule, value))
		})
	}
}

// The full choice inventory is optional developer output, derived from the same
// expressions as the witnesses. Calls remain named so the two grammars' own
// production boundaries are visible rather than flattening recursive bodies.
func grammarChoiceText(e *expression) string {
	text := e.text
	switch e.kind {
	case exprLiteral:
		text = fmt.Sprintf("%q", e.text)
	case exprReference:
		text = "[" + e.text + "]"
	case exprAction:
		text = "{}"
	case exprAssignment:
		text = e.feature + e.operator + grammarChoiceText(e.children[0])
	case exprSequence, exprUnordered, exprAlternative:
		var parts []string
		for _, child := range e.children {
			parts = append(parts, grammarChoiceText(child))
		}
		separator := " "
		if e.kind == exprAlternative {
			separator = " / "
		}
		if e.kind == exprUnordered {
			separator = " & "
		}
		text = "(" + strings.Join(parts, separator) + ")"
	}
	if e.cardinality != 0 {
		text += string(e.cardinality)
	}
	return strings.ReplaceAll(text, "|", "&#124;")
}

func TestEditingDatatypeMeaningInvalidatesAuthoredSpelling(t *testing.T) {
	file, err := Parse([]byte(`BoundedContext Alpha {Aggregate Group {Entity Obj { ^String id key; }}}`))
	if err != nil {
		t.Fatal(err)
	}
	attribute := file.BoundedContexts[0].Aggregates[0].DomainObjects[0].(*Entity).Attributes[0]
	delete(attribute.Spellings, "type")
	data, err := FormatChecked(file)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "^String id") {
		t.Fatal("deleted datatype spelling still emitted")
	}
	if !strings.Contains(string(data), "String id key;") {
		t.Fatalf("new builtin meaning lost: %s", data)
	}
}

func TestConcreteNodesCannotPrintAsAnotherDeclaration(t *testing.T) {
	g, err := cmlGrammar()
	if err != nil {
		t.Fatal(err)
	}
	p := printer{grammar: g}
	if tokens, ok := p.node("Application", &Service{Name: "Worker"}); ok {
		t.Fatalf("service accepted as Application: %s", layout(tokens))
	}
}

func TestCombinedGrammarTerminalShadowing(t *testing.T) {
	g, err := cmlGrammar()
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ spelling, kind string }{{"=>", tokenDelegate}, {"delegates to", "delegates to"}, {"opposite", tokenOpposite}, {symmetricArrow, symmetricArrow}} {
		tokens, err := lex("terminal", []byte(tt.spelling), g)
		if err != nil || tokens[0].kind != tt.kind {
			t.Fatalf("%s: %v, %v", tt.spelling, tokens, err)
		}
	}
}
