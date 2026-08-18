// Package cobraadapter translates the neutral ArcLint CLI Interface
// into Cobra. It is the only package permitted to import Cobra, and
// Cobra types never cross the sealed Interface.
package cobraadapter

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/wixregiga/arclint/internal/delivery/cli"
)

// Adapter is the sealed Cobra implementation of the CLI Interface.
type Adapter struct{}

// New returns the Cobra adapter.
func New() Adapter { return Adapter{} }

// Run translates the neutral command tree, executes it, and maps the
// result onto the exit-code contract.
func (Adapter) Run(root cli.Command, invocation cli.Invocation) cli.Outcome {
	command := translate(root)
	command.SetArgs(invocation.Args)
	command.SetOut(invocation.Stdout)
	command.SetErr(invocation.Stderr)
	if err := command.Execute(); err != nil {
		var exit *cli.ExitError
		if errors.As(err, &exit) {
			if exit.Message != "" {
				// The nonzero exit code already carries the failure; a
				// failed stderr write leaves nothing further to do.
				_, _ = fmt.Fprintln(invocation.Stderr, "arclint: "+exit.Message)
			}
			return cli.Outcome{ExitCode: exit.Code}
		}
		_, _ = fmt.Fprintln(invocation.Stderr, "arclint: "+err.Error())
		return cli.Outcome{ExitCode: cli.ExitConfigError}
	}
	return cli.Outcome{ExitCode: cli.ExitClean}
}

func translate(c cli.Command) *cobra.Command {
	out := &cobra.Command{
		Use:           c.Name,
		Short:         c.Short,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	if c.Version != "" {
		out.Version = c.Version
	}
	values := map[string]*string{}
	bools := map[string]*bool{}
	for _, f := range c.Flags {
		if f.Bool {
			bools[f.Name] = out.Flags().Bool(f.Name, f.Default == "true", f.Doc)
		} else {
			values[f.Name] = out.Flags().String(f.Name, f.Default, f.Doc)
			// Registration fails only for an unknown flag name; the
			// flag was defined on the line above.
			switch {
			case f.Complete != nil:
				_ = out.RegisterFlagCompletionFunc(f.Name, dynamicFlagCompletion(f.Complete))
			case len(f.Options) > 0:
				_ = out.RegisterFlagCompletionFunc(f.Name, staticCompletion(f.Options))
			}
		}
	}
	if c.CompleteArgs != nil {
		out.ValidArgsFunction = argsCompletion(c.CompleteArgs, c.MaxArgs)
	}
	if c.Run != nil {
		run := c.Run
		if c.MaxArgs < 0 {
			out.Args = cobra.ArbitraryArgs
		} else {
			out.Args = cobra.MaximumNArgs(c.MaxArgs)
		}
		out.RunE = func(cmd *cobra.Command, args []string) error {
			flags := make(map[string]string, len(values)+len(bools))
			for name, v := range values {
				flags[name] = *v
			}
			for name, v := range bools {
				flags[name] = strconv.FormatBool(*v)
			}
			return run(cli.Context{
				Args:   args,
				Flags:  flags,
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
			})
		}
	}
	for _, sub := range c.Subcommands {
		out.AddCommand(translate(sub))
	}
	return out
}

// staticCompletion completes a value flag's closed Options set, with
// file completion suppressed.
func staticCompletion(options []string) cobra.CompletionFunc {
	return func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
		return options, cobra.ShellCompDirectiveNoFileComp
	}
}

// argsCompletion adapts the neutral CompleteArgs seam onto Cobra's
// ValidArgsFunction: candidates take Cobra's "value\tdescription" form
// when Doc is set, and completion stops once the positional-argument
// budget is spent. Cobra's default root supplies the rest of the
// machinery — the `completion bash|zsh|fish|powershell` subcommand and
// the hidden `__complete` command — because translate never sets
// CompletionOptions.
func argsCompletion(complete func([]string, string) []cli.AutoCompleteCandidate, maxArgs int) cobra.CompletionFunc {
	return func(_ *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		if maxArgs >= 0 && len(args) >= maxArgs {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return renderCandidates(complete(args, toComplete)), cobra.ShellCompDirectiveNoFileComp
	}
}

// dynamicFlagCompletion adapts the neutral Flag.Complete seam onto
// Cobra's flag completion, with file completion suppressed.
func dynamicFlagCompletion(complete func(string) []cli.AutoCompleteCandidate) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		return renderCandidates(complete(toComplete)), cobra.ShellCompDirectiveNoFileComp
	}
}

// renderCandidates maps neutral candidates onto Cobra's
// "value\tdescription" completion form.
func renderCandidates(candidates []cli.AutoCompleteCandidate) []cobra.Completion {
	completions := make([]cobra.Completion, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Doc != "" {
			completions = append(completions, candidate.Value+"\t"+candidate.Doc)
		} else {
			completions = append(completions, candidate.Value)
		}
	}
	return completions
}
