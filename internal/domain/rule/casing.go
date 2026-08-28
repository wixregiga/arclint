package rule

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Term casing turns a recorded vocabulary term into a path or
// identifier segment for Template expansion. The transform vocabulary
// is deliberately close to the naming CaseSpec vocabulary but distinct
// in role: CaseSpec MATCHES file stems, a term case PRODUCES a
// segment, and only producible spellings are published here (regex has
// no producing form; flatcase exists here for Go package segments and
// has no matching need).
var termCases = map[string]func(words []string) string{
	"flatcase":   func(w []string) string { return strings.Join(w, "") },
	"snake_case": func(w []string) string { return strings.Join(w, "_") },
	"kebab-case": func(w []string) string { return strings.Join(w, "-") },
	"camelCase": func(w []string) string {
		var out strings.Builder
		out.WriteString(w[0])
		for _, word := range w[1:] {
			out.WriteString(titleWord(word))
		}
		return out.String()
	},
	"PascalCase": func(w []string) string {
		var out strings.Builder
		for _, word := range w {
			out.WriteString(titleWord(word))
		}
		return out.String()
	},
}

// TermCaseNames returns the published term-case names, sorted.
func TermCaseNames() []string {
	out := make([]string, 0, len(termCases))
	for name := range termCases {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// CaseTerm renders a recorded term in one published term case. A term
// yielding no words (no letters or digits) is an error, never an empty
// segment.
func CaseTerm(term, caseName string) (string, error) {
	transform, ok := termCases[caseName]
	if !ok {
		return "", fmt.Errorf("term case %q: not one of %s", caseName, strings.Join(TermCaseNames(), ", "))
	}
	words := termWords(term)
	if len(words) == 0 {
		return "", fmt.Errorf("term %q: no letters or digits to case", term)
	}
	return transform(words), nil
}

// termWords splits a recorded term into lowercase words: on
// whitespace, hyphens, and underscores, and on camel boundaries
// (lower-or-digit to upper, and the last upper of an upper run
// followed by a lower). Characters outside letters and digits never
// enter a word.
func termWords(term string) []string {
	runes := []rune(term)
	var words []string
	var current []rune
	flush := func() {
		if len(current) > 0 {
			words = append(words, strings.ToLower(string(current)))
			current = nil
		}
	}
	for i, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			flush()
			continue
		}
		if len(current) > 0 {
			prev := current[len(current)-1]
			lowerToUpper := unicode.IsUpper(r) &&
				(unicode.IsLower(prev) || unicode.IsDigit(prev))
			upperRunEnds := unicode.IsUpper(r) && unicode.IsUpper(prev) &&
				i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if lowerToUpper || upperRunEnds {
				flush()
			}
		}
		current = append(current, r)
	}
	flush()
	return words
}

func titleWord(w string) string {
	if w == "" {
		return ""
	}
	return strings.ToUpper(w[:1]) + w[1:]
}
