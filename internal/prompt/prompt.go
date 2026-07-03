// Package prompt renders the interactive TUI forms for arclint new and
// arclint make (docs/design/cli.md, TUI prompting model). Prompts write to
// stderr, keeping stdout clean for --format json and pipes; every prompt
// maps 1:1 to a --var flag and the Tip line teaches the flag afterwards.
package prompt

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/mattn/go-isatty"

	"github.com/jofyi/arclint/internal/template"
)

// Interactive reports whether a prompt may fire at all: stdin and stdout are
// both TTYs. The caller additionally checks --no-input.
func Interactive() bool {
	return isTTY(os.Stdin) && isTTY(os.Stdout)
}

func isTTY(f *os.File) bool {
	fd := f.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

// useAccessibleMode reports whether huh should run in plain line-based IO
// mode (WithAccessible) instead of the Bubble Tea full-screen renderer. This
// matters when stdin is a pty-piped stream (e.g. under a screen reader, or a
// dumb terminal) where isatty reports true but redrawing views breaks input.
// TERM=dumb is the standard signal a terminal can't handle cursor movement;
// ARCLINT_ACCESSIBLE=1 is an explicit opt-in for anything else.
func useAccessibleMode(env func(string) string) bool {
	return env("TERM") == "dumb" || env("ARCLINT_ACCESSIBLE") == "1"
}

// Ask renders one huh form on stderr with one field per unresolved required
// variable, batched so the user answers everything in one pass. Values are
// validated inline with the manifest's constraints and returned normalized.
func Ask(vars []*template.Variable) (map[string]string, error) {
	strVals := make([]string, len(vars))
	boolVals := make([]bool, len(vars))
	fields := make([]huh.Field, 0, len(vars))
	for i, v := range vars {
		title := v.Description
		if title == "" {
			title = v.Name
		}
		switch v.Type {
		case "choice":
			fields = append(fields, huh.NewSelect[string]().
				Title(title).
				Options(huh.NewOptions(v.Choices...)...).
				Value(&strVals[i]))
		case "bool":
			fields = append(fields, huh.NewConfirm().
				Title(title).
				Value(&boolVals[i]))
		default:
			vv := v
			fields = append(fields, huh.NewInput().
				Title(title).
				Validate(func(s string) error {
					_, err := vv.Check(s)
					return err
				}).
				Value(&strVals[i]))
		}
	}
	form := huh.NewForm(huh.NewGroup(fields...)).
		WithOutput(os.Stderr).
		WithAccessible(useAccessibleMode(os.Getenv))
	if err := form.Run(); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(vars))
	for i, v := range vars {
		if v.Type == "bool" {
			out[v.Name] = strconv.FormatBool(boolVals[i])
			continue
		}
		val, err := v.Check(strVals[i])
		if err != nil {
			return nil, err
		}
		out[v.Name] = val
	}
	return out, nil
}

// Tip prints the dimmed non-interactive equivalent of what was just answered
// (docs/design/cli.md, prompts teach flags). Callers pass stderr and skip the
// call entirely under --quiet or when nothing came from a prompt.
func Tip(w io.Writer, cmdline string) {
	fmt.Fprintf(w, "tip: next time, skip the prompts with:\n  %s\n", cmdline)
}

// ShellQuote wraps a value in single quotes when it contains characters that
// a shell would interpret, so Tip output is copy-pasteable.
func ShellQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n\"'`$&|;<>()*?[]{}!#~^\\") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
