package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/application"
)

type configuration struct {
	Permissions map[string]any `json:"permissions"`
	Hooks       map[string][]struct {
		Matcher string `json:"matcher"`
		Hooks   []struct {
			Type           string `json:"type"`
			Command        string `json:"command"`
			CommandWindows string `json:"commandWindows"`
			Timeout        int    `json:"timeout"`
		} `json:"hooks"`
	} `json:"hooks"`
}

func readTestConfiguration(t *testing.T, path string) configuration {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document configuration
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return document
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// testBinary is an executable that prints its arguments, standing in for
// the arclint the hooks run.
func testBinary(t *testing.T, directory, label string) string {
	t.Helper()
	path := filepath.Join(directory, "arclint")
	writeTestFile(t, path, "#!/bin/sh\necho "+label+` "$@"`+"\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// homes is the user configuration directories of one test: the
// distribution's and the Windows profile's.
type homes struct{ native, windows UserConfiguration }

func newHomes(t *testing.T) homes {
	t.Helper()
	base := t.TempDir()
	directories := func(side string) map[string]string {
		return map[string]string{"claude": filepath.Join(base, side, ".claude"), "codex": filepath.Join(base, side, ".codex")}
	}
	return homes{native: UserConfiguration{Directories: directories("linux")}, windows: UserConfiguration{Windows: true, Directories: directories("windows")}}
}

func (h homes) users(distro string) func() ([]UserConfiguration, error) {
	return func() ([]UserConfiguration, error) {
		if distro == "" {
			return []UserConfiguration{h.native}, nil
		}
		return []UserConfiguration{h.native, h.windows}, nil
	}
}

func (h homes) file(windows bool, host string) string {
	side := h.native
	if windows {
		side = h.windows
	}
	for _, candidate := range hookHosts {
		if candidate.name == host {
			return filepath.Join(side.Directories[host], candidate.user)
		}
	}
	panic(host)
}

func TestUserInstallWritesEachHostsUserConfiguration(t *testing.T) {
	root, h := t.TempDir(), newHomes(t)
	binary := testBinary(t, t.TempDir(), "native")
	installer := NewInstaller(root, "", binary, h.users(""))
	installation, err := installer.Install(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{h.file(false, "claude"), h.file(false, "codex")}
	if !reflect.DeepEqual(installation.Written, want) || len(installation.Removed) != 0 {
		t.Fatalf("installation %+v, want written %v", installation, want)
	}
	for _, host := range hookHosts {
		document := readTestConfiguration(t, h.file(false, host.name))
		if len(document.Hooks) != len(hookEvents) {
			t.Fatalf("%s events: %v", host.name, document.Hooks)
		}
		for _, event := range hookEvents {
			groups := document.Hooks[event]
			if len(groups) != 1 || len(groups[0].Hooks) != 1 || groups[0].Hooks[0].Command != binary+" "+eventArgs || groups[0].Hooks[0].CommandWindows != "" {
				t.Fatalf("%s %s: %+v", host.name, event, groups)
			}
			if wantMatcher := map[bool]string{true: host.tools}[event == postToolUseEvent]; groups[0].Matcher != wantMatcher {
				t.Fatalf("%s %s matcher %q, want %q", host.name, event, groups[0].Matcher, wantMatcher)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".claude")); !os.IsNotExist(err) {
		t.Fatalf("a user install wrote into the project: %v", err)
	}
	status, err := installer.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.Binary != binary || !status.LastEvent.IsZero() || len(status.Files) != 4 {
		t.Fatalf("status: %+v", status)
	}
	for _, entry := range status.Files {
		if want := map[bool]application.HookInstallation{true: application.HooksComplete, false: application.HooksAbsent}[entry.Scope == scopeUser]; entry.Installation != want || len(entry.Problems) != 0 {
			t.Fatalf("status entry after install: %+v", entry)
		}
	}
}

// Inside WSL the Windows host apps read the Windows profile. Claude Code
// runs the command through Git Bash there, which must not rewrite the
// distribution path, and Codex reads commandWindows.
func TestWSLUserInstallWritesTheWindowsProfileForWindowsApps(t *testing.T) {
	root, h := t.TempDir(), newHomes(t)
	binary := testBinary(t, t.TempDir(), "native")
	installation, err := NewInstaller(root, "Ubuntu", binary, h.users("Ubuntu")).Install(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(installation.Written) != 4 {
		t.Fatalf("written: %v", installation.Written)
	}
	windows := "wsl.exe -d Ubuntu -e " + binary + " " + eventArgs
	if native := readTestConfiguration(t, h.file(false, "claude")).Hooks[stopEvent][0].Hooks[0]; native.Command != binary+" "+eventArgs {
		t.Fatalf("distribution claude handler: %+v", native)
	}
	codex := readTestConfiguration(t, h.file(true, "codex")).Hooks[stopEvent][0].Hooks[0]
	if codex.Command != windows || codex.CommandWindows != windows {
		t.Fatalf("windows codex handler: %+v", codex)
	}
	claude := readTestConfiguration(t, h.file(true, "claude")).Hooks[stopEvent][0].Hooks[0].Command
	if claude != "MSYS_NO_PATHCONV=1 "+windows {
		t.Fatalf("windows claude command: %s", claude)
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is unavailable")
	}
	stubs := t.TempDir()
	writeTestFile(t, filepath.Join(stubs, "wsl.exe"), "#!/bin/sh\necho wsl.exe \"$MSYS_NO_PATHCONV\" \"$@\"\n")
	if err := os.Chmod(filepath.Join(stubs, "wsl.exe"), 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(shell, "-c", claude)
	command.Env = []string{"PATH=" + stubs + string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/bin"}
	output, err := command.CombinedOutput()
	if want := "wsl.exe 1 -d Ubuntu -e " + binary + " " + eventArgs; err != nil || strings.TrimSpace(string(output)) != want {
		t.Fatalf("windows claude command ran %q %v, want %q", output, err, want)
	}
}

func TestUserInstallRemovesTheProjectsDuplicateHooks(t *testing.T) {
	root, h := t.TempDir(), newHomes(t)
	local := filepath.Join(root, ".claude/settings.local.json")
	writeTestFile(t, local, `{"permissions":{"allow":["Bash(ls)"]},"hooks":{"Stop":[{"hooks":[{"type":"command","command":"notify"},{"type":"command","command":"/old/arclint agents hooks event"}]}],"SessionStart":[{"hooks":[{"type":"command","command":"/old/arclint agents hooks event"}]}]}}`)
	installation, err := NewInstaller(root, "", testBinary(t, t.TempDir(), "native"), h.users("")).Install([]string{"claude"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(installation.Removed, []string{local}) {
		t.Fatalf("removed %v", installation.Removed)
	}
	document := readTestConfiguration(t, local)
	if !reflect.DeepEqual(document.Permissions, map[string]any{"allow": []any{"Bash(ls)"}}) {
		t.Fatalf("permissions changed: %v", document.Permissions)
	}
	if len(document.Hooks) != 1 || len(document.Hooks[stopEvent]) != 1 || len(document.Hooks[stopEvent][0].Hooks) != 1 || document.Hooks[stopEvent][0].Hooks[0].Command != "notify" {
		t.Fatalf("project hooks after removal: %+v", document.Hooks)
	}
}

// Codex reads project hooks from a file a repository can commit, so a user
// install leaves it as it is and status names the duplicate.
func TestUserInstallLeavesTheSharedCodexProjectFile(t *testing.T) {
	root, h := t.TempDir(), newHomes(t)
	shared := filepath.Join(root, ".codex/hooks.json")
	content := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"notify"}]},{"hooks":[{"type":"command","command":"/team/bin/arclint ` + eventArgs + `"}]}]}}`
	writeTestFile(t, shared, content)
	installer := NewInstaller(root, "", testBinary(t, t.TempDir(), "native"), h.users(""))
	installation, err := installer.Install([]string{"codex"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(installation.Removed) != 0 {
		t.Fatalf("removed from the shared file: %v", installation.Removed)
	}
	if data, err := os.ReadFile(shared); err != nil || string(data) != content {
		t.Fatalf("the shared project file changed: %v\n%s", err, data)
	}
	if problems := strings.Join(statusOf(t, installer)["codex project"].Problems, "\n"); !strings.Contains(problems, h.file(false, "codex")+" already lists the hooks for every project") {
		t.Fatalf("duplicate not reported: %s", problems)
	}
}

func TestProjectInstallWritesThePersonalFilesForBothSides(t *testing.T) {
	root, h := t.TempDir(), newHomes(t)
	binary := testBinary(t, t.TempDir(), "native")
	installation, err := NewInstaller(root, "Ubuntu", binary, h.users("Ubuntu")).Install(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(root, ".claude/settings.local.json"), filepath.Join(root, ".codex/hooks.json")}
	if !reflect.DeepEqual(installation.Written, want) {
		t.Fatalf("written %v, want %v", installation.Written, want)
	}
	if _, err := os.Stat(h.file(false, "claude")); !os.IsNotExist(err) {
		t.Fatalf("a project install wrote the user configuration: %v", err)
	}
	codex := readTestConfiguration(t, want[1]).Hooks[stopEvent][0].Hooks[0]
	if codex.Command != binary+" "+eventArgs || codex.CommandWindows != "wsl.exe -d Ubuntu -e "+binary+" "+eventArgs {
		t.Fatalf("project codex handler: %+v", codex)
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is unavailable")
	}
	// The distribution runs the binary; Git Bash, which cannot see it,
	// reaches it through wsl.exe.
	stubs := t.TempDir()
	writeTestFile(t, filepath.Join(stubs, "wsl.exe"), "#!/bin/sh\necho wsl.exe \"$@\"\n")
	if err := os.Chmod(filepath.Join(stubs, "wsl.exe"), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(command string) string {
		t.Helper()
		shellCommand := exec.Command(shell, "-c", command)
		shellCommand.Env = []string{"PATH=" + stubs + string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/bin"}
		output, err := shellCommand.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v %s", command, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	claude := readTestConfiguration(t, want[0]).Hooks[stopEvent][0].Hooks[0].Command
	if got := run(claude); got != "native "+eventArgs {
		t.Fatalf("inside the distribution the command ran %q", got)
	}
	if err := os.Remove(binary); err != nil {
		t.Fatal(err)
	}
	if got := run(claude); got != "wsl.exe -d Ubuntu -e "+binary+" "+eventArgs {
		t.Fatalf("outside the distribution the command ran %q", got)
	}
}

func TestProjectInstallRefusesWhileTheUserConfigurationRunsTheHooks(t *testing.T) {
	root, h := t.TempDir(), newHomes(t)
	installer := NewInstaller(root, "", testBinary(t, t.TempDir(), "native"), h.users(""))
	if _, err := installer.Install([]string{"codex"}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := installer.Install([]string{"codex"}, true); err == nil || !strings.Contains(err.Error(), "would run each event twice") {
		t.Fatalf("project install beside a user install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".codex")); !os.IsNotExist(err) {
		t.Fatalf("the refused install wrote the project: %v", err)
	}
	if _, err := installer.Install([]string{"claude"}, true); err != nil {
		t.Fatalf("a host without a user install: %v", err)
	}
}

func TestNativeUserConfigurationFollowsTheHostsVariables(t *testing.T) {
	variables := map[string]string{"CODEX_HOME": "/srv/codex"}
	configuration := NativeUserConfiguration("/home/me", func(name string) string { return variables[name] })
	if !reflect.DeepEqual(configuration.Directories, map[string]string{"claude": "/home/me/.claude", "codex": "/srv/codex"}) || configuration.Windows {
		t.Fatalf("configuration: %+v", configuration)
	}
}

func TestInstallKeepsOtherSettingsAndHooks(t *testing.T) {
	root, h := t.TempDir(), newHomes(t)
	settings := h.file(false, "claude")
	writeTestFile(t, settings, `{"permissions":{"allow":["Bash(ls)"]},"hooks":{"PostToolUse":[{"matcher":"Write","hooks":[{"type":"command","command":"prettier --write","timeout":5}]}]}}`)
	binary := testBinary(t, t.TempDir(), "native")
	if _, err := NewInstaller(root, "", binary, h.users("")).Install([]string{"claude"}, false); err != nil {
		t.Fatal(err)
	}
	document := readTestConfiguration(t, settings)
	if !reflect.DeepEqual(document.Permissions, map[string]any{"allow": []any{"Bash(ls)"}}) {
		t.Fatalf("permissions changed: %v", document.Permissions)
	}
	post := document.Hooks[postToolUseEvent]
	if len(post) != 2 || post[0].Hooks[0].Command != "prettier --write" || post[0].Hooks[0].Timeout != 5 || post[1].Hooks[0].Command != binary+" "+eventArgs {
		t.Fatalf("PostToolUse groups: %+v", post)
	}
	if _, err := os.Stat(h.native.Directories["codex"]); !os.IsNotExist(err) {
		t.Fatalf("a host that was not selected was installed: %v", err)
	}
}

func TestInstallIsIdempotentAndReplacesAnEarlierHookGroupInPlace(t *testing.T) {
	root, h := t.TempDir(), newHomes(t)
	hooks := h.file(false, "codex")
	writeTestFile(t, hooks, `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"/old/bin/arclint `+eventArgs+`","timeout":5}]},{"hooks":[{"type":"command","command":"notify"}]}]}}`)
	binary := testBinary(t, t.TempDir(), "native")
	installer := NewInstaller(root, "", binary, h.users(""))
	if _, err := installer.Install([]string{"codex"}, false); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(hooks)
	if err != nil {
		t.Fatal(err)
	}
	stop := readTestConfiguration(t, hooks).Hooks[stopEvent]
	if len(stop) != 2 || stop[0].Hooks[0].Command != binary+" "+eventArgs || stop[0].Hooks[0].Timeout != 30 || stop[1].Hooks[0].Command != "notify" {
		t.Fatalf("Stop groups: %+v", stop)
	}
	if _, err := installer.Install([]string{"codex"}, false); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(hooks)
	if err != nil || string(first) != string(second) {
		t.Fatalf("second install changed the file: %v\n%s\n%s", err, first, second)
	}
}

// A user configuration kept elsewhere and linked into place stays linked.
func TestInstallWritesThroughALinkedConfiguration(t *testing.T) {
	root, h := t.TempDir(), newHomes(t)
	kept := filepath.Join(t.TempDir(), "settings.json")
	writeTestFile(t, kept, `{"theme":"dark"}`)
	settings := h.file(false, "claude")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(kept, settings); err != nil {
		t.Fatal(err)
	}
	if _, err := NewInstaller(root, "", testBinary(t, t.TempDir(), "native"), h.users("")).Install([]string{"claude"}, false); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(settings); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the link was replaced: %v", err)
	}
	if document := readTestConfiguration(t, kept); len(document.Hooks) != len(hookEvents) {
		t.Fatalf("the linked file lacks the hooks: %+v", document)
	}
}

func TestInstallRefusesWhatItCannotPreserveOrPin(t *testing.T) {
	binaries := t.TempDir()
	binary := testBinary(t, binaries, "native")
	for name, content := range map[string]string{
		"invalid_JSON":          `{"hooks":`,
		"hooks_is_not_object":   `{"hooks":[]}`,
		"event_is_not_an_array": `{"hooks":{"Stop":{}}}`,
		"document_is_null":      `null`,
		"trailing_document":     `{"permissions":{}}` + "\n" + `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"notify"}]}]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			h := newHomes(t)
			settings := h.file(false, "claude")
			writeTestFile(t, settings, content)
			if _, err := NewInstaller(t.TempDir(), "", binary, h.users("")).Install([]string{"claude"}, false); err == nil {
				t.Fatal("unpreservable configuration accepted")
			}
			if data, err := os.ReadFile(settings); err != nil || string(data) != content {
				t.Fatalf("configuration changed: %q %v", data, err)
			}
		})
	}
	h := newHomes(t)
	for name, installer := range map[string]*Installer{
		"unsupported host":       NewInstaller(t.TempDir(), "", binary, h.users("")),
		"unsafe distribution":    NewInstaller(t.TempDir(), `bad"name`, binary, h.users("bad")),
		"unlocated binary":       NewInstaller(t.TempDir(), "", "", h.users("")),
		"binary needing quoting": NewInstaller(t.TempDir(), "", "/opt/my tools/arclint", h.users("")),
	} {
		hosts := []string(nil)
		if name == "unsupported host" {
			hosts = []string{"cursor"}
		}
		if _, err := installer.Install(hosts, false); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	if entries, err := os.ReadDir(filepath.Dir(h.native.Directories["claude"])); err == nil && len(entries) != 0 {
		t.Fatalf("a refused install wrote %v", entries)
	}
}

