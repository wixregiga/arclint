package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The hosts run `arclint agents hooks event` from the session's working
// directory, which can be below the project root. ArcLint's own context,
// check and domain commands report themselves to the session.
func TestHooksThroughTheBinary(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", `runtime: [go]
zones:
  app: internal/**
rules:
  app/no-panic:
    on: app
    files: "internal/**/*.go"
    content:
      forbid: '\bpanic\('
`)
	write(t, root, "go.mod", "module example.com/app\n\ngo 1.24\n")
	write(t, root, "internal/app/app.go", "package app\n")
	// The install writes the user configuration, so it must land in a home
	// of this test's own.
	home := t.TempDir()
	env := append(os.Environ(), "WSL_DISTRO_NAME=", "HOME="+home, "CLAUDE_CONFIG_DIR=", "CODEX_HOME=")

	stdout, stderr, code := runBin(t, root, env, "agents", "hooks", "install", "--format", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("install: %d %s %s", code, stdout, stderr)
	}
	var install struct {
		Operation, Host, Orientation string
		Paths                        []string
	}
	userFiles := []string{filepath.Join(home, ".claude", "settings.json"), filepath.Join(home, ".codex", "hooks.json")}
	if err := json.Unmarshal([]byte(stdout), &install); err != nil || install.Operation != "hooks" || install.Host != "claude, codex" || !slices.Equal(install.Paths, userFiles) || !strings.HasPrefix(install.Orientation, "Work in this order:\n1. Run `arclint context <paths>`") {
		t.Fatalf("install report: %v %s", err, stdout)
	}
	settings, err := os.ReadFile(userFiles[0])
	if err != nil || !strings.Contains(string(settings), `"command": "`+binPath+` agents hooks event"`) {
		t.Fatalf("the hooks do not run this binary by its path: %v\n%s", err, settings)
	}
	stdout, _, code = runBin(t, root, env, "agents", "hooks", "status", "--format", "json")
	var status struct {
		Binary    string
		LastEvent string
		Files     []struct {
			Host, Scope string
			Installed   bool
			Problems    []string
		}
	}
	if err := json.Unmarshal([]byte(stdout), &status); err != nil || code != 0 || status.Binary != binPath || status.LastEvent != "" || len(status.Files) != 4 {
		t.Fatalf("status: %d %v %s", code, err, stdout)
	}
	for _, entry := range status.Files {
		if entry.Installed != (entry.Scope == "user") || len(entry.Problems) != 0 {
			t.Fatalf("status entry: %+v", entry)
		}
	}
	if stdout, stderr, code := runBin(t, root, env, "agents", "hooks", "install", "--project"); code == 0 || !strings.Contains(stderr, "would run each event twice") {
		t.Fatalf("project install beside the user install: %d %s %s", code, stdout, stderr)
	}

	subdirectory := filepath.Join(root, "internal")
	event := func(payload map[string]any) string {
		t.Helper()
		payload["session_id"] = "e2e"
		payload["cwd"] = subdirectory
		input, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := runBinStdin(t, subdirectory, env, string(input), "agents", "hooks", "event")
		if code != 0 || stderr != "" {
			t.Fatalf("event %v: %d %s %s", payload["hook_event_name"], code, stdout, stderr)
		}
		return strings.TrimSpace(stdout)
	}
	edit := func(name string) map[string]any {
		return map[string]any{"hook_event_name": "PostToolUse", "tool_name": "Edit", "tool_input": map[string]any{"file_path": filepath.Join(root, name)}}
	}
	arclint := func(args ...string) {
		t.Helper()
		if stdout, stderr, code := runBin(t, subdirectory, env, args...); code != 0 {
			t.Fatalf("%v: %d %s %s", args, code, stdout, stderr)
		}
	}

	if out := event(map[string]any{"hook_event_name": "SessionStart", "source": "startup"}); !strings.Contains(out, `"additionalContext":"ArcLint workflow hooks run in this project. Work in this order:`) {
		t.Fatalf("session start: %s", out)
	}
	if out := event(edit("internal/app/app.go")); !strings.Contains(out, "internal/app/app.go changed before `arclint context` showed every Zone that owns it") || !strings.Contains(out, "changed before domain.arclint.yaml changed") {
		t.Fatalf("first edit: %s", out)
	}
	if out := event(edit("internal/app/app.go")); out != "{}" {
		t.Fatalf("repeated edit: %s", out)
	}
	arclint("context", "internal")
	if out := event(edit("internal/app/other.go")); out != "{}" {
		t.Fatalf("edit after the binary obtained context: %s", out)
	}
	if out := event(map[string]any{"hook_event_name": "Stop", "stop_hook_active": false}); !strings.Contains(out, `"systemMessage":"ArcLint workflow: internal/app/app.go, internal/app/other.go changed since the last`) {
		t.Fatalf("stop: %s", out)
	}
	arclint("check", ".")
	if out := event(map[string]any{"hook_event_name": "Stop", "stop_hook_active": false}); out != "{}" {
		t.Fatalf("stop after the binary ran the check: %s", out)
	}

	// A new session is credited only with what changes or shows something:
	// initializing a recording that exists changes nothing, and context of
	// "." shows no Zone.
	arclint("domain", "init")
	payload := func(session string, fields map[string]any) string {
		t.Helper()
		fields["session_id"] = session
		fields["cwd"] = root
		input, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := runBinStdin(t, root, env, string(input), "agents", "hooks", "event")
		if code != 0 || stderr != "" {
			t.Fatalf("event: %d %s %s", code, stdout, stderr)
		}
		return strings.TrimSpace(stdout)
	}
	payload("second", map[string]any{"hook_event_name": "SessionStart", "source": "startup"})

	// The user install runs the same command in directories that are not
	// ArcLint projects; there it answers nothing and writes nothing.
	elsewhere := t.TempDir()
	start, _ := json.Marshal(map[string]any{"hook_event_name": "SessionStart", "session_id": "other", "cwd": elsewhere, "source": "startup"})
	if stdout, stderr, code := runBinStdin(t, elsewhere, env, string(start), "agents", "hooks", "event"); code != 0 || stderr != "" || strings.TrimSpace(stdout) != "{}" {
		t.Fatalf("event outside a project: %d %q %q", code, stdout, stderr)
	}
	if entries, err := os.ReadDir(elsewhere); err != nil || len(entries) != 0 {
		t.Fatalf("files written outside a project: %v %v", entries, err)
	}
	arclint("domain", "init")
	arclint("context", ".")
	out := payload("second", edit("internal/app/app.go"))
	if !strings.Contains(out, "internal/app/app.go changed before `arclint context` showed every Zone that owns it") || !strings.Contains(out, "changed before domain.arclint.yaml changed") {
		t.Fatalf("a no-op domain init or context of . was credited: %s", out)
	}
}
