package filesystempattern_test

import (
	"os"
	"path/filepath"
	"testing"

	filesystempattern "github.com/wixregiga/arclint/internal/infrastructure/pattern/filesystem"
)

const samplePattern = `
pattern:
  namespace: arclint
  name: sample
  version: 1.0.0
  coverage: [go]
modules:
  core:
    paths: ["internal/core/**"]
contracts:
  core:
    consumes:
      id: "arclint:core/stdlib-only"
      internal: []
      external: forbid
`

func TestPatternsLoadsValidatedPackages(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "sample")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkg, filesystempattern.FileName), []byte(samplePattern), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
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
	if p.Reference().String() != "arclint/sample@1.0.0" {
		t.Errorf("reference = %q", p.Reference())
	}
	rules := p.Rules()
	if len(rules) != 1 {
		t.Fatalf("pattern rules = %d, want 1", len(rules))
	}
	if ref, ok := rules[0].Provenance(); !ok || ref.Name() != "sample" {
		t.Errorf("carried rule lacks pattern provenance")
	}
}

func TestPatternsAbsenceAndInvalidity(t *testing.T) {
	source, err := filesystempattern.NewSource(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	patterns, err := source.Patterns()
	if err != nil || patterns != nil {
		t.Errorf("missing directory must mean no patterns, got (%v, %v)", patterns, err)
	}

	dir := t.TempDir()
	pkg := filepath.Join(dir, "broken")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	headerless := "modules:\n  core:\n    paths: [\"core/**\"]\n"
	if err := os.WriteFile(filepath.Join(pkg, filesystempattern.FileName), []byte(headerless), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	broken, err := filesystempattern.NewSource(dir)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if _, err := broken.Patterns(); err == nil {
		t.Errorf("a pattern file without an identity header must be an error, never skipped")
	}
}
