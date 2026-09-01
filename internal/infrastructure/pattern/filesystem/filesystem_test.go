package filesystempattern_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	filesystempattern "github.com/wixregiga/arclint/internal/infrastructure/pattern/filesystem"
)

const samplePattern = `pattern:
  namespace: acme
  name: sample
  version: 1.0.0
coverage: [go]
modules:
  - name: core
    paths: ["internal/core/**"]
rules:
  - id: acme:core/stdlib-only
    kind: consumes
    module: core
    allow: []
    forbid: [external]
extensions:
  - name: acme/sample/check
    entry: extensions/check.ts
tests:
  root: tests
`

func TestPatternsEnumeratesPackageRootsAndUsesManifestLoader(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, "sample", samplePattern)
	writeFile(t, filepath.Join(dir, "sample", "extensions", "check.ts"), "export default { type: \"sample/check\" }\n")
	writeFile(t, filepath.Join(dir, "sample", "tests", "stdlib.yaml"), `rule: acme:core/stdlib-only
files:
  internal/core/value.go: |
    package core
expect: []
`)
	writeFile(t, filepath.Join(dir, "README.md"), "not a package")
	if err := os.Mkdir(filepath.Join(dir, "not-a-package"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	source, err := filesystempattern.NewSource(dir)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	patterns, err := source.Patterns()
	if err != nil {
		t.Fatalf("Patterns: %v", err)
	}
	if len(patterns) != 1 {
		t.Fatalf("patterns = %d, want 1", len(patterns))
	}
	p := patterns[0]
	if got := p.Reference().String(); got != "acme/sample@1.0.0" {
		t.Errorf("reference = %q", got)
	}
	if len(p.Modules()) != 1 || len(p.Rules()) != 1 || len(p.Extensions()) != 1 || len(p.Tests()) != 1 {
		t.Errorf("loaded package counts = modules %d, rules %d, extensions %d, tests %d", len(p.Modules()), len(p.Rules()), len(p.Extensions()), len(p.Tests()))
	}
	if p.Digest() == "" {
		t.Error("full-tree digest is empty")
	}
	if ref, ok := p.Rules()[0].Provenance(); !ok || ref.String() != "acme/sample@1.0.0" {
		t.Errorf("carried Rule provenance = (%v, %v)", ref, ok)
	}
}

func TestPatternsMissingDirectoryMeansNoLocalPatterns(t *testing.T) {
	source, err := filesystempattern.NewSource(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	patterns, err := source.Patterns()
	if err != nil || patterns != nil {
		t.Errorf("missing directory must mean no patterns, got (%v, %v)", patterns, err)
	}
}

func TestPatternsReportsInvalidManifestLocation(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, "broken", `pattern:
  namespace: acme
  name: broken
  version: latest
modules: []
rules: []
`)
	source, err := filesystempattern.NewSource(dir)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	_, err = source.Patterns()
	if err == nil {
		t.Fatal("invalid exact version loaded")
	}
	if !strings.Contains(err.Error(), "pattern.version") {
		t.Errorf("error = %v, want manifest location pattern.version", err)
	}
}

func writePackage(t *testing.T, root, name, manifest string) {
	t.Helper()
	writeFile(t, filepath.Join(root, name, filesystempattern.FileName), manifest)
}

func writeFile(t *testing.T, file, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
