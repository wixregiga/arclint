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
	"github.com/wixregiga/arclint/internal/domain/rule"
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
	// disable is the setting that turns off every hook the host reads,
	// empty when the host has none.
	disable string
}

var hookHosts = []hookHost{
	{name: claudeHost, tools: "Edit|Write|MultiEdit|NotebookEdit", homeVariable: "CLAUDE_CONFIG_DIR", homeDirectory: ".claude", user: "settings.json", project: ".claude/settings.local.json", personal: true, shared: ".claude/settings.json", disable: "disableAllHooks"},
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
	hookEvents = []string{sessionStartEvent, promptEvent, postToolUseEvent, stopEvent}
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

// userTargets are the user configuration files of host that can be
// located.
func userTargets(host hookHost, configurations []UserConfiguration) []target {
	targets := make([]target, 0, len(configurations))
	for _, configuration := range configurations {
		if configuration.Unreadable != "" {
			continue
		}
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
	for _, configuration := range configurations {
		if configuration.Unreadable != "" {
			return installation, fmt.Errorf("install workflow hooks: the Windows profile's configuration cannot be located: %s", configuration.Unreadable)
		}
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
	running, disabled := false, false
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
		shared := ""
		if host.shared != "" {
			shared = filepath.Join(installer.root, filepath.FromSlash(host.shared))
		}
		// A reader of this project runs no hook while a file it reads turns
		// every hook off: the distribution's host reads its user file, this
		// project's file and the shared one; a Windows host app reads its
		// user file, the shared one, and this project's file when it was
		// written for both sides.
		sharedOff, localOff := disablesHooks(host, shared), disablesHooks(host, local.path)
		off := map[commandForm]bool{nativeForm: sharedOff || localOff, windowsForm: sharedOff || (localOff && local.form == sharedForm)}
		for _, user := range users {
			off[user.form] = off[user.form] || disablesHooks(host, user.path)
		}
		disabled = disabled || off[nativeForm] || off[windowsForm]
		// A Windows profile that cannot be located may turn its hooks off,
		// so its host apps are not said to run this project's hooks.
		windowsUnknown := slices.ContainsFunc(configurations, func(configuration UserConfiguration) bool {
			return configuration.Windows && configuration.Unreadable != ""
		})
		// This project's file runs while any of its readers runs hooks.
		readersOff := func(file target) bool {
			if file.scope == scopeUser {
				return off[file.form]
			}
			return off[nativeForm] && (local.form != sharedForm || off[windowsForm] || windowsUnknown)
		}
		entries := make([]application.HookFileStatus, 0, len(users)+1)
		for _, file := range append(slices.Clone(users), local) {
			entry, runs := installer.inspect(file, readersOff(file))
			if disablesHooks(host, file.path) {
				entry.Problems = append(entry.Problems, "This file sets "+host.disable+", so the host runs none of the hooks it reads where this file applies; remove the setting to run them.")
			}
			entries = append(entries, entry)
			running = running || runs
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
				localEntry.Problems = append(localEntry.Problems, duplicateProblem(user.path+" already lists the hooks for every project; with both, the host", off[user.form]))
			}
			if entry.Installation == application.HooksAbsent && (localEntry.Installation != application.HooksComplete || !sees) {
				entry.Problems = append(entry.Problems, "Workflow hooks are not installed.")
			}
		}
		if shared != "" {
			if disablesHooks(host, shared) {
				localEntry.Problems = append(localEntry.Problems, host.shared+" sets "+host.disable+", so the host runs no hooks in this project; remove the setting to run them.")
			}
			if listed, err := listsArclintHooks(shared); err == nil && listed {
				listing := host.shared + " also lists the workflow hooks; every file that lists them runs each event."
				if readersOff(local) {
					listing = host.shared + " also lists the workflow hooks; once hooks are turned back on, every file that lists them runs each event."
				}
				localEntry.Problems = append(localEntry.Problems, listing)
			}
		}
		// A user file that cannot be located is listed with the other user
		// files, before this project's.
		status.Files = append(status.Files, entries[:len(users)]...)
		status.Files = append(status.Files, unreadableFiles(host, configurations)...)
		status.Files = append(status.Files, entries[len(users)])
	}
	status.LastEvent, err = lastEvent(installer.root)
	if err != nil {
		return status, fmt.Errorf("workflow hook status: %w", err)
	}
	if status.LastEvent.IsZero() {
		status.Silence = installer.silence(status.Files, running, disabled)
	}
	return status, nil
}

// unreadableFiles reports host's file in each user configuration that
// could not be located; the problem belongs to that file alone. The path is
// where the host keeps the file unless its variable moves it.
func unreadableFiles(host hookHost, configurations []UserConfiguration) []application.HookFileStatus {
	var files []application.HookFileStatus
	for _, configuration := range configurations {
		if configuration.Unreadable == "" {
			continue
		}
		files = append(files, application.HookFileStatus{
			Host:         host.name,
			Scope:        scopeUser,
			Windows:      configuration.Windows,
			Path:         `%USERPROFILE%\` + host.homeDirectory + `\` + host.user,
			Installation: application.HooksUnknown,
			Problems:     []string{"The Windows profile's configuration cannot be located, so whether it lists the hooks is unknown: " + configuration.Unreadable + ". Windows host apps that open this project read this file; install from a shell that can run cmd.exe to write it."},
		})
	}
	return files
}

// silence says why no hook event has reached the project, in the order a
// user can rule the causes out. running reports a file with a hook that
// runs, and disabled a file that turns off every hook of its host.
func (installer *Installer) silence(files []application.HookFileStatus, running, disabled bool) []string {
	var reasons []string
	installed := func(installation application.HookInstallation) bool {
		return slices.ContainsFunc(files, func(file application.HookFileStatus) bool { return file.Installation == installation })
	}
	switch {
	case installed(application.HooksComplete):
	case running:
		reasons = append(reasons, "No configuration file lists every workflow hook, but the hooks install wrote into a file still run, and record an event each time one fires while rules.arclint.yaml exists; each file below names the hooks it lists, those that still run, and those it lacks. Run `arclint agents hooks install` to add the missing ones.")
	case installed(application.HooksPartial):
		reasons = append(reasons, "No configuration file lists every workflow hook, and no hook a file lists is established to run here; each file below says why. Run `arclint agents hooks install`.")
	default:
		reasons = append(reasons, "No configuration file lists the workflow hooks, so no host runs them here; run `arclint agents hooks install`.")
	}
	if disabled {
		reasons = append(reasons, "A configuration file turns off every hook its host reads; the file below that sets it names the setting.")
	}
	ruleset := filepath.Join(installer.root, rule.RulesetFileName)
	if info, err := os.Stat(ruleset); err != nil {
		reasons = append(reasons, "This directory holds no "+rule.RulesetFileName+", so the hooks answer nothing here and record no event. Run status in the project's root.")
	} else {
		reasons = append(reasons, "The hooks answer only where "+rule.RulesetFileName+" exists when an event arrives; it was last written at "+info.ModTime().Format("2006-01-02 15:04:05 -0700")+". A session that started before then, such as one in a worktree made from an older commit, records its first event here at its next prompt, edit or turn end.")
	}
	return append(reasons,
		"A session reports to the project that holds its working directory: one started in a parent directory, or in another worktree of this repository, reports there or nowhere.",
		"Claude Code runs the hooks only in sessions that started after they were installed.",
	)
}

// inspect reports what a target lists against what install writes there,
// and whether a hook it lists runs: one install wrote for this file runs
// the running arclint, while a differing one may name a binary that no
// longer exists or a matcher that selects nothing. While off, a setting
// turns off every hook of the host, and none runs.
func (installer *Installer) inspect(file target, off bool) (application.HookFileStatus, bool) {
	entry := application.HookFileStatus{Host: file.host.name, Scope: file.scope, Windows: file.form == windowsForm, Path: file.path, Installation: application.HooksUnknown}
	document, _, err := readConfiguration(file.path)
	if err != nil {
		entry.Problems = append(entry.Problems, "Configuration is unreadable: "+err.Error())
		return entry, false
	}
	hooks, _ := document["hooks"].(map[string]any)
	want := installer.groups(file)
	// A hook install wrote runs the arclint running now, unless it was
	// removed since.
	runs := !off && binaryProblem(installer.binary) == ""
	var running, differing []string
	binaries := map[string]bool{}
	for _, event := range hookEvents {
		listed, _ := hooks[event].([]any)
		for _, binary := range arclintBinaries(listed) {
			binaries[binary] = true
		}
		switch {
		case listsGroup(listed, want[event]):
			entry.Listed = append(entry.Listed, event)
			if runs {
				running = append(running, event)
			}
		case slices.ContainsFunc(listed, holdsArclintHandler):
			entry.Listed = append(entry.Listed, event)
			differing = append(differing, event)
		default:
			entry.Missing = append(entry.Missing, event)
		}
	}
	switch {
	case len(entry.Listed) == 0:
		entry.Installation = application.HooksAbsent
		return entry, false
	case len(entry.Missing) == 0:
		entry.Installation = application.HooksComplete
	default:
		entry.Installation = application.HooksPartial
	}
	for _, event := range differing {
		entry.Problems = append(entry.Problems, "The "+event+" hook differs from the one install writes here; installing again replaces it.")
	}
	if entry.Installation == application.HooksPartial {
		entry.Problems = append(entry.Problems, partialProblem(entry.Listed, running, entry.Missing))
	}
	for _, binary := range slices.Sorted(maps.Keys(binaries)) {
		if problem := binaryProblem(binary); problem != "" {
			entry.Problems = append(entry.Problems, problem)
		}
	}
	return entry, len(running) > 0
}

// partialProblem names the hooks a file lists, those of them that still
// run because install wrote them as they are, and the hooks it lacks.
func partialProblem(listed, running, missing []string) string {
	sentence := englishList(listed) + " " + plural(listed, "is", "are") + " installed"
	switch {
	case len(running) == len(listed):
		sentence += " and still " + plural(running, "runs", "run")
	case len(running) > 0:
		sentence += ", and " + englishList(running) + " still " + plural(running, "runs", "run")
	}
	return sentence + "; " + englishList(missing) + " " + plural(missing, "is", "are") + " missing. Run `arclint agents hooks install` to add " + plural(missing, "it", "them") + "."
}

// plural chooses the word that agrees with the number of names.
func plural(names []string, one, many string) string {
	if len(names) == 1 {
		return one
	}
	return many
}

// englishList joins names as a sentence lists them: "A", "A and B",
// "A, B and C".
func englishList(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// duplicateProblem completes a sentence about hooks two files list: the
// host runs each event twice, or would once the setting that turns its
// hooks off is removed.
func duplicateProblem(subject string, off bool) string {
	if off {
		return subject + " would run each event twice here once its hooks are turned back on, and ArcLint would answer the first."
	}
	return subject + " runs each event twice here and ArcLint answers the first."
}

// disablesHooks reports whether the configuration file at path sets the
// host's setting that turns off every hook it reads.
func disablesHooks(host hookHost, path string) bool {
	if host.disable == "" || path == "" {
		return false
	}
	document, _, err := readConfiguration(path)
	if err != nil {
		return false
	}
	off, _ := document[host.disable].(bool)
	return off
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
