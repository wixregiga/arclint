package hooks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"

	"github.com/wixregiga/arclint/internal/domain/agent"
)

const preInvocationEvent = "PreInvocation"

var antigravityEditTools = []string{"edit_file", "write_file", "write_to_file", "replace_file_content", "multi_replace_file_content", "code_action"}

type antigravityEvent struct {
	Name              string   `json:"hookEventName"`
	Conversation      string   `json:"conversationId"`
	Cwd               string   `json:"cwd"`
	Workspaces        []string `json:"workspacePaths"`
	Error             string   `json:"error"`
	FullyIdle         bool     `json:"fullyIdle"`
	TerminationReason string   `json:"terminationReason"`
	Tool              struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"args"`
	} `json:"toolCall"`
}

func decodeAntigravity(input []byte) (antigravityEvent, bool) {
	var event antigravityEvent
	err := json.Unmarshal(input, &event)
	return event, err == nil && event.Name != ""
}

// AntigravityDirectory locates the agent's working directory for ruleset
// discovery. The hook process itself runs from the configuration directory.
func AntigravityDirectory(input []byte, distro string) (string, bool) {
	event, ok := decodeAntigravity(input)
	return event.directory(distro), ok
}

func (event antigravityEvent) directory(distro string) string {
	name := event.Cwd
	if name == "" && len(event.Workspaces) == 1 {
		name = event.Workspaces[0]
	}
	name = (project{distro: distro}).local(name)
	if !filepath.IsAbs(name) {
		return ""
	}
	return name
}

func (events *Events) handleAntigravity(input []byte, event antigravityEvent) ([]byte, error) {
	if event.Name == stopEvent {
		// A Stop reason is injected only if the hook forces continuation.
		events.observeAntigravity(input, event)
		return nativeOutput(map[string]string{"decision": "allow"})
	}
	text := events.observeAntigravity(input, event)
	if text == "" {
		return nativeOutput(map[string]any{})
	}
	return nativeOutput(map[string]any{
		"injectSteps": []map[string]map[string]string{{systemMessage: {systemMessage: text}}},
	})
}

func (events *Events) observeAntigravity(input []byte, event antigravityEvent) string {
	if _, err := os.Stat(events.ruleset); err != nil || event.Conversation == "" {
		return ""
	}
	cwd := event.directory(events.project.distro)
	if cwd == "" {
		return ""
	}
	if _, inside := events.project.relative(events.project.root, cwd); !inside {
		return ""
	}
	var observed []agent.Activity
	var step antigravityStep
	switch event.Name {
	case sessionStartEvent:
		step = antigravityStep{closeIdle: true, deliver: true}
	case preInvocationEvent:
		step = antigravityStep{closeIdle: true, startTurn: true, deliver: true}
	case postToolUseEvent:
		if event.Error == "" {
			observed = events.changes(cwd, event.editedPath())
		}
	case stopEvent:
		if event.finished() {
			step.finishTurn = true
			observed = []agent.Activity{{Kind: agent.TurnFinished}}
		}
	default:
		return ""
	}
	digest := sha256.Sum256(input)
	// Namespace conversation IDs away from other hosts' session IDs.
	session := antigravityHost + ":" + event.Conversation
	guidance, first, err := events.sessions.updateAntigravity(session, hex.EncodeToString(digest[:]), step,
		func(progress agent.Progress, recorded []agent.Activity) (agent.Progress, []agent.Guidance) {
			return events.guide.Evaluate(progress, append(recorded, observed...))
		},
		events.guide.PathOwners(),
	)
	if event.Name == sessionStartEvent {
		if err != nil {
			return events.orientation() + "\nArcLint workflow guidance unavailable for this session: " + err.Error()
		}
		if first {
			text := events.orientation()
			if len(guidance) > 0 {
				text += "\n" + events.advice(guidance)
			}
			return text
		}
	}
	if err != nil {
		return ""
	}
	return events.advice(guidance)
}

func (event antigravityEvent) finished() bool {
	return event.Name == stopEvent && event.FullyIdle &&
		(event.TerminationReason == "NO_TOOL_CALL" || event.TerminationReason == "TERMINAL_STEP_TYPE")
}

func (event antigravityEvent) editedPath() string {
	if !slices.Contains(antigravityEditTools, event.Tool.Name) {
		return ""
	}
	var args map[string]json.RawMessage
	if json.Unmarshal(event.Tool.Args, &args) != nil {
		return ""
	}
	for _, key := range []string{"Path", "path", "TargetFile", "targetFile", "AbsolutePath", "absolutePath"} {
		var path string
		if json.Unmarshal(args[key], &path) == nil && path != "" {
			return path
		}
	}
	return ""
}
