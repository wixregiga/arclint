package cli

import (
	"errors"
	"slices"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	"github.com/wixregiga/arclint/internal/domain/workflow"
)

// The commands whose runs the workflow hooks credit.
const (
	contextCommand = "context"
	checkCommand   = "check"
)

// WorkflowActivityLog keeps the workflow activities ArcLint's own commands
// perform, so the workflow hooks credit what actually ran.
type WorkflowActivityLog interface {
	Record(workflow.Activity) error
	// RecordingState identifies the domain recording's current content, so
	// a domain command that leaves it unchanged is not credited.
	RecordingState() string
}

// RecordWorkflowActivity makes the commands that obtain context, run the
// full check or change the domain recording report themselves to log after
// they run. Recording is a side channel: a failed record never changes the
// command's outcome; it costs at most one extra reminder.
func RecordWorkflowActivity(root Command, log WorkflowActivityLog) Command {
	root.Subcommands = slices.Clone(root.Subcommands)
	for index, command := range root.Subcommands {
		switch command.Name {
		case contextCommand:
			root.Subcommands[index].Run = recorded(command.Run, log, contextActivity)
		case checkCommand:
			root.Subcommands[index].Run = recorded(command.Run, log, checkActivity)
		case "domain":
			subcommands := slices.Clone(command.Subcommands)
			for position, subcommand := range subcommands {
				if subcommand.Name == "init" || subcommand.Name == "define" || subcommand.Name == "remove" {
					subcommands[position].Run = recordedDomainChange(subcommand.Run, log)
				}
			}
			root.Subcommands[index].Subcommands = subcommands
		}
	}
	return root
}

func recorded(run func(Context) error, log WorkflowActivityLog, activity func(Context, error) (workflow.Activity, bool)) func(Context) error {
	if run == nil {
		return nil
	}
	return func(ctx Context) error {
		err := run(ctx)
		if performed, ok := activity(ctx, err); ok {
			_ = log.Record(performed)
		}
		return err
	}
}

// recordedDomainChange credits a domain command only when the recording's
// content changed: defining an entry again unchanged, or initializing a
// recording that exists, changes nothing.
func recordedDomainChange(run func(Context) error, log WorkflowActivityLog) func(Context) error {
	if run == nil {
		return nil
	}
	return func(ctx Context) error {
		before := log.RecordingState()
		err := run(ctx)
		if err == nil && log.RecordingState() != before {
			_ = log.Record(workflow.Activity{Kind: workflow.DomainChanged, Paths: []string{vocab.UbiquitousLanguageFileName}})
		}
		return err
	}
}

// contextActivity is context for the paths and Zones the command named,
// passed exactly as the command passes them on; the hooks resolve the
// Zones that own the paths the way the command does.
func contextActivity(ctx Context, err error) (workflow.Activity, bool) {
	if err != nil {
		return workflow.Activity{}, false
	}
	var paths []string
	for _, argument := range ctx.Args {
		paths = append(paths, strings.TrimPrefix(argument, "./"))
	}
	return workflow.Activity{Kind: workflow.ContextObtained, Paths: paths, Zones: splitSelectors(ctx.String("zone"))}, true
}

// checkActivity is a check of every rule; one narrowed by --only or
// --exclude verifies only part of the work. A check that reports gating
// findings still ran.
func checkActivity(ctx Context, err error) (workflow.Activity, bool) {
	var exit *ExitError
	ran := err == nil || (errors.As(err, &exit) && exit.Code == ExitViolations)
	if !ran || ctx.String("only") != "" || ctx.String("exclude") != "" {
		return workflow.Activity{}, false
	}
	return workflow.Activity{Kind: workflow.CheckRan}, true
}
