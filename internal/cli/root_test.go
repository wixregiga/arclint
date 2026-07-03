package cli

import (
	"bytes"
	"strings"
	"testing"
)

// runRootCLI executes `arclint <args...>` in-process against the shared
// rootCmd and returns both cobra's own captured output (help text,
// non-error command output) and whatever error rootCmd.Execute() itself
// returns. It deliberately does not go through the package-level Execute()
// function, because that prints the final "error: " line straight to
// os.Stderr rather than through the cobra-managed writer — the error text
// itself (produced by flagError) is what F1 is about, so assert on it
// directly.
func runRootCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	resetCheckFlags(t)
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return buf.String(), err
}

// TestUnknownFlagErrorIsStyled covers the F1 bug report: an unknown flag
// produced bare cobra output ("error: unknown flag: --bogus-flag") with no
// fix, breaking the "what happened — how to fix" style guide
// (docs/design/cli.md). The flagErrorFunc set on rootCmd must rewrite it
// to name the bad flag in backticks and point at --help.
func TestUnknownFlagErrorIsStyled(t *testing.T) {
	_, err := runRootCLI(t, "--bogus-flag")
	if err == nil {
		t.Fatal("Execute accepted an unknown flag, want an error")
	}

	msg := err.Error()
	for _, want := range []string{
		"unknown flag `--bogus-flag`",
		"run `arclint --help` to see valid flags",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error = %q, want it to contain %q", msg, want)
		}
	}
	// The old bare cobra message ("unknown flag: --bogus-flag" with a
	// colon, no backticks) must not leak through unstyled.
	if strings.Contains(msg, "unknown flag: --bogus-flag") {
		t.Errorf("error = %q, still contains the bare unstyled cobra message", msg)
	}
}

// TestUnknownFlagOnSubcommandNamesTheSubcommand checks the FlagErrorFunc
// set once on rootCmd also covers subcommands, since cobra looks up a
// command's own FlagErrorFunc first and only falls back to the parent's
// when the subcommand has none of its own.
func TestUnknownFlagOnSubcommandNamesTheSubcommand(t *testing.T) {
	_, err := runRootCLI(t, "check", "--nope")
	if err == nil {
		t.Fatal("Execute accepted an unknown flag on `check`, want an error")
	}

	msg := err.Error()
	for _, want := range []string{
		"unknown flag `--nope`",
		"run `arclint check --help` to see valid flags",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error = %q, want it to contain %q", msg, want)
		}
	}
}

// TestUnknownFlagKeepsExitUsage checks the exit-code contract is
// unaffected by the message rewrite: an unknown flag is still exit 2,
// via the package-level Execute() (which maps any non-*ExitError to
// ExitUsage).
func TestUnknownFlagKeepsExitUsage(t *testing.T) {
	resetCheckFlags(t)
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"--bogus-flag"})
	if code := Execute(); code != ExitUsage {
		t.Errorf("Execute() = %d, want ExitUsage (%d)", code, ExitUsage)
	}
}
