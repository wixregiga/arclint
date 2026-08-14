package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/patterns"
)

// Dynamic completion: the shell re-executes the binary on every TAB via
// the hidden __complete command, so callbacks must stay silent and
// fast, and they must degrade to an empty list — never an error — when
// no rules.yaml exists. Candidates use cobra's "value\tdescription"
// form; only zsh/fish render the description, bash ignores it.

type compFunc = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

// completionRuleSet loads the ruleset for a completion callback:
// silent, nil on any failure.
func completionRuleSet(rulesFlag *string) *config.RuleSet {
	flag := ""
	if rulesFlag != nil {
		flag = *rulesFlag
	}
	path, err := resolveRules(flag, ".")
	if err != nil {
		return nil
	}
	rs, _, err := config.LoadCached(path, version)
	if err != nil {
		return nil
	}
	return rs
}

func completeModules(rulesFlag *string) compFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		rs := completionRuleSet(rulesFlag)
		if rs == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		out := make([]string, 0, len(rs.Modules))
		for name, def := range rs.Modules {
			if def.Description != "" {
				out = append(out, name+"\t"+def.Description)
			} else {
				out = append(out, name)
			}
		}
		sort.Strings(out)
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// ruleSelectors lists every rule id plus every namespace prefix, the
// exact vocabulary `rules show` and `check --only` accept. Extension
// instance ids derive positionally, without loading the registry:
// completion latency must not pay for esbuild.
func ruleSelectors(rs *config.RuleSet) []string {
	seen := map[string]bool{}
	var out []string
	add := func(id, desc string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		if desc != "" {
			out = append(out, id+"\t"+desc)
		} else {
			out = append(out, id)
		}
		if k := strings.IndexByte(id, ':'); k > 0 {
			ns := id[:k]
			if !seen[ns] {
				seen[ns] = true
				out = append(out, ns+"\tnamespace")
			}
		}
	}
	for _, inst := range rs.Instances() {
		add(inst.ID, inst.Description)
	}
	for i, r := range rs.Rules {
		id := r.ID
		if id == "" {
			id = fmt.Sprintf("rules.%s[%d]", r.Type, i)
		}
		add(id, r.Type)
	}
	sort.Strings(out)
	return out
}

func completeRuleSelectors(rulesFlag *string) compFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		rs := completionRuleSet(rulesFlag)
		if rs == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return ruleSelectors(rs), cobra.ShellCompDirectiveNoFileComp
	}
}

// completePatterns works without a rules.yaml: builtins always exist,
// local patterns join when the directory has them.
func completePatterns() compFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		all, err := patterns.All(".")
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		out := make([]string, 0, len(all))
		for _, p := range all {
			if p.Description != "" {
				out = append(out, p.Name+"\t"+p.Description)
			} else {
				out = append(out, p.Name)
			}
		}
		sort.Strings(out)
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeExplainKinds offers the builtin doc table always, plus the
// extension types armed in rules.yaml when one is found.
func completeExplainKinds(rulesFlag *string) compFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var out []string
		for _, d := range config.RuleDocs {
			out = append(out, d.Kind+"\t"+d.Summary)
		}
		if rs := completionRuleSet(rulesFlag); rs != nil {
			seen := map[string]bool{}
			for _, r := range rs.Rules {
				if !seen[r.Type] {
					seen[r.Type] = true
					out = append(out, r.Type+"\textension type")
				}
			}
		}
		sort.Strings(out)
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeValues returns a static value completer for enum-like flags.
func completeValues(values ...string) compFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return values, cobra.ShellCompDirectiveNoFileComp
	}
}

// mustFlagCompletion registers a flag completion; a bad flag name is a
// programmer error and fails loudly at startup, which every test hits.
func mustFlagCompletion(cmd *cobra.Command, flag string, fn compFunc) {
	if err := cmd.RegisterFlagCompletionFunc(flag, fn); err != nil {
		panic(err)
	}
}
