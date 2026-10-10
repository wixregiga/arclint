package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

const (
	agentHostFlag       = "host"
	agentHostClaude     = "claude"
	agentHostCodex      = "codex"
	agentInstallCommand = "install"
	hooksProjectFlag    = "project"
)

// Hooks is the CLI-owned boundary for writing and inspecting the
// workflow hooks in each Agent Host's project configuration.
type Hooks interface {
	Install(hosts []string, project bool) (application.HooksInstallation, error)
	Status() (application.HooksStatus, error)
}

// HookEvents is the CLI-owned boundary that answers one host hook
// event. Its answer advises the agent; it never blocks a tool call.
type HookEvents interface {
	Handle(input []byte) ([]byte, error)
}

// HooksActivation tells the user what each host needs after installation.
const HooksActivation = "Claude Code runs the hooks in sessions that start after installation. " +
	"Codex runs them only after you trust them: open /hooks in Codex, trust the ArcLint hooks, and start a new session. " +
	"`arclint agents hooks status` shows when a hook event last reached this project."

// NewHooksCommand is the workflow hook surface: install and status
// manage host configuration, event answers the hosts.
func NewHooksCommand(hooks Hooks, events HookEvents, render Renderer) Command {
	return Command{Name: "hooks", Short: "advise coding agents through the domain workflow with native hooks", Subcommands: []Command{
		{Name: agentInstallCommand, Short: "write the workflow hooks into Claude Code and Codex user configuration, for every project", Flags: []Flag{
			{Name: agentHostFlag, Repeat: true, Options: []string{agentHostClaude, agentHostCodex}, Doc: "agent host to install for (claude, codex); repeatable; default both"},
			{Name: hooksProjectFlag, Bool: true, Doc: "write this project's personal hook files instead, for this project only"},
		}, Run: func(ctx Context) error {
			hosts := ctx.Strings(agentHostFlag)
			installation, err := hooks.Install(hosts, ctx.Bool(hooksProjectFlag))
			if err != nil {
				return ConfigError(err)
			}
			return render.Render(ctx.Stdout, AgentInstallReport{Operation: "hooks", Host: hostList(hosts), Paths: installation.Written, Removed: installation.Removed, Orientation: application.WorkflowOrientation(vocab.UbiquitousLanguageFileName), Activation: HooksActivation})
		}},
		{Name: "status", Short: "show which configuration files list the workflow hooks and when an event last reached this project", Run: func(ctx Context) error {
			status, err := hooks.Status()
			if err != nil {
				return ConfigError(err)
			}
			return render.Render(ctx.Stdout, HooksStatusReport{Status: status})
		}},
		{Name: "event", Short: "answer one Claude Code or Codex hook event read from stdin", Run: func(ctx Context) error {
			input, err := io.ReadAll(ctx.Stdin)
			if err != nil {
				return ConfigError(fmt.Errorf("read workflow event: %w", err))
			}
			output, err := events.Handle(input)
			if err != nil {
				return ConfigError(err)
			}
			if _, err := ctx.Stdout.Write(output); err != nil {
				return fmt.Errorf("write workflow event: %w", err)
			}
			return nil
		}},
	}}
}

func hostList(hosts []string) string {
	if len(hosts) == 0 {
		hosts = []string{agentHostClaude, agentHostCodex}
	}
	return strings.Join(hosts, ", ")
}
