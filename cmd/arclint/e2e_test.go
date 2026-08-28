package main

// End-to-end tests over the compiled binary: the self-hosted engine
// checking its own repository, the SDK showcase extension gating a
// fixture, and the full baseline lifecycle.

import (
	"bytes"
	"encoding/json"
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

func runBin(t *testing.T, dir string, env []string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Dir = dir
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("run %v: %v", args, err)
	}
	return stdout.String(), stderr.String(), code
}

// runBinStdin is runBin with a scripted stdin payload (guided authoring).
func runBinStdin(t *testing.T, dir string, env []string, stdin string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("run %v: %v", args, err)
	}
	return stdout.String(), stderr.String(), code
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

type diagnosticDoc struct {
	Kind     string `json:"kind"`
	RuleID   string `json:"ruleId"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Severity string `json:"severity"`
	Status   string `json:"status"`
	Message  string `json:"message"`
}

// TestSelfCheckClean is the self-hosting gate: the engine's own
// repository satisfies every one of its own rules, extension included.
func TestSelfCheckClean(t *testing.T) {
	stdout, stderr, code := runBin(t, repoRoot(t), os.Environ(), "check")
	if code != 0 {
		t.Fatalf("self-check exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "0 active finding(s)") {
		t.Errorf("self-check output: %s", stdout)
	}
}

func TestRulesListsRuleset(t *testing.T) {
	stdout, stderr, code := runBin(t, repoRoot(t), os.Environ(), "rules")
	if code != 0 {
		t.Fatalf("rules exit %d\nstderr: %s", code, stderr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 24 {
		t.Errorf("rules listed = %d, want 24\n%s", len(lines), stdout)
	}
	for _, want := range []string{
		"arclint:domain/stdlib-only", "arclint:domain/no-panic",
		"arclint:domain-model/aggregate-skeleton",
		"arclint:domain-model/contexts-respect-relations",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("missing %s in listing", want)
		}
	}
}

func TestRuleDetailAndContext(t *testing.T) {
	stdout, stderr, code := runBin(t, repoRoot(t), os.Environ(), "rules", "arclint:domain/stdlib-only")
	if code != 0 || !strings.Contains(stdout, "when violated: fails the gate") {
		t.Errorf("rules detail exit %d, output %q, stderr %s", code, stdout, stderr)
	}

	stdout, stderr, code = runBin(t, repoRoot(t), os.Environ(),
		"context", "internal/domain/rule/root.go", "--format", "json")
	if code != 0 {
		t.Fatalf("context exit %d\nstderr: %s", code, stderr)
	}
	var ctx struct {
		Modules []struct{ Name string }
		Rules   []struct{ Reason string }
	}
	if err := json.Unmarshal([]byte(stdout), &ctx); err != nil {
		t.Fatalf("context json: %v\n%s", err, stdout)
	}
	names := map[string]bool{}
	for _, m := range ctx.Modules {
		names[m.Name] = true
	}
	if !names["domain"] || len(ctx.Rules) == 0 {
		t.Errorf("context = %+v", ctx)
	}
}

// TestAgentsBlockCurrent replaces the docs drift test: the installed
// AGENTS.md block must match what the binary generates from rules.yaml.
func TestAgentsBlockCurrent(t *testing.T) {
	root := repoRoot(t)
	stdout, stderr, code := runBin(t, root, os.Environ(), "agents", "md")
	if code != 0 {
		t.Fatalf("agents md exit %d\nstderr: %s", code, stderr)
	}
	installed, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(installed), strings.TrimSuffix(stdout, "\n")) {
		t.Errorf("AGENTS.md block is stale; run `arclint agents md --write`")
	}
}

// TestAgentsMDAliasesRenderIdenticalBlock checks that agents markdown and
// agents agentsmd emit the same architecture block as agents md.
func TestAgentsMDAliasesRenderIdenticalBlock(t *testing.T) {
	root := repoRoot(t)
	want, stderr, code := runBin(t, root, os.Environ(), "agents", "md")
	if code != 0 {
		t.Fatalf("agents md exit %d\nstderr: %s", code, stderr)
	}
	for _, alias := range []string{"markdown", "agentsmd"} {
		stdout, stderr, code := runBin(t, root, os.Environ(), "agents", alias)
		if code != 0 {
			t.Fatalf("agents %s exit %d\nstderr: %s", alias, code, stderr)
		}
		if stdout != want {
			t.Errorf("agents %s stdout differs from agents md", alias)
		}
	}
}

// TestAgentsGroupHelpOnly proves bare `arclint agents` is a command group:
// it exits cleanly with help text and does not print or install the block.
func TestAgentsGroupHelpOnly(t *testing.T) {
	root := repoRoot(t)
	stdout, stderr, code := runBin(t, root, os.Environ(), "agents")
	if code != 0 {
		t.Fatalf("agents exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	out := stdout + stderr
	if !strings.Contains(out, "Available Commands") && !strings.Contains(out, "Usage:") {
		t.Errorf("bare agents should show group help, got: %q", out)
	}
	if strings.Contains(out, "<!-- arclint:agents:begin -->") {
		t.Errorf("bare agents must not render the AGENTS.md block")
	}
	if strings.Contains(out, "wrote ") || strings.Contains(out, "already current") {
		t.Errorf("bare agents must not install the AGENTS.md block")
	}
}

// TestExtensionDemoGates proves the SDK showcase end to end: the
// forbid-content extension, configured with an input, gates a real
// fixture repository.
func TestExtensionDemoGates(t *testing.T) {
	demo, err := os.ReadFile(filepath.Join(repoRoot(t), ".arclint", "extensions", "forbid_content.ts"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	write(t, root, ".arclint/extensions/forbid_content.ts", string(demo))
	write(t, root, "rules.yaml", `runtime: [go]
modules:
  src:
    paths: ["src/**"]
contracts:
  src:
    invariants:
      - id: "repo:src/no-panic"
        kind: extension
        files: "src/**/*.go"
        uses: forbid-content
        with:
          pattern: '\bpanic\('
`)
	write(t, root, "src/bad.go", "package src\n\nfunc boom() {\n\tpanic(\"no\")\n}\n")
	write(t, root, "src/ok.go", "package src\n")

	stdout, stderr, code := runBin(t, root, os.Environ(), "check", "--format", "json")
	if code != 1 {
		t.Fatalf("exit %d, want 1\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	var diagnostics []diagnosticDoc
	if err := json.Unmarshal([]byte(stdout), &diagnostics); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout)
	}
	var active []diagnosticDoc
	for _, d := range diagnostics {
		if d.Kind == "violation" && d.Status == "active" {
			active = append(active, d)
		}
	}
	if len(active) != 1 || active[0].RuleID != "repo:src/no-panic" ||
		active[0].Path != "src/bad.go" || active[0].Line != 4 {
		t.Errorf("active = %+v, want the panic line in src/bad.go", active)
	}
}

// TestInitDraftLoads proves init against the engine itself: the drafted
// starter ruleset must check clean in a fresh repository.
func TestInitDraftLoads(t *testing.T) {
	root := t.TempDir()
	stdout, stderr, code := runBin(t, root, os.Environ(), "init")
	if code != 0 || !strings.Contains(stdout, "wrote ") {
		t.Fatalf("init exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	write(t, root, "hello.go", "package hello\n")
	if _, stderr, code := runBin(t, root, os.Environ(), "check"); code != 0 {
		t.Errorf("check on the starter ruleset: exit %d\nstderr: %s", code, stderr)
	}
	if _, _, code := runBin(t, root, os.Environ(), "init"); code != 2 {
		t.Errorf("second init must refuse without --force, got exit %d", code)
	}
}

func TestInitVerticalLoads(t *testing.T) {
	root := t.TempDir()
	stdout, stderr, code := runBin(t, root, os.Environ(), "init", "--pattern", "vertical")
	if code != 0 || !strings.Contains(stdout, "wrote ") {
		t.Fatalf("init --pattern vertical exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	write(t, root, "hello.go", "package hello\n")
	if _, stderr, code := runBin(t, root, os.Environ(), "check"); code != 0 {
		t.Errorf("check on the vertical ruleset: exit %d\nstderr: %s", code, stderr)
	}
}

// TestBaselineLifecycle drives adopt, gate, stale, refresh over a real
// fixture repository.
func TestBaselineLifecycle(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.yaml", `runtime: [go]
modules:
  src:
    paths: ["src/**"]
contracts:
  src:
    invariants:
      - id: "repo:src/snake"
        kind: naming
        files: "src/**/*.go"
        case: snake_case
`)
	write(t, root, "src/BadName.go", "package src\n")
	write(t, root, "src/ok.go", "package src\n")

	if _, _, code := runBin(t, root, os.Environ(), "check"); code != 1 {
		t.Fatalf("pre-baseline check must gate, got %d", code)
	}
	stdout, stderr, code := runBin(t, root, os.Environ(), "baseline", "capture")
	if code != 0 || !strings.Contains(stdout, "1 finding(s)") {
		t.Fatalf("capture exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if stdout, _, code := runBin(t, root, os.Environ(), "check"); code != 0 ||
		!strings.Contains(stdout, "1 baselined") {
		t.Fatalf("baselined check exit %d\n%s", code, stdout)
	}

	if err := os.Rename(filepath.Join(root, "src", "BadName.go"),
		filepath.Join(root, "src", "good_name.go")); err != nil {
		t.Fatal(err)
	}
	if stdout, _, code := runBin(t, root, os.Environ(), "check"); code != 0 ||
		!strings.Contains(stdout, "no longer occur") {
		t.Fatalf("stale entries must surface, exit %d\n%s", code, stdout)
	}
	stdout, stderr, code = runBin(t, root, os.Environ(), "baseline", "refresh")
	if code != 0 || !strings.Contains(stdout, "1 stale entr(ies) dropped") {
		t.Fatalf("refresh exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if stdout, _, code := runBin(t, root, os.Environ(), "check"); code != 0 ||
		strings.Contains(stdout, "no longer occur") {
		t.Fatalf("post-refresh check exit %d\n%s", code, stdout)
	}
}

// TestLegacyFormatRejected pins the strict grammar: the retired
// ruleset constructs fail loudly as configuration errors.
func TestLegacyFormatRejected(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.yaml", `runtime: [go]
modules:
  src: ["src/**"]
contracts:
  src:
    provides:
      - kind: correspondence
`)
	if _, stderr, code := runBin(t, root, os.Environ(), "check"); code != 2 ||
		!strings.Contains(stderr, "arclint:") {
		t.Errorf("legacy format: exit %d, stderr %q, want a loud configuration error", code, stderr)
	}
}

func TestSDKInitWritesTyping(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.yaml", "runtime: [go]\n")
	stdout, stderr, code := runBin(t, root, os.Environ(), "sdk", "init")
	if code != 0 || strings.Count(stdout, "wrote ") != 2 {
		t.Fatalf("sdk init exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	dts, err := os.ReadFile(filepath.Join(root, ".arclint", "extensions", "arclint.d.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dts), "defineRule") {
		t.Errorf("arclint.d.ts lacks the SDK surface")
	}
}

// TestExtensionFactsEndToEnd proves the declaration tier root to
// sandbox: the extension rule's enforcement declares the fact, the
// walker gathers it, the Go producer extracts exactly, and the
// extension reads it through ctx.facts.
func TestExtensionFactsEndToEnd(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".arclint/extensions/exported_inventory.ts", `
import { defineRule } from "arclint";
export default defineRule({
  type: "exported-inventory",
  check(ctx) {
    for (const f of ctx.files()) {
      const facts = ctx.facts(f.path);
      if (facts === null) continue;
      for (const d of facts.decls) {
        if (d.kind === "func" && d.exported) {
          ctx.report({ path: f.path, line: d.startLine, message: "exported func " + d.name });
        }
      }
    }
  },
});
`)
	write(t, root, "rules.yaml", `runtime: [go, ts]
modules:
  src:
    paths: ["src/**"]
contracts:
  src:
    invariants:
      - id: "repo:src/inventory"
        kind: extension
        uses: exported-inventory
`)
	write(t, root, "src/a.go", "package src\n\nfunc Public() {}\n\nfunc private() {}\n")
	write(t, root, "src/b.ts", "export function TsThing(): void {}\nfunction hidden(): void {}\n")

	stdout, stderr, code := runBin(t, root, os.Environ(), "check", "--format", "json")
	if code != 1 {
		t.Fatalf("exit %d, want 1\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	var diagnostics []diagnosticDoc
	if err := json.Unmarshal([]byte(stdout), &diagnostics); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout)
	}
	var active []diagnosticDoc
	for _, d := range diagnostics {
		if d.Kind == "violation" && d.Status == "active" {
			active = append(active, d)
		}
	}
	if len(active) != 2 ||
		active[0].Message != "exported func Public" || active[0].Path != "src/a.go" || active[0].Line != 3 ||
		active[1].Message != "exported func TsThing" || active[1].Path != "src/b.ts" || active[1].Line != 1 {
		t.Errorf("active = %+v, want the Go and TypeScript exported funcs at their lines", active)
	}
}
