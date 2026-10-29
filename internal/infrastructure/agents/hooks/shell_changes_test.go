package hooks

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/agent"
)

// newGitEvents answers hook events for an ArcLint project kept in git,
// whose Zone app owns internal/**.
func newGitEvents(t *testing.T) (*Events, string) {
	t.Helper()
	root := gitRepository(t, map[string]string{
		"rules.arclint.yaml": "zones:\n  app: internal/**\n",
		"internal/a.go":      "package a\n",
		recording:            "project: shop\n",
	})
	store := NewProgressStore(root, recording)
	guide, err := application.NewGuideWorkflow(store, appZone{t}, noDomain{}, TestConventions{})
	if err != nil {
		t.Fatal(err)
	}
	events, err := NewEvents(root, filepath.Join(root, "rules.arclint.yaml"), recording, "", testArclint, guide, store)
	if err != nil {
		t.Fatal(err)
	}
	return events, root
}

// prompt is the user's next prompt; each carries its own turn, as Codex
// numbers them, so no two prompts are one event delivered twice.
func prompt(session, cwd string) map[string]any {
	turns++
	return map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": session, "cwd": cwd, "turn_id": turns, "prompt": "go on"}
}

var turns int

// pause waits until the file clock has moved to its next tick, as it does
// between two tool calls of an agent, so what the test does next is
// stamped later than what it did before.
func pause(t *testing.T) {
	t.Helper()
	probe := t.TempDir()
	start := clockOf(t, probe)
	for clockOf(t, probe).Equal(start) {
		time.Sleep(time.Millisecond)
	}
}

// clockOf reads the file clock of the project at root.
func clockOf(t *testing.T, root string) time.Time {
	t.Helper()
	at, err := fileClock(root)
	if err != nil {
		t.Fatal(err)
	}
	return at
}

// A file a shell command changed gets context and domain advice at the
// agent's next edit, once, together with the edited file.
func TestAShellChangeIsAdvisedAtTheNextEditOnce(t *testing.T) {
	events, root := newGitEvents(t)
	handle(t, events, sessionStart("s", root))
	handle(t, events, prompt("s", root))
	writeFile(t, root, "internal/shell.go", "package a // through python\n")
	pause(t)
	edit := toolEvent("s", root, "Edit", map[string]any{"file_path": filepath.Join(root, "internal/a.go")})
	writeFile(t, root, "internal/a.go", "package a // edited\n")
	output := handle(t, events, edit)
	text := output.HookSpecificOutput.AdditionalContext
	for _, want := range []string{
		"internal/shell.go changed before `arclint context` showed every Zone that owns it",
		"internal/a.go changed before `arclint context`",
		"internal/shell.go changed before the project's domain recording changed",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("advice lacks %q:\n%s", want, text)
		}
	}
	if output.HookSpecificOutput.HookEventName != postToolUseEvent {
		t.Fatalf("answered as %q", output.HookSpecificOutput.HookEventName)
	}
	writeFile(t, root, "internal/shell.go", "package a // again\n")
	if again := handle(t, events, toolEvent("s", root, "Edit", map[string]any{"file_path": filepath.Join(root, "internal/a.go"), "old_string": "x"})); again != (hostOutput{}) {
		t.Fatalf("a second change of the same files repeated advice: %+v", again)
	}
}

// A file a Bash command changed is advised right after that command, once;
// a later command that changed nothing prints nothing.
func TestABashCommandIsAdvisedRightAfterItOnce(t *testing.T) {
	events, root := newGitEvents(t)
	handle(t, events, sessionStart("s", root))
	handle(t, events, prompt("s", root))
	writeFile(t, root, "internal/shell.go", "package a // through sed\n")
	output := handle(t, events, toolEvent("s", root, "Bash", map[string]any{"command": "sed -i 's/a/a/' internal/shell.go"}))
	if output.HookSpecificOutput.HookEventName != postToolUseEvent {
		t.Fatalf("answered as %q", output.HookSpecificOutput.HookEventName)
	}
	text := output.HookSpecificOutput.AdditionalContext
	for _, want := range []string{"internal/shell.go changed before `arclint context`", "internal/shell.go changed before the project's domain recording changed"} {
		if !strings.Contains(text, want) {
			t.Fatalf("Bash advice lacks %q:\n%s", want, text)
		}
	}
	if again := handle(t, events, toolEvent("s", root, "Bash", map[string]any{"command": "ls internal"})); again != (hostOutput{}) {
		t.Fatalf("a command that changed nothing repeated advice: %+v", again)
	}
}

