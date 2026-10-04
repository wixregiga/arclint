package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowInstallerPreservesCustomRulesThroughCLI(t *testing.T) {
	root := t.TempDir()
	policy := filepath.Join(root, "custom policy.yaml")
	write(t, root, "custom policy.yaml", "languages: [go]\nzones: {}\nrules: []\n")
	stdout, stderr, code := runBin(t, root, nil, "--rules", policy, "agents", "workflow", "install", "--format", "json")
	if code != 0 {
		t.Fatalf("install: %d %s %s", code, stdout, stderr)
	}
	data, err := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Hooks) != 5 {
		t.Fatalf("hook events = %d", len(document.Hooks))
	}
	for event, groups := range document.Hooks {
		if len(groups) != 1 || len(groups[0].Hooks) != 1 {
			t.Fatalf("unexpected %s registration", event)
		}
		if !strings.Contains(groups[0].Hooks[0].Command, "--rules '"+policy+"'") {
			t.Fatalf("%s lost selected policy: %s", event, groups[0].Hooks[0].Command)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "rules.arclint.yaml")); !os.IsNotExist(err) {
		t.Fatalf("created unselected policy: %v", err)
	}
}
