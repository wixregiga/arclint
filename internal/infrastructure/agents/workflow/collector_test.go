package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestCollectorPersistsSelectedPathsOnceAcrossRepeatedEvents(t *testing.T) {
	root := t.TempDir()
	collectorGit(t, root, "init", "-q")
	collector, err := NewCollector(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := collector.Collect(ctx, CodexEvent{HookEventName: userPromptSubmitEvent, SessionID: "deduplicate", Prompt: "Review current work."}); err != nil {
		t.Fatal(err)
	}
	writeCollectorFile(t, root, "current.sql", "select owner from records;\n")
	for range 12 {
		if _, err := collector.Collect(ctx, CodexEvent{HookEventName: postToolUseEvent, SessionID: "deduplicate", ToolName: "Read", ToolInput: json.RawMessage(`{"file_path":"current.sql"}`)}); err != nil {
			t.Fatal(err)
		}
	}
	sum := sha256.Sum256([]byte("deduplicate"))
	data, err := os.ReadFile(filepath.Join(root, ".arclint/cache/workflow-guard", hex.EncodeToString(sum[:])+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var state taskActivity
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Selected) != 1 || state.Selected[0] != "current.sql" {
		t.Fatalf("persisted duplicate selections: %v", state.Selected)
	}
}

func writeCollectorFile(t *testing.T, root, name, text string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func collectorGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, output)
	}
}

