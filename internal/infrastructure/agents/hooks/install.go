package hooks

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
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/wixregiga/arclint/internal/application"
)

// eventArgs are the arguments every workflow hook passes to arclint.
const eventArgs = "agents hooks event"

// handlerCommand is the hook handler field holding the shell command.
const handlerCommand = "command"

const (
	scopeUser    = "user"
	scopeProject = "project"
	claudeHost   = "claude"
)

// hookHost is one Agent Host's hook configuration.
type hookHost struct {
	name string
	// tools selects the PostToolUse calls that change files. ArcLint's own
	// commands record context, the check and domain changes themselves.
	tools string
	// homeVariable moves the host's user configuration directory, by
	// default homeDirectory under the user's home.
	homeVariable, homeDirectory string
	// user is the hooks file in the user configuration directory, and
	// project the file a project install writes.
	user, project string
	// personal marks a project file that belongs to one user, which a user
	// install may clean. Codex has no such file: its project hooks file can
	// be committed and shared.
	personal bool
	// shared is a project file the host also reads hooks from; install
	// never writes it, because it is meant to be committed.
	shared string
}

var hookHosts = []hookHost{
	{name: claudeHost, tools: "Edit|Write|MultiEdit|NotebookEdit", homeVariable: "CLAUDE_CONFIG_DIR", homeDirectory: ".claude", user: "settings.json", project: ".claude/settings.local.json", personal: true, shared: ".claude/settings.json"},
	{name: "codex", tools: "apply_patch", homeVariable: "CODEX_HOME", homeDirectory: ".codex", user: "hooks.json", project: ".codex/hooks.json"},
}

// commandForm is how the reader of a configuration file reaches arclint.
type commandForm int

const (
	// nativeForm runs arclint by its path in the environment it was
	// installed from.
	nativeForm commandForm = iota
	// windowsForm runs arclint inside the WSL distribution from a Windows
	// host app.
	windowsForm
	// sharedForm serves a project file that both the distribution's and
	// the Windows host apps read.
	sharedForm
)

// target is one configuration file an install writes.
type target struct {
	host  hookHost
	scope string
	form  commandForm
	path  string
}

var (
	hookEvents = []string{sessionStartEvent, postToolUseEvent, stopEvent}
	distroName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	// pinnable is a binary path every host shell runs unquoted.
	pinnable = regexp.MustCompile(`^(/|[A-Za-z]:/)[A-Za-z0-9._/+-]+$`)
	// pinned finds the binary a workflow hook command runs.
	pinned = regexp.MustCompile(`(\S+) ` + regexp.QuoteMeta(eventArgs))
	// wslDistribution finds the distribution a Windows workflow command
	// enters.
	wslDistribution = regexp.MustCompile(`wsl\.exe -d (\S+) -e `)
	errNotObject    = errors.New("expected a JSON object")
)

// HookHosts names the Agent Hosts the installer supports.
func HookHosts() []string {
	names := make([]string, len(hookHosts))
	for index, host := range hookHosts {
		names[index] = host.name
	}
	return names
}

// Installer writes the workflow hooks into each host's configuration,
// keeping every other setting and hook.
type Installer struct {
	root, distro, binary string
	users                func() ([]UserConfiguration, error)
}

// NewInstaller installs for the project at root. binary is the absolute
// path of the arclint the hooks run. distro names the WSL distribution this
// process runs in, empty outside WSL; Windows host apps then reach binary
// through wsl.exe. users locates the user configuration of each
// environment an install writes.
func NewInstaller(root, distro, binary string, users func() ([]UserConfiguration, error)) *Installer {
	return &Installer{root: root, distro: distro, binary: binary, users: users}
}

func (installer *Installer) validate() error {
	if installer.distro != "" && !distroName.MatchString(installer.distro) {
		return fmt.Errorf("unsupported WSL distribution name %q", installer.distro)
	}
	if installer.binary == "" {
		return errors.New("the running arclint could not be located; run it by its path")
	}
	if !pinnable.MatchString(filepath.ToSlash(installer.binary)) {
		return fmt.Errorf("the hooks run arclint by its path, which may hold only letters, digits and ._/+-: %s", installer.binary)
	}
	return nil
}

// handler is the hook definition the reader of a target runs.
func (installer *Installer) handler(file target) map[string]any {
	binary := filepath.ToSlash(installer.binary)
	handler := map[string]any{"type": "command", handlerCommand: nativeCommand(binary), "timeout": 30}
	switch {
	case file.form == windowsForm && file.host.name == claudeHost:
		handler[handlerCommand] = gitBashCommand(binary, installer.distro)
	case file.form == windowsForm:
		handler[handlerCommand] = windowsCommand(binary, installer.distro)
		handler["commandWindows"] = windowsCommand(binary, installer.distro)
	case file.form == sharedForm && file.host.name == claudeHost:
		handler[handlerCommand] = sharedCommand(binary, installer.distro)
	case file.form == sharedForm:
		handler["commandWindows"] = windowsCommand(binary, installer.distro)
	}
	return handler
}

