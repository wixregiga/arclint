// Command arclint enforces architecture contracts over a repository.
// Exit codes: 0 clean, 1 violations (severity error), 2 config/usage error.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/engine"
	"github.com/wixregiga/arclint/internal/ext"
	"github.com/wixregiga/arclint/internal/report"
)

var version = "0.1.0"

type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	root := newRootCmd()
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		if ee, ok := err.(*exitError); ok {
			if ee.msg != "" {
				fmt.Fprintln(os.Stderr, "arclint: "+ee.msg)
			}
			return ee.code
		}
		fmt.Fprintln(os.Stderr, "arclint: "+err.Error())
		return 2
	}
	return 0
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "arclint",
		Short:         "architecture contracts as data: consumes, provides, invariants",
		Version:       buildVersion(version),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newLoadCmd(), newListCmd(), newRulesCmd(), newCheckCmd(), newSdkCmd(),
		newModuleCmd(), newExplainCmd(), newInitCmd(), newPatternsCmd(), newFactsCmd(),
		newBaselineCmd(), newContextCmd(), newAgentsCmd())
	return root
}

// loadExtensionRegistry discovers and registers extensions for a loaded
// ruleset, and validates every rules.yaml extension instance against its
// provider's schema. Failures are configuration errors (exit 2).
func loadExtensionRegistry(rs *config.RuleSet) (*ext.Registry, error) {
	reg, err := ext.LoadDir(rs.Root, ext.Options{
		CacheDir: filepath.Join(rs.Root, ".arclint", "cache"),
	})
	if err != nil {
		return nil, &exitError{2, err.Error()}
	}
	for i, inst := range rs.Rules {
		rt := reg.Get(inst.Type)
		if rt == nil {
			return nil, &exitError{2, fmt.Sprintf("rules[%d]: no extension registers rule type %q (looked in %s)",
				i, inst.Type, filepath.Join(rs.Root, ext.ExtensionsDir))}
		}
		if _, err := rt.ValidateParams(inst.Params); err != nil {
			return nil, &exitError{2, fmt.Sprintf("rules[%d]: %v", i, err)}
		}
	}
	return reg, nil
}

// extensionRows presents rules.yaml extension instances for list/rules ls.
func extensionRows(rs *config.RuleSet, reg *ext.Registry) []config.RuleInstance {
	var rows []config.RuleInstance
	for i, inst := range rs.Rules {
		rt := reg.Get(inst.Type)
		id := inst.ID
		if id == "" {
			id = fmt.Sprintf("rules.%s[%d]", inst.Type, i)
		}
		sev := inst.Severity
		if sev == "" {
			sev = "error"
		}
		rows = append(rows, config.RuleInstance{
			ID:          id,
			Clause:      rt.Contract,
			Kind:        inst.Type,
			Provider:    "extension:" + rt.SourcePath,
			Severity:    sev,
			Capability:  rt.Capability,
			Description: rt.Describe(),
			Excepts:     inst.Except,
		})
	}
	return rows
}

// findRulesFile locates rules.yaml: from a directory upward to the
// filesystem root.
func findRulesFile(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	if fi, err := os.Stat(abs); err == nil && !fi.IsDir() {
		abs = filepath.Dir(abs)
	}
	dir := abs
	for {
		cand := filepath.Join(dir, "rules.yaml")
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			return cand, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no rules.yaml found in %s or any parent directory", abs)
		}
		dir = parent
	}
}

