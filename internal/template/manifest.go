// Package template implements the arclint templating engine: manifest
// loading and validation, the {{ }} renderer, input resolution, and
// unit rendering (docs/design/templating.md).
package template

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
)

// identRE is the identifier grammar shared by variable names and tag lookups.
var identRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Manifest is a parsed, validated template.yaml
// (docs/design/templating.md §2).
type Manifest struct {
	Version     int        `yaml:"version"`
	Description string     `yaml:"description"`
	Destination string     `yaml:"destination"`
	Variables   []Variable `yaml:"variables"`
}

// Variable is one declaration in the manifest's ordered variables list.
// A variable is required iff it has no default — there is no required field.
type Variable struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Type        string   `yaml:"type"`
	Default     any      `yaml:"default"`
	Choices     []string `yaml:"choices"`
	Validate    string   `yaml:"validate"`
	When        string   `yaml:"when"`

	validateRE  *regexp.Regexp
	whenClauses []whenClause
}

// Required reports whether the variable must be supplied (no default).
func (v *Variable) Required() bool { return v.Default == nil }

// DefaultString returns the default as a string plus whether one exists.
// YAML scalars (bool, int) are normalized to their string forms.
func (v *Variable) DefaultString() (string, bool) {
	switch d := v.Default.(type) {
	case nil:
		return "", false
	case string:
		return d, true
	case bool:
		return strconv.FormatBool(d), true
	case int:
		return strconv.Itoa(d), true
	case int64:
		return strconv.FormatInt(d, 10), true
	case uint64:
		return strconv.FormatUint(d, 10), true
	case float64:
		return strconv.FormatFloat(d, 'g', -1, 64), true
	default:
		return fmt.Sprint(d), true
	}
}

// Check validates and normalizes a candidate value for the variable:
// bools are parsed and canonicalized to true/false, choice values must be a
// member of choices, string values must full-match the validate pattern.
func (v *Variable) Check(raw string) (string, error) {
	switch v.Type {
	case "bool":
		b, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return "", fmt.Errorf("variable %q value %q is not a bool — use true or false", v.Name, raw)
		}
		return strconv.FormatBool(b), nil
	case "choice":
		for _, c := range v.Choices {
			if raw == c {
				return raw, nil
			}
		}
		return "", fmt.Errorf("variable %q value %q is not an allowed choice — pick one of: %s", v.Name, raw, strings.Join(v.Choices, ", "))
	default:
		if v.validateRE != nil && !v.validateRE.MatchString(raw) {
			return "", fmt.Errorf("variable %q value %q fails pattern %s — adjust the value to match", v.Name, raw, v.Validate)
		}
		return raw, nil
	}
}

// LoadManifest reads and validates a template.yaml. Unknown fields,
// bad regexes, choices on non-choice types, defaults outside choices,
// forward references in when, and duplicate names are all hard errors.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s — %w", path, err)
	}
	m := &Manifest{}
	if err := yaml.UnmarshalWithOptions(data, m, yaml.Strict()); err != nil {
		return nil, fmt.Errorf("%s: %s", path, yaml.FormatError(err, false, false))
	}
	if err := m.compile(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}

// compile validates the manifest and precompiles validate regexes and when
// clauses. Called by LoadManifest; exported behavior is tested through it.
func (m *Manifest) compile() error {
	if m.Version <= 0 {
		return fmt.Errorf("version must be a positive integer — set version: 1 and bump it on any template change")
	}
	if strings.TrimSpace(m.Destination) == "" {
		return fmt.Errorf("destination is required — an interpolated path relative to the repo root")
	}
	if len(m.Variables) == 0 {
		return fmt.Errorf("variables list is required — declare at least one variable")
	}
	earlier := map[string]bool{}
	for i := range m.Variables {
		v := &m.Variables[i]
		if !identRE.MatchString(v.Name) {
			return fmt.Errorf("variable %q: name must match %s", v.Name, identRE)
		}
		if earlier[v.Name] {
			return fmt.Errorf("variable %q: duplicate name — variable names must be unique", v.Name)
		}
		if strings.TrimSpace(v.Description) == "" {
			return fmt.Errorf("variable %q: description is required — it is the prompt text", v.Name)
		}
		if err := v.compileType(); err != nil {
			return err
		}
		if v.When != "" {
			clauses, err := parseWhen(v.When)
			if err != nil {
				return fmt.Errorf("variable %q: %w", v.Name, err)
			}
			for _, c := range clauses {
				if !earlier[c.ident] {
					return fmt.Errorf("variable %q: when references %q which is not declared earlier — when may only reference earlier variables", v.Name, c.ident)
				}
			}
			v.whenClauses = clauses
		}
		earlier[v.Name] = true
	}
	return nil
}

