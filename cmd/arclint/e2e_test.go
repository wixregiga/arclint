package main

// End-to-end tests over the compiled binary. The extension test runs with
// a COMPLETELY EMPTY environment: no PATH, no HOME — mechanical proof that
// TypeScript rules execute with no Node, npm, or tsc on the machine
// (M2 acceptance).

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/patterns"
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "arclint-e2e-*")
	if err != nil {
		panic(err)
	}
	binPath = filepath.Join(dir, "arclint")
	// Grammar subset tags mirror the Makefile: the e2e binary embeds only
	// the fact-provider grammars.
	build := exec.Command("go", "build",
		"-tags", "grammar_subset grammar_subset_typescript grammar_subset_tsx grammar_subset_javascript grammar_subset_python",
		"-o", binPath, ".")
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
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		return out.String(), errb.String(), ee.ExitCode()
	}
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	return out.String(), errb.String(), 0
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

// TestExceptOnExtensionInstance proves except works uniformly for
// extension rule instances, and that suppression is visible in output.
func TestExceptOnExtensionInstance(t *testing.T) {
	dir := copyFixture(t, "extension-handler-naming")
	rules, err := os.ReadFile(filepath.Join(dir, "rules.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	patched := strings.Replace(string(rules),
		"  - type: handler-naming\n    params: { suffix: \"Handler\" }",
		"  - type: handler-naming\n    params: { suffix: \"Handler\" }\n"+
			"    except:\n"+
			"      - paths: [\"internal/api/handlers/broken.go\"]\n"+
			"        reason: \"renamed in the next sweep\"", 1)
	if patched == string(rules) {
		t.Fatal("fixture rules.yaml shape changed; update the patch")
	}
	if err := os.WriteFile(filepath.Join(dir, "rules.yaml"), []byte(patched), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, code := runBin(t, dir, os.Environ(), "check", ".")
	// The wiring-audit finding survives (exit 1); the handler-naming
	// finding on broken.go is suppressed, visibly.
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, stdout)
	}
	if strings.Contains(stdout, "handler files must end in") {
		t.Errorf("excepted finding still reported:\n%s", stdout)
	}
	if !strings.Contains(stdout, "1 suppressed by except") {
		t.Errorf("suppression not visible:\n%s", stdout)
	}

	// --show-suppressed lists what was omitted, with the reason.
	stdout, _, _ = runBin(t, dir, os.Environ(), "check", ".", "--show-suppressed")
	for _, want := range []string{"SUPPRESSED (allowed by except)", "handler files must end in", "allowed: renamed in the next sweep"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("--show-suppressed missing %q:\n%s", want, stdout)
		}
	}
	stdout, _, _ = runBin(t, dir, os.Environ(), "check", ".", "--show-suppressed", "--format", "json")
	if !strings.Contains(stdout, "\"suppressed\": true") || !strings.Contains(stdout, "\"suppressedReason\"") {
		t.Errorf("json suppressed fields missing:\n%s", stdout)
	}

	// rules show displays the exception as part of the requirement.
	stdout, stderr, code := runBin(t, dir, os.Environ(), "rules", "show", "rules.handler-naming[0]")
	if code != 0 {
		t.Fatalf("rules show: exit %d, stderr %s", code, stderr)
	}
	for _, want := range []string{"exceptions:", "internal/api/handlers/broken.go", "renamed in the next sweep"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("rules show missing %q:\n%s", want, stdout)
		}
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