func TestStatusNamesMissingChangedAndUnrunnableHooks(t *testing.T) {
	root, h := t.TempDir(), newHomes(t)
	binary := testBinary(t, t.TempDir(), "native")
	installer := NewInstaller(root, "", binary, h.users(""))
	if _, err := installer.Install([]string{"codex"}, false); err != nil {
		t.Fatal(err)
	}
	hooks := h.file(false, "codex")
	data, err := os.ReadFile(hooks)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, hooks, strings.Replace(string(data), `"timeout": 30`, `"timeout": 31`, 1))
	entries := statusOf(t, installer)
	if claude := entries["claude user"]; claude.Installation != application.HooksAbsent || !reflect.DeepEqual(claude.Problems, []string{"Workflow hooks are not installed."}) {
		t.Fatalf("claude: %+v", claude)
	}
	if codex := entries["codex user"]; codex.Installation != application.HooksComplete || len(codex.Problems) != 1 || !strings.Contains(codex.Problems[0], "differs from the one install writes here") {
		t.Fatalf("changed codex hook: %+v", codex)
	}
	if err := os.Remove(binary); err != nil {
		t.Fatal(err)
	}
	if codex := statusOf(t, installer)["codex user"]; !strings.Contains(strings.Join(codex.Problems, "\n"), "The hooks run "+binary+", which does not exist.") {
		t.Fatalf("missing binary: %+v", codex)
	}
	writeTestFile(t, hooks, `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"/gone/arclint `+eventArgs+`"}]}]}}`)
	codex := statusOf(t, installer)["codex user"]
	problems := strings.Join(codex.Problems, "\n")
	if codex.Installation != application.HooksPartial || !strings.Contains(problems, "SessionStart is installed; UserPromptSubmit, PostToolUse and Stop are missing. Run `arclint agents hooks install` to add them.") || !strings.Contains(problems, "The hooks run /gone/arclint, which does not exist.") {
		t.Fatalf("partial install of a removed binary: %+v", codex)
	}
	status, err := installer.Status()
	if err != nil {
		t.Fatal(err)
	}
	if silence := strings.Join(status.Silence, "\n"); strings.Contains(silence, "still run") || !strings.Contains(silence, "no hook a file lists is established to run here") {
		t.Fatalf("silence for a partial file whose binary is gone:\n%s", silence)
	}
}

