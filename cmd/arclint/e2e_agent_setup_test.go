package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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

func TestRejectedAgentSetupPreservesEntireTree(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		args  []string
	}{
		{"missing custom recording", map[string]string{"owner.txt": "owner"}, []string{"--domain", "missing.yaml", "--languages", "ts"}},
		{"mixed missing default and custom recording", map[string]string{"other.yaml": "owner recording"}, []string{"--domain", "domain.arclint.yaml", "--domain", "other.yaml", "--languages", "ts"}},
		{"cleaned domain traversal", map[string]string{"sub/keep.txt": "owner", "domain.arclint.yaml": "version: 1\nproject: owner\ncontexts: {}\n"}, []string{"--domain", "sub/../domain.arclint.yaml"}},
		{"empty source glob", map[string]string{"internal/keep.txt": "owner"}, []string{"--source", "internal/**/*.go"}},
		{"source selects directory", map[string]string{"internal/keep.txt": "owner"}, []string{"--source", "internal"}},
		{"invalid source prefix", map[string]string{"owner.txt": "owner"}, []string{"--source", "missing/*.go", "--languages", "ts"}},
		{"edited guard", map[string]string{".omp/extensions/arclint-domain-guard/guard.mjs": "owner guard"}, []string{"--languages", "ts"}},
		{"invalid preserved source scope", map[string]string{".arclint/domain-guard.json": `{ "version":1,"domainFiles":["domain.arclint.yaml"],"sourcePatterns":["../outside.go"] }`}, nil},
		{"invalid preserved source shape", map[string]string{".arclint/domain-guard.json": `{ "version":1,"domainFiles":["domain.arclint.yaml"],"sourcePatterns":"src/*.go" }`}, nil},
		{"invalid receipt", map[string]string{".arclint/agent-assets.json": "broken"}, nil},
		{"conflicting workflow", map[string]string{".agents/skills/domain-librarian/ARCLINT.md": "owner workflow"}, nil},
		{"conflicting schema", map[string]string{".arclint/schemas/domain.arclint.schema.json": "owner schema"}, nil},
		{"invalid AGENTS markers", map[string]string{"AGENTS.md": "owner\n<!-- arclint:agents:begin -->"}, nil},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			for name, content := range test.files {
				write(t, root, name, content)
			}
			before := setupTree(t, root)
			args := append([]string{"agents", "setup", "--host", "omp"}, test.args...)
			out, stderr, code := runBin(t, root, nil, args...)
			if code != 2 {
				t.Fatalf("expected rejected setup: %d %s %s", code, out, stderr)
			}
			after := setupTree(t, root)
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("rejected setup changed tree: before %v after %v", before, after)
			}
		})
	}
}

func setupTree(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			result[rel+"/"] = "directory"
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			result[rel] = "symlink:" + link
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestRejectedAgentSetupPreservesSymlinkedGuidanceAndOutsideFile(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "owner.md")
	const owner = "Owner's external instructions must survive.\n"
	if err := os.WriteFile(outside, []byte(owner), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "AGENTS.md")); err != nil {
		t.Skipf("native symlinks unavailable: %v", err)
	}
	before := setupTree(t, root)
	out, stderr, code := runBin(t, root, nil, "agents", "setup", "--host", "omp")
	if code != 2 {
		t.Fatalf("accepted symlinked guidance: %d %s %s", code, out, stderr)
	}
	if !reflect.DeepEqual(before, setupTree(t, root)) {
		t.Fatal("rejected setup changed tree")
	}
	content, err := os.ReadFile(outside)
	if err != nil || string(content) != owner {
		t.Fatalf("changed external guidance: %v", err)
	}
}

func TestRejectedAgentSetupPreservesEscapingMatchedSource(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "owner.go")
	if err := os.WriteFile(outside, []byte("owner source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "internal"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "internal", "owner.go")); err != nil {
		t.Skipf("native symlinks unavailable: %v", err)
	}
	before := setupTree(t, root)
	out, stderr, code := runBin(t, root, nil, "agents", "setup", "--host", "omp", "--source", "internal/**/*.go")
	if code != 2 {
		t.Fatalf("accepted escaped matched source: %d %s %s", code, out, stderr)
	}
	if !reflect.DeepEqual(before, setupTree(t, root)) {
		t.Fatal("rejected setup changed tree")
	}
	content, err := os.ReadFile(outside)
	if err != nil || string(content) != "owner source" {
		t.Fatalf("changed external source: %v", err)
	}
}
