package hooks

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/agent"
)

// Host event names and the user-facing message field shared by Codex and
// Claude Code.
const (
	sessionStartEvent = "SessionStart"
	postToolUseEvent  = "PostToolUse"
	stopEvent         = "Stop"
	systemMessage     = "systemMessage"
)

// agentsFile is the repository instructions file the AGENTS.md block lives in.
const agentsFile = "AGENTS.md"

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
	project    project
	ruleset    string
	recording  string
	binary     string
	guide      application.GuideWorkflow
	deliveries Deliveries
}

// Deliveries tells a session's first delivery of an event from a repeat.
type Deliveries interface {
	// Deliver reports whether event, a digest of one host event, differs
	// from the event the session last received.
	Deliver(session, event string) (bool, error)
}

// NewEvents connects hook events for the project at root to guide. ruleset
// is the path of the project's ruleset; recording is the domain recording's
// path relative to root; distro names the WSL distribution this process
// runs in, empty outside WSL; binary is this arclint's path, which a
// workflow note names.
func NewEvents(root, ruleset, recording, distro, binary string, guide application.GuideWorkflow, deliveries Deliveries) (*Events, error) {
	p, err := newProject(root, distro)
	if err != nil {
		return nil, err
	}
	return &Events{project: p, ruleset: ruleset, recording: recording, binary: binary, guide: guide, deliveries: deliveries}, nil
}

// Handle answers one hook event read from the host. A session that cannot
// keep its progress hears so once, when it starts; later events stay
// silent rather than repeat the same message on every tool call. A
// directory with no ruleset is not an ArcLint project: a user install runs
// the hooks there too, and they answer nothing and write nothing.
func (events *Events) Handle(input []byte) ([]byte, error) {
	if _, err := os.Stat(events.ruleset); err != nil {
		return nativeOutput(map[string]any{})
	}
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
	// A host that lists the hooks in two configuration files delivers each
	// event twice; the repeat is not answered again.
	digest := sha256.Sum256(input)
	if first, err := events.deliveries.Deliver(event.SessionID, hex.EncodeToString(digest[:])); err == nil && !first {
		return nativeOutput(map[string]any{})
	}
	var observed []agent.Activity
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
		observed = []agent.Activity{{Kind: agent.TurnFinished}}
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
func (events *Events) toolActivities(cwd string, event hostEvent) []agent.Activity {
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
func (events *Events) changes(dir string, names ...string) []agent.Activity {
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
	var activities []agent.Activity
	if len(domain) > 0 {
		activities = append(activities, agent.Activity{Kind: agent.DomainChanged, Paths: domain})
	}
	if len(files) > 0 {
		activities = append(activities, agent.Activity{Kind: agent.FilesChanged, Paths: files})
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

// orientation states the workflow the generated AGENTS.md block states,
// from the same source, and notes when the committed block states another.
func (events *Events) orientation() string {
	text := "ArcLint workflow hooks run in this project. " + application.WorkflowOrientation(events.recording)
	if note := events.skew(); note != "" {
		text += "\n" + note
	}
	return text
}

// skew notes when the project's AGENTS.md block states a different
// workflow than this arclint: the binary the hooks run and the one that
// generated the block are not the same release.
func (events *Events) skew() string {
	content, err := os.ReadFile(filepath.Join(events.project.root, agentsFile))
	if err != nil {
		return ""
	}
	_, block, found := strings.Cut(strings.ReplaceAll(string(content), "\r\n", "\n"), application.AgentsBegin)
	if !found {
		return ""
	}
	block, _, _ = strings.Cut(block, application.AgentsEnd)
	if workflowSection(block) == strings.TrimSpace(application.AgentWorkflowSection()) {
		return ""
	}
	return "The arclint these hooks run (" + events.binary + ") states a different workflow than the " + agentsFile + " block. Follow " + agentsFile + "; then update the arclint the hooks run, or regenerate the block with `arclint agents md --write` if this arclint is the newer one."
}

// workflowSection is the Workflow section of a generated block, empty when
// the block has none.
func workflowSection(block string) string {
	const heading = "### Workflow"
	_, section, found := strings.Cut(block, heading)
	if !found {
		return ""
	}
	section, _, _ = strings.Cut(section, "\n### ")
	return strings.TrimSpace(heading + section)
}

func (events *Events) advice(guidance []agent.Guidance) string {
	lines := make([]string, 0, len(guidance))
	for _, given := range guidance {
		paths := strings.Join(given.Paths, ", ")
		switch given.Step {
		case agent.ContextStep:
			lines = append(lines, fmt.Sprintf("ArcLint workflow: %s changed before `arclint context` showed every Zone that owns it. Run `arclint context %s` to see those Zones, their contracts and the recorded domain that bind it.", paths, strings.Join(shellQuoted(given.Paths), " ")))
		case agent.DomainStep:
			lines = append(lines, fmt.Sprintf("ArcLint workflow: %s changed before %s changed in this session. If this work introduces or changes a meaning, record it in %s first, using the domain-librarian skill. If it does not, continue.", paths, events.recording, events.recording))
		case agent.CheckStep:
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
