package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
)

const agentHostClaude = "claude"

// WorkflowHooks is the CLI-owned boundary for writing and inspecting the
// workflow hooks in each Agent Host's project configuration.
type WorkflowHooks interface {
	Install(hosts []string) ([]string, error)
	Status() (application.WorkflowHookStatus, error)
}

// WorkflowEvents is the CLI-owned boundary that answers one host hook
// event. Its answer advises the agent; it never blocks a tool call.
type WorkflowEvents interface {
	Handle(input []byte) ([]byte, error)
}

// WorkflowActivation tells the user what each host needs after installation.
const WorkflowActivation = "Claude Code reads the hooks when a session starts. " +
	"Codex runs project hooks only after you trust them: open /hooks in Codex, trust the ArcLint hooks, and start a new session. " +
	"Each hook runs `arclint agents workflow event`, so arclint must be on the host's PATH."

// NewWorkflowCommand is the workflow hook surface: install and status
// manage host configuration, event answers the hosts.
func NewWorkflowCommand(hooks WorkflowHooks, events WorkflowEvents, render Renderer) Command {
	return Command{Name: "workflow", Short: "advise coding agents through the domain workflow with native hooks", Subcommands: []Command{
		{Name: agentInstallCommand, Short: "write the workflow hooks into Claude Code and Codex project configuration", Flags: []Flag{
			{Name: agentHostFlag, Repeat: true, Options: []string{agentHostClaude, agentHostCodex}, Doc: "agent host to install for (claude, codex); repeatable; default both"},
		}, Run: func(ctx Context) error {
			hosts := ctx.Strings(agentHostFlag)
			paths, err := hooks.Install(hosts)
			if err != nil {
				return ConfigError(err)
			}
			return render.Render(ctx.Stdout, AgentInstallReport{Operation: "workflow", Host: hostList(hosts), Paths: paths, Activation: WorkflowActivation})
		}},
		{Name: "status", Short: "show which hosts list the workflow hooks", Run: func(ctx Context) error {
			status, err := hooks.Status()
			if err != nil {
				return ConfigError(err)
			}
			return render.Render(ctx.Stdout, WorkflowStatusReport{Status: status})
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
