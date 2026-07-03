package cli

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jofyi/arclint/internal/config"
)

// expectedTree is every file a full (non-bare) init must create, relative
// to the repo root.
var expectedTree = []string{
	".arclint/rules.yaml",
	".arclint/answers/.gitkeep",
	".arclint/templates/repo/template.yaml",
	".arclint/templates/repo/files/README.md",
	".arclint/templates/repo/files/.github/workflows/ci.yml",
	".arclint/templates/service/template.yaml",
	".arclint/templates/service/files/README.md",
	".arclint/templates/service/files/service.yaml",
	".arclint/templates/service/files/cmd/{{ name | kebab }}/main.go",
	".arclint/templates/service/files/internal/handler.go",
	".arclint/templates/service/files/internal/handler_test.go",
	".arclint/templates/component/template.yaml",
	".arclint/templates/component/files/README.md",
	".arclint/templates/component/files/component.yaml",
}

func TestInitCreatesExpectedTree(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	if err := runInit(dir, initOptions{}, &out); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	for _, rel := range expectedTree {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}

	// No embed-encoding artifacts may leak to disk.
	err := filepath.WalkDir(filepath.Join(dir, ".arclint"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(d.Name(), ".tmpl") {
			t.Errorf("undecoded .tmpl suffix on disk: %s", p)
		}
		if strings.Contains(d.Name(), "__name_kebab__") {
			t.Errorf("undecoded placeholder on disk: %s", p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	// Summary output: created tree plus next-steps hints.
	for _, want := range []string{"created .arclint/", "templates/service/", "try: arclint check"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestInitRefusesWhenArclintExists(t *testing.T) {
	dir := t.TempDir()
	if err := runInit(dir, initOptions{Quiet: true}, os.Stderr); err != nil {
		t.Fatalf("first runInit: %v", err)
	}
	// Drop a marker to prove the refusal touches nothing.
	marker := filepath.Join(dir, ".arclint", "marker.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runInit(dir, initOptions{Quiet: true}, os.Stderr)
	if err == nil {
		t.Fatal("second runInit succeeded; want exit-2 refusal")
	}
	var xe *ExitError
	if !errors.As(err, &xe) || xe.Code != ExitUsage {
		t.Fatalf("want ExitError code %d, got %v", ExitUsage, err)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("refusal must suggest --force, got: %v", err)
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Errorf("refusal must not touch existing .arclint/: %v", statErr)
	}
}

func TestInitForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	if err := runInit(dir, initOptions{Quiet: true}, os.Stderr); err != nil {
		t.Fatalf("first runInit: %v", err)
	}
	marker := filepath.Join(dir, ".arclint", "marker.txt")
	if err := os.WriteFile(marker, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runInit(dir, initOptions{Force: true, Quiet: true}, os.Stderr); err != nil {
		t.Fatalf("force runInit: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("--force must replace .arclint/ wholesale; marker survived (err=%v)", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".arclint", "rules.yaml")); err != nil {
		t.Errorf("rules.yaml missing after --force: %v", err)
	}

	// The swap must not leave any staging or backup directories behind in
	// the repo root — only .arclint/ itself should exist.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	for _, e := range entries {
		if e.Name() != ".arclint" {
			t.Errorf("stray entry left in repo root after --force swap: %s", e.Name())
		}
	}
}

// TestInitForcePreservesOldConfigOnStagingFailure simulates a fallible write
// failing mid-staging (docs/design/cli.md init contract: --force must never
// leave the user with a deleted config and a partial replacement). Making
// root read-only after the pre-existing .arclint/ is populated blocks
// os.MkdirTemp from creating the staging directory, so runInit must fail
// before ever touching the real .arclint/. The original .arclint/ (with its
// marker file) must survive untouched, and no leftover staging directory
// should remain in the root once permissions are restored.
func TestInitForcePreservesOldConfigOnStagingFailure(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "locked")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runInit(root, initOptions{Quiet: true}, os.Stderr); err != nil {
		t.Fatalf("first runInit: %v", err)
	}
	marker := filepath.Join(root, ".arclint", "marker.txt")
	if err := os.WriteFile(marker, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })

	err := runInit(root, initOptions{Force: true, Quiet: true}, os.Stderr)
	if err == nil {
		t.Fatal("runInit with unwritable root succeeded; want staging failure")
	}
	if chmodErr := os.Chmod(root, 0o755); chmodErr != nil {
		t.Fatal(chmodErr)
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Errorf("original .arclint/ must survive a staging failure: %v", statErr)
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil {
		t.Fatalf("ReadDir(%s): %v", root, readErr)
	}
	for _, e := range entries {
		if e.Name() != ".arclint" {
			t.Errorf("stray entry left after failed staging: %s", e.Name())
		}
	}
}

func TestInitBareSkipsTemplates(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	if err := runInit(dir, initOptions{Bare: true}, &out); err != nil {
		t.Fatalf("runInit --bare: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".arclint", "rules.yaml")); err != nil {
		t.Errorf("bare init must still write rules.yaml: %v", err)
	}
	tdir := filepath.Join(dir, ".arclint", "templates")
	entries, err := os.ReadDir(tdir)
	if err != nil {
		t.Fatalf("bare init must create templates/: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("bare init must leave templates/ empty, found %d entries", len(entries))
	}
}

// TestInitRulesLoadAndValidate is the config half of the Gate 3 acceptance:
// the rules.yaml a fresh init writes must parse, schema-validate, and pass
// the semantic checks via the real config loader.
func TestInitRulesLoadAndValidate(t *testing.T) {
	dir := t.TempDir()
	if err := runInit(dir, initOptions{Quiet: true}, os.Stderr); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	f, err := config.Load(config.RulesPath(dir))
	if err != nil {
		t.Fatalf("config.Load on the default rules.yaml: %v", err)
	}
	if f.Version != 1 {
		t.Errorf("version = %d, want 1", f.Version)
	}
	for _, id := range []string{
		"require-ci-config", "no-utils-dir",
		"go-file-naming", "package-dir-naming",
		"layered-architecture", "domain-stays-pure",
		"no-println-outside-cmd", "openapi-lint",
	} {
		if _, ok := f.Rules[id]; !ok {
			t.Errorf("default rules.yaml missing rule %q", id)
		}
	}

	// Fresh-init cleanliness: rules that require files an empty repo lacks
	// must ship "off" so `arclint check` exits 0 right after init.
	if got := f.Rules["require-ci-config"].Severity; got != config.SeverityOff {
		t.Errorf("require-ci-config severity = %q; must be %q for a clean fresh init", got, config.SeverityOff)
	}
	if got := f.Rules["openapi-lint"].Severity; got != config.SeverityOff {
		t.Errorf("openapi-lint severity = %q, want %q", got, config.SeverityOff)
	}
}
