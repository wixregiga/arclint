// Package config loads and validates .arclint/rules.yaml — the single
// canonical arclint rule registry (docs/design/rules.md).
//
// The validation pipeline is the one specified in rules.md §7:
//
//  1. goccy/go-yaml parses the file into an untyped tree with duplicate-key
//     detection on.
//  2. The loader coerces a YAML-1.1 boolean false in a severity position to
//     the string "off" (rules.md §4, decision D10).
//  3. The tree is validated against the embedded JSON Schema
//     (schema/arclint-rules.schema.json) via santhosh-tekuri/jsonschema/v6.
//  4. Only then is the tree decoded into the typed structs below, followed
//     by the semantic checks JSON Schema cannot express (module references,
//     ignore rule ids, RE2 compilation).
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
)

// Severity of a rule: error | warn | off. Warn never affects the exit code
// (decision D6); off keeps the rule registered without executing it.
type Severity string

const (
	SeverityError Severity = "error"
	SeverityWarn  Severity = "warn"
	SeverityOff   Severity = "off"
)

// Category is the closed rule-type enum (decision D9).
type Category string

const (
	CategoryStructure    Category = "structure"
	CategoryNaming       Category = "naming"
	CategoryDependencies Category = "dependencies"
	CategoryContent      Category = "content"
	CategoryCustom       Category = "custom"
)

// File is a fully parsed and validated .arclint/rules.yaml.
type File struct {
	// Version is the rule file format version; currently always 1.
	Version int `yaml:"version"`
	// Extends lists inherited rule files or built-in presets, applied in
	// order before local rules. Resolution is the check engine's job.
	Extends []string `yaml:"extends"`
	// Baseline is the path to the grandfathered-violations file, relative
	// to the repo root. Empty when unset.
	Baseline string `yaml:"baseline"`
	// Exclude holds global exclude globs, on top of the built-ins.
	Exclude []string `yaml:"exclude"`
	// Ignore holds per-path rule suppressions.
	Ignore []Ignore `yaml:"ignore"`
	// Rules is the flat registry keyed by stable kebab-case rule id.
	Rules map[string]Rule `yaml:"rules"`
}

// Ignore silences all rules (or the listed rule ids) for paths matching
// the glob.
type Ignore struct {
	Path  string   `yaml:"path"`
	Rules []string `yaml:"rules"`
}

// FileFilter is per-rule file targeting. A nil filter means the whole tree
// minus global excludes; Exclude is subtracted from the Include set.
type FileFilter struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

// Rule is one entry in the registry. Exactly one of the typed params
// pointers (Structure, Naming, Dependencies, Content, Custom) is non-nil,
// matching Type.
type Rule struct {
	Type        Category    `yaml:"type"`
	Severity    Severity    `yaml:"severity"`
	Description string      `yaml:"description"`
	Files       *FileFilter `yaml:"files"`
	FixHint     string      `yaml:"fixHint"`

	// RawParams is the params block as parsed; kept for consumers that
	// need the original shape. Prefer the typed pointers below.
	RawParams map[string]any `yaml:"params"`

	Structure    *StructureParams    `yaml:"-"`
	Naming       *NamingParams       `yaml:"-"`
	Dependencies *DependenciesParams `yaml:"-"`
	Content      *ContentParams      `yaml:"-"`
	Custom       *CustomParams       `yaml:"-"`
}

// StructureParams: what the tree must and must not contain.
type StructureParams struct {
	// Require globs must each match at least one existing file, checked
	// against the whole tree minus global excludes (rule files targeting
	// does not apply to require).
	Require []string `yaml:"require"`
	// Forbid globs must match no file within the rule's file targeting.
	Forbid []string `yaml:"forbid"`
}

// NamingParams: ls-lint style convention per glob.
type NamingParams struct {
	// Style is a pipe expression: alternatives separated by '|'; the name
	// is valid if any alternative matches. Tokens: camelCase, PascalCase,
	// snake_case, kebab-case, SCREAMING_SNAKE_CASE, lowercase,
	// regex:<pattern>.
	Style string `yaml:"style"`
	// Target is "file" (default) or "dir".
	Target string `yaml:"target"`
}

// DependenciesParams: import-graph contracts between named modules.
type DependenciesParams struct {
	// Modules maps module name to path globs.
	Modules map[string][]string `yaml:"modules"`
	// Contract is one of layers | forbidden | independence | mayDependOn.
	Contract string `yaml:"contract"`
	// Layers is ordered top to bottom; a layer may import layers below it.
	Layers []string `yaml:"layers"`
	// Forbidden lists explicit deny edges.
	Forbidden []ForbiddenEdge `yaml:"forbidden"`
	// Among lists modules that may not import each other.
	Among []string `yaml:"among"`
	// MayDependOn whitelists dependencies per module; an empty list means
	// the module may depend on nothing.
	MayDependOn map[string][]string `yaml:"mayDependOn"`
}

