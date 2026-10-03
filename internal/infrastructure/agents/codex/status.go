package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	agentfiles "github.com/wixregiga/arclint/internal/infrastructure/agents/files"
)

const codexHookTypeCommand = "command"

// InspectHostInstallation recognizes ArcLint's required native hook registrations.
// It does not infer that Codex loaded, trusted or executed those registrations.
func (i *Installer) InspectHostInstallation() (application.AgentHostInstallation, error) {
	result := application.AgentHostInstallation{Name: "Codex", Paths: []string{".codex/hooks.json", ".codex/hooks/arclint-domain-guard/guard.py"}}
	root, err := filepath.Abs(i.root)
	if err != nil {
		return result, fmt.Errorf("inspect Codex project root: %w", err)
	}
	script := filepath.Join(root, result.Paths[1])
	if _, err := os.Lstat(script); err == nil {
		result.Present = true
	}
	hooksPath := filepath.Join(root, result.Paths[0])
	if err := agentfiles.CheckPath(root, hooksPath); err != nil {
		result.Problems = append(result.Problems, err.Error())
		return result, nil
	}
	data, err := os.ReadFile(hooksPath)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("read Codex hook registration: %w", err)
	}
	var document struct {
		Hooks map[string][]struct {
			Hooks []struct{ Type, Command string }
		}
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return result, fmt.Errorf("decode Codex hook registration: %w", err)
	}
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
	expected := "python3 " + quote(script) + " --root " + quote(root)
	registered := true
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop"} {
		found := false
		for _, group := range document.Hooks[event] {
			for _, hook := range group.Hooks {
				if hook.Type == codexHookTypeCommand && hook.Command == expected {
					found = true
					result.Present = true
				}
			}
		}
		if !found {
			registered = false
		}
	}
	result.Registered = registered
	if result.Present && !registered {
		result.Problems = append(result.Problems, "Codex is missing required ArcLint hook registrations")
	}
	return result, nil
}