// A file installed before UserPromptSubmit existed still runs its other
// hooks: status names the hooks it lists and the one it lacks, and does
// not say that no host runs them.
func TestStatusNamesTheHooksAPartialFileStillRuns(t *testing.T) {
	root, h := t.TempDir(), newHomes(t)
	installer := NewInstaller(root, "", testBinary(t, t.TempDir(), "native"), h.users(""))
	if _, err := installer.Install([]string{"claude"}, false); err != nil {
		t.Fatal(err)
	}
	settings := h.file(false, "claude")
	var document map[string]any
	data, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	delete(document["hooks"].(map[string]any), promptEvent)
	data, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, settings, string(data))
	status, err := installer.Status()
	if err != nil {
		t.Fatal(err)
	}
	var claude application.HookFileStatus
	for _, entry := range status.Files {
		if entry.Host == claudeHost && entry.Scope == scopeUser {
			claude = entry
		}
	}
	if claude.Installation != application.HooksPartial || !reflect.DeepEqual(claude.Listed, []string{sessionStartEvent, postToolUseEvent, stopEvent}) || !reflect.DeepEqual(claude.Missing, []string{promptEvent}) {
		t.Fatalf("partial claude file: %+v", claude)
	}
	if want := []string{"SessionStart, PostToolUse and Stop are installed and still run; UserPromptSubmit is missing. Run `arclint agents hooks install` to add it."}; !reflect.DeepEqual(claude.Problems, want) {
		t.Fatalf("problems %q, want %q", claude.Problems, want)
	}
	silence := strings.Join(status.Silence, "\n")
	if strings.Contains(silence, "no host runs them") || !strings.Contains(silence, "the hooks install wrote into a file still run") {
		t.Fatalf("silence for a partial file:\n%s", silence)
	}
	// A hook install did not write may name a binary that is gone or a
	// matcher that selects nothing, so status does not say it runs.
	hooks := document["hooks"].(map[string]any)
	hooks[stopEvent] = []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "/gone/arclint " + eventArgs}}}}
	delete(hooks, postToolUseEvent)
	data, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, settings, string(data))
	claude = statusOf(t, installer)["claude user"]
	if want := "SessionStart and Stop are installed, and SessionStart still runs; UserPromptSubmit and PostToolUse are missing. Run `arclint agents hooks install` to add them."; !slices.Contains(claude.Problems, want) {
		t.Fatalf("problems %q lack %q", claude.Problems, want)
	}
	status, err = installer.Status()
	if err != nil {
		t.Fatal(err)
	}
	if silence := strings.Join(status.Silence, "\n"); !strings.Contains(silence, "the hooks install wrote into a file still run") {
		t.Fatalf("silence with one hook install wrote:\n%s", silence)
	}
	// A hook install wrote runs the arclint that installed it, so none
	// runs once that binary is gone.
	if err := os.Remove(installer.binary); err != nil {
		t.Fatal(err)
	}
	if problems := strings.Join(statusOf(t, installer)["claude user"].Problems, "\n"); strings.Contains(problems, "still run") || !strings.Contains(problems, "which does not exist") {
		t.Fatalf("problems once the binary is gone:\n%s", problems)
	}
	// A directory in its place runs nothing either.
	if err := os.Mkdir(installer.binary, 0o755); err != nil {
		t.Fatal(err)
	}
	if problems := strings.Join(statusOf(t, installer)["claude user"].Problems, "\n"); strings.Contains(problems, "still run") || !strings.Contains(problems, "which is not a file") {
		t.Fatalf("problems with a directory in place of the binary:\n%s", problems)
	}
	if err := os.Remove(installer.binary); err != nil {
		t.Fatal(err)
	}
	// disableAllHooks turns off every hook Claude Code reads, so none is
	// said to run, wherever it is set.
	shared := filepath.Join(root, ".claude", "settings.json")
	for _, file := range []string{settings, shared} {
		document["disableAllHooks"] = file == settings
		data, err = json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, settings, string(data))
		writeTestFile(t, shared, fmt.Sprintf(`{"disableAllHooks": %t}`, file == shared))
		status, err = installer.Status()
		if err != nil {
			t.Fatal(err)
		}
		problems, silence := "", strings.Join(status.Silence, "\n")
		for _, entry := range status.Files {
			problems += strings.Join(entry.Problems, "\n") + "\n"
		}
		if strings.Contains(problems, "still run") || !strings.Contains(problems, "disableAllHooks") || strings.Contains(silence, "still run") || !strings.Contains(silence, "turns off every hook") {
			t.Fatalf("%s sets disableAllHooks:\nproblems:\n%s\nsilence:\n%s", file, problems, silence)
		}
	}
}

