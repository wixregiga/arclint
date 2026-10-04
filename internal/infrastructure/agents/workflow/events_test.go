package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	workflowdomain "github.com/wixregiga/arclint/internal/domain/workflow"
)

type eventEvaluator struct {
	inputs []workflowdomain.Evidence
	fail   bool
}

func (e *eventEvaluator) EvaluateWorkflow(_ context.Context, evidence workflowdomain.Evidence) (workflowdomain.Assessment, error) {
	e.inputs = append(e.inputs, evidence)
	if e.fail {
		return workflowdomain.Assessment{}, errors.New("test model unavailable")
	}
	return workflowdomain.Assessment{Findings: []workflowdomain.Finding{{Evidence: "current-task", Quote: "Keep seat decisions in Reservation", Departure: "The reviewed proposal puts the seat decision in HTTP only.", Correction: "Enforce it in Reservation and verify rejection."}}}, nil
}

func newEventFixture(t *testing.T) (string, *Events, *eventEvaluator) {
	t.Helper()
	root := t.TempDir()
	collector, err := NewCollector(root)
	if err != nil {
		t.Fatal(err)
	}
	evaluator := &eventEvaluator{}
	review, err := application.NewReviewWorkflow(evaluator)
	if err != nil {
		t.Fatal(err)
	}
	events, err := NewEvents(root, collector, review)
	if err != nil {
		t.Fatal(err)
	}
	return root, events, evaluator
}

func nativeEvent(t *testing.T, events *Events, event CodexEvent) map[string]json.RawMessage {
	t.Helper()
	input, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	output, err := events.Handle(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	var protocol map[string]json.RawMessage
	if err := json.Unmarshal(output, &protocol); err != nil {
		t.Fatalf("invalid native output %s: %v", output, err)
	}
	if protocol["decision"] != nil || protocol["continue"] != nil {
		t.Fatalf("advisory report acquired blocking state: %s", output)
	}
	return protocol
}

func TestEventsInitializeThenReportAndReassessPriorClaims(t *testing.T) {
	root, events, evaluator := newEventFixture(t)
	nativeEvent(t, events, CodexEvent{HookEventName: "SessionStart", SessionID: "one", Cwd: root})
	if len(evaluator.inputs) != 0 {
		t.Fatal("session initialization launched semantic review without a task")
	}
	response := nativeEvent(t, events, CodexEvent{HookEventName: "UserPromptSubmit", SessionID: "one", Cwd: root, Prompt: "Keep seat decisions in Reservation"})
	if !strings.Contains(string(response["hookSpecificOutput"]), "Enforce it in Reservation") {
		t.Fatalf("finding and correction lost: %s", response)
	}
	nativeEvent(t, events, CodexEvent{HookEventName: "PostToolUse", SessionID: "one", Cwd: root, ToolName: "apply_patch", ToolInput: json.RawMessage(`{"patch":"move check into Reservation"}`)})
	if len(evaluator.inputs) != 2 || !strings.Contains(evaluator.inputs[1].Passages["previous-workflow-reports"], "HTTP only") {
		t.Fatalf("next review lacks previous claim for reassessment: %#v", evaluator.inputs)
	}
	if evaluator.inputs[1].Task != "Keep seat decisions in Reservation" {
		t.Fatal("prior report replaced the original task")
	}
	if !strings.Contains(strings.Join(evaluator.inputs[1].Limits, "\n"), "claims to reassess") {
		t.Fatal("previous findings were presented as current defects")
	}
	data, err := os.ReadFile(filepath.Join(root, reportName("one")))
	if err != nil || strings.Count(string(data), "\n") != 3 {
		t.Fatalf("separate activity reports not retained: %s %v", data, err)
	}
}

func TestEventsUnavailableStopIsFeedbackAndOutsideProjectIsIgnored(t *testing.T) {
	root, events, evaluator := newEventFixture(t)
	evaluator.fail = true
	nativeEvent(t, events, CodexEvent{HookEventName: "UserPromptSubmit", SessionID: "one", Cwd: root, Prompt: "Keep seat decisions in Reservation"})
	response := nativeEvent(t, events, CodexEvent{HookEventName: "Stop", SessionID: "one", Cwd: root})
	if !strings.Contains(string(response["systemMessage"]), "review unavailable") {
		t.Fatalf("unavailable reviewer represented as success: %s", response)
	}
	calls := len(evaluator.inputs)
	response = nativeEvent(t, events, CodexEvent{HookEventName: "UserPromptSubmit", SessionID: "other", Cwd: t.TempDir(), Prompt: "unrelated task"})
	if len(response) != 0 || len(evaluator.inputs) != calls {
		t.Fatal("hook assessed an unrelated project")
	}
}

func TestEventsWSLDesktopCwdAcceptsOnlyCurrentDistroAndProject(t *testing.T) {
	root, events, evaluator := newEventFixture(t)
	t.Setenv("WSL_DISTRO_NAME", "FixtureUbuntu")
	for _, host := range []string{"wsl.localhost", "wsl$"} {
		cwd := `\\` + host + `\FixtureUbuntu` + strings.ReplaceAll(root, "/", `\`)
		response := nativeEvent(t, events, CodexEvent{HookEventName: userPromptSubmitEvent, SessionID: "desktop", Cwd: cwd, Prompt: "Keep seat decisions in Reservation"})
		if response["hookSpecificOutput"] == nil {
			t.Fatalf("current-distro desktop event silently discarded: %#v", response)
		}
	}
	if len(evaluator.inputs) != 2 {
		t.Fatal("desktop cwd did not reach collection and review")
	}
	for _, cwd := range []string{`\\wsl.localhost\OtherUbuntu` + strings.ReplaceAll(root, "/", `\`), `\\wsl$\FixtureUbuntu` + strings.ReplaceAll(t.TempDir(), "/", `\`)} {
		response := nativeEvent(t, events, CodexEvent{HookEventName: userPromptSubmitEvent, SessionID: "outside", Cwd: cwd, Prompt: "unrelated"})
		if len(response) != 0 || len(evaluator.inputs) != 2 {
			t.Fatal("accepted another distro or unrelated project")
		}
	}
	t.Setenv("WSL_DISTRO_NAME", "")
	cwd := `\\wsl.localhost\FixtureUbuntu` + strings.ReplaceAll(root, "/", `\`)
	if normalizeEventCwd(cwd) != cwd {
		t.Fatal("invented a current distro when runtime evidence was absent")
	}
}