// ForbiddenEdge is one deny edge of a forbidden contract.
type ForbiddenEdge struct {
	From []string `yaml:"from"`
	To   []string `yaml:"to"`
}

// ContentParams: line-oriented RE2 checks over file content.
type ContentParams struct {
	// MustContain: every targeted file must match every pattern at least
	// once.
	MustContain []ContentMatcher `yaml:"mustContain"`
	// MustNotContain: no targeted file may match any pattern.
	MustNotContain []ContentMatcher `yaml:"mustNotContain"`
}

// ContentMatcher is one pattern with an optional message override.
type ContentMatcher struct {
	Pattern string `yaml:"pattern"`
	Message string `yaml:"message"`
}

// CustomParams: external command escape hatch (decision D20).
type CustomParams struct {
	// Command is the argv executed from the repo root.
	Command []string `yaml:"command"`
	// TimeoutSeconds defaults to 30.
	TimeoutSeconds int `yaml:"timeoutSeconds"`
}

// DefaultCustomTimeoutSeconds is applied when timeoutSeconds is omitted.
const DefaultCustomTimeoutSeconds = 30

// RulesPath returns the canonical rules file path under a config root
// (the directory containing .arclint/).
func RulesPath(root string) string {
	return filepath.Join(root, ".arclint", "rules.yaml")
}

// Load reads, validates, and decodes the rules file at path. The error for
// a missing file names the fix per the CLI error style guide.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no rules file at %s — run `arclint init` in the repo root, or pass --config", path)
		}
		return nil, fmt.Errorf("cannot read %s — %w", path, err)
	}

	// 1. Parse into an untyped tree. goccy/go-yaml rejects duplicate
	//    mapping keys by default (AllowDuplicateMapKey is the opt-out we
	//    deliberately do not take), so rule-id uniqueness stays structural.
	var tree any
	if err := yaml.Unmarshal(data, &tree); err != nil {
		return nil, yamlParseError(path, data, err)
	}

	// 2. YAML 1.1 gotcha: a bare `off` may parse as boolean false; coerce
	//    it back to the string "off" in severity positions before schema
	//    validation (decision D10).
	coerceSeverity(tree)

	// 3. Validate against the embedded JSON Schema.
	if err := validateAgainstSchema(tree); err != nil {
		return nil, fmt.Errorf("%s does not match the arclint rules schema — %w", path, err)
	}

	// 4. Decode the (coerced) tree into typed structs via a YAML
	//    round-trip, then run the semantic checks the schema cannot
	//    express.
	f, err := decodeFile(tree)
	if err != nil {
		return nil, fmt.Errorf("%s cannot be decoded — %w", path, err)
	}
	if err := f.validateSemantics(); err != nil {
		return nil, fmt.Errorf("%s is invalid — %w", path, err)
	}
	return f, nil
}

// yamlParseError renders a goccy/go-yaml parse error per the CLI style
// guide: one summary line with line:col and the parser message, followed by
// at most one clean quoted source line and a caret — never goccy's own
// multi-line FormatError window, which repeats context lines and reads as
// merged/reflowed once glued behind our own "error: " prefix.
func yamlParseError(path string, data []byte, err error) error {
	var se *yaml.SyntaxError
	if !errors.As(err, &se) || se.Token == nil || se.Token.Position == nil {
		// Fallback: still one line, just without a source snippet.
		return fmt.Errorf("%s is not valid YAML — %s", path, firstLine(err.Error()))
	}

	pos := se.Token.Position
	summary := fmt.Errorf("%s invalid at %d:%d — %s", path, pos.Line, pos.Column, se.Message)

	lines := strings.Split(string(data), "\n")
	if pos.Line < 1 || pos.Line > len(lines) {
		return summary
	}
	src := lines[pos.Line-1]
	caret := pos.Column - 1
	if caret < 0 {
		caret = 0
	}
	if caret > len(src) {
		caret = len(src)
	}
	return fmt.Errorf("%w\n  %s\n  %s^", summary, src, strings.Repeat(" ", caret))
}

// firstLine trims a possibly multi-line error message down to its first
// line, so a parse error we don't recognize the shape of still renders as a
// single line rather than dumping goccy's raw formatted block.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// coerceSeverity rewrites a boolean false in any rule's severity position
// to the string "off". True is left alone so the schema rejects it with a
// precise message.
func coerceSeverity(tree any) {
	root, ok := tree.(map[string]any)
	if !ok {
		return
	}
	rules, ok := root["rules"].(map[string]any)
	if !ok {
		return
	}
	for _, rv := range rules {
		rm, ok := rv.(map[string]any)
		if !ok {
			continue
		}
		if b, ok := rm["severity"].(bool); ok && !b {
			rm["severity"] = string(SeverityOff)
		}
	}
}