// Each side of WSL reads its own user configuration: disableAllHooks in
// the Windows profile turns off the Windows host apps' hooks, not the
// distribution's.
func TestDisabledHooksBelongToTheSideThatReadsThem(t *testing.T) {
	root, h := t.TempDir(), newHomes(t)
	installer := NewInstaller(root, "Ubuntu", testBinary(t, t.TempDir(), "native"), h.users("Ubuntu"))
	if _, err := installer.Install([]string{"claude"}, false); err != nil {
		t.Fatal(err)
	}
	for _, windows := range []bool{false, true} {
		var document map[string]any
		data, err := os.ReadFile(h.file(windows, "claude"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatal(err)
		}
		delete(document["hooks"].(map[string]any), promptEvent)
		if windows {
			document["disableAllHooks"] = true
		}
		data, err = json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, h.file(windows, "claude"), string(data))
	}
	entries := statusOf(t, installer)
	if native := strings.Join(entries["claude user"].Problems, "\n"); !strings.Contains(native, "and still run") || strings.Contains(native, "disableAllHooks") {
		t.Fatalf("the distribution's file: %s", native)
	}
	if windows := strings.Join(entries["claude user windows"].Problems, "\n"); strings.Contains(windows, "still run") || !strings.Contains(windows, "disableAllHooks") {
		t.Fatalf("the Windows file: %s", windows)
	}
}

