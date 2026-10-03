// Package codex installs the project-local Codex domain guard.
package codex

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	agentfiles "github.com/wixregiga/arclint/internal/infrastructure/agents/files"
)

//go:embed assets/guard.py
var guard []byte

// Installer writes only the named hook assets and preserves other configuration.
type Installer struct{ root string }

// NewInstaller binds the installer to the project root.
func NewInstaller(root string) *Installer { return &Installer{root: root} }

// Install adds native command hooks. Codex still requires /hooks trust review.
func (i *Installer) Install(host string, domains []string, scope ...application.AgentSourceScope) ([]string, error) {
	if host != codexHost {
		return nil, fmt.Errorf("unsupported host %q", host)
	}
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("codex guard installer currently supports Linux/WSL projects; Windows desktop uses the WSL command override")
	}
	root, err := filepath.Abs(i.root)
	if err != nil {
		return nil, fmt.Errorf("project root: %w", err)
	}
	configPath := filepath.Join(root, ".arclint/domain-guard.json")
	config, err := agentfiles.Scope(root, domains, scope...)
	if err != nil {
		return nil, fmt.Errorf("agent setup: %w", err)
	}
	script := filepath.Join(root, ".codex/hooks/arclint-domain-guard/guard.py")
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
	command := "python3 " + quote(script) + " --root " + quote(root)
	handler := map[string]any{"type": "command", "command": command, "timeout": 90, "statusMessage": "ArcLint domain review"}
	if distro := os.Getenv("WSL_DISTRO_NAME"); distro != "" {
		if strings.ContainsAny(distro+script+root, "\"\r\n") {
			return nil, fmt.Errorf("unsupported character in Windows hook path")
		}
		handler["commandWindows"] = "wsl.exe -d \"" + distro + "\" -- python3 \"" + script + "\" --root \"" + root + "\""
	}
	hooksPath := filepath.Join(root, ".codex/hooks.json")
	document := map[string]any{}
	if existing, readErr := os.ReadFile(hooksPath); readErr == nil {
		if err := json.Unmarshal(existing, &document); err != nil {
			return nil, fmt.Errorf("preserving invalid existing hooks: %w", err)
		}
	} else if !os.IsNotExist(readErr) {
		return nil, fmt.Errorf("read hooks: %w", readErr)
	}
	hooks, ok := document["hooks"].(map[string]any)
	if !ok && document["hooks"] != nil {
		return nil, fmt.Errorf("existing hooks field is not an object")
	}
	if hooks == nil {
		hooks = map[string]any{}
		document["hooks"] = hooks
	}
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop"} {
		groups, ok := hooks[event].([]any)
		if !ok && hooks[event] != nil {
			return nil, fmt.Errorf("existing %s hooks are invalid", event)
		}
		found := false
		for _, group := range groups {
			data, _ := json.Marshal(group)
			if bytes.Contains(data, []byte(script)) {
				expected, _ := json.Marshal(map[string]any{"hooks": []any{handler}})
				if !bytes.Equal(data, expected) {
					return nil, fmt.Errorf("preserving modified ArcLint %s hook", event)
				}
				found = true
			}
		}
		if !found {
			hooks[event] = append(groups, map[string]any{"hooks": []any{handler}})
		}
	}
	hookBytes, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode hooks: %w", err)
	}
	hookBytes = append(hookBytes, '\n')
	files := map[string][]byte{configPath: config, script: guard, hooksPath: hookBytes}
	paths, err := agentfiles.Install(root, files, configPath, hooksPath)
	if err != nil {
		return nil, fmt.Errorf("install hooks: %w", err)
	}
	return paths, nil
}