// decodeFile turns the validated tree into typed structs, including the
// per-category params decode.
func decodeFile(tree any) (*File, error) {
	// Round-trip through YAML so the coerced tree (not the raw bytes) is
	// what the typed structs see.
	buf, err := yaml.Marshal(tree)
	if err != nil {
		return nil, err
	}
	var f File
	if err := yaml.Unmarshal(buf, &f); err != nil {
		return nil, err
	}
	for id, r := range f.Rules {
		if err := r.decodeParams(); err != nil {
			return nil, fmt.Errorf("rule %q: %w", id, err)
		}
		f.Rules[id] = r
	}
	return &f, nil
}

// decodeParams fills exactly one typed params pointer according to Type.
func (r *Rule) decodeParams() error {
	buf, err := yaml.Marshal(r.RawParams)
	if err != nil {
		return err
	}
	switch r.Type {
	case CategoryStructure:
		p := &StructureParams{}
		if err := yaml.Unmarshal(buf, p); err != nil {
			return err
		}
		r.Structure = p
	case CategoryNaming:
		p := &NamingParams{Target: "file"}
		if err := yaml.Unmarshal(buf, p); err != nil {
			return err
		}
		if p.Target == "" {
			p.Target = "file"
		}
		r.Naming = p
	case CategoryDependencies:
		p := &DependenciesParams{}
		if err := yaml.Unmarshal(buf, p); err != nil {
			return err
		}
		r.Dependencies = p
	case CategoryContent:
		p := &ContentParams{}
		if err := yaml.Unmarshal(buf, p); err != nil {
			return err
		}
		r.Content = p
	case CategoryCustom:
		p := &CustomParams{TimeoutSeconds: DefaultCustomTimeoutSeconds}
		if err := yaml.Unmarshal(buf, p); err != nil {
			return err
		}
		if p.TimeoutSeconds == 0 {
			p.TimeoutSeconds = DefaultCustomTimeoutSeconds
		}
		r.Custom = p
	default:
		// Unreachable after schema validation; kept as a guard.
		return fmt.Errorf("unknown rule type %q", r.Type)
	}
	return nil
}

// validateSemantics runs the checks JSON Schema cannot express
// (rules.md §7 step 3). All failures are collected before returning.
func (f *File) validateSemantics() error {
	var errs []error

	for _, ig := range f.Ignore {
		for _, id := range ig.Rules {
			if _, ok := f.Rules[id]; !ok {
				errs = append(errs, fmt.Errorf("ignore entry for path %q references unknown rule id %q — remove it or add the rule", ig.Path, id))
			}
		}
	}

	for id, r := range f.Rules {
		switch {
		case r.Naming != nil:
			for _, alt := range strings.Split(r.Naming.Style, "|") {
				alt = strings.TrimSpace(alt)
				if pat, ok := strings.CutPrefix(alt, "regex:"); ok {
					if _, err := regexp.Compile(pat); err != nil {
						errs = append(errs, fmt.Errorf("rule %q: naming regex %q does not compile as RE2 — %v", id, pat, err))
					}
				}
			}
		case r.Content != nil:
			for _, m := range r.Content.MustContain {
				if _, err := regexp.Compile(m.Pattern); err != nil {
					errs = append(errs, fmt.Errorf("rule %q: mustContain pattern %q does not compile as RE2 — %v", id, m.Pattern, err))
				}
			}
			for _, m := range r.Content.MustNotContain {
				if _, err := regexp.Compile(m.Pattern); err != nil {
					errs = append(errs, fmt.Errorf("rule %q: mustNotContain pattern %q does not compile as RE2 — %v", id, m.Pattern, err))
				}
			}
		case r.Dependencies != nil:
			errs = append(errs, r.Dependencies.checkModuleRefs(id)...)
		}
	}

	return errors.Join(errs...)
}

// checkModuleRefs verifies every module name a contract references exists
// in the modules map.
func (p *DependenciesParams) checkModuleRefs(ruleID string) []error {
	var errs []error
	check := func(where, name string) {
		if _, ok := p.Modules[name]; !ok {
			errs = append(errs, fmt.Errorf("rule %q: %s references module %q which is not declared under modules", ruleID, where, name))
		}
	}
	for _, m := range p.Layers {
		check("layers", m)
	}
	for _, m := range p.Among {
		check("among", m)
	}
	for _, e := range p.Forbidden {
		for _, m := range e.From {
			check("forbidden.from", m)
		}
		for _, m := range e.To {
			check("forbidden.to", m)
		}
	}
	for name, deps := range p.MayDependOn {
		check("mayDependOn", name)
		for _, m := range deps {
			check("mayDependOn."+name, m)
		}
	}
	return errs
}