// A Windows profile that cannot be located or read may turn its hooks
// off, so the project's file is not said to run for the Windows host apps while
// the distribution's own user file turns its hooks off.
func TestAnUnknownWindowsProfileIsNotEvidenceThatHooksRun(t *testing.T) {
	root, h := t.TempDir(), newHomes(t)
	binary := testBinary(t, t.TempDir(), "native")
	if _, err := NewInstaller(root, "Ubuntu", binary, h.users("Ubuntu")).Install([]string{"claude"}, true); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(root, ".claude", "settings.local.json")
	var document map[string]any
	data, err := os.ReadFile(local)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	delete(document["hooks"].(map[string]any), promptEvent)
	data, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, local, string(data))
	writeTestFile(t, h.file(false, "claude"), `{"disableAllHooks": true}`)
	unlocated := func() ([]UserConfiguration, error) {
		return []UserConfiguration{h.native, {Windows: true, Unreadable: "cmd.exe is missing"}}, nil
	}
	// An unreadable shared file is no evidence either, and status says why.
	writeTestFile(t, filepath.Join(root, ".claude", "settings.json"), `{`)
	writeTestFile(t, h.file(false, "claude"), `{}`)
	project := statusOf(t, NewInstaller(root, "Ubuntu", binary, h.users("Ubuntu")))["claude project"]
	if problems := strings.Join(project.Problems, "\n"); strings.Contains(problems, "still run") || !strings.Contains(problems, ".claude/settings.json cannot be read, so whether it sets disableAllHooks") {
		t.Fatalf("unreadable shared file: %+v", project)
	}
	if err := os.Remove(filepath.Join(root, ".claude", "settings.json")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, h.file(false, "claude"), `{"disableAllHooks": true}`)
	// A Windows profile file that cannot be read is no evidence either.
	writeTestFile(t, h.file(true, "claude"), `{"disableAllHooks": true,`)
	for name, users := range map[string]func() ([]UserConfiguration, error){"unlocated": unlocated, "unreadable": h.users("Ubuntu")} {
		status, err := NewInstaller(root, "Ubuntu", binary, users).Status()
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range status.Files {
			if entry.Host == claudeHost && entry.Scope == scopeProject {
				if problems := strings.Join(entry.Problems, "\n"); entry.Installation != application.HooksPartial || strings.Contains(problems, "still run") {
					t.Fatalf("%s Windows profile, project file: %+v", name, entry)
				}
			}
		}
		if silence := strings.Join(status.Silence, "\n"); strings.Contains(silence, "still run") {
			t.Fatalf("%s Windows profile, silence: %s", name, silence)
		}
	}
}

