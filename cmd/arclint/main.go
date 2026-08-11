// Command arclint enforces architecture contracts over a repository.
// Exit codes: 0 clean, 1 violations (severity error), 2 config/usage error.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/engine"
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
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newLoadCmd(), newListCmd(), newRulesCmd(), newCheckCmd())
	return root
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
			instances := rs.Instances()
			counts := config.CountByClause(instances)
			fmt.Fprintf(cmd.OutOrStdout(),
				"loaded %s: %d rules (%d consumes, %d provides, %d invariants), layers: %d declarative / %d expr / 0 extensions, targets %v, %d modules\n",
				rs.Path, len(instances), counts["consumes"], counts["provides"], counts["invariant"],
				len(instances)-counts["expr"], counts["expr"], rs.Runtime, len(rs.Modules))
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
			for _, inst := range rs.Instances() {
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
	rules := &cobra.Command{Use: "rules", Short: "inspect rules"}
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
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tCONTRACT\tKIND\tPROVIDER\tTARGETS\tSEVERITY\tDESCRIPTION")
			targets := fmt.Sprintf("%v", rs.Runtime)
			for _, inst := range rs.Instances() {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					inst.ID, inst.Clause, inst.Kind, inst.Provider, targets, inst.Severity, inst.Description)
			}
			return w.Flush()
		},
	}
	ls.Flags().StringVar(&rulesFlag, "rules", "", "path to rules.yaml (default: discovered upward from .)")
	rules.AddCommand(ls)
	return rules
}

func newCheckCmd() *cobra.Command {
	var rulesFlag, format string
	cmd := &cobra.Command{
		Use:   "check [path]",
		Short: "evaluate contracts against the repository",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "human" && format != "json" {
				return &exitError{2, fmt.Sprintf("unknown format %q (human, json)", format)}
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
			res, err := engine.Check(rs)
			if err != nil {
				return &exitError{2, err.Error()}
			}
			for _, warning := range res.Warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "warn: "+warning)
			}
			switch format {
			case "json":
				data, err := report.MarshalJSONList(res.Violations)
				if err != nil {
					return &exitError{2, err.Error()}
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			default:
				writeHuman(cmd.OutOrStdout(), res, time.Since(began))
			}
			if res.HasErrors() {
				return &exitError{code: 1}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&rulesFlag, "rules", "", "path to rules.yaml (default: discovered upward from [path])")
	cmd.Flags().StringVar(&format, "format", "human", "output format: human or json")
	return cmd
}
