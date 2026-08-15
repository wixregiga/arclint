//go:build agentbench

// Package agentbench_test measures the feedback-loop claim behind
// arclint's agent-facing surface: given a repository with architecture
// violations and arclint's diagnostics, does a coding agent converge to
// a conforming implementation, and does prompt-time context (the
// AGENTS.md block plus `arclint context` output) change convergence?
//
// Each trial: initialize a pattern into a temp repo, overlay violating
// code, then loop (check -> hand findings to the agent -> re-check) up
// to maxIterations. Success is `arclint check .` clean AND
// `go build ./...` green AND the contract files untouched (a repair
// that edits rules.yaml or .arclint/ is recorded as gamed, not fixed).
//
// Agent-and-network dependent by design; run with:
//
//	make agentbench
//
// The agent command is AGENTBENCH_AGENT_CMD (default: codex exec).
// Results are environment- and model-dependent measurements, not
// regression assertions: the test fails only on harness errors, never
// on an agent losing.
package agentbench_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const (
	maxIterations = 3
	agentTimeout  = 8 * time.Minute
)

type scenario struct {
	name    string
	pattern string
	overlay string // dir copied over the initialized tree (rules.yaml excluded)
}

var scenarios = []scenario{
	{name: "feature-slice-dirty", pattern: "feature-slice", overlay: "../../testdata/fixtures/pattern-feature-slice-dirty"},
	{name: "goprod-fat-interface", pattern: "go-prod", overlay: "../../testdata/agentbench/goprod-fat"},
}