// A turn that edits only through the shell is advised when it finishes.
func TestAShellOnlyTurnIsAdvisedWhenItFinishes(t *testing.T) {
	events, root := newGitEvents(t)
	handle(t, events, sessionStart("s", root))
	handle(t, events, prompt("s", root))
	writeFile(t, root, "internal/shell.go", "package a\n")
	text := handle(t, events, stop("s", root)).SystemMessage
	for _, want := range []string{"internal/shell.go changed before `arclint context`", "before the project's domain recording changed", "internal/shell.go changed since the last `arclint check`"} {
		if !strings.Contains(text, want) {
			t.Fatalf("stop advice lacks %q:\n%s", want, text)
		}
	}
}

// A turn that changed nothing prints nothing.
func TestATurnThatChangedNothingPrintsNothing(t *testing.T) {
	events, root := newGitEvents(t)
	writeFile(t, root, "internal/dirty.go", "package a // dirty before the session\n")
	for _, event := range []map[string]any{prompt("s", root), stop("s", root), prompt("s", root), stop("s", root)} {
		if output := handle(t, events, event); output != (hostOutput{}) {
			t.Fatalf("%v answered %+v", event["hook_event_name"], output)
		}
	}
}

// Activities happen in the order they ran, wherever they were seen: a
// shell change after context needs no context, and a shell change before
// the check is verified by it.
func TestShellChangesTakeTheirPlaceAmongArclintsOwnActivities(t *testing.T) {
	events, root := newGitEvents(t)
	handle(t, events, sessionStart("s", root))
	handle(t, events, prompt("s", root))
	record(t, root, agent.Activity{Kind: agent.ContextObtained, Paths: []string{"internal"}})
	pause(t)
	writeFile(t, root, recording, "project: shop\ndescription: changed by the librarian\n")
	writeFile(t, root, "internal/shell.go", "package a\n")
	pause(t)
	record(t, root, agent.Activity{Kind: agent.CheckRan})
	if output := handle(t, events, stop("s", root)); output != (hostOutput{}) {
		t.Fatalf("work done in order was advised: %+v", output)
	}
}

// Files that change between turns, such as the user's own edits, are not
// the agent's work.
func TestChangesBetweenTurnsAreNotTheAgents(t *testing.T) {
	events, root := newGitEvents(t)
	handle(t, events, sessionStart("s", root))
	writeFile(t, root, "internal/before.go", "package a\n")
	handle(t, events, prompt("s", root))
	handle(t, events, stop("s", root))
	pause(t)
	writeFile(t, root, "internal/between.go", "package a\n")
	pause(t)
	if output := handle(t, events, prompt("s", root)); output != (hostOutput{}) {
		t.Fatalf("the user's changes were advised at the prompt: %+v", output)
	}
	if output := handle(t, events, stop("s", root)); output != (hostOutput{}) {
		t.Fatalf("the user's changes were advised when the turn finished: %+v", output)
	}
}

// A turn the user interrupted never finished; what it changed through the
// shell is advised to the agent at the next prompt.
func TestAnInterruptedTurnIsAdvisedAtTheNextPrompt(t *testing.T) {
	events, root := newGitEvents(t)
	handle(t, events, sessionStart("s", root))
	handle(t, events, prompt("s", root))
	writeFile(t, root, "internal/interrupted.go", "package a\n")
	output := handle(t, events, prompt("s", root))
	if output.HookSpecificOutput.HookEventName != promptEvent || !strings.Contains(output.HookSpecificOutput.AdditionalContext, "internal/interrupted.go changed before `arclint context`") {
		t.Fatalf("the interrupted turn's change was not advised at the prompt: %+v", output)
	}
}

// A test changed through the shell needs context and the check, but no
// domain recording; the reminder stays for a change that bears meaning.
func TestAShellChangedTestGetsNoDomainAdvice(t *testing.T) {
	events, root := newGitEvents(t)
	handle(t, events, sessionStart("s", root))
	handle(t, events, prompt("s", root))
	writeFile(t, root, "internal/a_test.go", "package a\n")
	text := handle(t, events, stop("s", root)).SystemMessage
	if !strings.Contains(text, "internal/a_test.go changed before `arclint context`") || strings.Contains(text, "domain recording") {
		t.Fatalf("a test was advised wrongly:\n%s", text)
	}
	handle(t, events, prompt("s", root))
	writeFile(t, root, "internal/b.go", "package a\n")
	if text := handle(t, events, stop("s", root)).SystemMessage; !strings.Contains(text, "internal/b.go changed before the project's domain recording changed") {
		t.Fatalf("the domain reminder was used up by a test:\n%s", text)
	}
}

