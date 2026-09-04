package filesystempattern_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/distribution"
	filesystempattern "github.com/wixregiga/arclint/internal/infrastructure/pattern/filesystem"
)

const samplePattern = `
pattern:
  namespace: acme
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

// writePackage lays a pattern.yaml and extension files out under
// <dir>/<namespace>/<name>.
func writePackage(t *testing.T, dir, namespace, name, doc string, extensions map[string]string) string {
	t.Helper()
	pkg := filepath.Join(dir, namespace, name)
	if err := os.MkdirAll(filepath.Join(pkg, "extensions"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkg, distribution.PatternFileName), []byte(doc), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	for file, content := range extensions {
		if err := os.WriteFile(filepath.Join(pkg, "extensions", file), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	return pkg
}

func TestAvailableLoadsAuthoredPackages(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, "acme", "sample", samplePattern, nil)
	source, err := filesystempattern.NewSource(dir)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	available, err := source.Available()
	if err != nil {
		t.Fatalf("Available: %v", err)
	}
	if len(available) != 1 {
		t.Fatalf("available = %d, want 1", len(available))
	}
	a := available[0]
	if a.Kind != distribution.SourceLocal || !a.Authored {
		t.Errorf("a package without manifest.json is authored local: %+v", a.Kind)
	}
	if a.Reference().String() != "acme/sample@1.0.0" {
		t.Errorf("reference = %q", a.Reference())
	}
	if files := a.Vendored.Files(); len(files) != 1 || files[0].Path() != "pattern.yaml" {
		t.Errorf("files = %+v, want pattern.yaml only", files)
	}
	rules := a.Pattern.Rules()
	if len(rules) != 1 || rules[0].ID().Qualified() != "acme/sample:core/stdlib-only" {
		t.Errorf("rules = %+v", rules)
	}
	if ref, ok := rules[0].Provenance(); !ok || ref.Name() != "sample" {
		t.Errorf("carried rule lacks pattern provenance")
	}
	patterns, err := source.Patterns()
	if err != nil || len(patterns) != 1 || patterns[0].Reference() != a.Reference() {
		t.Errorf("Patterns = %v, %v", patterns, err)
	}
}

func TestAvailableAbsenceAndLayout(t *testing.T) {
	source, err := filesystempattern.NewSource(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if available, err := source.Available(); err != nil || available != nil {
		t.Errorf("missing directory must mean no patterns, got (%v, %v)", available, err)
	}

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "acme", "empty"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("notes"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".hidden", "x"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	source, err = filesystempattern.NewSource(dir)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if available, err := source.Available(); err != nil || len(available) != 0 {
		t.Errorf("directories without pattern.yaml are not packages, got (%v, %v)", available, err)
	}

	flat := t.TempDir()
	if err := os.MkdirAll(filepath.Join(flat, "sample"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(flat, "sample", distribution.PatternFileName), []byte(samplePattern), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	source, err = filesystempattern.NewSource(flat)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if _, err := source.Available(); err == nil || !strings.Contains(err.Error(), "<namespace>/<name>/pattern.yaml") {
		t.Errorf("a pattern.yaml directly under the namespace level must explain the layout, got %v", err)
	}

	mismatched := t.TempDir()
	writePackage(t, mismatched, "other", "sample", samplePattern, nil)
	source, err = filesystempattern.NewSource(mismatched)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if _, err := source.Available(); err == nil || !strings.Contains(err.Error(), "declares acme/sample") {
		t.Errorf("a directory disagreeing with the header must fail, got %v", err)
	}

	headerless := t.TempDir()
	writePackage(t, headerless, "acme", "broken", "modules:\n  core: \"core/**\"\n", nil)
	source, err = filesystempattern.NewSource(headerless)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if _, err := source.Available(); err == nil || !strings.Contains(err.Error(), "missing pattern header") {
		t.Errorf("a pattern file without an identity header must be an error, never skipped, got %v", err)
	}
}

func TestAvailableLoadsExtensions(t *testing.T) {
	dir := t.TempDir()
	source := "export default { type: \"sample/ok\" }\n"
	pkg := writePackage(t, dir, "acme", "sample", samplePattern, map[string]string{
		"ok.ts":        source,
		"types.d.ts":   "export {}",
		".hidden.ts":   "export default {}",
		"readme.md":    "no",
		"package.json": "{}",
	})
	if err := os.MkdirAll(filepath.Join(pkg, "extensions", "helpers"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "extensions", "helpers", "util.ts"), []byte("export const x = 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	src, err := filesystempattern.NewSource(dir)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	available, err := src.Available()
	if err != nil {
		t.Fatalf("Available: %v", err)
	}
	exts := available[0].Pattern.Extensions()
	if len(exts) != 1 || exts[0].FileName() != "ok.ts" || exts[0].Source() != source {
		t.Errorf("extensions = %+v, want the installable ok.ts only", exts)
	}
	var shipped []string
	for _, f := range available[0].Vendored.Files() {
		shipped = append(shipped, f.Path())
	}
	want := "extensions/helpers/util.ts extensions/ok.ts extensions/package.json extensions/readme.md extensions/types.d.ts pattern.yaml"
	if got := strings.Join(shipped, " "); got != want {
		t.Errorf("shipped files = %q, want %q", got, want)
	}
}

func TestAvailableInvalidExtensionEntry(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, "acme", "sample", samplePattern, map[string]string{"empty.ts": "   \n"})
	src, err := filesystempattern.NewSource(dir)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if _, err := src.Available(); err == nil || !strings.Contains(err.Error(), "empty.ts") {
		t.Errorf("blank extension source must be an error naming the file, got %v", err)
	}
}

func TestWriteVendorsAndVerifies(t *testing.T) {
	origin := t.TempDir()
	writePackage(t, origin, "acme", "sample", samplePattern, map[string]string{"ok.ts": "export default { type: \"sample/ok\" }\n"})
	from, err := filesystempattern.NewSource(origin)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	available, err := from.Available()
	if err != nil {
		t.Fatalf("Available: %v", err)
	}

	dir := t.TempDir()
	store, err := filesystempattern.NewSource(filepath.Join(dir, ".arclint", "patterns"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	stored, err := store.Write(available[0].Vendored)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if stored.Replaced != "" || !strings.HasSuffix(filepath.ToSlash(stored.Path), ".arclint/patterns/acme/sample") {
		t.Errorf("stored = %+v", stored)
	}
	vendored, err := store.Available()
	if err != nil {
		t.Fatalf("Available after write: %v", err)
	}
	if len(vendored) != 1 || vendored[0].Authored || vendored[0].Kind != distribution.SourceLocal {
		t.Fatalf("vendored = %+v", vendored)
	}
	if !vendored[0].Digest().Equals(available[0].Digest()) {
		t.Errorf("vendored digest %s differs from origin %s", vendored[0].Digest(), available[0].Digest())
	}
	if _, err := os.Stat(filepath.Join(store.Dir(), "acme", "sample", distribution.ManifestFileName)); err != nil {
		t.Errorf("manifest.json not written: %v", err)
	}

	newer := strings.Replace(samplePattern, "version: 1.0.0", "version: 1.1.0", 1)
	originPkg := writePackage(t, origin, "acme", "sample", newer, nil)
	if err := os.Remove(filepath.Join(originPkg, "extensions", "ok.ts")); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	available, err = from.Available()
	if err != nil {
		t.Fatalf("Available: %v", err)
	}
	stored, err = store.Write(available[0].Vendored)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if stored.Replaced != "1.0.0" {
		t.Errorf("replaced = %q, want 1.0.0", stored.Replaced)
	}
	if _, err := os.Stat(filepath.Join(store.Dir(), "acme", "sample", "extensions", "ok.ts")); err == nil {
		t.Errorf("replacing a package must drop the previous files")
	}
	entries, err := os.ReadDir(filepath.Join(store.Dir(), "acme"))
	if err != nil || len(entries) != 1 {
		t.Errorf("staging directories must not linger: %v %v", entries, err)
	}

	drifted := filepath.Join(store.Dir(), "acme", "sample", distribution.PatternFileName)
	if err := os.WriteFile(drifted, []byte(newer+"\n# edited\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := store.Available(); err == nil || !strings.Contains(err.Error(), "has digest") || !strings.Contains(err.Error(), "patterns vendor") {
		t.Errorf("an edited vendored file must fail with guidance, got %v", err)
	}
	if err := os.Remove(filepath.Join(store.Dir(), "acme", "sample", distribution.ManifestFileName)); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	authored, err := store.Available()
	if err != nil || len(authored) != 1 || !authored[0].Authored {
		t.Errorf("without manifest.json the package is authored again: %v %v", authored, err)
	}
}
