package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/engine"
	"github.com/wixregiga/arclint/internal/ext"
)

// newModuleCmd inspects declared modules: `module ls` and `module info`.
func newModuleCmd() *cobra.Command {
	module := &cobra.Command{Use: "module", Short: "inspect declared modules"}
	var rulesFlag string

	ls := &cobra.Command{
		Use:   "ls",
		Short: "table of declared modules: files, languages, description",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rs, err := loadForRead(rulesFlag)
			if err != nil {
				return err
			}
			infos, err := engine.ModuleInfos(rs)
			if err != nil {
				return &exitError{2, err.Error()}
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(w, "MODULE\tFILES\tLANGS\tPATHS\tDESCRIPTION")
			for _, m := range infos {
				fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\n",
					m.Name, m.Files, strings.Join(m.Langs, ","),
					strings.Join(m.Paths, " "), m.Description)
			}
			return w.Flush()
		},
	}

	info := &cobra.Command{
		Use:   "info <module>",
		Short: "everything about one module: description, members, contracts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rs, err := loadForRead(rulesFlag)
			if err != nil {
				return err
			}
			name := args[0]
			if _, ok := rs.Modules[name]; !ok {
				known := make([]string, 0, len(rs.Modules))
				for n := range rs.Modules {
					known = append(known, n)
				}
				sort.Strings(known)
				return &exitError{2, fmt.Sprintf("unknown module %q (declared: %s)", name, strings.Join(known, ", "))}
			}
			infos, err := engine.ModuleInfos(rs)
			if err != nil {
				return &exitError{2, err.Error()}
			}
			var m engine.ModuleInfo
			for _, mi := range infos {
				if mi.Name == name {
					m = mi
					break
				}
			}

			out := cmd.OutOrStdout()
			langs := ""
			if len(m.Langs) > 0 {
				langs = "[" + strings.Join(m.Langs, ",") + "] "
			}
			desc := m.Description
			if desc == "" {
				desc = "(no description; add one under modules." + name + ".description)"
			}
			fmt.Fprintf(out, "%s — %s%s\n", m.Name, langs, desc)
			fmt.Fprintf(out, "paths: %s\n", strings.Join(m.Paths, " "))
			fmt.Fprintf(out, "files: %d\n", m.Files)

			var rows []config.RuleInstance
			for _, inst := range rs.Instances() {
				if inst.Module == name || graphRuleNames(rs, inst, name) {
					rows = append(rows, inst)
				}
			}
			if len(rows) == 0 {
				fmt.Fprintln(out, "rules: none bind this module")
				return nil
			}
			fmt.Fprintln(out, "rules:")
			w := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
			for _, inst := range rows {
				fmt.Fprintf(w, "  %s\t[%s/%s]\t%s\t%s\n",
					inst.ID, inst.Clause, inst.Kind, inst.Severity, inst.Description)
			}
			return w.Flush()
		},
	}

	info.ValidArgsFunction = completeModules(&rulesFlag)
	module.AddCommand(ls, info)
	module.PersistentFlags().StringVar(&rulesFlag, "rules", "", "path to rules.yaml (default: discovered upward from .)")
	return module
}

// graphRuleNames reports whether a graph-wide instance (Module == "")
// binds the named module through any of its fields. Instances() derives
// graph rule ids positionally, so the id re-derivation here matches.
func graphRuleNames(rs *config.RuleSet, inst config.RuleInstance, name string) bool {
	if inst.Module != "" || inst.Provider != "builtin" {
		return false
	}
	for idx, r := range rs.Dependencies {
		id := r.ID
		if id == "" {
			id = fmt.Sprintf("dependencies.%s[%d]", r.Kind, idx)
		}
		if id != inst.ID {
			continue
		}
		if slices.Contains(r.Layers, name) || slices.Contains(r.From, name) ||
			slices.Contains(r.To, name) || slices.Contains(r.Modules, name) ||
			r.Module == name || slices.Contains(r.Allow, name) {
			return true
		}
		// acyclic with no module list covers every declared module.
		return r.Kind == "acyclic" && len(r.Modules) == 0
	}
	return false
}

