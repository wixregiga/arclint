package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/wixregiga/arclint/internal/exprenv"
)

var (
	moduleNameRe  = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	templateRefRe = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)
	namedCases    = map[string]bool{
		"kebab-case": true, "snake_case": true, "camelCase": true, "PascalCase": true,
	}
)

// validateSemantics checks everything the schema cannot: cross-references
// between rules and modules, regex compilation, capture/template coherence,
// and expr type-checking. All problems are reported at once.
func (rs *RuleSet) validateSemantics() error {
	var errs []string
	addf := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf(format, args...))
	}

	// Targets go, ts, py are all implemented; the schema enum rejects
	// anything else before this runs.
	for _, g := range rs.Scan.Exclude {
		if !doublestar.ValidatePattern(g) {
			addf("scan.exclude: invalid glob %q", g)
		}
	}

	if len(rs.Modules) == 0 {
		addf("modules: at least one module is required")
	}
	for name, def := range rs.Modules {
		if !moduleNameRe.MatchString(name) {
			addf("modules.%s: names must match %s (they appear in rule ids)", name, moduleNameRe)
		}
		if len(def.Paths) == 0 {
			addf("modules.%s: at least one glob is required", name)
		}
		for _, g := range def.Paths {
			if !doublestar.ValidatePattern(g) {
				addf("modules.%s: invalid glob %q", name, g)
			}
		}
	}
	moduleExists := func(name string) bool {
		_, ok := rs.Modules[name]
		return ok
	}
	checkRefs := func(where string, names []string) {
		for _, n := range names {
			if !moduleExists(n) {
				addf("%s: unknown module %q", where, n)
			}
		}
	}

	for name, contract := range rs.Contracts {
		if !moduleExists(name) {
			addf("contracts.%s: unknown module", name)
			continue
		}
		if c := contract.Consumes; c != nil {
			if c.Internal != nil {
				checkRefs(fmt.Sprintf("contracts.%s.consumes.internal.allow", name), c.Internal.Allow)
				checkRefs(fmt.Sprintf("contracts.%s.consumes.internal.deny", name), c.Internal.Deny)
			}
		}
		for i, rule := range contract.Provides {
			where := fmt.Sprintf("contracts.%s.provides[%d]", name, i)
			switch rule.Kind {
			case "registration":
				validateRegistration(addf, where, rule, moduleExists)
			case "correspondence":
				validateCorrespondence(addf, where, rule)
			}
		}
		for i, rule := range contract.Invariants {
			where := fmt.Sprintf("contracts.%s.invariants[%d]", name, i)
			validateInvariant(addf, where, rule)
		}
	}

	for i, rule := range rs.Dependencies {
		where := fmt.Sprintf("dependencies[%d] (%s)", i, rule.Kind)
		switch rule.Kind {
		case "layers":
			if len(rule.Layers) < 2 {
				addf("%s: needs at least two layers", where)
			}
			seen := map[string]bool{}
			for _, l := range rule.Layers {
				if seen[l] {
					addf("%s: duplicate layer %q", where, l)
				}
				seen[l] = true
			}
			checkRefs(where+".layers", rule.Layers)
		case "forbidden":
			if len(rule.From) == 0 || len(rule.To) == 0 {
				addf("%s: from and to are required", where)
			}
			checkRefs(where+".from", rule.From)
			checkRefs(where+".to", rule.To)
		case "independence":
			if len(rule.Modules) < 2 {
				addf("%s: needs at least two modules", where)
			}
			checkRefs(where+".modules", rule.Modules)
		case "protected":
			if rule.Module == "" {
				addf("%s: module is required", where)
			} else {
				checkRefs(where+".module", []string{rule.Module})
			}
			checkRefs(where+".allow", rule.Allow)
		case "acyclic":
			checkRefs(where+".modules", rule.Modules)
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("invalid rules.yaml:\n  - %s", strings.Join(errs, "\n  - "))
}

// namedGroups returns the named capture groups of a pattern, or nil when
// it does not compile (reported separately).
func namedGroups(pattern string) map[string]bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	groups := map[string]bool{}
	for _, n := range re.SubexpNames() {
		if n != "" {
			groups[n] = true
		}
	}
	return groups
}

func checkTemplateRefs(addf func(string, ...any), where, template string, groups map[string]bool) {
	for _, m := range templateRefRe.FindAllStringSubmatch(template, -1) {
		if !groups[m[1]] {
			addf("%s: template references {%s}, which is not a named capture group", where, m[1])
		}
	}
}