// The hooks' own records are not the agent's changes, even in a repository
// that does not ignore ArcLint's cache.
func TestTheHooksOwnRecordsAreNotChanges(t *testing.T) {
	events, root := newGitEvents(t)
	handle(t, events, sessionStart("s", root))
	handle(t, events, prompt("s", root))
	if _, err := os.Stat(filepath.Join(root, progressDirectory)); err != nil {
		t.Fatal(err)
	}
	if output := handle(t, events, stop("s", root)); output != (hostOutput{}) {
		t.Fatalf("the cache was taken for a change: %+v", output)
	}
}

// A resumed session starts between turns: what changed while it was away,
// such as the user's own edits, is not the agent's work. Found with a real
// Claude Code session, which sends SessionStart again on --resume.
func TestFilesChangedBeforeASessionResumesAreNotTheAgents(t *testing.T) {
	events, root := newGitEvents(t)
	handle(t, events, sessionStart("s", root))
	handle(t, events, prompt("s", root))
	handle(t, events, stop("s", root))
	pause(t)
	writeFile(t, root, "internal/between.go", "package a\n")
	pause(t)
	resume := sessionStart("s", root)
	resume["source"] = "resume"
	if text := handle(t, events, resume).HookSpecificOutput.AdditionalContext; strings.Contains(text, "internal/between.go") {
		t.Fatalf("the resumed session was advised on the user's change:\n%s", text)
	}
	handle(t, events, prompt("s", root))
	if output := handle(t, events, stop("s", root)); output != (hostOutput{}) {
		t.Fatalf("the user's change was advised after the session resumed: %+v", output)
	}
}

// A session whose turn never finished resumes with what that turn changed,
// and the advice reaches the agent with the session's orientation.
func TestAResumedSessionHearsWhatItsUnfinishedTurnChanged(t *testing.T) {
	events, root := newGitEvents(t)
	handle(t, events, sessionStart("s", root))
	handle(t, events, prompt("s", root))
	writeFile(t, root, "internal/unfinished.go", "package a\n")
	resume := sessionStart("s", root)
	resume["source"] = "resume"
	text := handle(t, events, resume).HookSpecificOutput.AdditionalContext
	if !strings.Contains(text, "ArcLint workflow hooks run in this project.") || !strings.Contains(text, "internal/unfinished.go changed before `arclint context`") {
		t.Fatalf("the unfinished turn's change was not advised on resume:\n%s", text)
	}
}

// A shell removal before `arclint check` is verified by it: the removal
// takes the time its directory changed, not the time the hooks noticed it.
func TestAShellRemovalBeforeTheCheckIsVerified(t *testing.T) {
	events, root := newGitEvents(t)
	handle(t, events, sessionStart("s", root))
	handle(t, events, prompt("s", root))
	record(t, root, agent.Activity{Kind: agent.ContextObtained})
	record(t, root, agent.Activity{Kind: agent.DomainChanged, Paths: []string{recording}})
	pause(t)
	if err := os.Remove(filepath.Join(root, "internal/a.go")); err != nil {
		t.Fatal(err)
	}
	pause(t)
	record(t, root, agent.Activity{Kind: agent.CheckRan})
	pause(t)
	if output := handle(t, events, stop("s", root)); output != (hostOutput{}) {
		t.Fatalf("a removal the check verified was advised: %+v", output)
	}
}

// A parallel tool call can run `arclint context` and then change a file
// while the hooks sample the checkout. Both happened after the sample
// began, so both wait for the next update, which puts the change after the
// context that preceded it.
func TestContextRecordedWhileTheCheckoutIsSampledIsCredited(t *testing.T) {
	root := gitRepository(t, map[string]string{"rules.arclint.yaml": "zones:\n  app: internal/**\n", "internal/a.go": "package a\n"})
	store := NewProgressStore(root, recording)
	observe(t, store, "s")
	sample := store.sample
	store.sample = func() (snapshot, error) {
		record(t, root, agent.Activity{Kind: agent.ContextObtained, Paths: []string{"internal"}})
		pause(t)
		writeFile(t, root, "internal/a.go", "package a // changed in parallel\n")
		return sample()
	}
	if seen := observe(t, store, "s"); len(seen.recorded) != 0 {
		t.Fatalf("the update reported what happened during its sample: %+v", seen.recorded)
	}
	store.sample = sample
	want := []agent.Activity{{Kind: agent.ContextObtained, Paths: []string{"internal"}}, {Kind: agent.FilesChanged, Paths: []string{"internal/a.go"}}}
	if seen := observe(t, store, "s"); !reflect.DeepEqual(seen.recorded, want) {
		t.Fatalf("recorded %+v, want %+v", seen.recorded, want)
	}
}

