package workflow

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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

func TestInstallWritesTheSameHookForBothHosts(t *testing.T) {
	root := t.TempDir()
	installer := NewInstaller(root, "")
	paths, err := installer.Install(nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(root, ".claude/settings.json"), filepath.Join(root, ".codex/hooks.json")}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths %v, want %v", paths, want)
	}
	for _, host := range hookHosts {
		document := readTestConfiguration(t, filepath.Join(root, host.path))
		if len(document.Hooks) != len(hookEvents) {
			t.Fatalf("%s events: %v", host.name, document.Hooks)
		}
		for _, event := range hookEvents {
			groups := document.Hooks[event]
			if len(groups) != 1 || len(groups[0].Hooks) != 1 || groups[0].Hooks[0].Command != eventCommand || groups[0].Hooks[0].Type != "command" {
				t.Fatalf("%s %s: %+v", host.name, event, groups)
			}
			if wantMatcher := map[bool]string{true: host.tools}[event == postToolUseEvent]; groups[0].Matcher != wantMatcher {
				t.Fatalf("%s %s matcher %q, want %q", host.name, event, groups[0].Matcher, wantMatcher)
			}
		}
	}
	status, err := installer.Status()
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range status.Hosts {
		if !host.Installed || len(host.Problems) != 0 {
			t.Fatalf("status after install: %+v", host)
		}
	}
}

func TestInstallKeepsOtherSettingsAndHooks(t *testing.T) {
	root := t.TempDir()
	settings := filepath.Join(root, ".claude/settings.json")
	writeTestFile(t, settings, `{"permissions":{"allow":["Bash(ls)"]},"hooks":{"PostToolUse":[{"matcher":"Write","hooks":[{"type":"command","command":"prettier --write","timeout":5}]}]}}`)
	if _, err := NewInstaller(root, "").Install([]string{"claude"}); err != nil {
		t.Fatal(err)
	}
	document := readTestConfiguration(t, settings)
	if !reflect.DeepEqual(document.Permissions, map[string]any{"allow": []any{"Bash(ls)"}}) {
		t.Fatalf("permissions changed: %v", document.Permissions)
	}
	post := document.Hooks[postToolUseEvent]
	if len(post) != 2 || post[0].Hooks[0].Command != "prettier --write" || post[0].Hooks[0].Timeout != 5 || post[1].Hooks[0].Command != eventCommand {
		t.Fatalf("PostToolUse groups: %+v", post)
	}
	if _, err := os.Stat(filepath.Join(root, ".codex")); !os.IsNotExist(err) {
		t.Fatalf("a host that was not selected was installed: %v", err)
	}
}

func TestInstallIsIdempotentAndReplacesAnEarlierWorkflowGroupInPlace(t *testing.T) {
	root := t.TempDir()
	hooks := filepath.Join(root, ".codex/hooks.json")
	writeTestFile(t, hooks, `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"`+eventCommand+`","timeout":5}]},{"hooks":[{"type":"command","command":"notify"}]}]}}`)
	installer := NewInstaller(root, "")
	if _, err := installer.Install([]string{"codex"}); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(hooks)
	if err != nil {
		t.Fatal(err)
	}
	stop := readTestConfiguration(t, hooks).Hooks[stopEvent]
	if len(stop) != 2 || stop[0].Hooks[0].Command != eventCommand || stop[0].Hooks[0].Timeout != 30 || stop[1].Hooks[0].Command != "notify" {
		t.Fatalf("Stop groups: %+v", stop)
	}
	if _, err := installer.Install([]string{"codex"}); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(hooks)
	if err != nil || string(first) != string(second) {
		t.Fatalf("second install changed the file: %v\n%s\n%s", err, first, second)
	}
}

func TestInstallRefusesConfigurationItCannotPreserve(t *testing.T) {
	for name, content := range map[string]string{
		"invalid JSON":          `{"hooks":`,
		"hooks is not object":   `{"hooks":[]}`,
		"event is not an array": `{"hooks":{"Stop":{}}}`,
		"document is null":      `null`,
		"trailing document":     `{"permissions":{}}` + "\n" + `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"notify"}]}]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			settings := filepath.Join(root, ".claude/settings.json")
			writeTestFile(t, settings, content)
			if _, err := NewInstaller(root, "").Install([]string{"claude"}); err == nil {
				t.Fatal("unpreservable configuration accepted")
			}
			if data, err := os.ReadFile(settings); err != nil || string(data) != content {
				t.Fatalf("configuration changed: %q %v", data, err)
			}
		})
	}
	if _, err := NewInstaller(t.TempDir(), "").Install([]string{"cursor"}); err == nil {
		t.Fatal("unsupported host accepted")
	}
	if _, err := NewInstaller(t.TempDir(), `bad"name`).Install(nil); err == nil {
		t.Fatal("unsafe distribution name accepted")
	}
}

