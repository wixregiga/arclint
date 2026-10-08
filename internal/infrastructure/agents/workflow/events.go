package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/workflow"
)

// Host event names and the user-facing message field shared by Codex and
// Claude Code.
const (
	sessionStartEvent = "SessionStart"
	postToolUseEvent  = "PostToolUse"
	stopEvent         = "Stop"
	systemMessage     = "systemMessage"
)

var patchHeaders = []string{"*** Add File: ", "*** Update File: ", "*** Delete File: ", "*** Move to: "}

// hostEvent is the part of a Codex or Claude Code hook event the workflow
// reads. Both hosts use these field names.
type hostEvent struct {
	HookEventName string `json:"hook_event_name"`
	SessionID     string `json:"session_id"`
	Cwd           string `json:"cwd"`
	ToolName      string `json:"tool_name"`
	ToolInput     struct {
		Command      json.RawMessage `json:"command"`
		FilePath     string          `json:"file_path"`
		NotebookPath string          `json:"notebook_path"`
	} `json:"tool_input"`
}

// patch is the text of a Codex apply_patch call; a shape other than a JSON
// string carries no patch.
func (event hostEvent) patch() string {
	var text string
	if json.Unmarshal(event.ToolInput.Command, &text) != nil {
		return ""
	}
	return text
}

// Events answers Agent Host hook events with workflow Guidance in the
// host's native output. It never blocks the agent.
type Events struct {
	project   project
	recording string
	guide     application.GuideWorkflow
}

// NewEvents connects hook events for the project at root to guide.
// recording is the domain recording's path relative to root; distro names
// the WSL distribution this process runs in, empty outside WSL.
func NewEvents(root, recording, distro string, guide application.GuideWorkflow) (*Events, error) {
	p, err := newProject(root, distro)
	if err != nil {
		return nil, err
	}
	return &Events{project: p, recording: recording, guide: guide}, nil
}

// Handle answers one hook event read from the host. A session that cannot
// keep its progress hears so once, when it starts; later events stay
// silent rather than repeat the same message on every tool call.
func (events *Events) Handle(input []byte) ([]byte, error) {
	var event hostEvent
	if err := json.Unmarshal(input, &event); err != nil {
		return nativeOutput(map[string]any{systemMessage: "ArcLint workflow guidance unavailable: the hook event is not valid JSON."})
	}
	cwd := events.project.root
	if event.Cwd != "" {
		cwd = events.project.local(event.Cwd)
	}
	if _, inside := events.project.relative(events.project.root, cwd); !inside {
		return nativeOutput(map[string]any{})
	}
	var observed []workflow.Activity
	switch event.HookEventName {
	case sessionStartEvent:
		output := map[string]any{"hookSpecificOutput": map[string]string{"hookEventName": sessionStartEvent, "additionalContext": events.orientation()}}
		if _, err := events.guide.Execute(event.SessionID, nil); err != nil {
			output[systemMessage] = "ArcLint workflow guidance unavailable for this session: " + err.Error()
		}
		return nativeOutput(output)
	case postToolUseEvent:
		observed = events.toolActivities(cwd, event)
	case stopEvent:
		observed = []workflow.Activity{{Kind: workflow.TurnFinished}}
	default:
		return nativeOutput(map[string]any{})
	}
	guidance, err := events.guide.Execute(event.SessionID, observed)
	if err != nil || len(guidance) == 0 {
		return nativeOutput(map[string]any{})
	}
	text := events.advice(guidance)
	if event.HookEventName == stopEvent {
		return nativeOutput(map[string]any{systemMessage: text})
	}
	return nativeOutput(map[string]any{"hookSpecificOutput": map[string]string{"hookEventName": postToolUseEvent, "additionalContext": text}})
}

// toolActivities reads the files a finished edit tool changed.
func (events *Events) toolActivities(cwd string, event hostEvent) []workflow.Activity {
	input := event.ToolInput
	switch event.ToolName {
	case "apply_patch":
		return events.changes(cwd, patchPaths(event.patch())...)
	case "Edit", "Write", "MultiEdit":
		return events.changes(cwd, input.FilePath)
	case "NotebookEdit":
		return events.changes(cwd, input.NotebookPath)
	}
	return nil
}

// changes classifies changed files as the domain recording or other files.
// Paths outside the project and tool metadata are not the agent's work.
func (events *Events) changes(dir string, names ...string) []workflow.Activity {
	var domain, files []string
	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			continue
		}
		relative, inside := events.project.relative(dir, name)
		switch {
		case !inside || relative == "." || relative == ".git" || strings.HasPrefix(relative, ".git/") || strings.HasPrefix(relative, ".arclint/cache/"):
		case relative == events.recording:
			domain = append(domain, relative)
		default:
			files = append(files, relative)
		}
	}
	var activities []workflow.Activity
	if len(domain) > 0 {
		activities = append(activities, workflow.Activity{Kind: workflow.DomainChanged, Paths: domain})
	}
	if len(files) > 0 {
		activities = append(activities, workflow.Activity{Kind: workflow.FilesChanged, Paths: files})
	}
	return activities
}

// patchPaths returns the files an apply_patch adds, updates, deletes or
// moves to.
func patchPaths(text string) []string {
	var paths []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		for _, header := range patchHeaders {
			if name, found := strings.CutPrefix(line, header); found && strings.TrimSpace(name) != "" {
				paths = append(paths, strings.TrimSpace(name))
			}
		}
	}
	return paths
}

func (events *Events) orientation() string {
	return "ArcLint workflow hooks are installed in this project. Work in this order:\n" +
		"1. Run `arclint context <paths>` before reading or changing files under those paths.\n" +
		"2. When the work introduces or changes a meaning, record it in " + events.recording + " first, using the domain-librarian skill.\n" +
		"3. Implement.\n" +
		"4. Run `arclint check .` and the project's tests before finishing.\n" +
		"The hooks advise when observed work skips a step. They never block a tool call."
}

func (events *Events) advice(guidance []workflow.Guidance) string {
	lines := make([]string, 0, len(guidance))
	for _, given := range guidance {
		paths := strings.Join(given.Paths, ", ")
		switch given.Step {
		case workflow.ContextStep:
			lines = append(lines, fmt.Sprintf("ArcLint workflow: %s changed before `arclint context` showed every Zone that owns it. Run `arclint context %s` to see those Zones, their contracts and the recorded domain that bind it.", paths, strings.Join(shellQuoted(given.Paths), " ")))
		case workflow.DomainStep:
			lines = append(lines, fmt.Sprintf("ArcLint workflow: %s changed before %s changed in this session. If this work introduces or changes a meaning, record it in %s first, using the domain-librarian skill. If it does not, continue.", paths, events.recording, events.recording))
		case workflow.CheckStep:
			lines = append(lines, fmt.Sprintf("ArcLint workflow: %s changed since the last `arclint check`. Run `arclint check .` and the project's tests before finishing.", paths))
		}
	}
	return strings.Join(lines, "\n")
}

// shellQuoted quotes the paths that a shell would otherwise split.
func shellQuoted(paths []string) []string {
	quoted := make([]string, len(paths))
	for index, path := range paths {
		if strings.ContainsAny(path, " \t'\"\\$`*?[]{}()<>|&;#~") {
			path = "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
		}
		quoted[index] = path
	}
	return quoted
}

func nativeOutput(value any) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, fmt.Errorf("encode workflow hook output: %w", err)
	}
	return output.Bytes(), nil
}
