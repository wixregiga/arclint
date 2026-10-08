package workflow

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
)

// eventCommand is the hook command every host runs. It is the same in
// every checkout, so a host that trusts hooks by their content trusts the
// workflow hooks once for all worktrees.
const eventCommand = "arclint agents workflow event"

// handlerCommand is the hook handler field holding the shell command.
const handlerCommand = "command"

// hookHost is one Agent Host's project hook configuration.
type hookHost struct {
	name string
	path string
	// tools selects the PostToolUse calls that change files. ArcLint's own
	// commands record context, the check and domain changes themselves.
	tools string
}

var hookHosts = []hookHost{
	{name: "claude", path: ".claude/settings.json", tools: "Edit|Write|MultiEdit|NotebookEdit"},
	{name: "codex", path: ".codex/hooks.json", tools: "apply_patch"},
}

var (
	hookEvents   = []string{sessionStartEvent, postToolUseEvent, stopEvent}
	distroName   = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	errNotObject = errors.New("expected a JSON object")
)

// HookHosts names the Agent Hosts the installer supports.
func HookHosts() []string {
	names := make([]string, len(hookHosts))
	for index, host := range hookHosts {
		names[index] = host.name
	}
	return names
}

// Installer writes the workflow hooks into each host's project
// configuration, keeping every other setting and hook.
type Installer struct{ root, distro string }

// NewInstaller installs into the project at root. distro names the WSL
// distribution this process runs in, empty outside WSL; a Windows host
// then reaches arclint inside that distribution through wsl.exe.
func NewInstaller(root, distro string) *Installer {
	return &Installer{root: root, distro: distro}
}

// handler is the hook definition one host runs.
func (installer *Installer) handler(host string) (map[string]any, error) {
	handler := map[string]any{"type": "command", handlerCommand: eventCommand, "timeout": 30}
	if installer.distro == "" {
		return handler, nil
	}
	if !distroName.MatchString(installer.distro) {
		return nil, fmt.Errorf("unsupported WSL distribution name %q", installer.distro)
	}
	windows := "wsl.exe -d " + installer.distro + ` -- bash -lc "exec ` + eventCommand + `"`
	switch host {
	case "codex":
		handler["commandWindows"] = windows
	case "claude":
		// Claude Code runs a command through sh, or Git Bash on Windows,
		// where arclint lives only inside the distribution.
		handler[handlerCommand] = "if command -v arclint >/dev/null 2>&1; then exec " + eventCommand + "; else exec " + windows + "; fi"
	}
	return handler, nil
}

// groups is the hook groups one host lists for each event.
func (installer *Installer) groups(host hookHost) (map[string]any, error) {
	handler, err := installer.handler(host.name)
	if err != nil {
		return nil, err
	}
	groups := map[string]any{}
	for _, event := range hookEvents {
		group := map[string]any{"hooks": []any{handler}}
		if event == postToolUseEvent {
			group["matcher"] = host.tools
		}
		groups[event] = group
	}
	return groups, nil
}

