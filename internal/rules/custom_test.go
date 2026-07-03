package rules

import (
	"runtime"
	"testing"
	"time"

	"github.com/jofyi/arclint/internal/config"
)

// customRule builds a minimal custom rule for compileCustom tests.
func customRule(command []string, timeoutSeconds int) config.Rule {
	return config.Rule{
		Type:     config.CategoryCustom,
		Severity: config.SeverityError,
		Custom: &config.CustomParams{
			Command:        command,
			TimeoutSeconds: timeoutSeconds,
		},
		FixHint: "fix it",
	}
}

// evalOneErr runs compileCustom directly (bypassing Evaluate's baseline
// suppression) so tests can assert on raw violations and errors.
func evalOneErr(root string, r config.Rule) ([]Violation, error) {
	fn := compileCustom("t", r)
	c := &evalCtx{root: root, paths: nil}
	return fn(c)
}

// TestCustomTimeoutKillsProcessGroup exercises BLOCKER 1: a command that
// forks a background grandchild and then waits (simulating a hung
// pipe/orphan) must not be able to survive past the configured timeout.
// exec.CommandContext alone only kills the direct child; without
// Setpgid+group-kill+WaitDelay, the forked "sleep 30 &" grandchild keeps
// stdout/stderr pipes open and cmd.Run() hangs well past the 1s deadline.
// The forked sleep is long (30s) precisely so the test fails loudly (by
// timing out this test) if the group-kill regresses, instead of
// silently passing.
func TestCustomTimeoutKillsProcessGroup(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("process-group kill is Linux/POSIX-only")
	}
	root := t.TempDir()

	rule := customRule([]string{"sh", "-c", "sleep 30 & wait"}, 1)
	done := make(chan error, 1)
	go func() {
		_, err := evalOneErr(root, rule)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("want timeout error, got nil")
		}
	case <-time.After(4 * time.Second):
		t.Fatal("compileCustom did not return within 4s of a 1s timeout — process group was not killed")
	}
}

// TestCustomNormalizesViolationPaths covers BLOCKER 2: raw paths emitted
// by a custom command ("./a.go", backslash-separated) must be normalized
// to a clean, forward-slash, repo-relative shape before becoming a
// Violation, so baseline suppression and ignore globs (which key on that
// canonical shape) still match.
func TestCustomNormalizesViolationPaths(t *testing.T) {
	root := t.TempDir()
	script := `printf '[{"path":"./a.go","message":"m1"},{"path":"sub\\\\b.go","message":"m2"},{"path":"c/../c/d.go","message":"m3"}]'`
	rule := customRule([]string{"sh", "-c", script}, 5)

	vs, err := evalOneErr(root, rule)
	if err != nil {
		t.Fatalf("evalOneErr: %v", err)
	}
	got := map[string]bool{}
	for _, v := range vs {
		got[v.Path] = true
	}
	want := []string{"a.go", "sub/b.go", "c/d.go"}
	for _, w := range want {
		if !got[w] {
			t.Errorf("want normalized path %q among violations, got %+v", w, vs)
		}
	}
}

// TestCustomEnvIsSanitized covers MEDIUM 3: rule commands must not
// inherit the parent's full environment. A secret set only in the test
// process must not reach the child.
func TestCustomEnvIsSanitized(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ARCLINT_TEST_SECRET", "shh-do-not-leak")

	script := `if [ -n "$ARCLINT_TEST_SECRET" ]; then printf '[{"path":"leak","message":"leaked"}]'; else printf '[]'; fi`
	rule := customRule([]string{"sh", "-c", script}, 5)

	vs, err := evalOneErr(root, rule)
	if err != nil {
		t.Fatalf("evalOneErr: %v", err)
	}
	if len(vs) != 0 {
		t.Fatalf("secret env var leaked into custom command: %+v", vs)
	}
}

// TestSanitizedEnvOnlyKnownVars locks down the exact allowlist so a
// future edit can't silently widen it back to os.Environ().
func TestSanitizedEnvOnlyKnownVars(t *testing.T) {
	t.Setenv("SOME_OTHER_SECRET", "x")
	env := sanitizedEnv()
	for _, e := range env {
		if len(e) >= len("SOME_OTHER_SECRET=") && e[:len("SOME_OTHER_SECRET=")] == "SOME_OTHER_SECRET=" {
			t.Fatalf("sanitizedEnv leaked an unexpected var: %v", env)
		}
	}
}
