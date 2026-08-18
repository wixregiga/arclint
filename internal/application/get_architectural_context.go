package application

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// ContextRequest selects the scope: no paths and no modules means the
// repository; Paths are repo-relative files or folders (a folder
// matches through the Module path globs), and Modules name declared
// Modules directly.
type ContextRequest struct {
	Paths   []string
	Modules []string
}

// PathBinding maps one requested path to the declared Modules owning
// it.
type PathBinding struct {
	Path    string
	Modules []string
}

// KindInUse pairs one Rule Type appearing in the configuration with
// its one-line meaning.
type KindInUse struct {
	Kind    string
	Meaning string
}

// ModulePolicy is the plain view of one declared Module and its
// dependency policy, derived from the consumes Rule bound to it.
type ModulePolicy struct {
	Name        string
	Description string
	Paths       []string
	// Internal is the allow-list of other Modules; nil when
	// unrestricted, empty when the Module may import no other Module.
	Internal           []string
	InternalRestricted bool
	External           string // allow or forbid
	Stdlib             string // allow or forbid
}

// AppliedRule pairs one Rule with the reason it applies to the scope.
type AppliedRule struct {
	Summary RuleSummary
	Reason  string
	// Via lists the scope parts that pulled the Rule in; empty when
	// the scope has a single part.
	Via []string
}

// ArchitecturalContext is the human- and agent-readable view of the
// Rules, Modules, and applicability reasons for one scope: the same
// facts for both audiences, distinguishing intended Rules from
// observed code.
type ArchitecturalContext struct {
	// Scope is "repository" or a repo-relative path.
	Scope     string
	Languages []string
	RuleCount int
	// Modules holds every declared Module for repository scope, the
	// involved Modules for a worksite scope.
	Modules []ModulePolicy
	// Rules holds, for a worksite scope, each Rule that applies and
	// why, deduplicated across the scope parts.
	Rules []AppliedRule
	// Paths maps each requested path to its owning Modules; empty for
	// repository scope.
	Paths []PathBinding
	// Kinds lists the Rule Types the configuration uses with their
	// meanings; repository scope only.
	Kinds []KindInUse
	// UnknownImports is the effective scan policy for unclassifiable
	// imports; repository scope only.
	UnknownImports string
}

// GetArchitecturalContext projects Rules, Modules, and applicability
// reasons for a selected scope.
type GetArchitecturalContext struct {
	rules rule.Repository
}

// NewGetArchitecturalContext requires the Rule repository port.
func NewGetArchitecturalContext(rules rule.Repository) (GetArchitecturalContext, error) {
	if rules == nil {
		return GetArchitecturalContext{}, fmt.Errorf("architectural context: missing rule repository")
	}
	return GetArchitecturalContext{rules: rules}, nil
}

// Execute projects the context for one scope: an empty request means
// the repository (every Module, the Rule kinds in use, and the
// enforcement posture); paths and named Modules select a worksite (its
// per-path bindings, the involved Modules, and the deduplicated Rules
// that govern it).
func (uc GetArchitecturalContext) Execute(req ContextRequest) (ArchitecturalContext, error) {
	cfg, err := uc.rules.ConfiguredRules()
	if err != nil {
		return ArchitecturalContext{}, fmt.Errorf("load configured rules: %w", err)
	}
	out := ArchitecturalContext{Scope: "repository", RuleCount: len(cfg.Rules)}
	for _, l := range cfg.Languages {
		out.Languages = append(out.Languages, string(l))
	}
	if len(req.Paths) == 0 && len(req.Modules) == 0 {
		for _, m := range cfg.Modules {
			out.Modules = append(out.Modules, modulePolicy(m, cfg.Rules))
		}
		out.Kinds = kindsInUse(cfg.Rules)
		policy := cfg.Scan.UnknownImports
		if policy == "" {
			policy = rule.UnknownImportsWarn
		}
		out.UnknownImports = string(policy)
		return out, nil
	}
	return worksite(out, cfg, req)
}

// worksite assembles the scoped view: per-path Module bindings, each
// involved Module once, and the union of governing Rules with the
// scope parts that pulled each in.
func worksite(out ArchitecturalContext, cfg rule.Configured, req ContextRequest) (ArchitecturalContext, error) {
	declared := map[rule.ModuleName]rule.Module{}
	for _, m := range cfg.Modules {
		declared[m.Name()] = m
	}
	seenCard := map[rule.ModuleName]bool{}
	addCard := func(name rule.ModuleName) {
		if !seenCard[name] {
			seenCard[name] = true
			out.Modules = append(out.Modules, modulePolicy(declared[name], cfg.Rules))
		}
	}
	ruleIndex := map[string]int{}
	addRule := func(r rule.Rule, reason, via string) {
		id := r.ID().Qualified()
		if i, ok := ruleIndex[id]; ok {
			out.Rules[i].Via = appendUnique(out.Rules[i].Via, via)
			return
		}
		ruleIndex[id] = len(out.Rules)
		out.Rules = append(out.Rules, AppliedRule{Summary: summarize(r), Reason: reason, Via: []string{via}})
	}

	var scopeParts []string
	for _, p := range req.Paths {
		binding := PathBinding{Path: p}
		var owning []rule.ModuleName
		for _, m := range cfg.Modules {
			if m.Contains(p) {
				owning = append(owning, m.Name())
				binding.Modules = append(binding.Modules, string(m.Name()))
				addCard(m.Name())
			}
		}
		out.Paths = append(out.Paths, binding)
		scopeParts = append(scopeParts, p)
		for _, r := range cfg.Rules {
			if reason, applies := appliesToScope(r, p, owning); applies {
				addRule(r, reason, p)
			}
		}
	}
	for _, name := range req.Modules {
		mn := rule.ModuleName(name)
		if _, ok := declared[mn]; !ok {
			return ArchitecturalContext{}, fmt.Errorf(
				"module %q is not declared; declared modules: %s", name, declaredNames(cfg.Modules))
		}
		addCard(mn)
		part := "module " + name
		scopeParts = append(scopeParts, part)
		for _, r := range cfg.Rules {
			if reason, applies := appliesToModule(r, mn); applies {
				addRule(r, reason, part)
			}
		}
	}
	out.Scope = strings.Join(scopeParts, ", ")
	if len(scopeParts) == 1 {
		for i := range out.Rules {
			out.Rules[i].Via = nil
		}
	}
	return out, nil
}

