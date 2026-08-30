package cli

import "io"

const commandInit = "init"

// Flag is one framework-neutral flag description: value flags parse a
// string, bool flags parse presence.
type Flag struct {
	Name    string
	Default string
	Bool    bool
	// Repeat marks a value flag that collects every occurrence into a
	// string slice (Context.Strings) instead of a single string.
	Repeat bool
	Doc    string
	// Options is the static closed value set of a value flag. A
	// non-empty set lets shells complete the flag's value; parsing
	// stays with the command, which still rejects unknown values.
	Options []string
	// Complete supplies dynamic candidates for the flag's value and
	// takes precedence over Options. The CompleteArgs contract applies:
	// fast, silent, and an empty list on any failure.
	Complete func(toComplete string) []AutoCompleteCandidate
}

// AutoCompleteCandidate is one framework-neutral completion candidate: the value
// the shell inserts, plus optional descriptive text for shells that
// render descriptions.
type AutoCompleteCandidate struct {
	Value string
	Doc   string
}

// Context carries one parsed invocation into a command handler.
type Context struct {
	Args  []string
	Flags map[string]string
	// Lists holds values for Repeat flags; absent or empty when the
	// flag was never set.
	Lists map[string][]string
	// Set reports whether each flag appeared on the command line, so
	// handlers can distinguish an omitted value flag from an explicit
	// empty string.
	Set    map[string]bool
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// String returns the parsed value of a value flag.
func (c Context) String(name string) string { return c.Flags[name] }

// Bool returns the parsed value of a bool flag.
func (c Context) Bool(name string) bool { return c.Flags[name] == "true" }

// Strings returns the values of a Repeat flag, or nil when unset.
func (c Context) Strings(name string) []string {
	if c.Lists == nil {
		return nil
	}
	return c.Lists[name]
}

// Changed reports whether the named flag appeared on the command line.
func (c Context) Changed(name string) bool {
	if c.Set == nil {
		return false
	}
	return c.Set[name]
}

// Command is one framework-neutral command description.
type Command struct {
	Name    string
	Short   string
	Long    string
	Example string
	Aliases []string
	Version string // root command only; not a root discriminator
	// Root marks the process root command. The Cobra translator uses
	// this explicit state (not Version) when deciding persistent flags.
	Root        bool
	Flags       []Flag
	Subcommands []Command
	// MaxArgs bounds positional arguments; negative means unlimited.
	MaxArgs int
	// Run handles the invocation; nil for pure group commands.
	Run func(Context) error
	// CompleteArgs supplies dynamic candidates for the next positional
	// argument; nil keeps the adapter's default (file) completion.
	// Completion callbacks run when the shell re-executes the binary
	// on TAB, so they must be fast, silent, and degrade to an empty
	// candidate list on any failure — never an error and never output
	// to stderr.
	CompleteArgs func(args []string, toComplete string) []AutoCompleteCandidate
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

// Root assembles the arclint command tree. Process-level presentation
// flags (--format, --no-color) are declared here so help and completion
// advertise them; the composition root peels them before the adapter
// runs and selects the sealed Renderer. They are inert on the command
// tree itself — handlers never read them.
func Root(version string, subcommands ...Command) Command {
	return Command{
		Name:    "arclint",
		Short:   "architecture conformance for repositories (target engine)",
		Version: version,
		Root:    true,
		Flags: []Flag{
			{
				Name:    "format",
				Default: "human",
				Doc:     "output format: human, json",
				Options: []string{"human", "json"},
			},
			{
				Name: "no-color",
				Bool: true,
				Doc:  "disable color output",
			},
		},
		Subcommands: subcommands,
	}
}
