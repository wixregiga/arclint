package cobraadapter_test

import (
	"bytes"
	"io"
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
	return runTree(t, tree(), nil, args...)
}

func runTree(t *testing.T, root cli.Command, stdin io.Reader, args ...string) (cli.Outcome, string, string) {
	t.Helper()
	adapter, err := clifactory.Select(cli.AdapterCobra)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	var stdout, stderr bytes.Buffer
	outcome := adapter.Run(root, cli.Invocation{Args: args, Stdin: stdin, Stdout: &stdout, Stderr: &stderr})
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

func TestLongExampleAliasesStdinRepeatAndChanged(t *testing.T) {
	var (
		gotLong    string
		gotExample string
		gotStdin   string
		gotAliases []string
		gotChanged bool
		gotDef     string
		gotLists   []string
	)
	root := cli.Root("9.9.9", cli.Command{
		Name:    "probe",
		Short:   "probe short",
		Long:    "probe long description",
		Example: "arclint probe --alias a",
		Aliases: []string{"p"},
		MaxArgs: 0,
		Flags: []cli.Flag{
			{Name: "definition", Default: "", Doc: "definition text"},
			{Name: "alias", Repeat: true, Doc: "alias value"},
			{Name: "guided", Bool: true, Doc: "guided"},
		},
		Run: func(ctx cli.Context) error {
			gotStdinBytes, err := io.ReadAll(ctx.Stdin)
			if err != nil {
				return err
			}
			gotStdin = string(gotStdinBytes)
			gotChanged = ctx.Changed("definition")
			gotDef = ctx.String("definition")
			gotAliases = ctx.Strings("alias")
			gotLists = append([]string(nil), gotAliases...)
			_, _ = io.WriteString(ctx.Stdout, "ok")
			return nil
		},
	})

	// Confirm Long/Example/Aliases reach Cobra by exercising the alias
	// path and inspecting help text for the long description and example.
	adapter, err := clifactory.Select(cli.AdapterCobra)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	var helpOut bytes.Buffer
	helpOutcome := adapter.Run(root, cli.Invocation{
		Args:   []string{"probe", "--help"},
		Stdout: &helpOut,
		Stderr: io.Discard,
	})
	if helpOutcome.ExitCode != cli.ExitClean {
		t.Fatalf("help exit = %d", helpOutcome.ExitCode)
	}
	help := helpOut.String()
	if !strings.Contains(help, "probe long description") {
		t.Errorf("help missing Long text: %q", help)
	}
	if !strings.Contains(help, "arclint probe --alias a") {
		t.Errorf("help missing Example text: %q", help)
	}
	gotLong = "probe long description"
	gotExample = "arclint probe --alias a"

	outcome, stdout, _ := runTree(t, root, strings.NewReader("stdin-body"),
		"p", "--definition", "", "--alias", "one", "--alias", "two")
	if outcome.ExitCode != cli.ExitClean {
		t.Fatalf("alias invocation exit = %d", outcome.ExitCode)
	}
	if stdout != "ok" {
		t.Errorf("stdout = %q", stdout)
	}
	if gotStdin != "stdin-body" {
		t.Errorf("stdin = %q, want stdin-body", gotStdin)
	}
	if !gotChanged {
		t.Errorf("Changed(definition) = false, want true for explicit empty string")
	}
	if gotDef != "" {
		t.Errorf("definition = %q, want empty", gotDef)
	}
	if len(gotLists) != 2 || gotLists[0] != "one" || gotLists[1] != "two" {
		t.Errorf("alias list = %#v, want [one two]", gotLists)
	}

	// Omitted definition must report Changed=false.
	gotChanged = true
	outcome, _, _ = runTree(t, root, nil, "probe")
	if outcome.ExitCode != cli.ExitClean {
		t.Fatalf("plain probe exit = %d", outcome.ExitCode)
	}
	if gotChanged {
		t.Errorf("Changed(definition) = true when flag omitted")
	}

	_ = gotLong
	_ = gotExample
	_ = gotAliases
}
