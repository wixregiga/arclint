package filesystempattern_test

import (
	"os"
	"path/filepath"
	"strings"
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
  core: "The core of the sample."
rules:
  core/stdlib-only:
    description: "The core imports no other Module and no third-party package."
    on: core
    imports:
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
	if rules[0].ID().Qualified() != "arclint:core/stdlib-only" {
		t.Errorf("rule id = %q, want the local id qualified with the pattern namespace", rules[0].ID().Qualified())
	}
	if mods := p.Modules(); len(mods) != 1 || mods[0].Name().String() != "core" || mods[0].Description() != "The core of the sample." {
		t.Errorf("modules = %+v", mods)
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
	headerless := "modules:\n  core: \"core/**\"\n"
	if err := os.WriteFile(filepath.Join(pkg, filesystempattern.FileName), []byte(headerless), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	broken, err := filesystempattern.NewSource(dir)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if _, err := broken.Patterns(); err == nil {
		t.Errorf("a pattern file without an identity header must be an error, never skipped")
	} else if !strings.Contains(err.Error(), "missing pattern header") {
		t.Errorf("headerless error = %v", err)
	}
}

func TestPatternsLoadsExtensions(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "sample")
	extDir := filepath.Join(pkg, "extensions")
	if err := os.MkdirAll(extDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkg, filesystempattern.FileName), []byte(samplePattern), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	source := "export default { type: \"sample/ok\" }\n"
	if err := os.WriteFile(filepath.Join(extDir, "ok.ts"), []byte(source), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "types.d.ts"), []byte("export {}"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(extDir, ".hidden.ts"), []byte("export default {}"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "readme.md"), []byte("no"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(extDir, "helpers"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	src, err := filesystempattern.NewSource(dir)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	patterns, err := src.Patterns()
	if err != nil {
		t.Fatalf("Patterns: %v", err)
	}
	exts := patterns[0].Extensions()
	if len(exts) != 1 || exts[0].FileName() != "ok.ts" || exts[0].Source() != source {
		t.Errorf("extensions = %+v, want preserved ok.ts", exts)
	}
}

func TestPatternsMissingExtensionsDirectory(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "sample")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkg, filesystempattern.FileName), []byte(samplePattern), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	src, err := filesystempattern.NewSource(dir)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	patterns, err := src.Patterns()
	if err != nil {
		t.Fatalf("Patterns: %v", err)
	}
	if len(patterns[0].Extensions()) != 0 {
		t.Errorf("missing extensions directory must yield no extensions")
	}
}

func TestPatternsInvalidExtensionEntry(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "sample")
	extDir := filepath.Join(pkg, "extensions")
	if err := os.MkdirAll(extDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkg, filesystempattern.FileName), []byte(samplePattern), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "empty.ts"), []byte("   \n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	src, err := filesystempattern.NewSource(dir)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if _, err := src.Patterns(); err == nil {
		t.Errorf("blank extension source must be an error")
	} else if !strings.Contains(err.Error(), "empty.ts") {
		t.Errorf("invalid-entry error must name the asset path, got %v", err)
	}
}
