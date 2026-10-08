package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The hosts run `arclint agents workflow event` from the session's working
// directory, which can be below the project root. ArcLint's own context,
// check and domain commands report themselves to the session.
func TestWorkflowHooksThroughTheBinary(t *testing.T) {
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
	env := append(os.Environ(), "WSL_DISTRO_NAME=")

	stdout, stderr, code := runBin(t, root, env, "agents", "workflow", "install", "--format", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("install: %d %s %s", code, stdout, stderr)
	}
	var install struct {
		Operation, Host string
		Paths           []string
	}
	if err := json.Unmarshal([]byte(stdout), &install); err != nil || install.Operation != "workflow" || install.Host != "claude, codex" || len(install.Paths) != 2 {
		t.Fatalf("install report: %v %s", err, stdout)
	}
	stdout, _, code = runBin(t, root, env, "agents", "workflow", "status", "--format", "json")
	var status struct {
		Command string
		Hosts   []struct {
			Host      string
			Installed bool
			Problems  []string
		}
	}
	if err := json.Unmarshal([]byte(stdout), &status); err != nil || code != 0 || status.Command != "arclint agents workflow event" || len(status.Hosts) != 2 || !status.Hosts[0].Installed || !status.Hosts[1].Installed || len(status.Hosts[0].Problems)+len(status.Hosts[1].Problems) != 0 {
		t.Fatalf("status: %d %v %s", code, err, stdout)
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
		stdout, stderr, code := runBinStdin(t, subdirectory, env, string(input), "agents", "workflow", "event")
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

	if out := event(map[string]any{"hook_event_name": "SessionStart", "source": "startup"}); !strings.Contains(out, `"additionalContext":"ArcLint workflow hooks are installed`) {
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
		stdout, stderr, code := runBinStdin(t, root, env, string(input), "agents", "workflow", "event")
		if code != 0 || stderr != "" {
			t.Fatalf("event: %d %s %s", code, stdout, stderr)
		}
		return strings.TrimSpace(stdout)
	}
	payload("second", map[string]any{"hook_event_name": "SessionStart", "source": "startup"})
	arclint("domain", "init")
	arclint("context", ".")
	out := payload("second", edit("internal/app/app.go"))
	if !strings.Contains(out, "internal/app/app.go changed before `arclint context` showed every Zone that owns it") || !strings.Contains(out, "changed before domain.arclint.yaml changed") {
		t.Fatalf("a no-op domain init or context of . was credited: %s", out)
	}
}
