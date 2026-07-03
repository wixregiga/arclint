package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jofyi/arclint/internal/answers"
)

// newTestRepo builds a temp repo with one "svc" template and returns its root.
func newTestRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeRepoFiles(t, root, map[string]string{
		".arclint/templates/svc/template.yaml": `version: 1
description: "A tiny test service"
destination: "services/{{ name | kebab }}"
variables:
  - name: name
    description: "Service name"
    type: string
    validate: "^[a-zA-Z][a-zA-Z0-9 _-]*$"
  - name: transport
    description: "Transport"
    type: choice
    choices: [http, grpc]
    default: http
  - name: with_db
    description: "Owns a database?"
    type: bool
    default: false
  - name: db_name
    description: "Database name"
    type: string
    default: "{{ name | snake }}"
    when: "with_db == true"
`,
		".arclint/templates/svc/files/main.go":     "package main // {{ name | pascal }} {{ transport }}\n",
		".arclint/templates/svc/files/service.yml": "name: {{ name | kebab }}\ndb: {{ db_name }}\n",
		".arclint/templates/docs/template.yaml": `version: 1
description: "A docs page"
destination: "docs/{{ title | kebab }}"
variables:
  - name: title
    description: "Title"
    type: string
`,
		".arclint/templates/docs/files/index.md": "# {{ title }}\n",
	})
	return root
}

func writeRepoFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, content := range files {
		abs := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// runArclint executes a fresh command tree (isolated flag state) inside dir
// and returns stdout, stderr, and the exit code per the Execute contract.
func runArclint(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()
	t.Chdir(dir)
	root := newRootCmd()
	root.AddCommand(newNewCmd())
	root.AddCommand(newMakeCmd())
	var so, se bytes.Buffer
	root.SetOut(&so)
	root.SetErr(&se)
	root.SetArgs(args)
	err := root.Execute()
	code := 0
	if err != nil {
		var xe *ExitError
		if errors.As(err, &xe) {
			code = xe.Code
			if xe.Err != nil {
				fmt.Fprintf(&se, "error: %s\n", xe.Err)
			}
		} else {
			code = ExitUsage
			fmt.Fprintf(&se, "error: %s\n", err)
		}
	}
	return so.String(), se.String(), code
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestNewGeneratesUnitAndShard(t *testing.T) {
	root := newTestRepo(t)
	stdout, stderr, code := runArclint(t, root, "new", "svc", "pay gw", "--var", "with_db=true", "--var", "db_name=payments")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	main := mustReadFile(t, filepath.Join(root, "services/pay-gw/main.go"))
	if main != "package main // PayGw http\n" {
		t.Errorf("main.go = %q", main)
	}
	svc := mustReadFile(t, filepath.Join(root, "services/pay-gw/service.yml"))
	if svc != "name: pay-gw\ndb: payments\n" {
		t.Errorf("service.yml = %q", svc)
	}
	if !strings.Contains(stdout, "created services/pay-gw/main.go") {
		t.Errorf("stdout missing created line: %q", stdout)
	}

	u, err := answers.Load(answers.Path(root, "services/pay-gw"))
	if err != nil {
		t.Fatal(err)
	}
	if u.Template != "svc" || u.TemplateVersion != 1 {
		t.Errorf("shard = %+v", u)
	}
	if u.Answers["name"] != "pay gw" || u.Answers["db_name"] != "payments" || u.Answers["with_db"] != "true" {
		t.Errorf("answers = %v", u.Answers)
	}
	if len(u.Files) != 2 {
		t.Errorf("file hashes = %v", u.Files)
	}
}

func TestNewSkippedWhenVariableAbsentFromShard(t *testing.T) {
	root := newTestRepo(t)
	_, stderr, code := runArclint(t, root, "new", "svc", "plain")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	u, err := answers.Load(answers.Path(root, "services/plain"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := u.Answers["db_name"]; ok {
		t.Errorf("when-skipped variable must be absent, got %v", u.Answers)
	}
	// Skipped variable renders as empty string.
	svc := mustReadFile(t, filepath.Join(root, "services/plain/service.yml"))
	if svc != "name: plain\ndb: \n" {
		t.Errorf("service.yml = %q", svc)
	}
}

func TestNewUnknownThingSuggestsClosest(t *testing.T) {
	root := newTestRepo(t)
	_, stderr, code := runArclint(t, root, "new", "svcc", "x")
	if code != ExitUsage {
		t.Fatalf("exit %d, want 2", code)
	}
	for _, part := range []string{`unknown template "svcc"`, "available:", "docs", "svc", `did you mean "svc"?`} {
		if !strings.Contains(stderr, part) {
			t.Errorf("stderr missing %q: %s", part, stderr)
		}
	}
}

func TestNewDestinationExistsRefuses(t *testing.T) {
	root := newTestRepo(t)
	if _, stderr, code := runArclint(t, root, "new", "svc", "dup"); code != 0 {
		t.Fatalf("first new failed: %s", stderr)
	}
	_, stderr, code := runArclint(t, root, "new", "svc", "dup")
	if code != ExitUsage {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr, "already exists") || !strings.Contains(stderr, "arclint make services/dup") {
		t.Errorf("stderr must point to arclint make: %s", stderr)
	}
}

func TestNewCrossTemplateCollisionRefuses(t *testing.T) {
	root := newTestRepo(t)
	if _, stderr, code := runArclint(t, root, "new", "svc", "shared"); code != 0 {
		t.Fatalf("first new failed: %s", stderr)
	}
	// docs template forced onto the svc unit's destination via --out.
	_, stderr, code := runArclint(t, root, "new", "docs", "--var", "title=Shared", "--out", "services/shared")
	if code != ExitUsage {
		t.Fatalf("exit %d, want 2", code)
	}
	for _, part := range []string{`already claimed by template "svc"`, `"docs"`} {
		if !strings.Contains(stderr, part) {
			t.Errorf("stderr must name both templates, missing %q: %s", part, stderr)
		}
	}
}

func TestNewDryRunWritesNothing(t *testing.T) {
	root := newTestRepo(t)
	stdout, stderr, code := runArclint(t, root, "new", "svc", "ghost", "--dry-run")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "would create services/ghost/main.go") {
		t.Errorf("stdout = %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(root, "services/ghost")); !os.IsNotExist(err) {
		t.Error("dry-run must not write the destination")
	}
	if _, err := os.Stat(answers.Path(root, "services/ghost")); !os.IsNotExist(err) {
		t.Error("dry-run must not write an answers shard")
	}
}

func TestNewNoInputMissingRequiredExits2(t *testing.T) {
	root := newTestRepo(t)
	_, stderr, code := runArclint(t, root, "new", "svc", "--no-input")
	if code != ExitUsage {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr, `missing required input "name"`) || !strings.Contains(stderr, "--var name=") {
		t.Errorf("stderr = %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "services")); !os.IsNotExist(err) {
		t.Error("nothing may be written when required input is missing")
	}
}

func TestNewFlagValidationErrors(t *testing.T) {
	root := newTestRepo(t)
	_, stderr, code := runArclint(t, root, "new", "svc", "x", "--var", "transport=carrier-pigeon")
	if code != ExitUsage || !strings.Contains(stderr, "not an allowed choice") {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	_, stderr, code = runArclint(t, root, "new", "svc", "x", "--var", "trnsport=http")
	if code != ExitUsage || !strings.Contains(stderr, `unknown variable "trnsport"`) {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	_, stderr, code = runArclint(t, root, "new", "svc", "9bad")
	if code != ExitUsage || !strings.Contains(stderr, "fails pattern") {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
}

func TestNewList(t *testing.T) {
	root := newTestRepo(t)
	stdout, stderr, code := runArclint(t, root, "new", "--list")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "svc — A tiny test service") || !strings.Contains(stdout, "docs — A docs page") {
		t.Errorf("list output = %q", stdout)
	}
}

func TestNewOutOverride(t *testing.T) {
	root := newTestRepo(t)
	_, stderr, code := runArclint(t, root, "new", "svc", "custom", "--out", "elsewhere/custom")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "elsewhere/custom/main.go")); err != nil {
		t.Errorf("--out destination missing: %v", err)
	}
	u, err := answers.Load(answers.Path(root, "elsewhere/custom"))
	if err != nil || u.Destination != "elsewhere/custom" {
		t.Errorf("shard destination = %+v, %v", u, err)
	}
}

// TestNewOutAllowsDotDotCachePrefix pins the low-6 fix: a legit directory name
// like "..cache" (which starts with "..") must be accepted, not rejected by a
// naive prefix check.
func TestNewOutAllowsDotDotCachePrefix(t *testing.T) {
	root := newTestRepo(t)
	_, stderr, code := runArclint(t, root, "new", "svc", "cached", "--out", "..cache/svc")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "..cache/svc/main.go")); err != nil {
		t.Errorf("--out ..cache/svc should be written: %v", err)
	}
	// A real traversal must still be refused.
	_, stderr, code = runArclint(t, root, "new", "svc", "escape", "--out", "../evil")
	if code != ExitUsage || !strings.Contains(stderr, "relative path inside the repo") {
		t.Fatalf("traversal not refused: exit %d, stderr %q", code, stderr)
	}
}

// TestNewPartialWriteUnwinds pins the blocker-2 fix: if the write cannot
// complete (here the destination parent is read-only, so the temp render dir
// cannot be created under it), no partial unit and no shard are left behind,
// and the destination is not silently blocked from a later re-run.
func TestNewPartialWriteUnwinds(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: read-only dir permission is not enforced")
	}
	root := newTestRepo(t)
	// Pre-create the destination parent read-only so temp-dir creation fails
	// after the refusal checks but before any file lands under the unit.
	parent := filepath.Join(root, "services")
	if err := os.MkdirAll(parent, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })

	_, stderr, code := runArclint(t, root, "new", "svc", "thing")
	if code == 0 {
		t.Fatalf("expected failure, got success; stderr %q", stderr)
	}

	if err := os.Chmod(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	// No partial unit directory left behind.
	if _, err := os.Stat(filepath.Join(root, "services/thing")); !os.IsNotExist(err) {
		t.Errorf("partial unit must be unwound; services/thing still exists (err=%v)", err)
	}
	// No shard recorded.
	if _, err := os.Stat(answers.Path(root, "services/thing")); !os.IsNotExist(err) {
		t.Errorf("no shard may be recorded on partial write")
	}
	// No leftover temp dirs under the destination parent, so a re-run is clean.
	entries, err := os.ReadDir(parent)
	if err == nil {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".arclint-new-") {
				t.Errorf("leftover temp dir: %s", e.Name())
			}
		}
	}
	// Re-run now succeeds (destination was never blocked).
	if _, stderr, code := runArclint(t, root, "new", "svc", "thing"); code != 0 {
		t.Fatalf("re-run after unwind failed: exit %d, stderr %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "services/thing/main.go")); err != nil {
		t.Errorf("re-run should produce the unit: %v", err)
	}
}