func TestCollectorPreservesTaskBaselineActionsAndSteering(t *testing.T) {
	root := t.TempDir()
	collectorGit(t, root, "init", "-q")
	writeCollectorFile(t, root, "existing.txt", "pre-existing owner change")
	writeCollectorFile(t, root, "domain.arclint.yaml", "project: specimen\n")
	writeCollectorFile(t, root, "rules.arclint.yaml", "runtime: [go]\n")
	writeCollectorFile(t, root, "AGENTS.md", "Run arclint context before reading source.\n")
	writeCollectorFile(t, root, "nested/AGENTS.md", "Record justified meanings before implementation.\n")
	writeCollectorFile(t, root, "nested/report.sql", "select owner from records;\n")
	collector, err := NewCollector(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	initial, err := collector.Collect(ctx, CodexEvent{HookEventName: userPromptSubmitEvent, SessionID: "one", Cwd: root, Prompt: "Review nested/report.sql and preserve existing work."})
	if err != nil || initial.Passages["work/nested/report.sql"] == "" {
		t.Fatalf("missing explicitly requested unchanged file: %+v %v", initial, err)
	}
	if _, found := initial.Passages["work/existing.txt"]; found {
		t.Fatal("attributed pre-existing unrelated owner work to this task")
	}
	_, err = collector.Collect(ctx, CodexEvent{HookEventName: postToolUseEvent, SessionID: "one", Cwd: root, ToolName: "exec_command", ToolInput: json.RawMessage(`{"cmd":"arclint context nested/report.sql"}`), ToolResult: json.RawMessage(`{"exit_code":0,"output":"recorded domain"}`)})
	if err != nil {
		t.Fatal(err)
	}
	writeCollectorFile(t, root, "changed.ruby", "owner_name = 'recorded'\n")
	resumed, err := collector.Collect(ctx, CodexEvent{HookEventName: sessionStartEvent, SessionID: "one", Cwd: root})
	if err != nil || !strings.Contains(resumed.Task, "Review nested/report.sql") || !strings.Contains(resumed.Passages["task-actions"], "recorded domain") || resumed.Passages["work/changed.ruby"] == "" {
		t.Fatalf("same-session resume erased task/activity/baseline: %+v %v", resumed, err)
	}
	fresh, err := collector.Collect(ctx, CodexEvent{HookEventName: sessionStartEvent, SessionID: "new-session", Cwd: root})
	if err != nil || fresh.Task != "" || fresh.Passages["task-actions"] != "" {
		t.Fatalf("new session inherited unrelated task activity: %+v %v", fresh, err)
	}
	repaired, err := collector.Collect(ctx, CodexEvent{HookEventName: userPromptSubmitEvent, SessionID: "one", Cwd: root, Prompt: "Repair the previous finding and reconsider my rebuttal."})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(repaired.Task, "Review nested/report.sql") || !strings.Contains(repaired.Task, "Repair the previous finding") {
		t.Fatalf("lost original or follow-up task: %s", repaired.Task)
	}
	if !strings.Contains(repaired.Passages["task-actions"], "recorded domain") {
		t.Fatal("lost actual earlier action result on follow-up")
	}
	if repaired.Passages["work/changed.ruby"] == "" {
		t.Fatal("reset baseline on follow-up or excluded another source language")
	}
	if repaired.Passages["guidance/nested/AGENTS.md"] == "" {
		t.Fatal("omitted applicable nested instructions")
	}
	if strings.Contains(strings.Join(repaired.Limits, "\n"), ".arclint/rules.yaml") {
		t.Fatal("reported absent fallback despite present configured rules")
	}
}

func TestCollectorPersistsNamedToolFilesAndKeepsMissingEvidenceExplicit(t *testing.T) {
	root := t.TempDir()
	writeCollectorFile(t, root, "custom-language.yml", "project: custom\n")
	writeCollectorFile(t, root, "review-me.toml", "owner = 'user'\n")
	collector, err := NewCollector(root, "custom-language.yml")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_, err = collector.Collect(ctx, CodexEvent{HookEventName: userPromptSubmitEvent, SessionID: "two", Prompt: "Inspect the ownership of this module."})
	if err != nil {
		t.Fatal(err)
	}
	_, err = collector.Collect(ctx, CodexEvent{HookEventName: preToolUseEvent, SessionID: "two", ToolName: "Read", ToolInput: json.RawMessage(`{"file_path":"review-me.toml"}`)})
	if err != nil {
		t.Fatal(err)
	}
	final, err := collector.Collect(ctx, CodexEvent{HookEventName: stopEvent, SessionID: "two"})
	if err != nil || final.Passages["work/review-me.toml"] == "" || final.Passages["guidance/custom-language.yml"] == "" {
		t.Fatalf("lost named evidence: %+v %v", final, err)
	}
	if err := os.Remove(filepath.Join(root, "review-me.toml")); err != nil {
		t.Fatal(err)
	}
	missing, err := collector.Collect(ctx, CodexEvent{HookEventName: stopEvent, SessionID: "two"})
	if err != nil || !strings.Contains(strings.Join(missing.Limits, "\n"), "review-me.toml") {
		t.Fatalf("missing evidence silently disappeared: %+v %v", missing, err)
	}
}

func TestCollectorRetainsActionTruncationLimitAcrossResume(t *testing.T) {
	root := t.TempDir()
	collector, err := NewCollector(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := collector.Collect(ctx, CodexEvent{HookEventName: userPromptSubmitEvent, SessionID: "bounded", Prompt: "Review current work."}); err != nil {
		t.Fatal(err)
	}
	input, err := json.Marshal(map[string]string{"cmd": strings.Repeat("x", int(passageByteLimit)+1)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := collector.Collect(ctx, CodexEvent{HookEventName: postToolUseEvent, SessionID: "bounded", ToolName: "exec_command", ToolInput: input}); err != nil {
		t.Fatal(err)
	}
	resumed, err := collector.Collect(ctx, CodexEvent{HookEventName: sessionStartEvent, SessionID: "bounded"})
	if err != nil || !strings.Contains(strings.Join(resumed.Limits, "\n"), "was truncated") {
		t.Fatalf("resumed evidence lost known truncation: %+v %v", resumed, err)
	}
}

func TestCollectorRejectsAnotherProjectAndEscapingEvidence(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeCollectorFile(t, outside, "secret.txt", "outside content")
	collector, err := NewCollector(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := collector.Collect(context.Background(), CodexEvent{Cwd: outside, Prompt: "review"}); err == nil {
		t.Fatal("collected another project's files")
	}
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "linked.txt")); err != nil {
		t.Skipf("native symlinks unavailable: %v", err)
	}
	_, err = collector.Collect(context.Background(), CodexEvent{HookEventName: userPromptSubmitEvent, SessionID: "three", Prompt: "Review the named evidence."})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := collector.Collect(context.Background(), CodexEvent{HookEventName: preToolUseEvent, SessionID: "three", ToolName: "Read", ToolInput: json.RawMessage(`{"path":"linked.txt"}`)})
	if err != nil {
		t.Fatal(err)
	}
	for _, passage := range evidence.Passages {
		if strings.Contains(passage, "outside content") {
			t.Fatal("submitted escaped evidence")
		}
	}
	if !strings.Contains(strings.Join(evidence.Limits, "\n"), "linked.txt") {
		t.Fatal("did not explain unavailable named evidence")
	}
}

func TestCollectorConcurrentActionHistoryIsNotLost(t *testing.T) {
	root := t.TempDir()
	collector, err := NewCollector(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := collector.Collect(ctx, CodexEvent{HookEventName: userPromptSubmitEvent, SessionID: "parallel", Prompt: "Review workflow."}); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	failures := make(chan error, 8)
	for index := 0; index < 8; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := collector.Collect(ctx, CodexEvent{HookEventName: postToolUseEvent, SessionID: "parallel", ToolName: "Read", ToolInput: json.RawMessage(`{"path":"missing"}`)})
			failures <- err
		}()
	}
	group.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	result, err := collector.Collect(ctx, CodexEvent{HookEventName: stopEvent, SessionID: "parallel"})
	if err != nil || strings.Count(result.Passages["task-actions"], `"Tool":"Read"`) != 8 {
		t.Fatalf("lost concurrent activity: %+v %v", result, err)
	}
}

func TestCollectorRetainsCommittedTaskWorkIncludingUnbornRepository(t *testing.T) {
	for _, initialCommit := range []bool{false, true} {
		t.Run(map[bool]string{false: "unborn", true: "existing"}[initialCommit], func(t *testing.T) {
			root := t.TempDir()
			collectorGit(t, root, "init", "-q")
			collectorGit(t, root, "config", "user.email", "test@example.invalid")
			collectorGit(t, root, "config", "user.name", "Workflow Test")
			writeCollectorFile(t, root, "owner.txt", "initial work")
			if initialCommit {
				collectorGit(t, root, "add", "owner.txt")
				collectorGit(t, root, "commit", "-qm", "initial")
			}
			collector, err := NewCollector(root)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if _, err := collector.Collect(ctx, CodexEvent{HookEventName: userPromptSubmitEvent, SessionID: "commit", Prompt: "Implement the requested behavior."}); err != nil {
				t.Fatal(err)
			}
			writeCollectorFile(t, root, "changed.sql", "select recorded_policy;\n")
			post, err := collector.Collect(ctx, CodexEvent{HookEventName: postToolUseEvent, SessionID: "commit", ToolName: "exec_command", ToolInput: json.RawMessage(`{"cmd":"edit requested source"}`)})
			if err != nil || post.Passages["work/changed.sql"] == "" {
				t.Fatalf("missed changed untracked source: %+v %v", post, err)
			}
			collectorGit(t, root, "add", "changed.sql")
			collectorGit(t, root, "commit", "-qm", "implemented")
			final, err := collector.Collect(ctx, CodexEvent{HookEventName: stopEvent, SessionID: "commit"})
			if err != nil || final.Passages["work/changed.sql"] == "" {
				t.Fatalf("lost source after commit: %+v %v", final, err)
			}
		})
	}
}
