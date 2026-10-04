package workflow

import "encoding/json"

const (
	sessionStartEvent     = "SessionStart"
	userPromptSubmitEvent = "UserPromptSubmit"
	preToolUseEvent       = "PreToolUse"
	postToolUseEvent      = "PostToolUse"
	stopEvent             = "Stop"
	hooksField            = "hooks"
)

// CodexEvent is the native input envelope; arguments and results remain evidence.
type CodexEvent struct {
	HookEventName        string          `json:"hook_event_name"`
	SessionID            string          `json:"session_id"`
	Cwd                  string          `json:"cwd"`
	Prompt               string          `json:"prompt"`
	ToolName             string          `json:"tool_name"`
	ToolInput            json.RawMessage `json:"tool_input"`
	ToolResponse         json.RawMessage `json:"tool_response"`
	ToolResult           json.RawMessage `json:"tool_result"`
	LastAssistantMessage string          `json:"last_assistant_message"`
}
