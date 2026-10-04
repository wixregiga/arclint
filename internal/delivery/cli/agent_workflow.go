package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/workflow"
)

const (
	workflowInstallCommand = "install"
	workflowStatusCommand  = "status"
)

// WorkflowHooks is the CLI-owned installation and status boundary.
type WorkflowHooks interface {
	Install() ([]string, error)
	Status() (application.WorkflowHookStatus, error)
}

// WorkflowEvents is the CLI-owned native event translation boundary.
// Its response is an advisory host protocol, never a domain approval.
type WorkflowEvents interface {
	Handle(context.Context, []byte) ([]byte, error)
}

// NewWorkflowCommand supplies the independent task-focused reporting surface.
func NewWorkflowCommand(review application.ReviewWorkflow, hooks WorkflowHooks, events WorkflowEvents, render Renderer) Command {
	return Command{Name: "workflow", Short: "report task-focused domain workflow departures", Subcommands: []Command{
		{Name: workflowInstallCommand, Short: "install independent advisory workflow hooks", Run: func(ctx Context) error {
			paths, err := hooks.Install()
			if err != nil {
				return ConfigError(err)
			}
			return render.Render(ctx.Stdout, AgentInstallReport{Operation: "workflow", Host: agentHostCodex, Paths: paths, Activation: "Run bash .codex/hooks/arclint-workflow-guard/start-codex.sh. Review and approve the displayed workflow hooks in Codex /hooks, then restart with the same launcher. " + WorkflowStatusLimits})
		}},
		{Name: workflowStatusCommand, Short: "inspect independent workflow hook installation", Run: func(ctx Context) error {
			status, err := hooks.Status()
			if err != nil {
				return ConfigError(err)
			}
			return render.Render(ctx.Stdout, WorkflowStatusReport{Status: status})
		}},
		{Name: "event", Short: "translate a Codex event on stdin into advisory feedback", Run: func(ctx Context) error {
			input, err := io.ReadAll(ctx.Stdin)
			if err != nil {
				return ConfigError(fmt.Errorf("read workflow event: %w", err))
			}
			output, err := events.Handle(context.Background(), input)
			if err != nil {
				return ConfigError(err)
			}
			if _, err := ctx.Stdout.Write(output); err != nil {
				return fmt.Errorf("write workflow event: %w", err)
			}
			return nil
		}},
		{Name: "review", Short: "assess supplied task evidence JSON on stdin", Run: func(ctx Context) error {
			var evidence workflow.Evidence
			decoder := json.NewDecoder(ctx.Stdin)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&evidence); err != nil {
				return ConfigError(fmt.Errorf("workflow evidence JSON: %w", err))
			}
			var trailing any
			if err := decoder.Decode(&trailing); err != io.EOF {
				return ConfigError(fmt.Errorf("workflow evidence must contain one JSON object"))
			}
			assessment, err := review.Execute(context.Background(), evidence)
			if err != nil {
				return ConfigError(err)
			}
			// Review JSON is a raw assessment protocol, like emitted schema products.
			if err := json.NewEncoder(ctx.Stdout).Encode(assessment); err != nil {
				return fmt.Errorf("write workflow assessment: %w", err)
			}
			return nil
		}},
	}}
}
