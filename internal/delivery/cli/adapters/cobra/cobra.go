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
		}
	}
	if c.Run != nil {
		run := c.Run
		out.Args = cobra.MaximumNArgs(c.MaxArgs)
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