// A check that finishes while the checkout is sampled, after a parallel
// tool call changed a file the sample missed, is read at the next update,
// together with that change, and keeps its place after it.
func TestACheckFinishingDuringTheSampleWaitsForTheNextUpdate(t *testing.T) {
	root := gitRepository(t, map[string]string{"rules.arclint.yaml": "zones:\n  app: internal/**\n", "internal/a.go": "package a\n"})
	store := NewProgressStore(root, recording)
	observe(t, store, "s")
	sample := store.sample
	store.sample = func() (snapshot, error) {
		taken, err := sample()
		pause(t)
		writeFile(t, root, "internal/a.go", "package a // changed after the sample\n")
		pause(t)
		record(t, root, agent.Activity{Kind: agent.CheckRan})
		return taken, err
	}
	if seen := observe(t, store, "s"); len(seen.recorded) != 0 {
		t.Fatalf("the update read a check that finished during its sample: %+v", seen.recorded)
	}
	store.sample = sample
	want := []agent.Activity{{Kind: agent.FilesChanged, Paths: []string{"internal/a.go"}}, {Kind: agent.CheckRan}}
	if seen := observe(t, store, "s"); !reflect.DeepEqual(seen.recorded, want) {
		t.Fatalf("recorded %+v, want %+v", seen.recorded, want)
	}
}

// When the checkout cannot be read at the prompt, the next sample still
// reports the agent's work in the turn and leaves out what changed before
// the prompt.
func TestAFailedSampleAtThePromptKeepsTheTurnsWork(t *testing.T) {
	root := gitRepository(t, map[string]string{"rules.arclint.yaml": "zones:\n  app: internal/**\n", "internal/a.go": "package a\n"})
	store := NewProgressStore(root, recording)
	observe(t, store, "s")
	if err := store.FinishTurn("s"); err != nil {
		t.Fatal(err)
	}
	pause(t)
	writeFile(t, root, "internal/users.go", "package a // the user's edit between turns\n")
	pause(t)
	if err := store.StartTurn("s"); err != nil {
		t.Fatal(err)
	}
	sample := store.sample
	store.sample = func() (snapshot, error) { return nil, errGitTimedOut }
	if seen := observe(t, store, "s"); len(seen.recorded) != 0 {
		t.Fatalf("an update without a sample reported %+v", seen.recorded)
	}
	store.sample = sample
	pause(t)
	writeFile(t, root, "internal/agents.go", "package a // the agent's edit in the turn\n")
	want := []agent.Activity{{Kind: agent.FilesChanged, Paths: []string{"internal/agents.go"}}}
	if seen := observe(t, store, "s"); !reflect.DeepEqual(seen.recorded, want) {
		t.Fatalf("recorded %+v, want %+v", seen.recorded, want)
	}
}

var errGitTimedOut = errors.New("git status timed out")

// A sample that fails once leaves the log for the next update, which puts
// a shell edit before the check that verified it.
func TestAFailedSampleLeavesTheLogForTheNextUpdate(t *testing.T) {
	root := gitRepository(t, map[string]string{"rules.arclint.yaml": "zones:\n  app: internal/**\n", "internal/a.go": "package a\n"})
	store := NewProgressStore(root, recording)
	observe(t, store, "s")
	writeFile(t, root, "internal/a.go", "package a // changed\n")
	pause(t)
	record(t, root, agent.Activity{Kind: agent.CheckRan})
	pause(t)
	sample := store.sample
	store.sample = func() (snapshot, error) { return nil, errGitTimedOut }
	if seen := observe(t, store, "s"); len(seen.recorded) != 0 {
		t.Fatalf("an update whose sample failed read %+v", seen.recorded)
	}
	store.sample = sample
	want := []agent.Activity{{Kind: agent.FilesChanged, Paths: []string{"internal/a.go"}}, {Kind: agent.CheckRan}}
	if seen := observe(t, store, "s"); !reflect.DeepEqual(seen.recorded, want) {
		t.Fatalf("recorded %+v, want %+v", seen.recorded, want)
	}
}