func selectHosts(names []string) ([]hookHost, error) {
	if len(names) == 0 {
		return hookHosts, nil
	}
	var selected []hookHost
	for _, name := range names {
		found := false
		for _, host := range hookHosts {
			if host.name == name {
				selected = append(selected, host)
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("unsupported agent host %q; use %s", name, strings.Join(HookHosts(), " or "))
		}
	}
	return selected, nil
}

// Install writes the workflow hooks for the named hosts, or every
// supported host when none is named, and returns the configuration paths.
// It replaces an earlier workflow hook group and keeps all other content.
func (installer *Installer) Install(hosts []string) (paths []string, returnErr error) {
	selected, err := selectHosts(hosts)
	if err != nil {
		return nil, err
	}
	project, err := os.OpenRoot(installer.root)
	if err != nil {
		return nil, fmt.Errorf("install workflow hooks: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, project.Close()) }()
	for _, host := range selected {
		document, mode, err := readConfiguration(project, host.path)
		if err != nil {
			return nil, fmt.Errorf("install workflow hooks: %s: %w", host.path, err)
		}
		before, err := encodeConfiguration(document)
		if err != nil {
			return nil, err
		}
		groups, err := installer.groups(host)
		if err != nil {
			return nil, fmt.Errorf("install workflow hooks: %w", err)
		}
		if err := placeGroups(document, groups); err != nil {
			return nil, fmt.Errorf("install workflow hooks: %s: %w", host.path, err)
		}
		after, err := encodeConfiguration(document)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(before, after) {
			if err := writeConfiguration(project, host.path, after, mode); err != nil {
				return nil, fmt.Errorf("install workflow hooks: %s: %w", host.path, err)
			}
		}
		paths = append(paths, filepath.Join(installer.root, host.path))
	}
	return paths, nil
}

// Status reports, for every supported host, whether its configuration
// lists the workflow hooks. A hook written from another environment, such
// as outside WSL, still counts as installed; the difference is reported,
// because installing from here would rewrite it.
func (installer *Installer) Status() (status application.WorkflowHookStatus, returnErr error) {
	status = application.WorkflowHookStatus{Project: installer.root, Command: eventCommand}
	project, err := os.OpenRoot(installer.root)
	if err != nil {
		return status, fmt.Errorf("workflow hook status: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, project.Close()) }()
	for _, host := range hookHosts {
		hostStatus := application.WorkflowHostStatus{Host: host.name, Path: filepath.Join(installer.root, host.path)}
		groups, err := installer.groups(host)
		if err != nil {
			return status, fmt.Errorf("workflow hook status: %w", err)
		}
		document, _, err := readConfiguration(project, host.path)
		hooks, _ := document["hooks"].(map[string]any)
		switch {
		case err != nil:
			hostStatus.Problems = append(hostStatus.Problems, "Configuration is unreadable: "+err.Error())
		case hooks == nil:
			hostStatus.Problems = append(hostStatus.Problems, "Workflow hooks are not installed.")
		default:
			hostStatus.Installed = true
			for _, event := range hookEvents {
				listed, _ := hooks[event].([]any)
				switch {
				case listsGroup(listed, groups[event]):
				case slices.ContainsFunc(listed, holdsWorkflowHandler):
					hostStatus.Problems = append(hostStatus.Problems, "The "+event+" hook differs from the one install writes here; installing again replaces it.")
				default:
					hostStatus.Installed = false
					hostStatus.Problems = append(hostStatus.Problems, "The "+event+" hook is missing.")
				}
			}
		}
		status.Hosts = append(status.Hosts, hostStatus)
	}
	return status, nil
}

// placeGroups puts each event's workflow group where an earlier workflow
// handler stood, or after the event's other groups. A group that also
// holds other handlers keeps them and loses only the workflow handler.
func placeGroups(document map[string]any, groups map[string]any) error {
	if document["hooks"] == nil {
		document["hooks"] = map[string]any{}
	}
	hooks, ok := document["hooks"].(map[string]any)
	if !ok {
		return fmt.Errorf("hooks: %w", errNotObject)
	}
	for _, event := range hookEvents {
		existing, ok := hooks[event].([]any)
		if hooks[event] != nil && !ok {
			return fmt.Errorf("hooks.%s: expected an array", event)
		}
		placed := false
		kept := make([]any, 0, len(existing)+1)
		for _, group := range existing {
			if !holdsWorkflowHandler(group) {
				kept = append(kept, group)
				continue
			}
			if others := withoutWorkflowHandlers(group); others != nil {
				kept = append(kept, others)
			}
			if !placed {
				kept = append(kept, groups[event])
				placed = true
			}
		}
		if !placed {
			kept = append(kept, groups[event])
		}
		hooks[event] = kept
	}
	return nil
}

func isWorkflowHandler(handler any) bool {
	object, _ := handler.(map[string]any)
	command, _ := object[handlerCommand].(string)
	return strings.Contains(command, eventCommand)
}

func holdsWorkflowHandler(group any) bool {
	object, _ := group.(map[string]any)
	handlers, _ := object["hooks"].([]any)
	return slices.ContainsFunc(handlers, isWorkflowHandler)
}

// withoutWorkflowHandlers returns group without its workflow handlers, or
// nil when no other handler remains.
func withoutWorkflowHandlers(group any) any {
	object, _ := group.(map[string]any)
	handlers, _ := object["hooks"].([]any)
	others := slices.DeleteFunc(slices.Clone(handlers), isWorkflowHandler)
	if len(others) == 0 {
		return nil
	}
	copied := maps.Clone(object)
	copied["hooks"] = others
	return copied
}

func listsGroup(listed []any, want any) bool {
	wanted, err := json.Marshal(want)
	if err != nil {
		return false
	}
	for _, group := range listed {
		if encoded, err := json.Marshal(group); err == nil && bytes.Equal(encoded, wanted) {
			return true
		}
	}
	return false
}

// readConfiguration reads a host's JSON configuration, an empty object
// when the file does not exist yet, and its permissions.
func readConfiguration(project *os.Root, name string) (map[string]any, fs.FileMode, error) {
	data, err := project.ReadFile(name)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]any{}, 0o644, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("read: %w", err)
	}
	info, err := project.Stat(name)
	if err != nil {
		return nil, 0, fmt.Errorf("read: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var document map[string]any
	if err := decoder.Decode(&document); err != nil {
		return nil, 0, fmt.Errorf("invalid JSON: %w", err)
	}
	if document == nil {
		return nil, 0, errNotObject
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, 0, fmt.Errorf("invalid JSON: content follows the configuration object")
	}
	return document, info.Mode().Perm(), nil
}

func encodeConfiguration(document map[string]any) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		return nil, fmt.Errorf("encode hook configuration: %w", err)
	}
	return output.Bytes(), nil
}

func writeConfiguration(project *os.Root, name string, content []byte, mode fs.FileMode) error {
	if err := project.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	temporary := name + ".tmp-" + rand.Text()
	if err := project.WriteFile(temporary, content, mode); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	if err := project.Rename(temporary, name); err != nil {
		return errors.Join(fmt.Errorf("write: %w", err), project.Remove(temporary))
	}
	return nil
}
