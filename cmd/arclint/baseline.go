package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/engine"
	"github.com/wixregiga/arclint/internal/report"
)

// newBaselineCmd adopts the current findings so check reports only new
// ones: the tool becomes adoptable in a repository with existing debt
// without turning the debt invisible.
func newBaselineCmd() *cobra.Command {
	var rulesFlag string
	cmd := &cobra.Command{
		Use:   "baseline [path]",
		Short: "adopt current findings into .arclint/baseline.json; check then reports only new ones",
		Long: "Runs a full check (except clauses still apply) and records every\n" +
			"remaining finding in " + engine.BaselinePath + ", keyed by a fingerprint\n" +
			"of rule, path, and message — line moves do not reopen findings.\n" +
			"Commit the file. check subtracts it, always prints the count, and\n" +
			"warns when adopted findings no longer occur; rerun this command to\n" +
			"refresh after fixes. check --no-baseline shows everything.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			res, err := engine.CheckWith(rs, engine.CheckOptions{SkipBaseline: true})
			if err != nil {
				return &exitError{2, err.Error()}
			}
			for _, warning := range res.Warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "warn: "+warning)
			}
			written, err := engine.WriteBaseline(rs.Root, res.Violations)
			if err != nil {
				return &exitError{2, err.Error()}
			}
			counts := map[report.Severity]int{}
			for _, v := range res.Violations {
				counts[v.Severity]++
			}
			fmt.Fprintf(cmd.OutOrStdout(), "baseline written: %d findings (%d error, %d warn, %d info) -> %s\n",
				len(res.Violations), counts[report.SeverityError], counts[report.SeverityWarn],
				counts[report.SeverityInfo], written)
			if len(res.Suppressed) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%d findings stay suppressed by except and are not part of the baseline\n", len(res.Suppressed))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&rulesFlag, "rules", "", "path to rules.yaml (default: discovered upward from [path])")
	return cmd
}