// TestBuiltinPatternSuites is the M7 pattern gate: every builtin
// pattern's bundled rule tests pass through the real binary, and every
// namespaced rule id the pattern declares is exercised by at least one
// expectation. Patterns are curated test suites, not hardcoded
// primitives.
func TestBuiltinPatternSuites(t *testing.T) {
	builtins, err := patterns.Builtins()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range builtins {
		t.Run(p.Name, func(t *testing.T) {
			if p.Namespace == "" {
				t.Fatalf("builtin pattern %q must declare a namespace", p.Name)
			}
			if len(p.Tests) == 0 {
				t.Fatalf("builtin pattern %q ships no rule tests", p.Name)
			}
			dir := t.TempDir()
			stdout, stderr, code := runBin(t, dir, os.Environ(),
				"rules", "test", "--pattern", p.Name, "--format", "json")
			if code != 0 {
				t.Fatalf("rules test --pattern %s: exit %d\nstdout: %s\nstderr: %s", p.Name, code, stdout, stderr)
			}
			var results []struct {
				Case  string   `json:"case"`
				Pass  bool     `json:"pass"`
				Rules []string `json:"rules"`
			}
			if err := json.Unmarshal([]byte(stdout), &results); err != nil {
				t.Fatalf("json: %v\n%s", err, stdout)
			}
			tested := map[string]bool{}
			for _, r := range results {
				if !r.Pass {
					t.Errorf("case %q failed", r.Case)
				}
				for _, id := range r.Rules {
					tested[id] = true
				}
			}

			// Coverage: every namespaced id in the rendered template must
			// be exercised. Derived (unprefixed) ids are exempt.
			rendered, err := p.RenderRules(p.Runtimes[:1])
			if err != nil {
				t.Fatal(err)
			}
			rs, err := config.Parse(rendered, filepath.Join(dir, "rules.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			var declared []string
			for _, inst := range rs.Instances() {
				if strings.HasPrefix(inst.ID, p.Namespace+":") {
					declared = append(declared, inst.ID)
				}
			}
			for _, r := range rs.Rules {
				if strings.HasPrefix(r.ID, p.Namespace+":") {
					declared = append(declared, r.ID)
				}
			}
			if len(declared) == 0 {
				t.Fatalf("pattern %q declares no namespaced rule ids", p.Name)
			}
			for _, id := range declared {
				if !tested[id] {
					t.Errorf("rule id %q has no test expectation in the pattern's suite", id)
				}
			}
		})
	}
}

// TestInitEveryBuiltinPattern is the pattern gate: each shipped pattern,
// initialized for each runtime it supports through the real binary, must
// validate and check clean on an empty tree.
func TestInitEveryBuiltinPattern(t *testing.T) {
	builtins, err := patterns.Builtins()
	if err != nil {
		t.Fatal(err)
	}
	if len(builtins) < 3 {
		t.Fatalf("builtins = %d, want the shipped set", len(builtins))
	}
	for _, p := range builtins {
		for _, rt := range p.Runtimes {
			t.Run(p.Name+"/"+rt, func(t *testing.T) {
				dir := t.TempDir()
				stdout, stderr, code := runBin(t, dir, os.Environ(),
					"init", "--runtimes", rt, "--pattern", p.Name)
				if code != 0 {
					t.Fatalf("init: exit %d\n%s%s", code, stdout, stderr)
				}
				if !strings.Contains(stdout, "wrote rules.yaml") {
					t.Errorf("init output: %s", stdout)
				}
				if _, _, code := runBin(t, dir, os.Environ(), "check", "."); code != 0 {
					t.Errorf("materialized pattern does not check clean (exit %d)", code)
				}
			})
		}
	}
}

// TestFeatureSlicePattern proves the shipped pattern against a real
// repository shape: the conforming library app checks clean; the
// violating copy fires representative findings from every enforcement
// source (YAML protected, registration, structure; extension matrix,
// purity, and the provides/provider port finding).
func TestFeatureSlicePattern(t *testing.T) {
	clean := copyFixture(t, "pattern-feature-slice-clean")
	stdout, stderr, code := runBin(t, clean, os.Environ(),
		"init", "--runtimes", "go", "--pattern", "feature-slice")
	if code != 0 {
		t.Fatalf("init: exit %d\n%s%s", code, stdout, stderr)
	}
	if stdout, stderr, code = runBin(t, clean, os.Environ(), "check", "."); code != 0 {
		t.Fatalf("conforming repo not clean: exit %d\n%s%s", code, stdout, stderr)
	}

	dirty := copyFixture(t, "pattern-feature-slice-dirty")
	if _, stderr, code = runBin(t, dirty, os.Environ(),
		"init", "--runtimes", "go", "--pattern", "feature-slice"); code != 0 {
		t.Fatalf("init dirty: exit %d\n%s", code, stderr)
	}
	stdout, _, code = runBin(t, dirty, os.Environ(), "check", ".", "--format", "json")
	if code != 1 {
		t.Fatalf("violating repo: exit %d, want 1\n%s", code, stdout)
	}
	var violations []map[string]any
	if err := json.Unmarshal([]byte(stdout), &violations); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout)
	}
	find := func(pred func(map[string]any) bool) *map[string]any {
		for _, v := range violations {
			if pred(v) {
				return &v
			}
		}
		return nil
	}
	type want struct {
		desc string
		pred func(map[string]any) bool
	}
	wants := []want{
		{"YAML protected: feature imports shared", func(v map[string]any) bool {
			return v["ruleId"] == "slice:deps.shared-only-via-app" && v["path"] == "internal/borrowbook/sneaky.go"
		}},
		{"YAML registration: returnbook unwired", func(v map[string]any) bool {
			return v["ruleId"] == "slice:repo.features-wired" && strings.Contains(v["message"].(string), "returnbook")
		}},
		{"YAML structure: dumping ground", func(v map[string]any) bool {
			return v["ruleId"] == "slice:repo.no-dumping-grounds" && v["path"] == "internal/shared/utils.go"
		}},
		{"extension matrix: feature imports feature", func(v map[string]any) bool {
			return v["ruleId"] == "slice:repo.slices" && v["path"] == "internal/returnbook/grab.go" &&
				v["contract"] == "consumes" && v["blame"] == "consumer"
		}},
		{"extension purity: concept imports net/http", func(v map[string]any) bool {
			return v["ruleId"] == "slice:repo.slices" && v["path"] == "internal/member/web.go"
		}},
		{"extension port finding carries provides/provider", func(v map[string]any) bool {
			return v["ruleId"] == "slice:repo.slices" && v["path"] == "internal/reporting/report.go" &&
				v["contract"] == "provides" && v["blame"] == "provider"
		}},
	}
	for _, w := range wants {
		if find(w.pred) == nil {
			t.Errorf("missing finding: %s\nall: %s", w.desc, stdout)
		}
	}
}

