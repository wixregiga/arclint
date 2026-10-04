package workflow

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestWorkflowLauncherCapture(t *testing.T) {
	capture := os.Getenv("ARCLINT_LAUNCH_CAPTURE")
	if capture == "" {
		return
	}
	for index, argument := range os.Args {
		if argument == "--" {
			content, err := json.Marshal(os.Args[index+1:])
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(capture, content, 0600); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatal("launcher argument separator missing")
}

func captureLaunch(t *testing.T, script []byte) []string {
	t.Helper()
	dir := t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	fake := "#!/bin/sh\nexec " + shellQuote(binary) + " -test.run=TestWorkflowLauncherCapture -- \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(fake), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "start.sh")
	if err := os.WriteFile(path, script, 0600); err != nil {
		t.Fatal(err)
	}
	capture := filepath.Join(dir, "arguments.json")
	command := exec.Command("bash", path)
	command.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "ARCLINT_LAUNCH_CAPTURE="+capture)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("launcher execution: %v: %s", err, output)
	}
	content, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var arguments []string
	if err := json.Unmarshal(content, &arguments); err != nil {
		t.Fatal(err)
	}
	return arguments
}

func gitFixture(t *testing.T, dir string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, arguments...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("fixture Git: %v: %s", err, output)
	}
}

func TestLaunchOrdinaryProjectDoesNotDuplicateProjectHooks(t *testing.T) {
	root := t.TempDir()
	gitFixture(t, root, "init", "--quiet")
	script, err := launchScript(root, map[string]any{"type": "command", "command": "review"})
	if err != nil {
		t.Fatal(err)
	}
	arguments := captureLaunch(t, script)
	if !reflect.DeepEqual(arguments, []string{"--no-daemon", "-C", root}) {
		t.Fatalf("ordinary project acquired additional hooks: %#v", arguments)
	}
}

func TestLaunchLinkedWorktreePreservesExactHandlersAndInheritedProvider(t *testing.T) {
	repository := t.TempDir()
	gitFixture(t, repository, "init", "--quiet")
	gitFixture(t, repository, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--quiet", "--allow-empty", "-m", "fixture")
	root := filepath.Join(t.TempDir(), "linked space ' quote")
	gitFixture(t, repository, "worktree", "add", "--quiet", "--detach", root)
	handler := map[string]any{"type": "command", "command": "'binary space' --rules 'project rules' agents workflow event", "commandWindows": `wsl.exe -- python3 "C:\Space\review.py"`, "timeout": 90, "statusMessage": "Workflow review", "async": false}
	script, err := launchScript(root, handler)
	if err != nil {
		t.Fatal(err)
	}
	arguments := captureLaunch(t, script)
	if len(arguments) != 3+2*len(workflowEvents) || !reflect.DeepEqual(arguments[:3], []string{"--no-daemon", "-C", root}) {
		t.Fatalf("linked launcher flags: %#v", arguments)
	}
	for index, event := range workflowEvents {
		if arguments[3+index*2] != "-c" {
			t.Fatal("additional hook must use supported session configuration")
		}
		configuration := arguments[4+index*2]
		if !strings.HasPrefix(configuration, "hooks."+event+"=") {
			t.Fatalf("wrong event: %s", configuration)
		}
		var decoded struct {
			Hooks map[string][]struct{ Hooks []map[string]any }
		}
		if err := toml.Unmarshal([]byte(configuration), &decoded); err != nil {
			t.Fatalf("invalid actual shell-passed TOML: %v: %s", err, configuration)
		}
		groups := decoded.Hooks[event]
		if len(groups) != 1 {
			t.Fatalf("event groups: %#v", groups)
		}
		handlers := groups[0].Hooks
		if len(handlers) != 1 {
			t.Fatalf("event handlers: %#v", handlers)
		}
		actual, err := json.Marshal(handlers[0])
		if err != nil {
			t.Fatal(err)
		}
		expected, err := json.Marshal(handler)
		if err != nil {
			t.Fatal(err)
		}
		if string(actual) != string(expected) || len(groups) != 1 || len(handlers) != 1 {
			t.Fatalf("handler fields changed: %s != %s", actual, expected)
		}
	}
	// No inherited hook definition is rewritten: only additional session flags are produced.
	if _, err := os.Stat(filepath.Join(repository, ".codex")); !os.IsNotExist(err) {
		t.Fatal("launcher generation changed the main checkout")
	}
}