func resolveRules(rulesFlag, start string) (string, error) {
	if rulesFlag != "" {
		abs, err := filepath.Abs(rulesFlag)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	return findRulesFile(start)
}

func newLoadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "load [rules.yaml]",
		Short: "parse, validate, and cache a ruleset",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "rules.yaml"
			if len(args) == 1 {
				path = args[0]
			}
			rs, err := config.Load(path)
			if err != nil {
				return &exitError{2, err.Error()}
			}
			if err := config.WriteCache(rs, version); err != nil {
				return &exitError{2, fmt.Sprintf("cannot write cache: %v", err)}
			}
			reg, err := loadExtensionRegistry(rs)
			if err != nil {
				return err
			}
			instances := rs.Instances()
			counts := config.CountByClause(instances)
			total := len(instances) + len(rs.Rules)
			fmt.Fprintf(cmd.OutOrStdout(),
				"loaded %s: %d rules (%d consumes, %d provides, %d invariants, %d extension instances), layers: %d declarative / %d expr / %d extension types, targets %v, %d modules\n",
				rs.Path, total, counts["consumes"], counts["provides"], counts["invariant"], len(rs.Rules),
				len(instances)-counts["expr"], counts["expr"], len(reg.Types()), rs.Runtime, len(rs.Modules))
			// Inert-rule visibility (M10): extension types nobody armed,
			// and armed instances whose params disarm them silently.
			armed := map[string]bool{}
			for _, inst := range rs.Rules {
				armed[inst.Type] = true
			}
			var dormant []string
			for _, rt := range reg.Types() {
				if !armed[rt.Name] {
					dormant = append(dormant, rt.Name+" ("+rt.SourcePath+")")
				}
			}
			if len(dormant) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "registered but not instantiated: %s\n", strings.Join(dormant, ", "))
			}
			for i, inst := range rs.Rules {
				for _, p := range ext.EmptyListParams(inst.Params) {
					fmt.Fprintf(cmd.ErrOrStderr(), "warn: rules[%d] (%s): param %q is an empty list (rule may be inert)\n",
						i, inst.Type, p)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "cache written: %s\n", filepath.Join(rs.Root, ".arclint", "cache.json"))
			return nil
		},
	}
}

func loadForRead(rulesFlag string) (*config.RuleSet, error) {
	path, err := resolveRules(rulesFlag, ".")
	if err != nil {
		return nil, &exitError{2, err.Error()}
	}
	rs, _, err := config.LoadCached(path, version)
	if err != nil {
		return nil, &exitError{2, err.Error()}
	}
	return rs, nil
}

func newListCmd() *cobra.Command {
	var rulesFlag string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "one-line-per-rule summary of the loaded ruleset",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rs, err := loadForRead(rulesFlag)
			if err != nil {
				return err
			}
			rows := rs.Instances()
			if len(rs.Rules) > 0 {
				reg, err := loadExtensionRegistry(rs)
				if err != nil {
					return err
				}
				rows = append(rows, extensionRows(rs, reg)...)
			}
			for _, inst := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  [%s/%s]  %s  %s\n",
					inst.ID, inst.Clause, inst.Kind, inst.Severity, inst.Description)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&rulesFlag, "rules", "", "path to rules.yaml (default: discovered upward from .)")
	return cmd
}

func newRulesCmd() *cobra.Command {
	rules := &cobra.Command{Use: "rules", Aliases: []string{"rule"}, Short: "inspect and test rules"}
	var rulesFlag string
	ls := &cobra.Command{
		Use:   "ls",
		Short: "detailed rule table",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rs, err := loadForRead(rulesFlag)
			if err != nil {
				return err
			}
			rows := rs.Instances()
			if len(rs.Rules) > 0 {
				reg, err := loadExtensionRegistry(rs)
				if err != nil {
					return err
				}
				rows = append(rows, extensionRows(rs, reg)...)
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tCONTRACT\tKIND\tMODULE\tPROVIDER\tTARGETS\tSEVERITY\tCAPABILITY\tDESCRIPTION")
			targets := fmt.Sprintf("%v", rs.Runtime)
			for _, inst := range rows {
				module := inst.Module
				if module == "" {
					module = "-"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					inst.ID, inst.Clause, inst.Kind, module, inst.Provider, targets, inst.Severity, inst.Capability, inst.Description)
			}
			return w.Flush()
		},
	}
	ls.Flags().StringVar(&rulesFlag, "rules", "", "path to rules.yaml (default: discovered upward from .)")
	rules.AddCommand(ls, newRulesShowCmd(), newRulesTestCmd(), newRulesScaffoldCmd())
	return rules
}

func newSdkCmd() *cobra.Command {
	sdk := &cobra.Command{Use: "sdk", Short: "extension SDK utilities"}
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "write arclint.d.ts (generated from the Go host types) and tsconfig.json into .arclint/extensions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return &exitError{2, err.Error()}
			}
			files, err := ext.SDKInit(cwd)
			if err != nil {
				return &exitError{2, err.Error()}
			}
			for _, f := range files {
				fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", f)
			}
			return nil
		},
	}
	sdk.AddCommand(initCmd)
	return sdk
}

// checkFormats is the closed set `check --format` accepts; completion
// and validation read the same list so they cannot drift. line feeds
// VS Code problemMatchers and vim errorformat; sarif feeds GitHub code
// scanning and the SARIF Viewer.
var checkFormats = []string{"human", "json", "line", "sarif"}

