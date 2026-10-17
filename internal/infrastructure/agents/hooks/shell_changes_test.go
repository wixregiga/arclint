package hooks

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

// pause lets the file system clock move past the last write, as the time
// between two tool calls of an agent does.
func pause() { time.Sleep(30 * time.Millisecond) }

// A file a shell command changed gets context and domain advice at the
// agent's next edit, once, together with the edited file.
func TestAShellChangeIsAdvisedAtTheNextEditOnce(t *testing.T) {
	events, root := newGitEvents(t)
	handle(t, events, sessionStart("s", root))
	handle(t, events, prompt("s", root))
	writeFile(t, root, "internal/shell.go", "package a // through python\n")
	pause()
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
	pause()
	writeFile(t, root, recording, "project: shop\ndescription: changed by the librarian\n")
	writeFile(t, root, "internal/shell.go", "package a\n")
	pause()
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
	writeFile(t, root, "internal/between.go", "package a\n")
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
	writeFile(t, root, "internal/between.go", "package a\n")
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
	pause()
	if err := os.Remove(filepath.Join(root, "internal/a.go")); err != nil {
		t.Fatal(err)
	}
	pause()
	record(t, root, agent.Activity{Kind: agent.CheckRan})
	pause()
	if output := handle(t, events, stop("s", root)); output != (hostOutput{}) {
		t.Fatalf("a removal the check verified was advised: %+v", output)
	}
}

// A parallel tool call can run `arclint context` and then change a file
// while the hooks sample the checkout. The sample comes first and the log
// is read after it, so the change arrives with the context that preceded
// it.
func TestContextRecordedWhileTheCheckoutIsSampledIsCredited(t *testing.T) {
	root := gitRepository(t, map[string]string{"rules.arclint.yaml": "zones:\n  app: internal/**\n", "internal/a.go": "package a\n"})
	store := NewProgressStore(root, recording)
	observe(t, store, "s")
	sample := store.sample
	store.sample = func() (snapshot, error) {
		record(t, root, agent.Activity{Kind: agent.ContextObtained, Paths: []string{"internal"}})
		pause()
		writeFile(t, root, "internal/a.go", "package a // changed in parallel\n")
		return sample()
	}
	seen := observe(t, store, "s")
	want := []agent.Activity{{Kind: agent.ContextObtained, Paths: []string{"internal"}}, {Kind: agent.FilesChanged, Paths: []string{"internal/a.go"}}}
	if !reflect.DeepEqual(seen.recorded, want) {
		t.Fatalf("recorded %+v, want %+v", seen.recorded, want)
	}
}
