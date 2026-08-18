package cobraadapter_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/delivery/cli"
	clifactory "github.com/wixregiga/arclint/internal/delivery/cli/factory"
)

// tree is a neutral command exercising every translated construct:
// value flags, bool flags, args, and the exit-code contract.
func tree() cli.Command {
	echo := cli.Command{
		Name:    "echo",
		Short:   "echo parsed input",
		MaxArgs: 1,
		Flags: []cli.Flag{
			{Name: "format", Default: "human", Doc: "format"},
			{Name: "loud", Bool: true, Doc: "loud"},
		},
		Run: func(ctx cli.Context) error {
			ctx.Stdout.Write([]byte("format=" + ctx.String("format")))
			if ctx.Bool("loud") {
				ctx.Stdout.Write([]byte(" loud"))
			}
			for _, a := range ctx.Args {
				ctx.Stdout.Write([]byte(" arg=" + a))
			}
			return nil
		},
	}
	failing := cli.Command{
		Name:  "gate",
		Short: "exit one silently",
		Run:   func(cli.Context) error { return cli.ViolationsExit() },
	}
	misconfigured := cli.Command{
		Name:  "broken",
		Short: "exit two with a message",
		Run:   func(cli.Context) error { return &cli.ExitError{Code: cli.ExitConfigError, Message: "bad input"} },
	}
	return cli.Root("9.9.9", echo, failing, misconfigured)
}

func run(t *testing.T, args ...string) (cli.Outcome, string, string) {
	t.Helper()
	adapter, err := clifactory.Select(cli.AdapterCobra)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	var stdout, stderr bytes.Buffer
	outcome := adapter.Run(tree(), cli.Invocation{Args: args, Stdout: &stdout, Stderr: &stderr})
	return outcome, stdout.String(), stderr.String()
}

func TestTranslatesFlagsAndArgs(t *testing.T) {
	outcome, stdout, _ := run(t, "echo", "--format", "json", "--loud", "x")
	if outcome.ExitCode != cli.ExitClean {
		t.Fatalf("exit = %d, want 0", outcome.ExitCode)
	}
	if stdout != "format=json loud arg=x" {
		t.Errorf("stdout = %q", stdout)
	}
	if outcome, _, _ := run(t, "echo", "a", "b"); outcome.ExitCode != cli.ExitConfigError {
		t.Errorf("excess arguments must exit 2, got %d", outcome.ExitCode)
	}
}

func TestExitContract(t *testing.T) {
	if outcome, _, stderr := run(t, "gate"); outcome.ExitCode != cli.ExitViolations || stderr != "" {
		t.Errorf("gate: exit %d stderr %q, want silent 1", outcome.ExitCode, stderr)
	}
	outcome, _, stderr := run(t, "broken")
	if outcome.ExitCode != cli.ExitConfigError || !strings.Contains(stderr, "bad input") {
		t.Errorf("broken: exit %d stderr %q, want 2 with the message", outcome.ExitCode, stderr)
	}
	if outcome, _, _ := run(t, "unknown-command"); outcome.ExitCode != cli.ExitConfigError {
		t.Errorf("unknown command must exit 2, got %d", outcome.ExitCode)
	}
}

func TestFactoryRejectsUnknownAdapters(t *testing.T) {
	if _, err := clifactory.Select("gtk"); err == nil {
		t.Errorf("unknown adapter identity must be rejected")
	}
}