func validateRegistration(addf func(string, ...any), where string, rule ProvidesRule, moduleExists func(string) bool) {
	if rule.Each == "" {
		addf("%s: each is required", where)
		return
	}
	if _, err := regexp.Compile(rule.Each); err != nil {
		addf("%s: each does not compile: %v", where, err)
		return
	}
	if rule.Match == "" {
		addf("%s: match is required", where)
	}
	if rule.InModule == "" {
		addf("%s: in must name a module", where)
	} else if !moduleExists(rule.InModule) {
		addf("%s: in: unknown module %q", where, rule.InModule)
	}
	if rule.Of != nil || rule.InSide != nil || rule.Relation != "" {
		addf("%s: of/relation belong to correspondence rules", where)
	}
	groups := namedGroups(rule.Each)
	checkTemplateRefs(addf, where+".match", rule.Match, groups)
	// The expanded pattern must compile with any capture value; QuoteMeta
	// guarantees values are inert, so a placeholder expansion is a valid
	// proof.
	probe := map[string]string{}
	for g := range groups {
		probe[g] = "x"
	}
	expanded := rule.Match
	for k, v := range probe {
		expanded = strings.ReplaceAll(expanded, "{"+k+"}", regexp.QuoteMeta(v))
	}
	if _, err := regexp.Compile(expanded); err != nil {
		addf("%s: match template does not compile: %v", where, err)
	}
}

func validateSide(addf func(string, ...any), where string, side *CaptureSide) map[string]bool {
	if side == nil {
		addf("%s: required", where)
		return nil
	}
	if side.Files == "" {
		addf("%s.files: required", where)
		return nil
	}
	groups := map[string]bool{}
	if _, err := regexp.Compile("^(?:" + side.Files + ")$"); err != nil {
		addf("%s.files: does not compile: %v", where, err)
	} else {
		for g := range namedGroups("^(?:" + side.Files + ")$") {
			groups[g] = true
		}
	}
	if side.Content != "" {
		if _, err := regexp.Compile(side.Content); err != nil {
			addf("%s.content: does not compile: %v", where, err)
		} else {
			for g := range namedGroups(side.Content) {
				groups[g] = true
			}
		}
	}
	if side.Value == "" {
		addf("%s.value: required", where)
	}
	checkTemplateRefs(addf, where+".value", side.Value, groups)
	return groups
}

func validateCorrespondence(addf func(string, ...any), where string, rule ProvidesRule) {
	validateSide(addf, where+".of", rule.Of)
	validateSide(addf, where+".in", rule.InSide)
	if rule.InModule != "" {
		addf("%s: in must be a capture side mapping for correspondence rules", where)
	}
	if rule.Each != "" || rule.Match != "" {
		addf("%s: each/match belong to registration rules", where)
	}
}

func validateInvariant(addf func(string, ...any), where string, rule InvariantRule) {
	if rule.Files != "" && !doublestar.ValidatePattern(rule.Files) {
		addf("%s.files: invalid glob %q", where, rule.Files)
	}
	switch rule.Kind {
	case "naming":
		if rule.Case == "" {
			addf("%s: case is required", where)
			return
		}
		for _, alt := range strings.Split(rule.Case, "|") {
			alt = strings.TrimSpace(alt)
			if namedCases[alt] {
				continue
			}
			if pat, ok := strings.CutPrefix(alt, "regex:"); ok {
				if _, err := regexp.Compile("^(?:" + pat + ")$"); err != nil {
					addf("%s.case: regex does not compile: %v", where, err)
				}
				continue
			}
			addf("%s.case: unknown case %q (kebab-case, snake_case, camelCase, PascalCase, regex:<pattern>)", where, alt)
		}
	case "structure":
		if len(rule.Require) == 0 && len(rule.Forbid) == 0 {
			addf("%s: require or forbid is required", where)
		}
		for _, g := range append(append([]string{}, rule.Require...), rule.Forbid...) {
			if !doublestar.ValidatePattern(g) {
				addf("%s: invalid glob %q", where, g)
			}
		}
	case "content":
		if len(rule.Must) == 0 && len(rule.MustNot) == 0 {
			addf("%s: must or must_not is required", where)
		}
		for _, p := range append(append([]string{}, rule.Must...), rule.MustNot...) {
			if _, err := regexp.Compile(p); err != nil {
				addf("%s: regex %q does not compile: %v", where, p, err)
			}
		}
	case "expr":
		if rule.Assert == "" {
			addf("%s: assert is required", where)
			return
		}
		if _, err := exprenv.Compile(rule.Assert); err != nil {
			addf("%s: %v", where, err)
		}
	}
}
