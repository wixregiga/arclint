package workflow

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
	agentfiles "github.com/wixregiga/arclint/internal/infrastructure/agents/files"
)

const (
	workflowDirectory = ".codex/hooks/arclint-workflow-guard"
	workflowReceipt   = ".arclint/workflow-guard.json"
)

var workflowEvents = []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop"}

type installation struct {
	Version string         `json:"version"`
	Command string         `json:"command"`
	Handler map[string]any `json:"handler"`
}

// Installer installs an independent workflow hook without replacing existing hooks.
type Installer struct{ root, binary, version string }

// NewInstaller binds workflow delivery to one project and ArcLint release.
func NewInstaller(root, binary, version string) *Installer {
	return &Installer{root: root, binary: binary, version: version}
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }

// Install preserves unrelated hooks and refuses to overwrite edited assets.
func (i *Installer) Install() ([]string, error) {
	root, err := filepath.Abs(i.root)
	if err != nil {
		return nil, fmt.Errorf("install workflow hooks: %w", err)
	}
	contents, err := os.ReadFile(i.binary)
	if err != nil {
		return nil, fmt.Errorf("read ArcLint executable: %w", err)
	}
	executable := filepath.Join(root, workflowDirectory, "arclint")
	command := shellQuote(executable) + " --rules " + shellQuote(filepath.Join(root, "rules.arclint.yaml")) + " agents workflow event"
	handler := map[string]any{"type": "command", "command": command, "timeout": 90, "statusMessage": "ArcLint workflow review"}
	if distro := os.Getenv("WSL_DISTRO_NAME"); distro != "" {
		if strings.ContainsAny(distro+executable+root, "\"\r\n") {
			return nil, fmt.Errorf("unsupported character in Windows workflow hook path")
		}
		handler["commandWindows"] = "wsl.exe -d \"" + distro + "\" -- \"" + executable + "\" --rules \"" + filepath.Join(root, "rules.arclint.yaml") + "\" agents workflow event"
	}
	hooksPath := filepath.Join(root, ".codex/hooks.json")
	if err := agentfiles.CheckPath(root, hooksPath); err != nil {
		return nil, fmt.Errorf("workflow registration path: %w", err)
	}
	document := map[string]any{}
	if data, readErr := os.ReadFile(hooksPath); readErr == nil {
		if err := json.Unmarshal(data, &document); err != nil {
			return nil, fmt.Errorf("preserving invalid hooks file: %w", err)
		}
	} else if !os.IsNotExist(readErr) {
		return nil, fmt.Errorf("read workflow registration: %w", readErr)
	}
	hooks, ok := document[hooksField].(map[string]any)
	if document[hooksField] != nil && !ok {
		return nil, fmt.Errorf("preserving invalid hooks object")
	}
	if hooks == nil {
		hooks = map[string]any{}
		document[hooksField] = hooks
	}
	group := map[string]any{hooksField: []any{handler}}
	encodedGroup, err := json.Marshal(group)
	if err != nil {
		return nil, fmt.Errorf("install workflow hooks: %w", err)
	}
	for _, event := range workflowEvents {
		groups, valid := hooks[event].([]any)
		if hooks[event] != nil && !valid {
			return nil, fmt.Errorf("preserving invalid %s hook groups", event)
		}
		found := false
		for _, existing := range groups {
			data, marshalErr := json.Marshal(existing)
			if marshalErr != nil {
				return nil, fmt.Errorf("encode workflow hook: %w", marshalErr)
			}
			if bytes.Contains(data, []byte(executable)) {
				if !bytes.Equal(data, encodedGroup) {
					return nil, fmt.Errorf("preserving modified workflow hook: %s", event)
				}
				found = true
			}
		}
		if !found {
			hooks[event] = append(groups, group)
		}
	}
	registration, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("install workflow hooks: %w", err)
	}
	metadata, err := json.MarshalIndent(installation{Version: i.version, Command: command, Handler: handler}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("install workflow hooks: %w", err)
	}
	launcher, err := launchScript(root, handler)
	if err != nil {
		return nil, fmt.Errorf("prepare workflow launcher: %w", err)
	}
	assets := map[string][]byte{
		executable: contents,
		filepath.Join(root, workflowDirectory, "instructions.md"): []byte(Instructions()),
		filepath.Join(root, workflowDirectory, "start-codex.sh"):  launcher,
		filepath.Join(root, workflowReceipt):                      append(metadata, '\n'),
		hooksPath:                                                 append(registration, '\n'),
	}
	paths, err := agentfiles.Install(root, assets, hooksPath)
	if err != nil {
		return nil, fmt.Errorf("install workflow hooks: %w", err)
	}
	// The installed hook must be executable; only its owner receives access.
	if err := os.Chmod(executable, 0o700); err != nil { //nolint:gosec // Executable hook, owner-only access.
		return nil, fmt.Errorf("make installed workflow executable runnable: %w", err)
	}
	return paths, nil
}

