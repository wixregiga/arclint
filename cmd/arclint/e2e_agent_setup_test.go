package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentSetupCoordinatesAndPreserves(t *testing.T) {
	for _, host := range []string{"omp", "codex"} {
		t.Run(host, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "internal/domain/order.go", "package domain\nvar Name = \"order\"\n")
			write(t, root, "internal/app/place.go", "package app\n// supporting context\n")
			write(t, root, "AGENTS.md", "Owner guidance must survive.\n")
			args := []string{"agents", "setup", "--host", host, "--domain-source", "internal/domain/*.go", "--source", "internal/app/place.go"}
			out, stderr, code := runBin(t, root, nil, args...)
			if code != 0 {
				t.Fatalf("setup: %d %s %s", code, out, stderr)
			}
			for _, path := range []string{"rules.arclint.yaml", "domain.arclint.yaml", ".agents/skills/domain-librarian/SKILL.md", ".agents/skills/domain-librarian/VOCAB.yaml", ".arclint/schemas/domain.arclint.schema.json", ".arclint/agent-assets.json"} {
				if _, err := os.Stat(filepath.Join(root, path)); err != nil {
					t.Fatal(err)
				}
			}
			recording, _ := os.ReadFile(filepath.Join(root, "domain.arclint.yaml"))
			rules, _ := os.ReadFile(filepath.Join(root, "rules.arclint.yaml"))
			agents, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
			if !strings.Contains(string(agents), "Owner guidance must survive") || !strings.Contains(string(agents), "domain-librarian/SKILL.md") {
				t.Fatal("missing preserved guidance or skill pointer")
			}
			if bytes.Count(agents, []byte("\n")) > 25 {
				t.Fatal("setup copied the full protocol into AGENTS")
			}
			configBytes, _ := os.ReadFile(filepath.Join(root, ".arclint/domain-guard.json"))
			var scope struct{ DomainSourcePatterns, SourcePatterns []string }
			if err := json.Unmarshal(configBytes, &scope); err != nil {
				t.Fatal(err)
			}
			if len(scope.DomainSourcePatterns) != 1 || len(scope.SourcePatterns) != 1 {
				t.Fatal("lost subject/evidence distinction")
			}
			out, stderr, code = runBin(t, root, nil, args...)
			if code != 0 {
				t.Fatalf("repeat setup: %d %s %s", code, out, stderr)
			}
			for path, want := range map[string][]byte{"rules.arclint.yaml": rules, "domain.arclint.yaml": recording, "AGENTS.md": agents} {
				got, _ := os.ReadFile(filepath.Join(root, path))
				if !bytes.Equal(got, want) {
					t.Fatalf("repeat setup changed %s", path)
				}
			}
			out, stderr, code = runBin(t, root, nil, "agents", "status")
			if code != 0 || !strings.Contains(out, "Domain source subjects: internal/domain/*.go") || !strings.Contains(out, "not a review verdict") {
				t.Fatalf("status: %d %s %s", code, out, stderr)
			}
		})
	}
}

func TestAgentSetupPreservesCustomRecordingAndRequiresHost(t *testing.T) {
	root := t.TempDir()
	_, _, code := runBin(t, root, nil, "agents", "setup")
	if code != 2 {
		t.Fatalf("missing host exit %d", code)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("missing host caused writes")
	}
	write(t, root, "language.yaml", "product: Existing words\n")
	write(t, root, ".agents/skills/domain-librarian/SKILL.md", "Owner classification protocol\n")
	before, _ := os.ReadFile(filepath.Join(root, "language.yaml"))
	out, stderr, code := runBin(t, root, nil, "agents", "setup", "--host", "omp", "--domain", "language.yaml")
	if code != 0 {
		t.Fatalf("setup: %d %s %s", code, out, stderr)
	}
	skill, _ := os.ReadFile(filepath.Join(root, ".agents/skills/domain-librarian/SKILL.md"))
	if string(skill) != "Owner classification protocol\n" {
		t.Fatal("overwrote owner skill")
	}
	after, _ := os.ReadFile(filepath.Join(root, "language.yaml"))
	if !bytes.Equal(before, after) {
		t.Fatal("replaced custom recording")
	}
	if _, err := os.Stat(filepath.Join(root, "domain.arclint.yaml")); !os.IsNotExist(err) {
		t.Fatal("invented an extra recording")
	}
	script := filepath.Join(root, ".omp/extensions/arclint-domain-guard/guard.mjs")
	if err := os.WriteFile(script, []byte("user edited guard"), 0600); err != nil {
		t.Fatal(err)
	}
	_, stderr, code = runBin(t, root, nil, "agents", "setup", "--host", "omp", "--domain", "language.yaml")
	if code != 2 || !strings.Contains(stderr, "preserving existing different file") {
		t.Fatalf("overwrite not refused: %d %s", code, stderr)
	}
	got, _ := os.ReadFile(script)
	if string(got) != "user edited guard" {
		t.Fatal("overwrote guard")
	}
}

func TestAgentSetupDoesNotReplaceLegacyPolicy(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".arclint/rules.yaml", "legacy policy kept verbatim\n")
	write(t, root, "language.yaml", "product: existing\n")
	out, stderr, code := runBin(t, root, nil, "agents", "setup", "--host", "omp", "--domain", "language.yaml")
	if code != 0 {
		t.Fatalf("setup: %d %s %s", code, out, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "rules.arclint.yaml")); !os.IsNotExist(err) {
		t.Fatal("masked legacy policy with empty starter")
	}
	got, _ := os.ReadFile(filepath.Join(root, ".arclint/rules.yaml"))
	if string(got) != "legacy policy kept verbatim\n" {
		t.Fatal("changed legacy policy")
	}
	out, stderr, code = runBin(t, root, nil, "agents", "status")
	if code != 0 || !strings.Contains(out, "Legacy rules preserved") {
		t.Fatalf("status: %d %s %s", code, out, stderr)
	}
}
