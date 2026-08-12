package main

// End-to-end tests over the compiled binary. The extension test runs with
// a COMPLETELY EMPTY environment: no PATH, no HOME — mechanical proof that
// TypeScript rules execute with no Node, npm, or tsc on the machine
// (M2 acceptance).

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "arclint-e2e-*")
	if err != nil {
		panic(err)
	}
	binPath = filepath.Join(dir, "arclint")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		panic("build: " + err.Error() + "\n" + string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// copyFixture avoids polluting testdata with cache artifacts.
func copyFixture(t *testing.T, name string) string {
	t.Helper()
	src := fixturePath(t, name)
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func runBin(t *testing.T, dir string, env []string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Dir = dir
	cmd.Env = env
	var out, errb []byte
	out, err := cmd.Output()
	if ee, ok := err.(*exec.ExitError); ok {
		errb = ee.Stderr
		return string(out), string(errb), ee.ExitCode()
	}
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	return string(out), "", 0
}

func TestExtensionRuleWithoutToolchain(t *testing.T) {
	dir := copyFixture(t, "extension-handler-naming")
	// Empty environment: if anything tried to spawn node/tsc/npm it could
	// not even resolve a PATH.
	stdout, stderr, code := runBin(t, dir, []string{}, "check", ".", "--format", "json")
	if code != 1 {
		t.Fatalf("exit = %d, want 1\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	var violations []map[string]any
	if err := json.Unmarshal([]byte(stdout), &violations); err != nil {
		t.Fatalf("stdout is not a JSON violation array: %v\n%s", err, stdout)
	}
	if len(violations) != 2 {
		t.Fatalf("violations: %v", violations)
	}
	v := violations[0]
	if v["ruleId"] != "rules.handler-naming[0]" ||
		v["path"] != "internal/api/handlers/broken.go" ||
		v["contract"] != "invariant" ||
		v["blame"] != "provider" {
		t.Errorf("violation: %v", v)
	}
	// The second rule's finding overrides its type-level provides/provider.
	w := violations[1]
	if w["ruleId"] != "rules.wiring-audit[1]" ||
		w["contract"] != "consumes" ||
		w["blame"] != "consumer" {
		t.Errorf("override violation: %v", w)
	}
}

func TestModuleAndExplainCommands(t *testing.T) {
	dir := copyFixture(t, "extension-handler-naming")
	if _, stderr, code := runBin(t, dir, os.Environ(), "load", "rules.yaml"); code != 0 {
		t.Fatalf("load: %s", stderr)
	}

	stdout, stderr, code := runBin(t, dir, os.Environ(), "module", "info", "handlers")
	if code != 0 {
		t.Fatalf("module info: exit %d, stderr %s", code, stderr)
	}
	for _, want := range []string{
		"handlers — [go] HTTP handler layer; names end in Handler.",
		"paths: internal/api/handlers/**",
		"files: 2",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("module info output missing %q:\n%s", want, stdout)
		}
	}

	if _, stderr, code := runBin(t, dir, os.Environ(), "module", "info", "nope"); code != 2 ||
		!strings.Contains(stderr, `unknown module "nope"`) {
		t.Errorf("unknown module: exit %d, stderr %s", code, stderr)
	}

	stdout, stderr, code = runBin(t, dir, os.Environ(), "explain")
	if code != 0 {
		t.Fatalf("explain: exit %d, stderr %s", code, stderr)
	}
	for _, want := range []string{
		"consumes", "registration", "layers",
		// Extension types are merged into the listing with their
		// defineRule descriptions.
		"handler-naming", "Handler files carry the configured suffix.",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("explain listing missing %q:\n%s", want, stdout)
		}
	}

	stdout, _, code = runBin(t, dir, os.Environ(), "explain", "consumes")
	if code != 0 || !strings.Contains(stdout, "external  a third-party dependency") {
		t.Errorf("explain consumes: exit %d\n%s", code, stdout)
	}

	stdout, _, code = runBin(t, dir, os.Environ(), "explain", "handler-naming")
	if code != 0 || !strings.Contains(stdout, "params schema") || !strings.Contains(stdout, "suffix") {
		t.Errorf("explain extension type: exit %d\n%s", code, stdout)
	}

	if _, stderr, code = runBin(t, dir, os.Environ(), "explain", "nope"); code != 2 ||
		!strings.Contains(stderr, "unknown rule kind") {
		t.Errorf("explain unknown: exit %d, stderr %s", code, stderr)
	}
}

func TestExitCodeContract(t *testing.T) {
	cases := []struct {
		fixture string
		want    int
	}{
		{"external-import-violation", 1},
		{"external-import-clean", 0},
	}
	for _, tc := range cases {
		dir := copyFixture(t, tc.fixture)
		_, stderr, code := runBin(t, dir, os.Environ(), "check", ".")
		if code != tc.want {
			t.Errorf("%s: exit = %d, want %d (stderr: %s)", tc.fixture, code, tc.want, stderr)
		}
	}
	// Config error: no rules.yaml anywhere up from an isolated temp dir.
	_, _, code := runBin(t, t.TempDir(), os.Environ(), "check", ".", "--rules", "/nonexistent/rules.yaml")
	if code != 2 {
		t.Errorf("missing rules: exit = %d, want 2", code)
	}
}

func TestSdkInitCommand(t *testing.T) {
	dir := t.TempDir()
	stdout, stderr, code := runBin(t, dir, os.Environ(), "sdk", "init")
	if code != 0 {
		t.Fatalf("sdk init: exit %d\n%s%s", code, stdout, stderr)
	}
	for _, f := range []string{"arclint.d.ts", "tsconfig.json"} {
		if _, err := os.Stat(filepath.Join(dir, ".arclint", "extensions", f)); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}