// What status cannot read is unknown, never absent or turned off.
func TestUnknownSettingsAreNeitherAbsentNorOff(t *testing.T) {
	root, h := t.TempDir(), newHomes(t)
	binary := testBinary(t, t.TempDir(), "native")
	unlocated := func() ([]UserConfiguration, error) {
		return []UserConfiguration{h.native, {Windows: true, Unreadable: "cmd.exe is missing"}}, nil
	}
	status, err := NewInstaller(root, "Ubuntu", binary, unlocated).Status()
	if err != nil {
		t.Fatal(err)
	}
	if silence := strings.Join(status.Silence, "\n"); strings.Contains(silence, "no host runs them here") || !strings.Contains(silence, "cannot be read or located, so whether a host runs them from it is unknown") {
		t.Fatalf("silence with an unlocated Windows profile:\n%s", silence)
	}
	installer := NewInstaller(root, "", binary, h.users(""))
	if _, err := installer.Install([]string{"claude"}, false); err != nil {
		t.Fatal(err)
	}
	user, err := os.ReadFile(h.file(false, "claude"))
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, ".claude", "settings.local.json"), string(user))
	writeTestFile(t, filepath.Join(root, ".claude", "settings.json"), `{`)
	problems := strings.Join(statusOf(t, installer)["claude project"].Problems, "\n")
	if strings.Contains(problems, "turned back on") || !strings.Contains(problems, "runs each event twice here whenever it runs hooks") {
		t.Fatalf("duplicates beside an unreadable shared file:\n%s", problems)
	}
}

