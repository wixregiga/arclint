package template

import (
	"sort"
	"strings"
	"unicode"
)

// filters is the complete, closed filter table (docs/design/templating.md §3).
// Not exported, no registration function: adding a filter is a code change
// with a doc change, on purpose.
var filters = map[string]func(string) string{
	"pascal": toPascal,
	"camel":  toCamel,
	"snake":  toSnake,
	"kebab":  toKebab,
	"upper":  strings.ToUpper,
	"lower":  strings.ToLower,
	"plural": toPlural,
}

func filterNames() []string {
	names := make([]string, 0, len(filters))
	for n := range filters {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// splitWords splits on spaces, hyphens, underscores, and lower-to-upper case
// boundaries (plus the end of an acronym run: "HTTPServer" -> HTTP, Server).
func splitWords(s string) []string {
	runes := []rune(s)
	var words []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			words = append(words, string(cur))
			cur = nil
		}
	}
	for i, r := range runes {
		switch {
		case r == ' ' || r == '-' || r == '_':
			flush()
		case unicode.IsUpper(r):
			if i > 0 {
				prev := runes[i-1]
				if unicode.IsLower(prev) || unicode.IsDigit(prev) {
					flush()
				} else if unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
					flush()
				}
			}
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
	}
	flush()
	return words
}

func titleWord(w string) string {
	if w == "" {
		return w
	}
	r := []rune(strings.ToLower(w))
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func toPascal(s string) string {
	var b strings.Builder
	for _, w := range splitWords(s) {
		b.WriteString(titleWord(w))
	}
	return b.String()
}

func toCamel(s string) string {
	words := splitWords(s)
	var b strings.Builder
	for i, w := range words {
		if i == 0 {
			b.WriteString(strings.ToLower(w))
		} else {
			b.WriteString(titleWord(w))
		}
	}
	return b.String()
}

func toSnake(s string) string { return joinLower(s, "_") }
func toKebab(s string) string { return joinLower(s, "-") }

func joinLower(s, sep string) string {
	words := splitWords(s)
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	return strings.Join(words, sep)
}

// pluralIrregulars is the exceptions map for common irregular nouns
// (docs/design/templating.md §3): a convenience, not a linguistics engine.
var pluralIrregulars = map[string]string{
	"person": "people",
	"child":  "children",
	"man":    "men",
	"woman":  "women",
	"foot":   "feet",
	"tooth":  "teeth",
	"mouse":  "mice",
	"goose":  "geese",
}

func toPlural(s string) string {
	if s == "" {
		return s
	}
	// Pluralize the last word, preserving everything before it.
	lastStart := strings.LastIndexAny(s, " -_") + 1
	word := s[lastStart:]
	if word == "" {
		return s + "s"
	}
	lw := strings.ToLower(word)
	if p, ok := pluralIrregulars[lw]; ok {
		if r := []rune(word); unicode.IsUpper(r[0]) {
			p = titleWord(p)
		}
		return s[:lastStart] + p
	}
	switch {
	case hasAnySuffix(lw, "s", "x", "z", "ch", "sh"):
		return s + "es"
	case strings.HasSuffix(lw, "y") && len(lw) >= 2 && !isVowel(lw[len(lw)-2]):
		return s[:len(s)-1] + "ies"
	default:
		return s + "s"
	}
}

func hasAnySuffix(s string, suffixes ...string) bool {
	for _, suf := range suffixes {
		if strings.HasSuffix(s, suf) {
			return true
		}
	}
	return false
}

func isVowel(b byte) bool {
	switch b {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}
