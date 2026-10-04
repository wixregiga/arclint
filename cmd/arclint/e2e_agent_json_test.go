package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Decode the full process output, so a human activation trailer or a second
// document fails rather than being ignored after a valid initial JSON value.
func runAgentJSON(t *testing.T, root string, args ...string) map[string]json.RawMessage {
	t.Helper()
	stdout, stderr, code := runBin(t, root, nil, args...)
	if code != 0 || stderr != "" {
		t.Fatalf("%v: exit %d\nstdout: %s\nstderr: %s", args, code, stdout, stderr)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("%v full stdout is not one JSON object: %v\n%s", args, err, stdout)
	}
	for _, unsupported := range []string{"activated", "approved", "trusted", "reviewPassed"} {
		if _, found := doc[unsupported]; found {
			t.Fatalf("installation claimed %s", unsupported)
		}
	}
	return doc
}

func agentJSONField[T any](t *testing.T, doc map[string]json.RawMessage, name string) T {
	t.Helper()
	var value T
	raw, found := doc[name]
	if !found {
		t.Fatalf("missing %s in %s", name, doc)
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("field %s: %v", name, err)
	}
	return value
}

func TestReviewerCommandsRenderWholeJSON(t *testing.T) {
	root := t.TempDir()
	missing := runAgentJSON(t, root, "--format=json", "agents", "reviewer", "status")
	if agentJSONField[bool](t, missing, "installed") || agentJSONField[bool](t, missing, "intact") {
		t.Fatal("missing reviewer reported installed")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("status wrote files: %v %v", entries, err)
	}
	install := runAgentJSON(t, root, "agents", "reviewer", "install", "--host", "codex", "--format", "json")
	if agentJSONField[string](t, install, "operation") != "reviewer" || agentJSONField[string](t, install, "host") != "codex" || len(agentJSONField[[]string](t, install, "paths")) == 0 {
		t.Fatal("wrong reviewer installation result")
	}
	status := runAgentJSON(t, root, "--format=json", "agents", "reviewer", "status")
	if !agentJSONField[bool](t, status, "installed") || !agentJSONField[bool](t, status, "intact") || agentJSONField[string](t, status, "name") != "arclint-domain-reviewer" {
		t.Fatalf("wrong reviewer status: %s", status)
	}
	if agentJSONField[string](t, status, "installedVersion") != strings.TrimSpace(version) || agentJSONField[string](t, status, "availableVersion") != strings.TrimSpace(version) {
		t.Fatal("lost release identity")
	}
	if !strings.Contains(agentJSONField[string](t, status, "limits"), "do not prove") || len(agentJSONField[[]string](t, status, "problems")) != 0 {
		t.Fatal("incorrect status limits/problems")
	}
	path := filepath.Join(root, ".codex/agents/arclint-domain-reviewer.toml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(content, []byte("\n# owner edit\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	status = runAgentJSON(t, root, "agents", "reviewer", "status", "--format", "json")
	if agentJSONField[bool](t, status, "intact") || len(agentJSONField[[]string](t, status, "problems")) == 0 {
		t.Fatal("edited reviewer reported intact")
	}
}
