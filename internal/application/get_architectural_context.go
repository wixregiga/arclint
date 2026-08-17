package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

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
	// owning Modules for a path scope.
	Modules []ModulePolicy
	// Rules holds, for a path scope, each Rule that applies and why.
	Rules []AppliedRule
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

// Execute projects the context for one scope: "" for the repository,
// or a repo-relative file path.
func (uc GetArchitecturalContext) Execute(scope string) (ArchitecturalContext, error) {
	cfg, err := uc.rules.ConfiguredRules()
	if err != nil {
		return ArchitecturalContext{}, fmt.Errorf("load configured rules: %w", err)
	}
	out := ArchitecturalContext{Scope: "repository", RuleCount: len(cfg.Rules)}
	for _, l := range cfg.Languages {
		out.Languages = append(out.Languages, string(l))
	}
	if scope == "" {
		for _, m := range cfg.Modules {
			out.Modules = append(out.Modules, modulePolicy(m, cfg.Rules))
		}
		return out, nil
	}

	out.Scope = scope
	var owning []rule.ModuleName
	for _, m := range cfg.Modules {
		if m.Contains(scope) {
			owning = append(owning, m.Name())
			out.Modules = append(out.Modules, modulePolicy(m, cfg.Rules))
		}
	}
	for _, r := range cfg.Rules {
		if reason, applies := appliesToScope(r, scope, owning); applies {
			out.Rules = append(out.Rules, AppliedRule{Summary: summarize(r), Reason: reason})
		}
	}
	return out, nil
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
	case rule.ConsumesParams, rule.StructureParams, rule.NamingParams:
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