// Only the commands install writes are workflow hooks. A hook that merely
// mentions the same words is someone else's, and install and the duplicate
// cleanup keep it.
func TestHooksThatOnlyMentionTheEventAreKept(t *testing.T) {
	others := []string{
		`notify "agents hooks event fired"`,
		`echo agents hooks event`,
		`/usr/bin/logger agents hooks event happened`,
		`wsl.exe -d Ubuntu -e /usr/bin/logger agents hooks event happened`,
		`if [ -x /a/arclint ]; then exec /b/arclint agents hooks event; else MSYS_NO_PATHCONV=1 exec wsl.exe -d Ubuntu -e /a/arclint agents hooks event; fi`,
	}
	ours := []string{
		"/old/bin/arclint " + eventArgs,
		"C:/tools/arclint.exe " + eventArgs,
		windowsCommand("/old/bin/arclint", "Debian"),
		gitBashCommand("/old/bin/arclint", "Ubuntu"),
		sharedCommand("/old/bin/arclint", "Ubuntu"),
	}
	for _, command := range others {
		if isArclintHandler(map[string]any{handlerCommand: command}) {
			t.Errorf("another hook taken for a workflow hook: %s", command)
		}
	}
	for _, command := range ours {
		if !isArclintHandler(map[string]any{handlerCommand: command}) {
			t.Errorf("a command install writes was not recognized: %s", command)
		}
	}
	root, h := t.TempDir(), newHomes(t)
	groups := make([]string, len(others))
	for index, command := range others {
		encoded, err := json.Marshal(command)
		if err != nil {
			t.Fatal(err)
		}
		groups[index] = `{"hooks":[{"type":"command","command":` + string(encoded) + `}]}`
	}
	content := `{"hooks":{"Stop":[` + strings.Join(groups, ",") + `]}}`
	local := filepath.Join(root, ".claude/settings.local.json")
	writeTestFile(t, local, content)
	writeTestFile(t, h.file(false, "claude"), content)
	installation, err := NewInstaller(root, "", testBinary(t, t.TempDir(), "native"), h.users("")).Install([]string{"claude"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(installation.Removed) != 0 {
		t.Fatalf("cleanup removed another hook: %v", installation.Removed)
	}
	if data, err := os.ReadFile(local); err != nil || string(data) != content {
		t.Fatalf("the project file changed: %v\n%s", err, data)
	}
	stop := readTestConfiguration(t, h.file(false, "claude")).Hooks[stopEvent]
	if len(stop) != len(others)+1 {
		t.Fatalf("install replaced another hook: %+v", stop)
	}
	for index, command := range others {
		if stop[index].Hooks[0].Command != command {
			t.Fatalf("group %d is %q, want %q", index, stop[index].Hooks[0].Command, command)
		}
	}
}

func TestStatusNamesDuplicateFilesAndTheLastEvent(t *testing.T) {
	root, h := t.TempDir(), newHomes(t)
	binary := testBinary(t, t.TempDir(), "native")
	installer := NewInstaller(root, "", binary, h.users(""))
	if _, err := installer.Install([]string{"claude"}, false); err != nil {
		t.Fatal(err)
	}
	user, err := os.ReadFile(h.file(false, "claude"))
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, ".claude/settings.local.json"), string(user))
	writeTestFile(t, filepath.Join(root, ".claude/settings.json"), string(user))
	record := filepath.Join(root, progressDirectory, "session.json")
	writeTestFile(t, record, "{}")
	at := time.Date(2026, 10, 9, 20, 15, 3, 0, time.UTC)
	if err := os.Chtimes(record, at, at); err != nil {
		t.Fatal(err)
	}
	status, err := installer.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !status.LastEvent.Equal(at) {
		t.Fatalf("last event %v, want %v", status.LastEvent, at)
	}
	project := statusOf(t, installer)["claude project"]
	problems := strings.Join(project.Problems, "\n")
	if project.Installation != application.HooksComplete || !strings.Contains(problems, h.file(false, "claude")+" already lists the hooks for every project") || !strings.Contains(problems, ".claude/settings.json also lists the workflow hooks") {
		t.Fatalf("duplicates: %+v", project)
	}
	// With every hook turned off, the duplicates run nothing until the
	// setting is removed.
	writeTestFile(t, filepath.Join(root, ".claude/settings.json"), strings.Replace(string(user), "{", `{"disableAllHooks": true,`, 1))
	problems = strings.Join(statusOf(t, installer)["claude project"].Problems, "\n")
	if !strings.Contains(problems, "would run each event twice here once its hooks are turned back on") || !strings.Contains(problems, "once hooks are turned back on, every file that lists them runs each event") || strings.Contains(problems, " runs each event twice") {
		t.Fatalf("duplicates while hooks are off: %s", problems)
	}
}

// A user file that lists only some events still runs them beside a
// complete project install.
func TestStatusNamesADuplicateFromAPartialUserFile(t *testing.T) {
	root, h := t.TempDir(), newHomes(t)
	binary := testBinary(t, t.TempDir(), "native")
	installer := NewInstaller(root, "", binary, h.users(""))
	if _, err := installer.Install([]string{"codex"}, true); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, h.file(false, "codex"), `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"`+binary+` `+eventArgs+`"}]}]}}`)
	entries := statusOf(t, installer)
	if user := entries["codex user"]; user.Installation != application.HooksPartial {
		t.Fatalf("a partial user file reported installed: %+v", user)
	}
	if problems := strings.Join(entries["codex project"].Problems, "\n"); !strings.Contains(problems, h.file(false, "codex")+" already lists the hooks for every project") {
		t.Fatalf("duplicate from a partial user file not reported: %s", problems)
	}
}