func TestStatusNamesMissingAndChangedHooks(t *testing.T) {
	root := t.TempDir()
	installer := NewInstaller(root, "")
	if _, err := installer.Install([]string{"codex"}); err != nil {
		t.Fatal(err)
	}
	hooks := filepath.Join(root, ".codex/hooks.json")
	data, err := os.ReadFile(hooks)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(data), `"timeout": 30`, `"timeout": 31`, 1)
	writeTestFile(t, hooks, changed)
	var document map[string]map[string]any
	if err := json.Unmarshal([]byte(changed), &document); err != nil {
		t.Fatal(err)
	}
	delete(document["hooks"], stopEvent)
	withoutStop, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	status, err := installer.Status()
	if err != nil {
		t.Fatal(err)
	}
	if claude := status.Hosts[0]; claude.Installed || !reflect.DeepEqual(claude.Problems, []string{"Workflow hooks are not installed."}) {
		t.Fatalf("claude: %+v", claude)
	}
	if codex := status.Hosts[1]; !codex.Installed || len(codex.Problems) != 1 || !strings.Contains(codex.Problems[0], "differs from the one install writes here") {
		t.Fatalf("changed codex hook: %+v", codex)
	}
	writeTestFile(t, hooks, string(withoutStop))
	status, err = installer.Status()
	if err != nil {
		t.Fatal(err)
	}
	if codex := status.Hosts[1]; codex.Installed || len(codex.Problems) == 0 || codex.Problems[len(codex.Problems)-1] != "The Stop hook is missing." {
		t.Fatalf("missing codex hook: %+v", codex)
	}
}

func TestStatusRecognizesAnInstallFromAnotherEnvironment(t *testing.T) {
	root := t.TempDir()
	if _, err := NewInstaller(root, "Ubuntu-24.04").Install(nil); err != nil {
		t.Fatal(err)
	}
	status, err := NewInstaller(root, "").Status()
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range status.Hosts {
		if !host.Installed || len(host.Problems) != len(hookEvents) {
			t.Fatalf("%s: %+v", host.Host, host)
		}
	}
}

func TestInstallKeepsOtherHandlersOfAMixedGroup(t *testing.T) {
	root := t.TempDir()
	settings := filepath.Join(root, ".claude/settings.json")
	writeTestFile(t, settings, `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"notify"},{"type":"command","command":"`+eventCommand+` --old"}]}]}}`)
	if _, err := NewInstaller(root, "").Install([]string{"claude"}); err != nil {
		t.Fatal(err)
	}
	stop := readTestConfiguration(t, settings).Hooks[stopEvent]
	if len(stop) != 2 || len(stop[0].Hooks) != 1 || stop[0].Hooks[0].Command != "notify" || stop[1].Hooks[0].Command != eventCommand {
		t.Fatalf("Stop groups: %+v", stop)
	}
}

// The Claude Code command runs through sh (Git Bash on Windows). It runs
// arclint when the shell finds it and otherwise reaches the distribution
// through wsl.exe, exactly as Codex's commandWindows does.
func TestWSLInstallReachesArclintFromBothSides(t *testing.T) {
	root := t.TempDir()
	if _, err := NewInstaller(root, "Ubuntu-24.04").Install(nil); err != nil {
		t.Fatal(err)
	}
	windows := `wsl.exe -d Ubuntu-24.04 -- bash -lc "exec arclint agents workflow event"`
	codex := readTestConfiguration(t, filepath.Join(root, ".codex/hooks.json")).Hooks[stopEvent][0].Hooks[0]
	if codex.Command != eventCommand || codex.CommandWindows != windows {
		t.Fatalf("codex handler: %+v", codex)
	}
	claude := readTestConfiguration(t, filepath.Join(root, ".claude/settings.json")).Hooks[stopEvent][0].Hooks[0].Command
	raw, err := os.ReadFile(filepath.Join(root, ".claude/settings.json"))
	if err != nil || !strings.Contains(string(raw), ">/dev/null 2>&1") {
		t.Fatalf("command was HTML-escaped in the file: %v\n%s", err, raw)
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is unavailable")
	}
	for _, stub := range []struct{ name, want string }{
		{"arclint", "arclint agents workflow event"},
		{"wsl.exe", "wsl.exe -d Ubuntu-24.04 -- bash -lc exec arclint agents workflow event"},
	} {
		bin := t.TempDir()
		writeTestFile(t, filepath.Join(bin, stub.name), "#!/bin/sh\necho "+stub.name+` "$@"`+"\n")
		if err := os.Chmod(filepath.Join(bin, stub.name), 0o755); err != nil {
			t.Fatal(err)
		}
		command := exec.Command(shell, "-c", claude)
		command.Env = []string{"PATH=" + bin + string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/bin"}
		output, err := command.CombinedOutput()
		if err != nil || strings.TrimSpace(string(output)) != stub.want {
			t.Fatalf("with %s on PATH: %q %v", stub.name, output, err)
		}
	}
}
