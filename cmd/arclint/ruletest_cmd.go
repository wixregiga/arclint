package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/patterns"
	"github.com/wixregiga/arclint/internal/ruletest"
)

// newRulesShowCmd groups every clause bound to one rule id (or one
// namespace) so a requirement like "ddd:ARCH-002" reads as one unit even
// when several checks enforce it.
func newRulesShowCmd() *cobra.Command {
	var rulesFlag, format string
	cmd := &cobra.Command{
		Use:   "show <id|namespace>",
		Short: "every clause grouped under one rule id or namespace prefix",
		Args:  cobra.ExactArgs(1),
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
			q := args[0]
			var hits []config.RuleInstance
			for _, inst := range rows {
				if inst.ID == q || strings.HasPrefix(inst.ID, q+":") {
					hits = append(hits, inst)
				}
			}
			if len(hits) == 0 {
				return &exitError{2, fmt.Sprintf("no rule matches %q (exact id or namespace prefix); run `arclint rules ls`", q)}
			}
			if format == "json" {
				data, err := json.MarshalIndent(hits, "", "  ")
				if err != nil {
					return &exitError{2, err.Error()}
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tCONTRACT\tKIND\tMODULE\tSEVERITY\tCAPABILITY\tDESCRIPTION")
			hasExcepts := false
			for _, inst := range hits {
				module := inst.Module
				if module == "" {
					module = "-"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					inst.ID, inst.Clause, inst.Kind, module, inst.Severity, inst.Capability, inst.Description)
				hasExcepts = hasExcepts || len(inst.Excepts) > 0
			}
			if err := w.Flush(); err != nil {
				return err
			}
			if hasExcepts {
				out := cmd.OutOrStdout()
				fmt.Fprintln(out, "\nexceptions:")
				for _, inst := range hits {
					for _, e := range inst.Excepts {
						fmt.Fprintf(out, "  %s  %s  %s\n", inst.ID, strings.Join(e.Paths, " "), e.Reason)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&rulesFlag, "rules", "", "path to rules.yaml (default: discovered upward from .)")
	cmd.Flags().StringVar(&format, "format", "human", "output format: human or json")
	return cmd
}

// newRulesTestCmd runs rule test cases: the repository's own under
// .arclint/tests, explicit case files, or a pattern's bundled suite.
func newRulesTestCmd() *cobra.Command {
	var patternFlag, format string
	cmd := &cobra.Command{
		Use:   "test [case-file|dir ...]",
		Short: "run rule test cases (default: .arclint/tests against this repo's ruleset)",
		Long: "A case materializes files into a fresh tree, runs the ruleset, and\n" +
			"asserts the COMPLETE violation set: unexpected findings fail the case\n" +
			"just like missing ones. --pattern runs a pattern's bundled suite\n" +
			"against its own template instead of this repository's rules.yaml.",
		RunE: func(cmd *cobra.Command, args []string) error {
			var cases []ruletest.Case
			var target ruletest.Target

			if patternFlag != "" {
				p, err := patterns.Find(".", patternFlag)
				if err != nil {
					return &exitError{2, err.Error()}
				}
				cases, err = patternCases(p)
				if err != nil {
					return &exitError{2, err.Error()}
				}
				target = patternTarget(p)
			} else {
				paths := args
				if len(paths) == 0 {
					paths = []string{".arclint/tests"}
				}
				for _, path := range paths {
					cs, err := loadPath(path)
					if err != nil {
						return &exitError{2, err.Error()}
					}
					cases = append(cases, cs...)
				}
				var err error
				target, err = ruletest.RepoTarget(".")
				if err != nil {
					return &exitError{2, err.Error()}
				}
			}
			if len(cases) == 0 {
				return &exitError{2, "no test cases found"}
			}

			out := cmd.OutOrStdout()
			var results []ruletest.Result
			failed := 0
			for _, c := range cases {
				res := ruletest.Run(c, target)
				results = append(results, res)
				if !res.Pass {
					failed++
				}
				if format != "json" {
					status := "PASS"
					if !res.Pass {
						status = "FAIL"
					}
					fmt.Fprintf(out, "%s  %s (%s)\n", status, res.Case, res.Source)
					if res.Err != "" {
						fmt.Fprintf(out, "      error: %s\n", res.Err)
					}
					for _, m := range res.Missing {
						fmt.Fprintf(out, "      missing: %s\n", m)
					}
					for _, v := range res.Unexpected {
						fmt.Fprintf(out, "      unexpected: %s %s:%d %s\n", v.RuleID, v.Path, v.LineValue(), v.Message)
					}
				}
			}
			if format == "json" {
				data, err := json.MarshalIndent(results, "", "  ")
				if err != nil {
					return &exitError{2, err.Error()}
				}
				fmt.Fprintln(out, string(data))
			} else {
				fmt.Fprintf(out, "%d cases, %d failed\n", len(results), failed)
			}
			if failed > 0 {
				return &exitError{code: 1}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&patternFlag, "pattern", "", "run a pattern's bundled test suite instead")
	cmd.Flags().StringVar(&format, "format", "human", "output format: human or json")
	return cmd
}

func loadPath(path string) ([]ruletest.Case, error) {
	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		return ruletest.Load(path)
	}
	return ruletest.LoadDir(path)
}

// patternCases parses a pattern's bundled tests/*.yaml in name order.
func patternCases(p *patterns.Pattern) ([]ruletest.Case, error) {
	names := make([]string, 0, len(p.Tests))
	for name := range p.Tests {
		names = append(names, name)
	}
	sort.Strings(names)
	var cases []ruletest.Case
	for _, name := range names {
		cs, err := ruletest.Parse(p.Tests[name], p.FullName()+"/tests/"+name)
		if err != nil {
			return nil, err
		}
		cases = append(cases, cs...)
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("pattern %q ships no tests", p.FullName())
	}
	return cases, nil
}

// patternTarget renders the pattern template for each case's runtimes.
func patternTarget(p *patterns.Pattern) ruletest.Target {
	return ruletest.Target{
		RulesFor: func(runtimes []string) ([]byte, error) {
			if len(runtimes) == 0 {
				runtimes = p.Runtimes[:1]
			}
			return p.RenderRules(runtimes)
		},
		Extensions: p.Extensions,
	}
}