func TestInitInteractiveDefaults(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(binPath, "init")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader("\n\n") // accept both defaults
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("interactive init: %v\n%s", err, out)
	}
	// Empty dir detects nothing; the default is go, whose first compatible
	// pattern (alphabetical) is ddd-flat, which ships an extension and SDK
	// typings.
	for _, f := range []string{"rules.yaml", ".arclint/extensions/ddd-flat.ts", ".arclint/extensions/arclint.d.ts"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f))); err != nil {
			t.Errorf("%s not written: %v", f, err)
		}
	}
}

func TestInitRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rules.yaml"), []byte("runtime: [go]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := runBin(t, dir, os.Environ(), "init", "--runtimes", "go", "--pattern", "starter")
	if code != 2 || !strings.Contains(stderr, "already exists") {
		t.Errorf("exit %d, stderr %s", code, stderr)
	}
	if _, _, code := runBin(t, dir, os.Environ(), "init", "--runtimes", "go", "--pattern", "starter", "--force"); code != 0 {
		t.Errorf("--force: exit %d", code)
	}
}

func TestPatternsCommand(t *testing.T) {
	dir := t.TempDir()
	stdout, stderr, code := runBin(t, dir, os.Environ(), "patterns", "--extensions")
	if code != 0 {
		t.Fatalf("patterns: exit %d, stderr %s", code, stderr)
	}
	for _, want := range []string{
		"feature-slice", "layers", "starter", "builtin",
		".arclint/extensions/feature-slice.ts",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("patterns output missing %q:\n%s", want, stdout)
		}
	}

	// A local pattern under .arclint/patterns is listed with its source.
	local := filepath.Join(dir, ".arclint", "patterns", "fsd", "go")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "pattern.yaml"),
		[]byte("description: \"Team FSD variant.\"\nruntimes: [go]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "rules.yaml"),
		[]byte("runtime: [go]\nmodules:\n  all: [\"**\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, code = runBin(t, dir, os.Environ(), "patterns")
	if code != 0 || !strings.Contains(stdout, "fsd/go") || !strings.Contains(stdout, ".arclint/patterns/fsd/go") {
		t.Errorf("local pattern not listed (exit %d):\n%s", code, stdout)
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