// nativeCommand runs binary in the environment it was installed from.
func nativeCommand(binary string) string {
	return binary + " " + eventArgs
}

// windowsCommand runs binary inside the distribution from a Windows host app.
func windowsCommand(binary, distro string) string {
	return "wsl.exe -d " + distro + " -e " + nativeCommand(binary)
}

// gitBashCommand is windowsCommand for Claude Code, which runs a command
// through Git Bash on Windows; Git Bash would rewrite the distribution path
// passed to wsl.exe into a Windows path.
func gitBashCommand(binary, distro string) string {
	return "MSYS_NO_PATHCONV=1 " + windowsCommand(binary, distro)
}

// sharedCommand runs binary where it exists, inside the distribution, and
// reaches it through wsl.exe from Git Bash on Windows.
func sharedCommand(binary, distro string) string {
	return "if [ -x " + binary + " ]; then exec " + nativeCommand(binary) + "; else MSYS_NO_PATHCONV=1 exec " + windowsCommand(binary, distro) + "; fi"
}

// groups is the hook groups a target lists for each event.
func (installer *Installer) groups(file target) map[string]any {
	handler := installer.handler(file)
	groups := map[string]any{}
	for _, event := range hookEvents {
		group := map[string]any{"hooks": []any{handler}}
		if event == postToolUseEvent {
			group["matcher"] = file.host.tools
		}
		groups[event] = group
	}
	return groups
}

func (installer *Installer) projectTarget(host hookHost) target {
	form := nativeForm
	if installer.distro != "" {
		form = sharedForm
	}
	return target{host: host, scope: scopeProject, form: form, path: filepath.Join(installer.root, filepath.FromSlash(host.project))}
}

