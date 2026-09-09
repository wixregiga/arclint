package rule

import (
	"fmt"
	"regexp"
	"strings"
)

// CaseSpec is the finite file-naming vocabulary for naming Rules: one
// or more alternatives of kebab-case, snake_case, camelCase,
// PascalCase, or regex:<pattern>, combined with "|" (any-of). A case
// applies to the file stem, extension excluded.
type CaseSpec struct {
	spec string
	alts []caseAlternative
}

type caseAlternative struct {
	label string
	re    *regexp.Regexp
}

var namedCases = map[string]*regexp.Regexp{
	"kebab-case": regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`),
	"snake_case": regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`),
	"camelCase":  regexp.MustCompile(`^[a-z][a-z0-9]*([A-Z][a-z0-9]*)*$`),
	"PascalCase": regexp.MustCompile(`^([A-Z][a-z0-9]*)+$`),
}

// NewCaseSpec validates and compiles a case specification. Unknown case
// names and uncompilable regexes are construction errors, never
// silently skipped alternatives.
func NewCaseSpec(spec string) (CaseSpec, error) {
	if strings.TrimSpace(spec) == "" {
		return CaseSpec{}, fmt.Errorf("case: empty specification")
	}
	var alts []caseAlternative
	for _, alt := range strings.Split(spec, "|") {
		alt = strings.TrimSpace(alt)
		if re, ok := namedCases[alt]; ok {
			alts = append(alts, caseAlternative{alt, re})
			continue
		}
		pat, ok := strings.CutPrefix(alt, "regex:")
		if !ok {
			return CaseSpec{}, fmt.Errorf("case %q: not a named case or regex:<pattern>", alt)
		}
		re, err := regexp.Compile("^(?:" + pat + ")$")
		if err != nil {
			return CaseSpec{}, fmt.Errorf("case %q: %v", alt, err)
		}
		alts = append(alts, caseAlternative{alt, re})
	}
	return CaseSpec{spec: spec, alts: alts}, nil
}

// Matches reports whether a file stem satisfies any alternative.
func (c CaseSpec) Matches(stem string) bool {
	for _, a := range c.alts {
		if a.re.MatchString(stem) {
			return true
		}
	}
	return false
}

// IsZero reports an unconstructed CaseSpec.
func (c CaseSpec) IsZero() bool { return c.spec == "" }

func (c CaseSpec) String() string { return c.spec }