func (v *Variable) compileType() error {
	switch v.Type {
	case "string":
		if len(v.Choices) > 0 {
			return fmt.Errorf("variable %q: choices is only valid for type choice", v.Name)
		}
		if v.Validate != "" {
			re, err := regexp.Compile(`\A(?:` + v.Validate + `)\z`)
			if err != nil {
				return fmt.Errorf("variable %q: invalid validate pattern %q — %v", v.Name, v.Validate, err)
			}
			v.validateRE = re
		}
	case "bool":
		if len(v.Choices) > 0 {
			return fmt.Errorf("variable %q: choices is only valid for type choice", v.Name)
		}
		if v.Validate != "" {
			return fmt.Errorf("variable %q: validate is only valid for type string", v.Name)
		}
		if d, ok := v.DefaultString(); ok && !strings.Contains(d, "{{") {
			if _, err := strconv.ParseBool(d); err != nil {
				return fmt.Errorf("variable %q: default %q is not a bool — use true or false", v.Name, d)
			}
		}
	case "choice":
		if len(v.Choices) == 0 {
			return fmt.Errorf("variable %q: type choice requires a choices list", v.Name)
		}
		if v.Validate != "" {
			return fmt.Errorf("variable %q: validate is only valid for type string", v.Name)
		}
		if d, ok := v.DefaultString(); ok && !strings.Contains(d, "{{") {
			found := false
			for _, c := range v.Choices {
				if c == d {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("variable %q: default %q is not in choices [%s]", v.Name, d, strings.Join(v.Choices, ", "))
			}
		}
	default:
		return fmt.Errorf("variable %q: unknown type %q — use string, bool, or choice", v.Name, v.Type)
	}
	return nil
}

// whenClause is one "ident op literal" clause of a when condition.
type whenClause struct {
	ident   string
	op      string // "==" or "!="
	literal string
}

// clauseRE matches one clause: ident (== | !=) literal, where literal is a
// double-quoted string, single-quoted string, or bare word.
var clauseRE = regexp.MustCompile(`^\s*([a-z][a-z0-9_]*)\s*(==|!=)\s*(?:"([^"]*)"|'([^']*)'|(\S+))\s*$`)

// parseWhen parses the deliberately tiny when grammar
// (docs/design/templating.md §2): clause ("&&" clause)*, equality only.
func parseWhen(expr string) ([]whenClause, error) {
	parts := strings.Split(expr, "&&")
	out := make([]whenClause, 0, len(parts))
	for _, p := range parts {
		idx := clauseRE.FindStringSubmatchIndex(p)
		if idx == nil {
			return nil, fmt.Errorf("invalid when clause %q — the grammar is: ident == literal or ident != literal, joined with &&", strings.TrimSpace(p))
		}
		group := func(n int) (string, bool) {
			if idx[2*n] < 0 {
				return "", false
			}
			return p[idx[2*n]:idx[2*n+1]], true
		}
		ident, _ := group(1)
		op, _ := group(2)
		lit := ""
		for _, g := range []int{3, 4, 5} {
			if s, ok := group(g); ok {
				lit = s
				break
			}
		}
		out = append(out, whenClause{ident: ident, op: op, literal: lit})
	}
	return out, nil
}

// evalWhen evaluates parsed clauses against resolved values. A variable that
// was itself skipped (or is unresolved) compares as the empty string.
func evalWhen(clauses []whenClause, values map[string]string) bool {
	for _, c := range clauses {
		eq := values[c.ident] == c.literal
		if c.op == "!=" {
			eq = !eq
		}
		if !eq {
			return false
		}
	}
	return true
}
