// Package cli owns the arclint command tree: the cobra root command, the
// global flags every command accepts, the uniform 0/1/2 exit-code contract,
// and the self-registration hook subcommand files use to attach themselves.
//
// Cold-start rule: nothing in this package touches the filesystem or network
// at init time. Config loading is a per-command concern and happens after
// flag parsing, only for commands that need it (docs/design/cli.md).
package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wixregiga/arclint/internal/version"
)

// Exit codes — the uniform contract from docs/design/cli.md.
const (
	// ExitOK: success. Check clean, generation done, drift report ran.
	ExitOK = 0
	// ExitFindings: the tool ran and the repo has problems (error-severity
	// violations, or drift under --fail-on-drift).
	ExitFindings = 1
	// ExitUsage: the invocation or configuration is broken (unknown
	// command/flag/thing, invalid rules.yaml, missing required input
	// under --no-input, conflicting flags).
	ExitUsage = 2
)

// Format values accepted by --format.
const (
	FormatText = "text"
	FormatJSON = "json"
)

// GlobalFlags holds the values of the flags accepted by every command.
type GlobalFlags struct {
	// NoInput: never prompt; missing required input is exit 2.
	NoInput bool
	// Config: explicit config root; empty means discover .arclint/ upward
	// from the working directory (config.FindConfigRoot).
	Config string
	// Format: "text" or "json". JSON implies no color and no progress.
	Format string
	// Quiet suppresses everything except violations, diffs, and errors.
	Quiet bool
	// Verbose shows per-file progress, timing, rule evaluation detail.
	Verbose bool
}

var globals GlobalFlags

// Globals returns the parsed global flag values. Valid after flag parsing,
// i.e. inside any command's RunE.
func Globals() *GlobalFlags { return &globals }

// rootCmd is constructed statically at package init — pure in-memory work,
// no filesystem access, per the cold-start budget.
var rootCmd = newRootCmd()

func newRootCmd() *cobra.Command {
	// Chain persistent hooks so a subcommand defining its own
	// PersistentPreRunE never silently disables global-flag validation.
	cobra.EnableTraverseRunHooks = true

	cmd := &cobra.Command{
		Use:     "arclint",
		Short:   "Very fast architecture linter and template repo creator",
		Long:    "arclint lints project architecture (structure, naming, dependencies, content)\nfrom declarative rules in .arclint/rules.yaml and scaffolds new units from\ndrop-in templates in .arclint/templates/.",
		Version: version.Version,
		// Errors are printed by Execute with the style-guide "error: "
		// prefix; usage spam on runtime errors is suppressed.
		SilenceErrors:     true,
		SilenceUsage:      true,
		PersistentPreRunE: validateGlobals,
		// The root is runnable (it just prints help) so that global-flag
		// validation runs even for a bare `arclint --quiet --verbose`,
		// keeping the exit-2 contract for conflicting flags. NoArgs keeps
		// unknown subcommands an error instead of silent help.
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	pf := cmd.PersistentFlags()
	pf.BoolVar(&globals.NoInput, "no-input", false, "never prompt; missing required input is a hard error (exit 2)")
	pf.StringVar(&globals.Config, "config", "", "explicit config root instead of discovering .arclint/ upward from cwd")
	pf.StringVar(&globals.Format, "format", FormatText, "output format: text|json")
	pf.BoolVar(&globals.Quiet, "quiet", false, "suppress everything except violations, diffs, and errors")
	pf.BoolVar(&globals.Verbose, "verbose", false, "show per-file progress, timing breakdown, rule evaluation detail")

	// Set once on the root: cobra's FlagErrorFunc lookup walks up to the
	// parent when a command has none of its own, so this covers every
	// subcommand's flag-parse errors too. Without it, a bad flag surfaces
	// cobra's bare "unknown flag: --x" with no fix, breaking the
	// style-guide "what happened — how to fix" contract (docs/design/cli.md).
	cmd.SetFlagErrorFunc(flagError)

	return cmd
}

// flagError rewrites a raw pflag parse error (unknown flag, unknown
// shorthand, bad value, missing argument, ...) into the one-line
// "what happened — how to fix" shape the rest of the CLI uses. The
// exit code is untouched here: Execute still maps any non-nil error that
// isn't an *ExitError to ExitUsage.
func flagError(cmd *cobra.Command, err error) error {
	return fmt.Errorf("%s — run `%s --help` to see valid flags", backtickFlag(err.Error()), cmd.CommandPath())
}

// backtickFlag wraps the flag token pflag already embedded in its error
// message (e.g. "unknown flag: --bogus-flag" or "unknown shorthand flag:
// 'x' in -xyz") in backticks, matching the style guide's convention of
// quoting the offending token rather than repeating pflag's own quoting.
func backtickFlag(msg string) string {
	if rest, ok := strings.CutPrefix(msg, "unknown flag: "); ok {
		return fmt.Sprintf("unknown flag `%s`", rest)
	}
	return msg
}

// validateGlobals enforces the cross-flag rules from docs/design/cli.md.
func validateGlobals(_ *cobra.Command, _ []string) error {
	if globals.Quiet && globals.Verbose {
		return UsageErrorf("--quiet and --verbose are mutually exclusive — pass at most one of them")
	}
	switch globals.Format {
	case FormatText, FormatJSON:
	default:
		return UsageErrorf("unknown format %q — supported: text, json", globals.Format)
	}
	return nil
}

// Register appends a subcommand to the root command. Each command file calls
// Register from its own init(), so adding a command never edits a shared
// file:
//
//	func init() { cli.Register(newCheckCmd()) }
//
// Registration is in-memory only; it must not read config or the filesystem.
func Register(cmd *cobra.Command) { rootCmd.AddCommand(cmd) }

// ExitError carries an explicit process exit code out of a command's RunE.
// Construct via Findings or UsageErrorf rather than directly.
type ExitError struct {
	Code int
	Err  error // optional; printed as "error: <msg>" when non-nil
}

func (e *ExitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit status %d", e.Code)
}

func (e *ExitError) Unwrap() error { return e.Err }

// Findings signals exit code 1: the tool ran and found problems. The
// findings themselves are expected to have been printed already; no
// additional error line is emitted.
func Findings() error { return &ExitError{Code: ExitFindings} }

// UsageErrorf signals exit code 2 with a one-line, style-guide-conformant
// message: "<what happened> — <how to fix it>" (docs/design/cli.md).
func UsageErrorf(format string, args ...any) error {
	return &ExitError{Code: ExitUsage, Err: fmt.Errorf(format, args...)}
}

// Execute runs the CLI and returns the process exit code. main() passes the
// result straight to os.Exit.
func Execute() int {
	err := rootCmd.Execute()
	if err == nil {
		return ExitOK
	}
	var xe *ExitError
	if errors.As(err, &xe) {
		if xe.Err != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", xe.Err)
		}
		return xe.Code
	}
	// Anything cobra itself rejects (unknown command, unknown flag, bad
	// flag value) is a usage error by contract.
	fmt.Fprintf(os.Stderr, "error: %s\n", err)
	return ExitUsage
}
