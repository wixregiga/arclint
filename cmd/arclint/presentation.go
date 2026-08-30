package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wixregiga/arclint/internal/delivery/cli"
)

// Process-level presentation values. human is the default when --format
// is absent; json forces the JSON renderer regardless of color or TTY.
const (
	presentationFormatHuman = "human"
	presentationFormatJSON  = "json"
)

// resolvePresentation peels process-level --format and --no-color from
// raw argv (split and equal forms), then selects the Renderer identity.
//
// Selection:
//   - json → RendererJSON (wins over color/TTY)
//   - human + color enabled + stdout TTY → RendererLipgloss
//   - otherwise → RendererPlain
//
// Color is disabled by --no-color or a non-empty NO_COLOR env value.
// Shell completion invocations leave argv untouched and select plain so
// Cobra root flag completion still sees --format.
func resolvePresentation(args []string, noColorEnv string, stdoutIsTTY bool) (cli.RendererName, []string, error) {
	if isCompletionInvocation(args) {
		return cli.RendererPlain, args, nil
	}

	format := presentationFormatHuman
	noColor := false
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			rest = append(rest, args[i:]...)
			break
		}
		switch {
		case a == "--format":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("--format requires a value")
			}
			i++
			format = args[i]
		case strings.HasPrefix(a, "--format="):
			format = strings.TrimPrefix(a, "--format=")
		case a == "--no-color":
			noColor = true
		case strings.HasPrefix(a, "--no-color="):
			parsed, err := parseNoColorEqual(strings.TrimPrefix(a, "--no-color="))
			if err != nil {
				return "", nil, err
			}
			noColor = parsed
		default:
			rest = append(rest, a)
		}
	}

	switch format {
	case presentationFormatHuman, presentationFormatJSON:
	default:
		return "", nil, fmt.Errorf("unknown format %q (human, json)", format)
	}

	if format == presentationFormatJSON {
		return cli.RendererJSON, rest, nil
	}
	if colorEnabled(noColor, noColorEnv) && stdoutIsTTY {
		return cli.RendererLipgloss, rest, nil
	}
	return cli.RendererPlain, rest, nil
}

// parseNoColorEqual interprets the equal-form value of --no-color with
// the same accepted tokens as a Cobra bool flag (strconv.ParseBool).
func parseNoColorEqual(v string) (bool, error) {
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("invalid value %q for --no-color: %w", v, err)
	}
	return parsed, nil
}

func colorEnabled(noColorFlag bool, noColorEnv string) bool {
	return !noColorFlag && noColorEnv == ""
}

// isCompletionInvocation reports whether argv is a shell-completion
// request. Those keep process flags so the root descriptors can complete.
func isCompletionInvocation(args []string) bool {
	switch firstPositional(args) {
	case "__complete", "__completeNoDesc", "completion":
		return true
	default:
		return false
	}
}