// newExplainCmd documents rule types in the terminal, from the single
// source: the builtin doc table plus extension defineRule descriptions.
func newExplainCmd() *cobra.Command {
	var rulesFlag string
	cmd := &cobra.Command{
		Use:   "explain [kind]",
		Short: "what a rule kind means and how to write it",
		Long: "With no argument, lists every rule kind with a one-line summary.\n" +
			"With a kind (naming, layers, consumes, registration, ...) prints the\n" +
			"full explanation and a ready-to-paste example. Extension rule types\n" +
			"from .arclint/extensions/ are included when a rules.yaml is found.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			// Extension types are best-effort: explain works without a
			// rules.yaml (builtins only).
			var extTypes []*ext.RuleType
			if rs, err := loadForRead(rulesFlag); err == nil {
				if reg, err := ext.LoadDir(rs.Root, ext.Options{}); err == nil {
					extTypes = reg.Types()
				}
			}

			if len(args) == 0 {
				w := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
				fmt.Fprintln(w, "KIND\tCLAUSE\tCAPABILITY\tSUMMARY")
				for _, d := range config.RuleDocs {
					clause := d.Clause
					if clause == "" {
						clause = "-"
					}
					capability := config.CapabilityOf(d.Kind)
					if capability == "" {
						capability = "-"
					}
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", d.Kind, clause, capability, d.Summary)
				}
				for _, rt := range extTypes {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", rt.Name, rt.Contract, rt.Capability, rt.Describe())
				}
				if err := w.Flush(); err != nil {
					return err
				}
				fmt.Fprintln(out, "\nRun `arclint explain <kind>` for the full explanation and an example.")
				return nil
			}

			kind := args[0]
			if d := config.FindRuleDoc(kind); d != nil {
				fmt.Fprintf(out, "%s — %s\n", d.Kind, d.Summary)
				fmt.Fprintf(out, "where: %s\n", d.Where)
				if d.Clause != "" {
					fmt.Fprintf(out, "clause: %s (blame: %s)\n", d.Clause, d.Blame)
				}
				if capability := config.CapabilityOf(d.Kind); capability != "" {
					fmt.Fprintf(out, "capability: %s\n", capability)
				}
				fmt.Fprintf(out, "\n%s\n\nexample:\n\n", d.Doc)
				for _, line := range strings.Split(strings.TrimRight(d.Example, "\n"), "\n") {
					fmt.Fprintf(out, "  %s\n", line)
				}
				fmt.Fprintln(out, "\nProve rule behavior with `arclint rules test`: cases under .arclint/tests")
				fmt.Fprintln(out, "assert the complete finding set (scaffold one: `arclint rules scaffold <type>`).")
				return nil
			}
			for _, rt := range extTypes {
				if rt.Name != kind {
					continue
				}
				fmt.Fprintf(out, "%s — %s\n", rt.Name, rt.Describe())
				fmt.Fprintf(out, "provider: extension %s\n", rt.SourcePath)
				fmt.Fprintf(out, "clause: %s (blame: %s)\n", rt.Contract, rt.Blame)
				fmt.Fprintf(out, "capability: %s\n", rt.Capability)
				params, err := json.MarshalIndent(rt.RawSchema, "  ", "  ")
				if err != nil {
					return &exitError{2, err.Error()}
				}
				fmt.Fprintf(out, "\nparams schema:\n\n  %s\n", string(params))
				fmt.Fprintln(out, "\nProve rule behavior with `arclint rules test`: cases under .arclint/tests")
				fmt.Fprintln(out, "assert the complete finding set (scaffold one: `arclint rules scaffold <type>`).")
				return nil
			}
			return &exitError{2, fmt.Sprintf("unknown rule kind %q; run `arclint explain` for the list", kind)}
		},
	}
	cmd.ValidArgsFunction = completeExplainKinds(&rulesFlag)
	cmd.Flags().StringVar(&rulesFlag, "rules", "", "path to rules.yaml (default: discovered upward from .)")
	return cmd
}
