package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// contextRepo materializes a small two-module repo with a consumes
// allow-list, an invariant, and a protected graph rule, so every section
// of `arclint context` output has a source.
func contextRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"rules.yaml": `runtime: [go]

modules:
  app:
    paths: ["app/**"]
    description: "Application wiring."
  domain:
    paths: ["domain/**"]
    description: "Pure business rules."

contracts:
  app:
    consumes:
      internal: [domain]
  domain:
    consumes:
      internal: []
      external: forbid
    invariants:
      - id: "dom:no-panic"
        kind: content
        files: "domain/**/*.go"
        must_not: ['\bpanic\(']

dependencies:
  - id: "dom:protected"
    kind: protected
    module: domain
    allow: [app]
`,
		"app/main.go":   "package app\n",
		"domain/d.go":   "package domain\n",
		"stray/free.go": "package stray\n",
	}
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestContextByFilePath(t *testing.T) {
	dir := contextRepo(t)
	stdout, stderr, code := runBin(t, dir, os.Environ(), "context", "domain/d.go")
	if code != 0 {
		t.Fatalf("exit = %d\nstderr: %s", code, stderr)
	}
	for _, want := range []string{
		"path: domain/d.go",
		"domain — Pure business rules.",
		"none (may import no other declared module)",
		"external imports: forbid",
		"dom:no-panic",
		"dom:protected",
		"verify: arclint check .",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output lacks %q\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "app — ") {
		t.Errorf("unrelated module app rendered:\n%s", stdout)
	}
}

func TestContextByDirectoryAndModuleName(t *testing.T) {
	dir := contextRepo(t)
	// Directory resolution unions the files underneath.
	stdout, _, code := runBin(t, dir, os.Environ(), "context", "domain")
	if code != 0 || !strings.Contains(stdout, "domain — Pure business rules.") {
		t.Fatalf("directory resolution failed (exit %d):\n%s", code, stdout)
	}
	// An exact module name wins over path resolution.
	stdout, _, code = runBin(t, dir, os.Environ(), "context", "app")
	if code != 0 || !strings.Contains(stdout, "allow: domain") {
		t.Fatalf("module-name resolution failed (exit %d):\n%s", code, stdout)
	}
	if strings.Contains(stdout, "path: app") {
		t.Errorf("module-name arg should not render a path line:\n%s", stdout)
	}
}

func TestContextJSON(t *testing.T) {
	dir := contextRepo(t)
	stdout, stderr, code := runBin(t, dir, os.Environ(), "context", "--format", "json", "domain/d.go")
	if code != 0 {
		t.Fatalf("exit = %d\nstderr: %s", code, stderr)
	}
	var out struct {
		Path    string `json:"path"`
		Modules []struct {
			Name     string `json:"name"`
			Internal string `json:"internal"`
			External string `json:"external"`
			Rules    []struct {
				ID     string `json:"id"`
				Clause string `json:"clause"`
			} `json:"rules"`
		} `json:"modules"`
		Verify string `json:"verify"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout)
	}
	if out.Path != "domain/d.go" || len(out.Modules) != 1 || out.Modules[0].Name != "domain" {
		t.Fatalf("unexpected resolution: %+v", out)
	}
	if out.Modules[0].External != "forbid" {
		t.Errorf("external = %q, want forbid", out.Modules[0].External)
	}
	ids := map[string]bool{}
	for _, r := range out.Modules[0].Rules {
		ids[r.ID] = true
	}
	for _, want := range []string{"dom:no-panic", "dom:protected"} {
		if !ids[want] {
			t.Errorf("rules lack %s: %+v", want, out.Modules[0].Rules)
		}
	}
	if out.Verify != "arclint check ." {
		t.Errorf("verify = %q", out.Verify)
	}
}

func TestContextUnownedAndUnknownPaths(t *testing.T) {
	dir := contextRepo(t)
	// A walked file owned by no module reports that plainly, exit 0.
	stdout, _, code := runBin(t, dir, os.Environ(), "context", "stray/free.go")
	if code != 0 || !strings.Contains(stdout, "modules: none") {
		t.Fatalf("unowned path (exit %d):\n%s", code, stdout)
	}
	// A path outside the walked tree is a usage error, exit 2.
	_, stderr, code := runBin(t, dir, os.Environ(), "context", "missing/nope.go")
	if code != 2 {
		t.Fatalf("unknown path: exit = %d, want 2\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, "no walked file or directory") {
		t.Errorf("stderr lacks explanation: %s", stderr)
	}
}