// kindsInUse lists the distinct Rule Types of the configuration in
// published enum order, each with its meaning.
func kindsInUse(rules []rule.Rule) []KindInUse {
	seen := map[rule.Type]bool{}
	for _, r := range rules {
		seen[r.Type()] = true
	}
	var out []KindInUse
	for _, t := range rule.Types() {
		if seen[t] {
			out = append(out, KindInUse{Kind: string(t), Meaning: t.Meaning()})
		}
	}
	return out
}

func appendUnique(list []string, v string) []string {
	for _, e := range list {
		if e == v {
			return list
		}
	}
	return append(list, v)
}

func declaredNames(modules []rule.Module) string {
	names := make([]string, 0, len(modules))
	for _, m := range modules {
		names = append(names, string(m.Name()))
	}
	return strings.Join(names, ", ")
}

// modulePolicy derives one Module's dependency policy from the
// consumes Rule bound to it, when any.
func modulePolicy(m rule.Module, rules []rule.Rule) ModulePolicy {
	p := ModulePolicy{
		Name:        string(m.Name()),
		Description: m.Description(),
		External:    string(rule.ImportAllow),
		Stdlib:      string(rule.ImportAllow),
	}
	for _, g := range m.Paths() {
		p.Paths = append(p.Paths, g.String())
	}
	for _, r := range rules {
		params, ok := r.Params().(rule.ConsumesParams)
		if !ok || !r.Applicability().WouldSelectModule(m.Name()) {
			continue
		}
		if params.Internal != nil {
			p.InternalRestricted = true
			p.Internal = []string{}
			for _, name := range params.Internal.Modules() {
				p.Internal = append(p.Internal, string(name))
			}
		}
		if params.External.Forbids() {
			p.External = string(rule.ImportForbid)
		}
		if params.Stdlib.Forbids() {
			p.Stdlib = string(rule.ImportForbid)
		}
		break
	}
	return p
}

// appliesToScope decides whether one Rule binds a path and states the
// reason in domain language.
func appliesToScope(r rule.Rule, path string, owning []rule.ModuleName) (string, bool) {
	switch params := r.Params().(type) {
	case rule.ConsumesParams, rule.StructureParams, rule.NamingParams, rule.ExtensionParams:
		_ = params
		if r.AppliesToFile(path, owning) {
			return fmt.Sprintf("selects the file through Module(s) %s", joinNames(sharedModules(r, owning))), true
		}
		if r.Applicability().ExcludedFile(path) && r.Applicability().WouldSelectFile(path, owning) {
			return "excluded from this Rule's Applicability", true
		}
	case rule.LayersParams:
		for _, name := range params.Layers {
			if nameIn(owning, name) {
				return fmt.Sprintf("Module %q is layered by this Rule", name), true
			}
		}
	case rule.ProtectedParams:
		if nameIn(owning, params.Module) {
			return fmt.Sprintf("Module %q is protected by this Rule", params.Module), true
		}
	case rule.AcyclicParams:
		scope := params.Modules
		if len(scope) == 0 {
			if len(owning) > 0 {
				return "every declared Module is in the acyclic scope", true
			}
			return "", false
		}
		for _, name := range scope {
			if nameIn(owning, name) {
				return fmt.Sprintf("Module %q is in the acyclic scope", name), true
			}
		}
	}
	return "", false
}

// appliesToModule decides whether one Rule binds a declared Module
// named directly in the scope.
func appliesToModule(r rule.Rule, name rule.ModuleName) (string, bool) {
	switch params := r.Params().(type) {
	case rule.ConsumesParams, rule.StructureParams, rule.NamingParams, rule.ExtensionParams:
		_ = params
		if r.Applicability().WouldSelectModule(name) {
			return fmt.Sprintf("selects Module %q", name), true
		}
	case rule.LayersParams:
		if nameIn(params.Layers, name) {
			return fmt.Sprintf("Module %q is layered by this Rule", name), true
		}
	case rule.ProtectedParams:
		if params.Module == name {
			return fmt.Sprintf("Module %q is protected by this Rule", params.Module), true
		}
	case rule.AcyclicParams:
		if len(params.Modules) == 0 {
			return "every declared Module is in the acyclic scope", true
		}
		if nameIn(params.Modules, name) {
			return fmt.Sprintf("Module %q is in the acyclic scope", name), true
		}
	}
	return "", false
}

func sharedModules(r rule.Rule, owning []rule.ModuleName) []rule.ModuleName {
	var out []rule.ModuleName
	for _, m := range r.Applicability().Modules() {
		if nameIn(owning, m) {
			out = append(out, m)
		}
	}
	return out
}

func nameIn(list []rule.ModuleName, name rule.ModuleName) bool {
	for _, v := range list {
		if v == name {
			return true
		}
	}
	return false
}

func joinNames(names []rule.ModuleName) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += string(n)
	}
	return out
}
