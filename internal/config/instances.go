package config

import (
	"fmt"
	"sort"
	"strings"
)

// RuleInstance is the presentation view of one loaded rule, consumed by
// `arclint list` and `arclint rules ls`.
type RuleInstance struct {
	ID          string
	Clause      string // consumes | provides | invariant
	Kind        string
	Module      string // "" for graph-wide rules
	Provider    string // builtin | extension:<name>
	Severity    string
	Capability  string // exact | structural | heuristic | advisory
	Description string
	// Excepts lists the clause's exception entries, shown by rules show.
	Excepts []ExceptRule
}

func sevOrDefault(s string) string {
	if s == "" {
		return "error"
	}
	return s
}

// Instances enumerates every rule in deterministic order: per sorted
// module (consumes, provides, invariants), then graph-wide dependencies.
func (rs *RuleSet) Instances() []RuleInstance {
	var out []RuleInstance
	names := make([]string, 0, len(rs.Contracts))
	for n := range rs.Contracts {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, m := range names {
		c := rs.Contracts[m]
		if c.Consumes != nil {
			var parts []string
			if c.Consumes.Internal != nil {
				p := c.Consumes.Internal
				if p.Restricted {
					parts = append(parts, fmt.Sprintf("internal allow %v", p.Allow))
				}
				if len(p.Deny) > 0 {
					parts = append(parts, fmt.Sprintf("internal deny %v", p.Deny))
				}
			}
			if c.Consumes.External != "" {
				parts = append(parts, "external "+c.Consumes.External)
			}
			if c.Consumes.Stdlib != "" {
				parts = append(parts, "stdlib "+c.Consumes.Stdlib)
			}
			id := c.Consumes.ID
			if id == "" {
				id = m + ".consumes"
			}
			out = append(out, RuleInstance{
				ID: id, Clause: "consumes", Kind: "policy", Module: m,
				Provider: "builtin", Severity: sevOrDefault(c.Consumes.Severity),
				Capability:  CapabilityOf("policy"),
				Description: strings.Join(parts, "; "),
				Excepts:     c.Consumes.Except,
			})
		}
		for i, r := range c.Provides {
			id := r.ID
			if id == "" {
				id = fmt.Sprintf("%s.provides.%s[%d]", m, r.Kind, i)
			}
			desc := ""
			switch r.Kind {
			case "registration":
				desc = fmt.Sprintf("each /%s/ must match /%s/ in module %q", r.Each, r.Match, r.InModule)
			case "correspondence":
				rel := r.Relation
				if rel == "" {
					rel = "subset"
				}
				desc = fmt.Sprintf("values of /%s/ %s values of /%s/", r.Of.Files, rel, r.InSide.Files)
			}
			out = append(out, RuleInstance{
				ID: id, Clause: "provides", Kind: r.Kind, Module: m,
				Provider: "builtin", Severity: sevOrDefault(r.Severity),
				Capability: CapabilityOf(r.Kind), Description: desc,
				Excepts:    r.Except,
			})
		}
		for i, r := range c.Invariants {
			id := r.ID
			if id == "" {
				id = fmt.Sprintf("%s.invariants.%s[%d]", m, r.Kind, i)
			}
			desc := ""
			switch r.Kind {
			case "naming":
				desc = fmt.Sprintf("case %s", r.Case)
				if r.Files != "" {
					desc += " files " + r.Files
				}
			case "structure":
				var parts []string
				if len(r.Require) > 0 {
					parts = append(parts, fmt.Sprintf("require %v", r.Require))
				}
				if len(r.Forbid) > 0 {
					parts = append(parts, fmt.Sprintf("forbid %v", r.Forbid))
				}
				desc = strings.Join(parts, "; ")
			case "content":
				var parts []string
				if len(r.Must) > 0 {
					parts = append(parts, fmt.Sprintf("must %v", r.Must))
				}
				if len(r.MustNot) > 0 {
					parts = append(parts, fmt.Sprintf("must_not %v", r.MustNot))
				}
				desc = strings.Join(parts, "; ")
			case "expr":
				desc = "assert: " + r.Assert
			}
			out = append(out, RuleInstance{
				ID: id, Clause: "invariant", Kind: r.Kind, Module: m,
				Provider: "builtin", Severity: sevOrDefault(r.Severity),
				Capability: CapabilityOf(r.Kind), Description: desc,
				Excepts:    r.Except,
			})
		}
	}
	for i, r := range rs.Dependencies {
		id := r.ID
		if id == "" {
			id = fmt.Sprintf("dependencies.%s[%d]", r.Kind, i)
		}
		desc := ""
		switch r.Kind {
		case "layers":
			desc = "layers " + strings.Join(r.Layers, " > ")
		case "forbidden":
			desc = fmt.Sprintf("from %v to %v", r.From, r.To)
		case "independence":
			desc = fmt.Sprintf("independent %v", r.Modules)
		case "protected":
			desc = fmt.Sprintf("%q importable only by %v", r.Module, r.Allow)
		case "acyclic":
			if len(r.Modules) == 0 {
				desc = "no cycles among all modules"
			} else {
				desc = fmt.Sprintf("no cycles among %v", r.Modules)
			}
		}
		out = append(out, RuleInstance{
			ID: id, Clause: "consumes", Kind: r.Kind,
			Provider: "builtin", Severity: sevOrDefault(r.Severity),
			Capability: CapabilityOf(r.Kind), Description: desc,
			Excepts:    r.Except,
		})
	}
	return out
}

// CountByClause tallies instances for the load summary.
func CountByClause(instances []RuleInstance) map[string]int {
	counts := map[string]int{}
	for _, inst := range instances {
		counts[inst.Clause]++
		if inst.Kind == "expr" {
			counts["expr"]++
		}
	}
	return counts
}
