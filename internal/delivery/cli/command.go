package cli

import "io"

// Flag is one framework-neutral flag description: value flags parse a
// string, bool flags parse presence.
type Flag struct {
	Name    string
	Default string
	Bool    bool
	Doc     string
	// Options is the static closed value set of a value flag. A
	// non-empty set lets shells complete the flag's value; parsing
	// stays with the command, which still rejects unknown values.
	Options []string
}

// Candidate is one framework-neutral completion candidate: the value
// the shell inserts, plus optional descriptive text for shells that
// render descriptions.
type Candidate struct {
	Value string
	Doc   string
}

// Context carries one parsed invocation into a command handler.
type Context struct {
	Args   []string
	Flags  map[string]string
	Stdout io.Writer
	Stderr io.Writer
}

// String returns the parsed value of a value flag.
func (c Context) String(name string) string { return c.Flags[name] }

// Bool returns the parsed value of a bool flag.
func (c Context) Bool(name string) bool { return c.Flags[name] == "true" }

// Command is one framework-neutral command description.
type Command struct {
	Name        string
	Short       string
	Version     string // root command only
	Flags       []Flag
	Subcommands []Command
	// MaxArgs bounds positional arguments.
	MaxArgs int
	// Run handles the invocation; nil for pure group commands.
	Run func(Context) error
	// CompleteArgs supplies dynamic candidates for the next positional
	// argument; nil keeps the adapter's default (file) completion.
	// Completion callbacks run when the shell re-executes the binary
	// on TAB, so they must be fast, silent, and degrade to an empty
	// candidate list on any failure — never an error and never output
	// to stderr.
	CompleteArgs func(args []string, toComplete string) []Candidate
}

// ExitError carries an explicit exit code through the Adapter. An
// empty message exits silently.
type ExitError struct {
	Code    int
	Message string
}

func (e *ExitError) Error() string { return e.Message }

// ConfigError wraps a configuration failure as exit code 2.
func ConfigError(err error) error {
	return &ExitError{Code: ExitConfigError, Message: err.Error()}
}

// ViolationsExit is the silent exit for gating findings.
func ViolationsExit() error { return &ExitError{Code: ExitViolations} }

// Root assembles the arclint command tree.
func Root(version string, subcommands ...Command) Command {
	return Command{
		Name:        "arclint",
		Short:       "architecture conformance for repositories (target engine)",
		Version:     version,
		Subcommands: subcommands,
	}
}