// A repository git keeps failing on does not hold ArcLint's own work back
// for longer than one update.
func TestRepeatedSampleFailuresStillReadTheLog(t *testing.T) {
	root := gitRepository(t, map[string]string{"rules.arclint.yaml": "zones:\n  app: internal/**\n"})
	store := NewProgressStore(root, recording)
	observe(t, store, "s")
	store.sample = func() (snapshot, error) { return nil, errGitTimedOut }
	record(t, root, agent.Activity{Kind: agent.CheckRan})
	pause(t)
	observe(t, store, "s")
	if seen := observe(t, store, "s"); !reflect.DeepEqual(seen.recorded, []agent.Activity{{Kind: agent.CheckRan}}) {
		t.Fatalf("the second failed sample held the log back: %+v", seen.recorded)
	}
}

// A file that changed while the Stop sample ran belongs to the finished
// turn; the next prompt does not take it for the user's work.
func TestAChangeDeferredAtStopIsReportedAfterThePrompt(t *testing.T) {
	root := gitRepository(t, map[string]string{"rules.arclint.yaml": "zones:\n  app: internal/**\n", "internal/a.go": "package a\n"})
	store := NewProgressStore(root, recording)
	observe(t, store, "s")
	sample := store.sample
	store.sample = func() (snapshot, error) {
		taken, err := sample()
		pause(t)
		writeFile(t, root, "internal/background.go", "package a // written while Stop sampled\n")
		return taken, err
	}
	if seen := observe(t, store, "s"); len(seen.recorded) != 0 {
		t.Fatalf("the Stop update reported %+v", seen.recorded)
	}
	store.sample = sample
	if err := store.FinishTurn("s"); err != nil {
		t.Fatal(err)
	}
	pause(t)
	if err := store.StartTurn("s"); err != nil {
		t.Fatal(err)
	}
	want := []agent.Activity{{Kind: agent.FilesChanged, Paths: []string{"internal/background.go"}}}
	if seen := observe(t, store, "s"); !reflect.DeepEqual(seen.recorded, want) {
		t.Fatalf("recorded %+v, want %+v", seen.recorded, want)
	}
}

// A session that never saw the checkout reports the agent's changes when
// sampling recovers, and leaves out files changed before it began.
func TestASessionThatNeverSampledReportsItsChangesOnRecovery(t *testing.T) {
	root := gitRepository(t, map[string]string{"rules.arclint.yaml": "zones:\n  app: internal/**\n", "internal/a.go": "package a\n"})
	writeFile(t, root, "internal/a.go", "package a // dirty before the session\n")
	pause(t)
	store := NewProgressStore(root, recording)
	sample := store.sample
	store.sample = func() (snapshot, error) { return nil, errGitTimedOut }
	observe(t, store, "s")
	observe(t, store, "s")
	pause(t)
	writeFile(t, root, "internal/agents.go", "package a // the agent's change\n")
	store.sample = sample
	want := []agent.Activity{{Kind: agent.FilesChanged, Paths: []string{"internal/agents.go"}}}
	if seen := observe(t, store, "s"); !reflect.DeepEqual(seen.recorded, want) {
		t.Fatalf("recorded %+v, want %+v", seen.recorded, want)
	}
}

// Every time between turns stays excluded until a sample covers it, even
// when samples fail through a whole turn and the next turn ends.
func TestEveryTimeBetweenTurnsStaysExcludedUntilASampleCoversIt(t *testing.T) {
	root := gitRepository(t, map[string]string{"rules.arclint.yaml": "zones:\n  app: internal/**\n", "internal/a.go": "package a\n"})
	store := NewProgressStore(root, recording)
	observe(t, store, "s")
	for _, step := range []func(string) error{store.StartTurn, store.FinishTurn} {
		if err := step("s"); err != nil {
			t.Fatal(err)
		}
	}
	pause(t)
	writeFile(t, root, "internal/users.go", "package a // the user's edit between turns\n")
	pause(t)
	sample := store.sample
	store.sample = func() (snapshot, error) { return nil, errGitTimedOut }
	if err := store.StartTurn("s"); err != nil {
		t.Fatal(err)
	}
	observe(t, store, "s")
	pause(t)
	writeFile(t, root, "internal/agents.go", "package a // the agent's edit in the turn\n")
	pause(t)
	observe(t, store, "s")
	if err := store.FinishTurn("s"); err != nil {
		t.Fatal(err)
	}
	pause(t)
	if err := store.StartTurn("s"); err != nil {
		t.Fatal(err)
	}
	store.sample = sample
	want := []agent.Activity{{Kind: agent.FilesChanged, Paths: []string{"internal/agents.go"}}}
	if seen := observe(t, store, "s"); !reflect.DeepEqual(seen.recorded, want) {
		t.Fatalf("recorded %+v, want %+v", seen.recorded, want)
	}
}

