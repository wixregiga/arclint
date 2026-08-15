package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/engine"
)

// contextRule is one rule binding the queried location, JSON shape.
type contextRule struct {
	ID          string `json:"id"`
	Clause      string `json:"clause"`
	Kind        string `json:"kind"`
	Severity    string `json:"severity"`
	Capability  string `json:"capability,omitempty"`
	Description string `json:"description,omitempty"`
}

// contextModule is the architectural context of one owning module.
type contextModule struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Paths       []string `json:"paths"`
	// Internal mirrors the consumes contract: "unrestricted" when no
	// internal policy is declared, otherwise the allow or deny wording
	// shown in the text output.
	Internal string        `json:"internal"`
	External string        `json:"external"`
	Stdlib   string        `json:"stdlib"`
	Rules    []contextRule `json:"rules"`
}

// contextOut is the full `arclint context --format json` document.
type contextOut struct {
	Path      string          `json:"path,omitempty"`
	Modules   []contextModule `json:"modules"`
	RepoRules []contextRule   `json:"repoRules,omitempty"`
	Verify    string          `json:"verify"`
}

// newContextCmd resolves the architectural context of one location: the
// modules that own it, what they may import, and every rule binding
// them. The output is the smallest deterministic context an agent (or a
// human) needs before modifying code at that location.
func newContextCmd() *cobra.Command {
	var rulesFlag, format string
	cmd := &cobra.Command{
		Use:   "context <path|module>",
		Short: "the architectural context of one location: owning modules, allowed imports, binding rules",
		Long: `Resolves a repo-relative path (file or directory) to the modules that
own it and prints each module's contract surface: description, allowed
internal imports, external and stdlib policy, and every rule that binds
the module. An argument naming a declared module exactly is treated as
the module itself. Use --format json for the machine-readable form.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rs, err := loadForRead(rulesFlag)
			if err != nil {
				return err
			}
			arg := args[0]
			out := contextOut{Verify: verifyCommand(rulesFlag)}

			var names []string
			if _, ok := rs.Modules[arg]; ok {
				names = []string{arg}
			} else {
				mods, found, err := engine.PathModules(rs, arg)
				if err != nil {
					return &exitError{2, err.Error()}
				}
				if !found {
					return &exitError{2, fmt.Sprintf(
						"%q names no declared module and no walked file or directory (paths are repo-relative)", arg)}
				}
				names = mods
				out.Path = arg
			}

			for _, name := range names {
				out.Modules = append(out.Modules, moduleContext(rs, name))
			}
			if len(rs.Rules) > 0 {
				reg, err := loadExtensionRegistry(rs)
				if err != nil {
					return err
				}
				for _, inst := range extensionRows(rs, reg) {
					out.RepoRules = append(out.RepoRules, toContextRule(inst))
				}
			}

			if format == "json" {
				data, err := json.MarshalIndent(out, "", "  ")
				if err != nil {
					return &exitError{2, err.Error()}
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}
			renderContext(cmd, out)
			return nil
		},
	}
	cmd.Flags().StringVar(&rulesFlag, "rules", "", "path to rules.yaml (default: discovered upward from .)")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return cmd
}

func verifyCommand(rulesFlag string) string {
	if rulesFlag != "" {
		return "arclint check . --rules " + rulesFlag
	}
	return "arclint check ."
}

// moduleContext assembles one module's context from the loaded ruleset.
func moduleContext(rs *config.RuleSet, name string) contextModule {
	def := rs.Modules[name]
	m := contextModule{
		Name:        name,
		Description: def.Description,
		Paths:       def.Paths,
		Internal:    "unrestricted (no consumes contract)",
		External:    "allow",
		Stdlib:      "allow",
	}
	if c := rs.Contracts[name]; c.Consumes != nil {
		m.Internal = internalWording(c.Consumes.Internal)
		if c.Consumes.External != "" {
			m.External = c.Consumes.External
		}
		if c.Consumes.Stdlib != "" {
			m.Stdlib = c.Consumes.Stdlib
		}
	}
	for _, inst := range rs.Instances() {
		if inst.Module == name || graphRuleNames(rs, inst, name) {
			m.Rules = append(m.Rules, toContextRule(inst))
		}
	}
	return m
}

func internalWording(p *config.InternalPolicy) string {
	switch {
	case p == nil:
		return "unrestricted (any declared module)"
	case p.Restricted && len(p.Allow) == 0:
		return "none (may import no other declared module)"
	case p.Restricted:
		return "allow: " + strings.Join(p.Allow, ", ")
	default:
		return "any except: " + strings.Join(p.Deny, ", ")
	}
}

func toContextRule(inst config.RuleInstance) contextRule {
	return contextRule{
		ID:          inst.ID,
		Clause:      inst.Clause,
		Kind:        inst.Kind,
		Severity:    inst.Severity,
		Capability:  inst.Capability,
		Description: inst.Description,
	}
}

func renderContext(cmd *cobra.Command, out contextOut) {
	w := cmd.OutOrStdout()
	if out.Path != "" {
		fmt.Fprintf(w, "path: %s\n", out.Path)
	}
	if len(out.Modules) == 0 {
		fmt.Fprintln(w, "modules: none (no declared module owns this path)")
	}
	for _, m := range out.Modules {
		fmt.Fprintln(w)
		desc := m.Description
		if desc == "" {
			desc = "(no description; add one under modules." + m.Name + ".description)"
		}
		fmt.Fprintf(w, "%s — %s\n", m.Name, desc)
		fmt.Fprintf(w, "  paths: %s\n", strings.Join(m.Paths, " "))
		fmt.Fprintf(w, "  internal imports: %s\n", m.Internal)
		fmt.Fprintf(w, "  external imports: %s · stdlib: %s\n", m.External, m.Stdlib)
		if len(m.Rules) == 0 {
			fmt.Fprintln(w, "  rules: none bind this module")
			continue
		}
		fmt.Fprintln(w, "  rules:")
		tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
		for _, r := range m.Rules {
			fmt.Fprintf(tw, "    %s\t[%s/%s]\t%s\t%s\n", r.ID, r.Clause, r.Kind, r.Severity, r.Description)
		}
		tw.Flush()
	}
	if len(out.RepoRules) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "repo-wide extension rules:")
		tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
		for _, r := range out.RepoRules {
			fmt.Fprintf(tw, "  %s\t[%s/%s]\t%s\t%s\n", r.ID, r.Clause, r.Kind, r.Severity, r.Description)
		}
		tw.Flush()
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "verify: %s\n", out.Verify)
}
