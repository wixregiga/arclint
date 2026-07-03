package rules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/jofyi/arclint/internal/config"
)

// waitDelay bounds how long cmd.Wait() waits for the child's stdout/stderr
// pipes to close after Cancel kills the process group. Without it, a
// grandchild that inherited a pipe fd (e.g. a background process forked by
// the rule command) can hold the pipe open forever and hang cmd.Run()
// past the configured timeout.
const waitDelay = 2 * time.Second

// sanitizedEnv returns the minimal environment rule commands run with.
// Custom rule commands are third-party-ish and untrusted, so they run
// with a sanitized environment (PATH, HOME, LANG only) rather than
// inheriting the full parent environment, which may hold secrets.
func sanitizedEnv() []string {
	var out []string
	for _, k := range []string{"PATH", "HOME", "LANG"} {
		if v, ok := os.LookupEnv(k); ok {
			out = append(out, k+"="+v)
		}
	}
	return out
}

// normalizeViolationPath turns whatever path shape a rule command emitted
// (relative with "./", backslashes, non-clean segments) into the clean,
// forward-slash, repo-relative shape that baseline suppression and ignore
// globs expect. It replaces backslashes unconditionally rather than via
// filepath.ToSlash, which is a no-op on non-Windows build targets — a
// rule command can emit Windows-style separators regardless of the OS
// arclint itself is running on.
func normalizeViolationPath(p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	p = path.Clean(p)
	p = strings.TrimPrefix(p, "./")
	return p
}

// compileCustom builds the custom evaluator (rules.md §5.5): run the argv
// from the repo root, write {"files": [...]} (the rule's targeted files) to
// stdin, read a JSON array of {path, line?, message, fixHint?} from stdout.
// Exit 0 is the only success code — any non-zero exit, timeout, or
// unparseable stdout is a rule execution error (the caller's exit-2 path).
func compileCustom(id string, r config.Rule) ruleFunc {
	p := r.Custom
	return func(c *evalCtx) ([]Violation, error) {
		if len(p.Command) == 0 {
			return nil, fmt.Errorf("rule %q: custom command is empty — set params.command", id)
		}
		files := targeted(c.paths, r.Files)

		timeout := time.Duration(p.TimeoutSeconds) * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		stdin, err := json.Marshal(map[string]any{"files": files})
		if err != nil {
			return nil, fmt.Errorf("rule %q: cannot encode stdin — %v", id, err)
		}

		cmd := exec.CommandContext(ctx, p.Command[0], p.Command[1:]...)
		cmd.Dir = c.root
		cmd.Env = sanitizedEnv()
		cmd.Stdin = bytes.NewReader(stdin)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		// Run the command in its own process group and kill the whole
		// group (not just the direct child) when the context deadline
		// fires, so forking commands can't leave orphaned grandchildren
		// running past the timeout. WaitDelay bounds how long Wait()
		// waits for pipes held open by such grandchildren to close.
		setProcGroup(cmd)
		cmd.Cancel = func() error { return killProcGroup(cmd) }
		cmd.WaitDelay = waitDelay

		if err := cmd.Run(); err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return nil, fmt.Errorf("rule %q: command %v timed out after %ds — raise params.timeoutSeconds or speed the command up", id, p.Command, p.TimeoutSeconds)
			}
			detail := strings.TrimSpace(stderr.String())
			if detail != "" {
				detail = ": " + detail
			}
			return nil, fmt.Errorf("rule %q: command %v failed (%v)%s — fix the command or set the rule's severity to \"off\"", id, p.Command, err, detail)
		}

		out := bytes.TrimSpace(stdout.Bytes())
		if len(out) == 0 {
			return nil, nil
		}
		var raw []struct {
			Path    string `json:"path"`
			Line    *int   `json:"line"`
			Message string `json:"message"`
			FixHint string `json:"fixHint"`
		}
		if err := json.Unmarshal(out, &raw); err != nil {
			return nil, fmt.Errorf("rule %q: command output is not a JSON violation array — %v", id, err)
		}

		vs := make([]Violation, 0, len(raw))
		for _, rv := range raw {
			hint := rv.FixHint
			if hint == "" {
				hint = r.FixHint
			}
			vs = append(vs, Violation{
				RuleID:   id,
				Category: r.Type,
				Severity: r.Severity,
				Path:     normalizeViolationPath(rv.Path),
				Line:     rv.Line,
				Message:  rv.Message,
				FixHint:  hint,
			})
		}
		return vs, nil
	}
}