// Commands that finish at once append in the order of their times, so a
// reader stopping at a time misses no line before it.
func TestConcurrentRecordsAppendInTheOrderOfTheirTimes(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, progressDirectory), 0o700); err != nil {
		t.Fatal(err)
	}
	log := NewActivityLog(root, recording)
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			for range 25 {
				if err := log.Record(agent.Activity{Kind: agent.CheckRan}); err != nil {
					t.Error(err)
				}
			}
		})
	}
	group.Wait()
	data, err := os.ReadFile(filepath.Join(root, activityLog))
	if err != nil {
		t.Fatal(err)
	}
	var previous time.Time
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for index, line := range lines {
		var logged loggedActivity
		if err := json.Unmarshal([]byte(line), &logged); err != nil {
			t.Fatalf("line %d: %v", index, err)
		}
		if logged.At.Before(previous) {
			t.Fatalf("line %d at %v follows a line at %v", index, logged.At, previous)
		}
		previous = logged.At
	}
	if len(lines) != 200 {
		t.Fatalf("%d lines, want 200", len(lines))
	}
}

// A command that took its time before the sample but has not written its
// line yet holds the log; the update waits for the line, so a check timed
// before a shell edit is read with it and does not clear it later.
func TestAnUpdateWaitsForALineTimedBeforeItsSample(t *testing.T) {
	root := gitRepository(t, map[string]string{"rules.arclint.yaml": "zones:\n  app: internal/**\n", "internal/a.go": "package a\n"})
	store := NewProgressStore(root, recording)
	observe(t, store, "s")
	file, err := os.OpenFile(filepath.Join(root, activityLog), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	unlock, err := lockFile(file)
	if err != nil {
		t.Fatal(err)
	}
	checked := clockOf(t, root)
	pause(t)
	writeFile(t, root, "internal/a.go", "package a // changed after the check began writing\n")
	pause(t)
	seen := make(chan observation, 1)
	go func() { seen <- observe(t, store, "s") }()
	time.Sleep(200 * time.Millisecond)
	line, err := json.Marshal(loggedActivity{Kind: string(agent.CheckRan), At: checked})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(append(line, '\n')); err != nil {
		t.Fatal(err)
	}
	if err := unlock(); err != nil {
		t.Fatal(err)
	}
	want := []agent.Activity{{Kind: agent.CheckRan}, {Kind: agent.FilesChanged, Paths: []string{"internal/a.go"}}}
	if got := <-seen; !reflect.DeepEqual(got.recorded, want) {
		t.Fatalf("recorded %+v, want %+v", got.recorded, want)
	}
}

// A copy that keeps an old modification time, as `cp -p` does, changed the
// file when it ran; it is the agent's change in the turn, not one from
// before the prompt.
func TestACopyKeepingAnOldModificationTimeCountsWhenItRan(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file information records no change time")
	}
	root := gitRepository(t, map[string]string{"rules.arclint.yaml": "zones:\n  app: internal/**\n", "internal/a.go": "package a\n"})
	writeFile(t, root, "internal/a.go", "package a // dirty\n")
	old := time.Now().Add(-48 * time.Hour)
	pause(t)
	store := NewProgressStore(root, recording)
	observe(t, store, "s")
	if err := store.StartTurn("s"); err != nil {
		t.Fatal(err)
	}
	sample := store.sample
	store.sample = func() (snapshot, error) { return nil, errGitTimedOut }
	observe(t, store, "s")
	store.sample = sample
	pause(t)
	writeFile(t, root, "internal/a.go", "package a // copied\n")
	if err := os.Chtimes(filepath.Join(root, "internal/a.go"), old, old); err != nil {
		t.Fatal(err)
	}
	want := []agent.Activity{{Kind: agent.FilesChanged, Paths: []string{"internal/a.go"}}}
	if seen := observe(t, store, "s"); !reflect.DeepEqual(seen.recorded, want) {
		t.Fatalf("recorded %+v, want %+v", seen.recorded, want)
	}
}
