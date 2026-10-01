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
)

//go:embed assets/guard.py
var guard []byte

// Installer writes only the named hook assets and preserves other configuration.
type Installer struct{ root string }

// NewInstaller binds the installer to the project root.
func NewInstaller(root string) *Installer { return &Installer{root: root} }

// Install adds native command hooks. Codex still requires /hooks trust review.
func (i *Installer) Install(host string, domains []string) ([]string, error) {
	if host != "codex" {
		return nil, fmt.Errorf("unsupported host %q", host)
	}
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("codex guard installer currently supports Linux/WSL projects; Windows desktop uses the WSL command override")
	}
	root, err := filepath.Abs(i.root)
	if err != nil {
		return nil, fmt.Errorf("project root: %w", err)
	}
	for _, name := range domains {
		path, resolveErr := filepath.EvalSymlinks(filepath.Join(root, name))
		if resolveErr != nil {
			return nil, fmt.Errorf("domain file: %w", resolveErr)
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil || filepath.IsAbs(name) || rel == ".." || strings.HasPrefix(rel, "../") {
			return nil, fmt.Errorf("domain path escapes project: %s", name)
		}
	}
	configPath := filepath.Join(root, ".arclint/domain-guard.json")
	config, err := json.MarshalIndent(struct {
		Version     int      `json:"version"`
		DomainFiles []string `json:"domainFiles"`
	}{1, domains}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("guard config: %w", err)
	}
	config = append(config, '\n')
	if existing, readErr := os.ReadFile(configPath); readErr == nil {
		var parsed struct {
			Version     int      `json:"version"`
			DomainFiles []string `json:"domainFiles"`
		}
		if json.Unmarshal(existing, &parsed) != nil || parsed.Version != 1 || strings.Join(parsed.DomainFiles, "\x00") != strings.Join(domains, "\x00") {
			return nil, fmt.Errorf("preserving different domain scope in %s", configPath)
		}
		config = existing
	} else if !os.IsNotExist(readErr) {
		return nil, fmt.Errorf("read guard config: %w", readErr)
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
	previous := make(map[string][]byte, len(files))
	// Validate all paths and conflicts before the first write.
	for path, content := range files {
		for parent := path; parent != root; parent = filepath.Dir(parent) {
			info, statErr := os.Lstat(parent)
			if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("refusing symlinked install path: %s", parent)
			}
			if statErr != nil && !os.IsNotExist(statErr) {
				return nil, fmt.Errorf("inspect install path: %w", statErr)
			}
		}
		existing, readErr := os.ReadFile(path)
		previous[path] = existing
		if readErr == nil && path != hooksPath && !bytes.Equal(existing, content) {
			return nil, fmt.Errorf("preserving existing different file: %s", path)
		}
		if readErr != nil && !os.IsNotExist(readErr) {
			return nil, fmt.Errorf("inspect install target: %w", readErr)
		}
	}
	for _, path := range []string{configPath, script, hooksPath} {
		existing, readErr := os.ReadFile(path)
		if readErr == nil && bytes.Equal(existing, files[path]) {
			continue
		}
		if !bytes.Equal(existing, previous[path]) {
			return nil, fmt.Errorf("install target changed concurrently; retry: %s", path)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return nil, fmt.Errorf("create hook directory: %w", err)
		}
		file, createErr := os.CreateTemp(filepath.Dir(path), ".arclint-install-*")
		if createErr != nil {
			return nil, fmt.Errorf("stage hook: %w", createErr)
		}
		name := file.Name()
		if _, err := file.Write(files[path]); err != nil {
			_ = file.Close()
			_ = os.Remove(name)
			return nil, fmt.Errorf("write hook: %w", err)
		}
		if err := file.Close(); err != nil {
			_ = os.Remove(name)
			return nil, fmt.Errorf("close hook: %w", err)
		}
		if err := os.Rename(name, path); err != nil {
			_ = os.Remove(name)
			return nil, fmt.Errorf("install hook: %w", err)
		}
	}
	return []string{configPath, script, hooksPath}, nil
}