func userTargets(host hookHost, configurations []UserConfiguration) []target {
	targets := make([]target, 0, len(configurations))
	for _, configuration := range configurations {
		form := nativeForm
		if configuration.Windows {
			form = windowsForm
		}
		targets = append(targets, target{host: host, scope: scopeUser, form: form, path: filepath.Join(configuration.Directories[host.name], host.user)})
	}
	return targets
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

// Install writes the workflow hooks for the named hosts, or every supported
// host when none is named. By default it writes each host's user
// configuration, so the hooks run in every project and worktree, and
// removes them from this project's file when that file is personal, where
// they would run each event twice; a shared project file stays as it is,
// and status reports the duplicate. With project it writes this project's
// file only, and refuses while a user configuration already lists them.
func (installer *Installer) Install(hosts []string, project bool) (installation application.HooksInstallation, err error) {
	if err := installer.validate(); err != nil {
		return installation, fmt.Errorf("install workflow hooks: %w", err)
	}
	selected, err := selectHosts(hosts)
	if err != nil {
		return installation, err
	}
	configurations, err := installer.users()
	if err != nil {
		return installation, fmt.Errorf("install workflow hooks: %w", err)
	}
	for _, host := range selected {
		users := userTargets(host, configurations)
		local := installer.projectTarget(host)
		files := users
		if project {
			for _, user := range users {
				listed, err := listsArclintHooks(user.path)
				if err != nil {
					return installation, fmt.Errorf("install workflow hooks: %s: %w", user.path, err)
				}
				if listed {
					return installation, fmt.Errorf("install workflow hooks: %s already runs them in every project; a project install would run each event twice, so remove them there first", user.path)
				}
			}
			files = []target{local}
		}
		for _, file := range files {
			if err := installer.write(file); err != nil {
				return installation, fmt.Errorf("install workflow hooks: %s: %w", file.path, err)
			}
			installation.Written = append(installation.Written, file.path)
		}
		if project || !host.personal {
			continue
		}
		removed, err := removeArclintHooks(local.path)
		if err != nil {
			return installation, fmt.Errorf("install workflow hooks: %s: %w", local.path, err)
		}
		if removed {
			installation.Removed = append(installation.Removed, local.path)
		}
	}
	return installation, nil
}

// write places the workflow hook groups in a target and writes it when
// they changed it.
func (installer *Installer) write(file target) error {
	document, mode, err := readConfiguration(file.path)
	if err != nil {
		return err
	}
	before, err := encodeConfiguration(document)
	if err != nil {
		return err
	}
	if err := placeGroups(document, installer.groups(file)); err != nil {
		return err
	}
	after, err := encodeConfiguration(document)
	if err != nil {
		return err
	}
	if bytes.Equal(before, after) {
		return nil
	}
	return writeConfiguration(file.path, after, mode)
}

// Status reports every configuration file that can hold the workflow hooks
// for this project: each user configuration and this project's personal
// file, with what is wrong in each, and when a hook event last reached the
// project.
func (installer *Installer) Status() (status application.HooksStatus, err error) {
	status = application.HooksStatus{Project: installer.root, Binary: installer.binary}
	if err := installer.validate(); err != nil {
		return status, fmt.Errorf("workflow hook status: %w", err)
	}
	configurations, err := installer.users()
	if err != nil {
		return status, fmt.Errorf("workflow hook status: %w", err)
	}
	for _, host := range hookHosts {
		users := userTargets(host, configurations)
		local := installer.projectTarget(host)
		entries := make([]application.HookFileStatus, 0, len(users)+1)
		for _, file := range append(slices.Clone(users), local) {
			entries = append(entries, installer.inspect(file))
		}
		localEntry := &entries[len(entries)-1]
		// A workflow hook either file lists runs beside the other's, even
		// when a file lists only some events.
		localListed, _ := listsArclintHooks(local.path)
		for index, user := range users {
			entry := &entries[index]
			// A Windows host app reads this project's file only when it was
			// written for both sides.
			sees := user.form == nativeForm || local.form == sharedForm
			userListed, _ := listsArclintHooks(user.path)
			if userListed && localListed && sees {
				localEntry.Problems = append(localEntry.Problems, user.path+" already runs the hooks in every project; with both, the host runs each event twice here and ArcLint answers the first.")
			}
			if !entry.Installed && len(entry.Problems) == 0 && (!localEntry.Installed || !sees) {
				entry.Problems = append(entry.Problems, "Workflow hooks are not installed.")
			}
		}
		if host.shared != "" {
			shared := filepath.Join(installer.root, filepath.FromSlash(host.shared))
			if listed, err := listsArclintHooks(shared); err == nil && listed {
				localEntry.Problems = append(localEntry.Problems, host.shared+" also lists the workflow hooks; every file that lists them runs each event.")
			}
		}
		status.Files = append(status.Files, entries...)
	}
	status.LastEvent, err = lastEvent(installer.root)
	if err != nil {
		return status, fmt.Errorf("workflow hook status: %w", err)
	}
	return status, nil
}

// inspect reports what a target lists against what install writes there.
func (installer *Installer) inspect(file target) application.HookFileStatus {
	entry := application.HookFileStatus{Host: file.host.name, Scope: file.scope, Windows: file.form == windowsForm, Path: file.path}
	document, _, err := readConfiguration(file.path)
	if err != nil {
		entry.Problems = append(entry.Problems, "Configuration is unreadable: "+err.Error())
		return entry
	}
	hooks, _ := document["hooks"].(map[string]any)
	want := installer.groups(file)
	var differing, missing []string
	binaries := map[string]bool{}
	for _, event := range hookEvents {
		listed, _ := hooks[event].([]any)
		for _, binary := range arclintBinaries(listed) {
			binaries[binary] = true
		}
		switch {
		case listsGroup(listed, want[event]):
		case slices.ContainsFunc(listed, holdsArclintHandler):
			differing = append(differing, event)
		default:
			missing = append(missing, event)
		}
	}
	if len(missing) == len(hookEvents) {
		return entry
	}
	entry.Installed = len(missing) == 0
	for _, event := range differing {
		entry.Problems = append(entry.Problems, "The "+event+" hook differs from the one install writes here; installing again replaces it.")
	}
	for _, event := range missing {
		entry.Problems = append(entry.Problems, "The "+event+" hook is missing.")
	}
	for _, binary := range slices.Sorted(maps.Keys(binaries)) {
		if problem := binaryProblem(binary); problem != "" {
			entry.Problems = append(entry.Problems, problem)
		}
	}
	return entry
}

// arclintBinaries names the binaries the workflow handlers in groups run.
func arclintBinaries(groups []any) []string {
	var binaries []string
	for _, group := range groups {
		object, _ := group.(map[string]any)
		handlers, _ := object["hooks"].([]any)
		for _, handler := range handlers {
			if !isArclintHandler(handler) {
				continue
			}
			fields, _ := handler.(map[string]any)
			for _, field := range []string{handlerCommand, "commandWindows"} {
				command, _ := fields[field].(string)
				for _, match := range pinned.FindAllStringSubmatch(command, -1) {
					binaries = append(binaries, match[1])
				}
			}
		}
	}
	return binaries
}

// binaryProblem says why a hook cannot run binary, empty when it can or
// when this environment cannot tell, as for a Windows path seen from WSL.
func binaryProblem(binary string) string {
	if strings.HasPrefix(binary, "/") == (runtime.GOOS == "windows") {
		return ""
	}
	info, err := os.Stat(binary)
	switch {
	case err != nil:
		return "The hooks run " + binary + ", which does not exist."
	case runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0:
		return "The hooks run " + binary + ", which is not executable."
	}
	return ""
}

// lastEvent is when a hook event last updated a session record of the
// project, zero when none has.
func lastEvent(root string) (time.Time, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(progressDirectory)))
	if errors.Is(err, fs.ErrNotExist) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("read the hook sessions: %w", err)
	}
	var last time.Time
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(last) {
			last = info.ModTime()
		}
	}
	return last, nil
}

