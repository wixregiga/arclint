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

func TestAgentCommandsRenderWholeJSON(t *testing.T) {
	for _, host := range []string{"codex", "omp"} {
		t.Run(host, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "internal/domain/order.go", "package domain\nvar Name = \"order\"\n")
			write(t, root, "internal/app/place.go", "package app\n// supporting context\n")
			setup := runAgentJSON(t, root, "--format=json", "agents", "setup", "--host", host, "--domain-source", "internal/domain/*.go", "--source", "internal/app/place.go")
			if agentJSONField[string](t, setup, "operation") != "setup" || agentJSONField[string](t, setup, "host") != host {
				t.Fatalf("wrong setup facts: %s", setup)
			}
			if len(agentJSONField[[]string](t, setup, "paths")) == 0 || agentJSONField[string](t, setup, "activation") == "" {
				t.Fatal("missing paths or host guidance")
			}
			status := runAgentJSON(t, root, "agents", "status", "--format", "json")
			if agentJSONField[string](t, status, "project") != root || !agentJSONField[bool](t, status, "intact") {
				t.Fatalf("wrong installed status: %s", status)
			}
			if got := agentJSONField[[]string](t, status, "domainFiles"); len(got) != 1 || got[0] != "domain.arclint.yaml" {
				t.Fatalf("recordings %v", got)
			}
			if got := agentJSONField[[]string](t, status, "domainSourcePatterns"); len(got) != 1 || got[0] != "internal/domain/*.go" {
				t.Fatalf("subject scope %v", got)
			}
			if got := agentJSONField[[]string](t, status, "sourcePatterns"); len(got) != 1 || got[0] != "internal/app/place.go" {
				t.Fatalf("evidence scope %v", got)
			}
			if got := agentJSONField[[]string](t, status, "installedHosts"); len(got) != 1 || !strings.EqualFold(got[0], host) {
				t.Fatalf("installed hosts %v", got)
			}
			if len(agentJSONField[[]string](t, status, "changedAssets")) != 0 || !strings.Contains(agentJSONField[string](t, status, "limits"), "not a review verdict") {
				t.Fatal("incorrect integrity or missing limits")
			}
			path := filepath.Join(root, ".agents/skills/domain-librarian/SKILL.md")
			if err := os.WriteFile(path, []byte("owner edited instructions\n"), 0600); err != nil {
				t.Fatal(err)
			}
			status = runAgentJSON(t, root, "--format=json", "agents", "status")
			if agentJSONField[bool](t, status, "intact") || len(agentJSONField[[]string](t, status, "changedAssets")) == 0 {
				t.Fatal("edited asset reported intact")
			}
			hooksRoot := t.TempDir()
			write(t, hooksRoot, "language.yaml", "product: existing language\n")
			hooks := runAgentJSON(t, hooksRoot, "agents", "hooks", "--host", host, "--domain", "language.yaml", "--format", "json")
			if agentJSONField[string](t, hooks, "operation") != "hooks" || agentJSONField[string](t, hooks, "host") != host || len(agentJSONField[[]string](t, hooks, "paths")) == 0 {
				t.Fatal("wrong hooks installation result")
			}
		})
	}
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

func TestAgentStatusDoesNotCertifyAbsentReceiptEvidence(t *testing.T) {
	for _, state := range []string{"empty", "missing", "invalid", "omitted-script", "unrelated-hooks"} {
		t.Run(state, func(t *testing.T) {
			root := t.TempDir()
			runAgentJSON(t, root, "--format=json", "agents", "setup", "--host", "codex")
			receiptPath := filepath.Join(root, ".arclint/agent-assets.json")
			switch state {
			case "empty":
				if err := os.WriteFile(receiptPath, []byte("{}\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "invalid":
				if err := os.WriteFile(receiptPath, []byte("{broken"), 0600); err != nil {
					t.Fatal(err)
				}
			case "missing":
				if err := os.Remove(receiptPath); err != nil {
					t.Fatal(err)
				}
			case "omitted-script":
				data, err := os.ReadFile(receiptPath)
				if err != nil {
					t.Fatal(err)
				}
				var receipt map[string]string
				if err := json.Unmarshal(data, &receipt); err != nil {
					t.Fatal(err)
				}
				delete(receipt, ".codex/hooks/arclint-domain-guard/guard.py")
				data, err = json.Marshal(receipt)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(receiptPath, data, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(filepath.Join(root, ".codex/hooks/arclint-domain-guard/guard.py")); err != nil {
					t.Fatal(err)
				}
			case "unrelated-hooks":
				if err := os.WriteFile(receiptPath, []byte("{}\n"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(filepath.Join(root, ".codex/hooks/arclint-domain-guard/guard.py")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".codex/hooks.json"), []byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo unrelated"}]}]}}`), 0600); err != nil {
					t.Fatal(err)
				}
			}
			doc := runAgentJSON(t, root, "--format=json", "agents", "status")
			if agentJSONField[bool](t, doc, "intact") {
				t.Fatalf("%s receipt state certified integrity", state)
			}
			if state == "empty" || state == "missing" || state == "invalid" {
				problems := agentJSONField[[]string](t, doc, "problems")
				if len(problems) == 0 || !strings.Contains(strings.Join(problems, " "), "receipt") {
					t.Fatalf("unverified integrity lacks useful explanation: %v", problems)
				}
			}
			if state == "omitted-script" {
				changed := agentJSONField[[]string](t, doc, "changedAssets")
				if !strings.Contains(strings.Join(changed, " "), "guard.py") {
					t.Fatalf("missing required script hidden by receipt omission: %v", changed)
				}
			}
			if state == "omitted-script" || state == "unrelated-hooks" {
				if hosts := agentJSONField[[]string](t, doc, "installedHosts"); len(hosts) != 0 {
					t.Fatalf("incomplete/unrelated guard reported installed: %v", hosts)
				}
			}
		})
	}
}

func TestOMPStatusRequiresGuardImplementationDespiteReceiptOmission(t *testing.T) {
	root := t.TempDir()
	runAgentJSON(t, root, "--format=json", "agents", "setup", "--host", "omp")
	receiptPath := filepath.Join(root, ".arclint/agent-assets.json")
	data, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt map[string]string
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	script := ".omp/extensions/arclint-domain-guard/guard.mjs"
	delete(receipt, script)
	data, err = json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receiptPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, script)); err != nil {
		t.Fatal(err)
	}
	doc := runAgentJSON(t, root, "--format=json", "agents", "status")
	if agentJSONField[bool](t, doc, "intact") || len(agentJSONField[[]string](t, doc, "installedHosts")) != 0 {
		t.Fatal("entrypoint alone was certified as an intact OMP guard")
	}
	if changed := agentJSONField[[]string](t, doc, "changedAssets"); !strings.Contains(strings.Join(changed, " "), "guard.mjs") {
		t.Fatalf("missing implementation hidden by receipt omission: %v", changed)
	}
}