func TestInstallKeepsOtherHandlersOfAMixedGroup(t *testing.T) {
	root, h := t.TempDir(), newHomes(t)
	settings := h.file(false, "claude")
	writeTestFile(t, settings, `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"notify"},{"type":"command","command":"/old/bin/arclint `+eventArgs+`"}]}]}}`)
	binary := testBinary(t, t.TempDir(), "native")
	if _, err := NewInstaller(root, "", binary, h.users("")).Install([]string{"claude"}, false); err != nil {
		t.Fatal(err)
	}
	stop := readTestConfiguration(t, settings).Hooks[stopEvent]
	if len(stop) != 2 || len(stop[0].Hooks) != 1 || stop[0].Hooks[0].Command != "notify" || stop[1].Hooks[0].Command != binary+" "+eventArgs {
		t.Fatalf("Stop groups: %+v", stop)
	}
}

// statusOf keys each status entry by host and scope.
func statusOf(t *testing.T, installer *Installer) map[string]application.HookFileStatus {
	t.Helper()
	status, err := installer.Status()
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string]application.HookFileStatus{}
	for _, entry := range status.Files {
		key := entry.Host + " " + entry.Scope
		if entry.Windows {
			key += " windows"
		}
		entries[key] = entry
	}
	return entries
}

// Without cmd.exe the Windows profile cannot be located. That is a problem
// of the Windows files alone: status still reports the distribution's
// files, and install refuses rather than write half of what it promises.
func TestAnUnreachableWindowsProfileIsAProblemOfItsFilesOnly(t *testing.T) {
	root, h := t.TempDir(), newHomes(t)
	binary := testBinary(t, t.TempDir(), "native")
	reason := `read the Windows environment through cmd.exe: exec: "cmd.exe": executable file not found in $PATH`
	users := func() ([]UserConfiguration, error) {
		return []UserConfiguration{h.native, {Windows: true, Unreadable: reason}}, nil
	}
	if _, err := NewInstaller(root, "", binary, h.users("")).Install(nil, false); err != nil {
		t.Fatal(err)
	}
	installer := NewInstaller(root, "Ubuntu", binary, users)
	entries := statusOf(t, installer)
	for _, host := range []string{"claude", "codex"} {
		if native := entries[host+" user"]; native.Installation != application.HooksComplete || len(native.Problems) != 0 {
			t.Fatalf("%s native file: %+v", host, native)
		}
		windows := entries[host+" user windows"]
		if windows.Installation != application.HooksUnknown || len(windows.Problems) != 1 || !strings.Contains(windows.Problems[0], reason) {
			t.Fatalf("%s Windows file: %+v", host, windows)
		}
	}
	if _, err := installer.Install(nil, false); err == nil || !strings.Contains(err.Error(), "cmd.exe") {
		t.Fatalf("install without the Windows profile: %v", err)
	}
}

// Before any event, status says why none arrived; afterwards it says
// nothing about silence.
func TestStatusExplainsWhyNoEventArrived(t *testing.T) {
	root, h := t.TempDir(), newHomes(t)
	installer := NewInstaller(root, "", testBinary(t, t.TempDir(), "native"), h.users(""))
	status, err := installer.Status()
	if err != nil {
		t.Fatal(err)
	}
	silence := strings.Join(status.Silence, "\n")
	for _, want := range []string{"No configuration file lists the workflow hooks, so no host runs them here", "This directory holds no rules.arclint.yaml", "working directory", "started after they were installed"} {
		if !strings.Contains(silence, want) {
			t.Fatalf("silence lacks %q:\n%s", want, silence)
		}
	}
	if _, err := installer.Install(nil, false); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "rules.arclint.yaml"), "zones: {}\n")
	at := time.Date(2026, 10, 16, 13, 45, 48, 0, time.Local)
	if err := os.Chtimes(filepath.Join(root, "rules.arclint.yaml"), at, at); err != nil {
		t.Fatal(err)
	}
	status, err = installer.Status()
	if err != nil {
		t.Fatal(err)
	}
	silence = strings.Join(status.Silence, "\n")
	if strings.Contains(silence, "No configuration file") || strings.Contains(silence, "holds no") || !strings.Contains(silence, "last written at 2026-10-16 13:45:48") || !strings.Contains(silence, "worktree made from an older commit") {
		t.Fatalf("silence after install with a ruleset:\n%s", silence)
	}
	writeTestFile(t, filepath.Join(root, progressDirectory, "session.json"), "{}")
	if status, err = installer.Status(); err != nil || len(status.Silence) != 0 {
		t.Fatalf("silence after an event: %v %v", status.Silence, err)
	}
}