func hash(content []byte) string { sum := sha256.Sum256(content); return hex.EncodeToString(sum[:]) }

// Status observes receipt integrity and registrations without inferring activation.
func (i *Installer) Status() (application.WorkflowHookStatus, error) {
	root, err := filepath.Abs(i.root)
	if err != nil {
		return application.WorkflowHookStatus{}, fmt.Errorf("workflow status root: %w", err)
	}
	s := application.WorkflowHookStatus{Project: root, HooksPath: filepath.Join(root, ".codex/hooks.json")}
	for _, path := range []string{s.HooksPath, filepath.Join(root, workflowReceipt), filepath.Join(root, ".arclint/agent-assets.json")} {
		if err := agentfiles.CheckPath(root, path); err != nil {
			s.Problems = append(s.Problems, err.Error())
			return s, nil
		}
	}
	data, err := os.ReadFile(filepath.Join(root, workflowReceipt))
	if os.IsNotExist(err) {
		s.Problems = []string{"Workflow hooks are not installed."}
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("read workflow installation: %w", err)
	}
	var meta installation
	if err := json.Unmarshal(data, &meta); err != nil {
		return s, fmt.Errorf("invalid workflow installation receipt: %w", err)
	}
	s.Installed = true
	s.Command = meta.Command
	s.Version = meta.Version
	var receipt map[string]string
	data, err = os.ReadFile(filepath.Join(root, ".arclint/agent-assets.json"))
	if err != nil {
		s.Problems = append(s.Problems, "Asset receipt is unavailable.")
	} else if json.Unmarshal(data, &receipt) != nil {
		s.Problems = append(s.Problems, "Asset receipt is invalid.")
	}
	for _, name := range []string{workflowDirectory + "/arclint", workflowDirectory + "/instructions.md", workflowDirectory + "/start-codex.sh", workflowReceipt} {
		path := filepath.Join(root, name)
		if err := agentfiles.CheckPath(root, path); err != nil {
			s.Problems = append(s.Problems, err.Error())
			continue
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil || receipt[name] == "" || receipt[name] != hash(content) {
			s.Problems = append(s.Problems, "Missing or changed asset: "+name)
		}
	}
	if info, err := os.Stat(filepath.Join(root, workflowDirectory, "arclint")); err == nil && info.Mode()&0o100 == 0 {
		s.Problems = append(s.Problems, "Installed workflow command is not executable.")
	}
	var document struct {
		Hooks map[string][]json.RawMessage `json:"hooks"`
	}
	data, err = os.ReadFile(s.HooksPath)
	if err != nil || json.Unmarshal(data, &document) != nil {
		s.Problems = append(s.Problems, "Hook registration is unavailable or invalid.")
	} else {
		expected, marshalErr := json.Marshal(map[string]any{hooksField: []any{meta.Handler}})
		if marshalErr != nil {
			return s, fmt.Errorf("encode expected workflow registration: %w", marshalErr)
		}
		for _, event := range workflowEvents {
			found := false
			for _, raw := range document.Hooks[event] {
				var v any
				if json.Unmarshal(raw, &v) == nil {
					canonical, _ := json.Marshal(v)
					found = found || bytes.Equal(canonical, expected)
				}
			}
			if !found {
				s.Problems = append(s.Problems, "Missing or changed workflow hook: "+event)
			}
		}
	}
	s.Intact = len(s.Problems) == 0
	return s, nil
}
