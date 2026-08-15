package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentsPrint(t *testing.T) {
	dir := contextRepo(t)
	stdout, stderr, code := runBin(t, dir, os.Environ(), "agents")
	if code != 0 {
		t.Fatalf("exit = %d\nstderr: %s", code, stderr)
	}
	for _, want := range []string{
		"<!-- arclint:agents:begin -->",
		"<!-- arclint:agents:end -->",
		"**domain** — Pure business rules.",
		"internal imports none (may import no other declared module)",
		"external forbid",
		"**app** — Application wiring.",
		"arclint context <path|module>",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("block lacks %q\n%s", want, stdout)
		}
	}
}

func TestAgentsWriteLifecycle(t *testing.T) {
	dir := contextRepo(t)
	target := filepath.Join(dir, "AGENTS.md")

	// First write creates the file with only the block.
	stdout, _, code := runBin(t, dir, os.Environ(), "agents", "--write")
	if code != 0 || !strings.Contains(stdout, "wrote") {
		t.Fatalf("create (exit %d): %s", code, stdout)
	}
	created, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}

	// Second write is a no-op on identical content.
	stdout, _, code = runBin(t, dir, os.Environ(), "agents", "--write")
	if code != 0 || !strings.Contains(stdout, "already current") {
		t.Fatalf("idempotency (exit %d): %s", code, stdout)
	}

	// Hand-written prose around a stale block survives; the block is
	// replaced.
	manual := "# My notes\n\nkeep this line\n\n" +
		"<!-- arclint:agents:begin -->\nSTALE CONTENT\n<!-- arclint:agents:end -->\n\ntrailing notes\n"
	if err := os.WriteFile(target, []byte(manual), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, code = runBin(t, dir, os.Environ(), "agents", "--write"); code != 0 {
		t.Fatalf("refresh exit = %d", code)
	}
	refreshed, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	got := string(refreshed)
	for _, want := range []string{"keep this line", "trailing notes", "**domain**"} {
		if !strings.Contains(got, want) {
			t.Errorf("refreshed file lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "STALE CONTENT") {
		t.Errorf("stale block content survived:\n%s", got)
	}

	// A file without markers gets the block appended, prose intact.
	if err := os.WriteFile(target, []byte("just prose\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, code = runBin(t, dir, os.Environ(), "agents", "--write"); code != 0 {
		t.Fatalf("append exit = %d", code)
	}
	appended, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(appended), "just prose\n") ||
		!strings.Contains(string(appended), "<!-- arclint:agents:begin -->") {
		t.Errorf("append shape wrong:\n%s", appended)
	}

	// One marker without the other is a corruption error.
	if err := os.WriteFile(target, []byte("<!-- arclint:agents:begin -->\nhalf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runBin(t, dir, os.Environ(), "agents", "--write"); code != 2 ||
		!strings.Contains(stderr, "one arclint marker") {
		t.Fatalf("corruption: exit = %d, stderr = %s", code, stderr)
	}

	// The created block equals the printed block byte for byte.
	printed, _, _ := runBin(t, dir, os.Environ(), "agents")
	if string(created) != printed {
		t.Errorf("written block differs from printed block")
	}
}

// TestAgentsBlockSelfHostCurrent is the drift guard for this repo's own
// AGENTS.md: the committed generated block must match what the binary
// generates from the committed rules.yaml. Run `arclint agents --write`
// at the repo root after changing rules.yaml.
func TestAgentsBlockSelfHostCurrent(t *testing.T) {
	committed, err := os.ReadFile("../../AGENTS.md")
	if err != nil {
		t.Fatalf("repo AGENTS.md: %v (create it with `arclint agents --write`)", err)
	}
	content := string(committed)
	begin := strings.Index(content, agentsBegin)
	end := strings.Index(content, agentsEnd)
	if begin < 0 || end < begin {
		t.Fatalf("repo AGENTS.md carries no generated block; run `arclint agents --write`")
	}
	block := content[begin : end+len(agentsEnd)+1]

	generated, stderr, code := runBin(t, "../..", os.Environ(), "agents")
	if code != 0 {
		t.Fatalf("agents: exit %d\n%s", code, stderr)
	}
	if block != generated {
		t.Errorf("AGENTS.md block is stale; run `arclint agents --write` at the repo root.\n--- committed ---\n%s\n--- generated ---\n%s", block, generated)
	}
}