type violation struct {
	RuleID  string `json:"ruleId"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

type iterationRecord struct {
	Violations int     `json:"violations"`
	AgentSecs  float64 `json:"agentSecs"`
	AgentNote  string  `json:"agentNote,omitempty"`
}

type trialRecord struct {
	Scenario   string            `json:"scenario"`
	Condition  string            `json:"condition"`
	Initial    int               `json:"initialViolations"`
	Iterations []iterationRecord `json:"iterations"`
	Final      int               `json:"finalViolations"`
	Success    bool              `json:"success"`
	Gamed      bool              `json:"gamed"`
	BuildOK    bool              `json:"buildOK"`
}

func binPath(t *testing.T) string {
	t.Helper()
	bin := os.Getenv("ARCLINT_BIN")
	if bin == "" {
		t.Fatal("set ARCLINT_BIN to a compiled arclint binary")
	}
	return bin
}

func agentArgv() []string {
	if cmd := os.Getenv("AGENTBENCH_AGENT_CMD"); cmd != "" {
		return strings.Fields(cmd)
	}
	return []string{"codex", "exec", "--sandbox", "workspace-write", "--skip-git-repo-check"}
}

func TestAgentConvergence(t *testing.T) {
	bin := binPath(t)
	if _, err := exec.LookPath(agentArgv()[0]); err != nil {
		t.Fatalf("agent CLI %q not found; set AGENTBENCH_AGENT_CMD", agentArgv()[0])
	}
	var records []trialRecord
	for _, sc := range scenarios {
		for _, withContext := range []bool{false, true} {
			condition := "diag"
			if withContext {
				condition = "diag+context"
			}
			t.Run(sc.name+"/"+condition, func(t *testing.T) {
				rec := runTrial(t, bin, sc, withContext)
				records = append(records, rec)
			})
		}
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("results:\n%s", data)
	if out := os.Getenv("AGENTBENCH_OUT"); out != "" {
		if err := os.WriteFile(out, data, 0o644); err != nil {
			t.Errorf("write %s: %v", out, err)
		}
	}
}

func runTrial(t *testing.T, bin string, sc scenario, withContext bool) trialRecord {
	t.Helper()
	dir := t.TempDir()
	mustRun(t, dir, bin, "init", "--pattern", sc.pattern, "--runtimes", "go", "--force")
	copyOverlay(t, sc.overlay, dir)
	if withContext {
		mustRun(t, dir, bin, "agents", "--write")
	}
	guard := hashGuarded(t, dir)

	vs := checkViolations(t, dir, bin)
	if len(vs) == 0 {
		t.Fatalf("scenario %s starts clean; overlay lost its violations", sc.name)
	}
	if !buildOK(dir) {
		t.Fatalf("scenario %s does not build before repair; fixture broken", sc.name)
	}

	rec := trialRecord{
		Scenario:  sc.name,
		Condition: map[bool]string{false: "diag", true: "diag+context"}[withContext],
		Initial:   len(vs),
	}
	for iter := 0; iter < maxIterations && len(vs) > 0; iter++ {
		prompt := buildPrompt(t, dir, bin, withContext, vs)
		start := time.Now()
		note := runAgent(t, dir, prompt)
		rec.Iterations = append(rec.Iterations, iterationRecord{
			Violations: len(vs),
			AgentSecs:  time.Since(start).Seconds(),
			AgentNote:  note,
		})
		if hashGuarded(t, dir) != guard {
			rec.Gamed = true
			break
		}
		vs = checkViolations(t, dir, bin)
	}
	rec.Final = len(vs)
	rec.BuildOK = buildOK(dir)
	rec.Success = !rec.Gamed && rec.Final == 0 && rec.BuildOK
	t.Logf("%s/%s: initial=%d final=%d iterations=%d success=%v gamed=%v buildOK=%v",
		rec.Scenario, rec.Condition, rec.Initial, rec.Final, len(rec.Iterations), rec.Success, rec.Gamed, rec.BuildOK)
	return rec
}

// buildPrompt assembles the repair prompt: the task with its hard
// constraints, the human-readable findings (message + fix hints), and,
// in the context condition, `arclint context` output for the violating
// locations.
func buildPrompt(t *testing.T, dir, bin string, withContext bool, vs []violation) string {
	t.Helper()
	checkText, _ := run(dir, bin, "check", ".")
	var b strings.Builder
	b.WriteString("You are working in a Go repository whose architecture contracts are enforced by arclint.\n")
	b.WriteString("Task: change the CODE so that `arclint check .` reports zero violations and `go build ./...` still succeeds.\n")
	b.WriteString("Hard constraints: never modify or delete rules.yaml, anything under .arclint/, or AGENTS.md; those are the contract, the code is wrong. Do not delete functionality merely to silence a finding; restructure instead.\n\n")
	b.WriteString("Current findings from `arclint check .`:\n\n")
	b.WriteString(checkText)
	if withContext {
		b.WriteString("\nArchitectural context of the violating locations (from `arclint context`):\n")
		for _, p := range uniquePaths(vs, 3) {
			ctxText, err := run(dir, bin, "context", p)
			if err != nil {
				continue
			}
			b.WriteString("\n" + ctxText)
		}
	}
	return b.String()
}

func uniquePaths(vs []violation, limit int) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range vs {
		if v.Path == "" || seen[v.Path] {
			continue
		}
		seen[v.Path] = true
		out = append(out, v.Path)
		if len(out) == limit {
			break
		}
	}
	sort.Strings(out)
	return out
}

func runAgent(t *testing.T, dir, prompt string) string {
	t.Helper()
	argv := append(agentArgv(), prompt)
	ctx, cancel := context.WithTimeout(context.Background(), agentTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	tail := lastLines(string(out), 3)
	if ctx.Err() != nil {
		return "timeout: " + tail
	}
	if err != nil {
		return "error: " + err.Error() + ": " + tail
	}
	return tail
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}

func checkViolations(t *testing.T, dir, bin string) []violation {
	t.Helper()
	out, err := run(dir, bin, "check", ".", "--format", "json")
	// Exit 1 means violations; 0 means clean; anything else is a
	// harness or config failure.
	if err != nil && !strings.Contains(err.Error(), "exit status 1") {
		t.Fatalf("check: %v\n%s", err, out)
	}
	var vs []violation
	if strings.TrimSpace(out) == "" {
		return nil
	}
	if jsonErr := json.Unmarshal([]byte(out), &vs); jsonErr != nil {
		t.Fatalf("check json: %v\n%s", jsonErr, out)
	}
	return vs
}

func buildOK(dir string) bool {
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	return cmd.Run() == nil
}

// hashGuarded fingerprints the contract surface the agent must not
// touch: rules.yaml and everything under .arclint/ except the cache.
func hashGuarded(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	paths := []string{"rules.yaml"}
	root := filepath.Join(dir, ".arclint")
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.Contains(p, "cache") {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		paths = append(paths, rel)
		return nil
	})
	sort.Strings(paths)
	for _, rel := range paths {
		data, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			continue
		}
		fmt.Fprintf(h, "%s\x00%x\x00", rel, sha256.Sum256(data))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func copyOverlay(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		// The initialized pattern template stays authoritative.
		if rel == "rules.yaml" {
			return nil
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
}

func mustRun(t *testing.T, dir, bin string, args ...string) {
	t.Helper()
	if out, err := run(dir, bin, args...); err != nil {
		t.Fatalf("%s %s: %v\n%s", bin, strings.Join(args, " "), err, out)
	}
}

func run(dir, bin string, args ...string) (string, error) {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}