func newCheckCmd() *cobra.Command {
	var rulesFlag, format, only string
	var showSuppressed, showBaselined, noBaseline bool
	cmd := &cobra.Command{
		Use:   "check [path]",
		Short: "evaluate contracts against the repository",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !slices.Contains(checkFormats, format) {
				return &exitError{2, fmt.Sprintf("unknown format %q (%s)", format, strings.Join(checkFormats, ", "))}
			}
			start := "."
			if len(args) == 1 {
				start = args[0]
			}
			path, err := resolveRules(rulesFlag, start)
			if err != nil {
				return &exitError{2, err.Error()}
			}
			rs, _, err := config.LoadCached(path, version)
			if err != nil {
				return &exitError{2, err.Error()}
			}
			began := time.Now()
			res, err := engine.CheckWith(rs, engine.CheckOptions{SkipBaseline: noBaseline})
			if err != nil {
				return &exitError{2, err.Error()}
			}
			if only != "" {
				if err := filterOnly(rs, res, only); err != nil {
					return err
				}
			}
			for _, warning := range res.Warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "warn: "+warning)
			}
			// Suppressed and baselined findings join machine output only
			// on request, marked, and never affect the exit code.
			withOmitted := func() []report.Violation {
				violations := append([]report.Violation{}, res.Violations...)
				if showSuppressed {
					violations = append(violations, res.Suppressed...)
				}
				if showBaselined {
					violations = append(violations, res.Baselined...)
				}
				return violations
			}
			switch format {
			case "json":
				data, err := report.MarshalJSONList(withOmitted())
				if err != nil {
					return &exitError{2, err.Error()}
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			case "line":
				// Editors show problems, not policy: suppressed and
				// baselined findings stay out of the line format.
				cmd.OutOrStdout().Write(report.MarshalLineList(res.Violations))
			case "sarif":
				data, err := report.MarshalSARIF(withOmitted(), buildVersion(version))
				if err != nil {
					return &exitError{2, err.Error()}
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			default:
				writeHuman(cmd.OutOrStdout(), res, time.Since(began), showSuppressed, showBaselined)
			}
			if res.HasErrors() {
				return &exitError{code: 1}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&rulesFlag, "rules", "", "path to rules.yaml (default: discovered upward from [path])")
	cmd.Flags().StringVar(&format, "format", "human", "output format: "+strings.Join(checkFormats, ", "))
	cmd.Flags().BoolVar(&showSuppressed, "show-suppressed", false, "also list findings dropped by except clauses, with reasons")
	cmd.Flags().BoolVar(&showBaselined, "show-baselined", false, "also list findings covered by the committed baseline")
	cmd.Flags().BoolVar(&noBaseline, "no-baseline", false, "evaluate without subtracting .arclint/baseline.json")
	cmd.Flags().StringVar(&only, "only", "", "restrict findings to one rule id or namespace prefix (exit code follows the filtered set)")
	mustFlagCompletion(cmd, "only", completeRuleSelectors(&rulesFlag))
	mustFlagCompletion(cmd, "format", completeValues(checkFormats...))
	return cmd
}

// filterOnly restricts a check result to one rule id or namespace
// prefix, post-collection. An unknown selector is a usage error: a
// typo'd --only must not report a false clean.
func filterOnly(rs *config.RuleSet, res *engine.Result, only string) error {
	rows := rs.Instances()
	if len(rs.Rules) > 0 {
		reg, err := loadExtensionRegistry(rs)
		if err != nil {
			return err
		}
		rows = append(rows, extensionRows(rs, reg)...)
	}
	known := false
	for _, inst := range rows {
		if inst.ID == only || strings.HasPrefix(inst.ID, only+":") {
			known = true
			break
		}
	}
	if !known {
		return &exitError{2, fmt.Sprintf("--only %q matches no rule id or namespace; run `arclint rules ls`", only)}
	}
	keep := func(vs []report.Violation) []report.Violation {
		var out []report.Violation
		for _, v := range vs {
			if v.RuleID == only || strings.HasPrefix(v.RuleID, only+":") {
				out = append(out, v)
			}
		}
		return out
	}
	res.Violations = keep(res.Violations)
	res.Suppressed = keep(res.Suppressed)
	res.Baselined = keep(res.Baselined)
	return nil
}