// listsArclintHooks reports whether a configuration file lists a workflow
// handler for any event.
func listsArclintHooks(path string) (bool, error) {
	document, _, err := readConfiguration(path)
	if err != nil {
		return false, err
	}
	hooks, _ := document["hooks"].(map[string]any)
	for _, value := range hooks {
		groups, _ := value.([]any)
		if slices.ContainsFunc(groups, holdsArclintHandler) {
			return true, nil
		}
	}
	return false, nil
}

// removeArclintHooks removes every workflow handler from a configuration
// file and reports whether it held one. Groups and events left without a
// handler go with it; every other setting and hook stays.
func removeArclintHooks(path string) (bool, error) {
	document, mode, err := readConfiguration(path)
	if err != nil {
		return false, err
	}
	hooks, _ := document["hooks"].(map[string]any)
	removed := false
	for event, value := range hooks {
		groups, ok := value.([]any)
		if !ok {
			continue
		}
		kept := make([]any, 0, len(groups))
		for _, group := range groups {
			if !holdsArclintHandler(group) {
				kept = append(kept, group)
				continue
			}
			removed = true
			if others := withoutArclintHandlers(group); others != nil {
				kept = append(kept, others)
			}
		}
		if len(kept) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = kept
		}
	}
	if !removed {
		return false, nil
	}
	if len(hooks) == 0 {
		delete(document, "hooks")
	}
	content, err := encodeConfiguration(document)
	if err != nil {
		return false, err
	}
	return true, writeConfiguration(path, content, mode)
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
			if !holdsArclintHandler(group) {
				kept = append(kept, group)
				continue
			}
			if others := withoutArclintHandlers(group); others != nil {
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

// isArclintHandler reports whether handler runs a command install writes,
// for any pinned binary and distribution. A command that only mentions the
// same words is another hook, and install keeps it.
func isArclintHandler(handler any) bool {
	object, _ := handler.(map[string]any)
	command, _ := object[handlerCommand].(string)
	binary := pinned.FindStringSubmatch(command)
	if binary == nil || !pinnable.MatchString(binary[1]) {
		return false
	}
	if command == nativeCommand(binary[1]) {
		return true
	}
	distro := wslDistribution.FindStringSubmatch(command)
	if distro == nil || !distroName.MatchString(distro[1]) {
		return false
	}
	return slices.Contains([]string{windowsCommand(binary[1], distro[1]), gitBashCommand(binary[1], distro[1]), sharedCommand(binary[1], distro[1])}, command)
}

func holdsArclintHandler(group any) bool {
	object, _ := group.(map[string]any)
	handlers, _ := object["hooks"].([]any)
	return slices.ContainsFunc(handlers, isArclintHandler)
}

// withoutArclintHandlers returns group without its workflow handlers, or
// nil when no other handler remains.
func withoutArclintHandlers(group any) any {
	object, _ := group.(map[string]any)
	handlers, _ := object["hooks"].([]any)
	others := slices.DeleteFunc(slices.Clone(handlers), isArclintHandler)
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
func readConfiguration(path string) (map[string]any, fs.FileMode, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]any{}, 0o644, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("read: %w", err)
	}
	info, err := os.Stat(path)
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

// writeConfiguration replaces a configuration file through a temporary
// file in its directory. A linked file is written where the link points,
// so a configuration kept elsewhere stays linked.
func writeConfiguration(path string, content []byte, mode fs.FileMode) (returnErr error) {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	directory, name := filepath.Split(path)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, root.Close()) }()
	temporary := name + ".tmp-" + rand.Text()
	if err := root.WriteFile(temporary, content, mode); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	if err := root.Rename(temporary, name); err != nil {
		return errors.Join(fmt.Errorf("write: %w", err), root.Remove(temporary))
	}
	return nil
}
