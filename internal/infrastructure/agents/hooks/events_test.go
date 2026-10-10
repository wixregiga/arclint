package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/agent"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

const (
	recording   = "domain.arclint.yaml"
	testArclint = "/usr/local/bin/arclint"
)

type hostOutput struct {
	SystemMessage      string `json:"systemMessage"`
	HookSpecificOutput struct {
		HookEventName     string `json:"hookEventName"`
		AdditionalContext string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

// appZone is a ruleset declaring the Zone app over internal/**.
type appZone struct{ t *testing.T }

func (z appZone) ConfiguredRules() (rule.Configured, error) {
	name, err := rule.NewZoneName("app")
	if err != nil {
		z.t.Fatal(err)
	}
	glob, err := rule.NewGlob("internal/**")
	if err != nil {
		z.t.Fatal(err)
	}
	zone, err := rule.NewZone(name, "", []rule.Glob{glob})
	if err != nil {
		z.t.Fatal(err)
	}
	return rule.Configured{Zones: []rule.Zone{zone}}, nil
}

func newTestEvents(t *testing.T, distro string) (*Events, string) {
	t.Helper()
	root := t.TempDir()
	store := NewProgressStore(root)
	guide, err := application.NewGuideWorkflow(store, appZone{t})
	if err != nil {
		t.Fatal(err)
	}
	ruleset := filepath.Join(root, "rules.arclint.yaml")
	if err := os.WriteFile(ruleset, []byte("zones:\n  app: internal/**\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	events, err := NewEvents(root, ruleset, recording, distro, testArclint, guide, store)
	if err != nil {
		t.Fatal(err)
	}
	return events, events.project.root
}

// A user install runs the hooks in every project the host opens; where no
// ruleset exists they answer nothing and leave no files.
func TestEventsOutsideAnArclintProjectAnswerNothing(t *testing.T) {
	root := t.TempDir()
	store := NewProgressStore(root)
	guide, err := application.NewGuideWorkflow(store, appZone{t})
	if err != nil {
		t.Fatal(err)
	}
	events, err := NewEvents(root, filepath.Join(root, "rules.arclint.yaml"), recording, "", testArclint, guide, store)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []map[string]any{
		sessionStart("s", root),
		{"hook_event_name": "PostToolUse", "session_id": "s", "cwd": root, "tool_name": "Edit", "tool_input": map[string]any{"file_path": filepath.Join(root, "main.go")}},
		{"hook_event_name": "Stop", "session_id": "s", "cwd": root},
	} {
		if output := handle(t, events, event); output != (hostOutput{}) {
			t.Fatalf("%v answered %+v", event["hook_event_name"], output)
		}
	}
	output, err := events.Handle([]byte("not json"))
	if err != nil || strings.TrimSpace(string(output)) != "{}" {
		t.Fatalf("malformed event outside a project: %q %v", output, err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("files written outside a project: %v %v", entries, err)
	}
}

func handle(t *testing.T, events *Events, event map[string]any) hostOutput {
	t.Helper()
	input, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	output, err := events.Handle(input)
	if err != nil {
		t.Fatal(err)
	}
	var decoded hostOutput
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatalf("output is not one JSON object: %v\n%s", err, output)
	}
	return decoded
}

func sessionStart(session, cwd string) map[string]any {
	return map[string]any{"hook_event_name": "SessionStart", "session_id": session, "cwd": cwd, "source": "startup"}
}

func stop(session, cwd string) map[string]any {
	return map[string]any{"hook_event_name": "Stop", "session_id": session, "cwd": cwd, "stop_hook_active": false}
}

func toolEvent(session, cwd, tool string, input map[string]any) map[string]any {
	return map[string]any{"hook_event_name": "PostToolUse", "session_id": session, "cwd": cwd, "tool_name": tool, "tool_input": input}
}

func record(t *testing.T, root string, activity agent.Activity) {
	t.Helper()
	if err := NewActivityLog(root, recording).Record(activity); err != nil {
		t.Fatal(err)
	}
}

func TestSessionStartStatesTheWorkflowOrder(t *testing.T) {
	events, root := newTestEvents(t, "")
	output := handle(t, events, sessionStart("s", root))
	text := output.HookSpecificOutput.AdditionalContext
	if output.HookSpecificOutput.HookEventName != "SessionStart" || output.SystemMessage != "" {
		t.Fatalf("wrong output: %+v", output)
	}
	for index, step := range application.AgentWorkflow(recording) {
		if !strings.Contains(text, fmt.Sprintf("%d. %s\n", index+1, step)) {
			t.Fatalf("orientation lacks workflow step %d %q:\n%s", index+1, step, text)
		}
	}
	if !strings.Contains(text, "They never block a tool call.") {
		t.Fatalf("orientation does not say the hooks never block:\n%s", text)
	}
}

func TestClaudeCodeEditBeforeContextAndDomain(t *testing.T) {
	events, root := newTestEvents(t, "")
	handle(t, events, sessionStart("s", root))
	edit := toolEvent("s", root, "Edit", map[string]any{"file_path": filepath.Join(root, "internal/a.go"), "old_string": "x", "new_string": "y"})
	text := handle(t, events, edit).HookSpecificOutput.AdditionalContext
	for _, want := range []string{"internal/a.go changed before `arclint context` showed every Zone that owns it", "Run `arclint context internal/a.go`", "changed before domain.arclint.yaml changed in this session"} {
		if !strings.Contains(text, want) {
			t.Fatalf("guidance lacks %q:\n%s", want, text)
		}
	}
	if repeated := handle(t, events, edit); repeated != (hostOutput{}) {
		t.Fatalf("an identical edit repeated guidance: %+v", repeated)
	}
}

func TestCodexWorkflowInOrderReceivesNoGuidance(t *testing.T) {
	events, root := newTestEvents(t, "")
	handle(t, events, sessionStart("s", root))
	record(t, root, agent.Activity{Kind: agent.ContextObtained, Paths: []string{"internal"}})
	for index, event := range []map[string]any{
		toolEvent("s", root, "apply_patch", map[string]any{"command": "*** Begin Patch\n*** Update File: domain.arclint.yaml\n*** End Patch\n"}),
		toolEvent("s", root, "apply_patch", map[string]any{"command": "*** Begin Patch\n*** Add File: internal/a.go\n+package a\n*** End Patch\n"}),
	} {
		if output := handle(t, events, event); output != (hostOutput{}) {
			t.Fatalf("event %d received %+v", index, output)
		}
	}
	record(t, root, agent.Activity{Kind: agent.CheckRan})
	if output := handle(t, events, stop("s", root)); output != (hostOutput{}) {
		t.Fatalf("stop after the check received %+v", output)
	}
}

func TestActivityBeforeASessionStartsIsNotCredited(t *testing.T) {
	events, root := newTestEvents(t, "")
	handle(t, events, sessionStart("earlier", root))
	record(t, root, agent.Activity{Kind: agent.ContextObtained})
	record(t, root, agent.Activity{Kind: agent.DomainChanged, Paths: []string{recording}})
	handle(t, events, sessionStart("later", root))
	text := handle(t, events, toolEvent("later", root, "Write", map[string]any{"file_path": filepath.Join(root, "internal/a.go")})).HookSpecificOutput.AdditionalContext
	if !strings.Contains(text, "internal/a.go changed before `arclint context` showed every Zone that owns it") || !strings.Contains(text, "before domain.arclint.yaml changed") {
		t.Fatalf("the later session was credited with earlier activity:\n%s", text)
	}
	if output := handle(t, events, toolEvent("earlier", root, "Write", map[string]any{"file_path": filepath.Join(root, "internal/a.go")})); output != (hostOutput{}) {
		t.Fatalf("the earlier session lost its recorded activity: %+v", output)
	}
}

func TestContextCreditFollowsTheZonesItShowed(t *testing.T) {
	events, root := newTestEvents(t, "")
	handle(t, events, sessionStart("s", root))
	record(t, root, agent.Activity{Kind: agent.ContextObtained, Paths: []string{"."}})
	text := handle(t, events, toolEvent("s", root, "Edit", map[string]any{"file_path": filepath.Join(root, "internal/a.go")})).HookSpecificOutput.AdditionalContext
	if !strings.Contains(text, "internal/a.go changed before `arclint context` showed every Zone that owns it") {
		t.Fatalf("context of . was credited:\n%s", text)
	}
	text = handle(t, events, toolEvent("s", root, "Edit", map[string]any{"file_path": filepath.Join(root, "README.md")})).HookSpecificOutput.AdditionalContext
	if strings.Contains(text, "arclint context") {
		t.Fatalf("a file no Zone owns was advised to obtain context:\n%s", text)
	}
	record(t, root, agent.Activity{Kind: agent.ContextObtained, Zones: []string{"app"}})
	if output := handle(t, events, toolEvent("s", root, "Edit", map[string]any{"file_path": filepath.Join(root, "internal/b.go")})); output != (hostOutput{}) {
		t.Fatalf("context named by Zone was not credited: %+v", output)
	}
}

func TestForgedLogLinesAreNotCredited(t *testing.T) {
	events, root := newTestEvents(t, "")
	handle(t, events, sessionStart("s", root))
	file, err := os.OpenFile(filepath.Join(root, activityLog), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(`{"kind":"change","paths":["internal/x.go"]}` + "\n" + `{"kind":"finish"}` + "\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if output := handle(t, events, stop("s", root)); output != (hostOutput{}) {
		t.Fatalf("forged lines produced %+v", output)
	}
}

func TestShellToolEventsAreNotRead(t *testing.T) {
	events, root := newTestEvents(t, "")
	handle(t, events, sessionStart("s", root))
	output := handle(t, events, toolEvent("s", root, "Bash", map[string]any{"command": `git commit -m "arclint check . passes" && echo x > a.go`}))
	if output != (hostOutput{}) {
		t.Fatalf("a shell event produced %+v", output)
	}
	text := handle(t, events, stop("s", root)).SystemMessage
	if text != "" {
		t.Fatalf("shell text was credited or counted: %q", text)
	}
}

func TestStopAdvisesTheCheckOnceForTheSameChanges(t *testing.T) {
	events, root := newTestEvents(t, "")
	handle(t, events, sessionStart("s", root))
	record(t, root, agent.Activity{Kind: agent.ContextObtained})
	handle(t, events, toolEvent("s", root, "Write", map[string]any{"file_path": filepath.Join(root, recording), "content": "x"}))
	output := handle(t, events, stop("s", root))
	if !strings.Contains(output.SystemMessage, "domain.arclint.yaml changed since the last `arclint check`") || output.HookSpecificOutput.AdditionalContext != "" {
		t.Fatalf("stop advice missing: %+v", output)
	}
	if again := handle(t, events, stop("s", root)); again != (hostOutput{}) {
		t.Fatalf("stop repeated identical advice: %+v", again)
	}
}

func TestWindowsHostPaths(t *testing.T) {
	events, root := newTestEvents(t, "Ubuntu")
	share := `\\wsl.localhost\ubuntu` + strings.ReplaceAll(root, "/", `\`)
	handle(t, events, sessionStart("s", share))
	for _, outside := range []string{`C:\Users\me\.claude\projects\p\memory\MEMORY.md`, `\\wsl.localhost\Debian` + strings.ReplaceAll(root, "/", `\`) + `\a.go`, `\\server\share\a.go`} {
		if output := handle(t, events, toolEvent("s", share, "Write", map[string]any{"file_path": outside})); output != (hostOutput{}) {
			t.Fatalf("%s was read as project work: %+v", outside, output)
		}
	}
	text := handle(t, events, toolEvent("s", share, "Edit", map[string]any{"file_path": share + `\internal\a.go`})).HookSpecificOutput.AdditionalContext
	if !strings.Contains(text, "internal/a.go changed before `arclint context` showed every Zone that owns it") {
		t.Fatalf("share path was not read as project work:\n%s", text)
	}
}

func TestLocalMapsWindowsPathsIntoTheDistribution(t *testing.T) {
	p := project{root: "/home/me/repo", distro: "Ubuntu"}
	for input, want := range map[string]string{
		`\\wsl.localhost\ubuntu\home\me\repo\a.go`: "/home/me/repo/a.go",
		`//wsl$/Ubuntu/home/me/repo`:               "/home/me/repo",
		`C:\Users\me\file.txt`:                     "/mnt/c/Users/me/file.txt",
		`d:/data`:                                  "/mnt/d/data",
		`\\wsl.localhost\Debian\home\me\repo\a.go`: `\\wsl.localhost\Debian\home\me\repo\a.go`,
		"/home/me/repo/a.go":                       "/home/me/repo/a.go",
	} {
		if got := p.local(input); got != want {
			t.Errorf("local(%q) = %q, want %q", input, got, want)
		}
	}
	if got := (project{}).local(`C:\x`); got != `C:\x` {
		t.Errorf("outside WSL a Windows path changed: %q", got)
	}
}

func TestEventsOutsideTheProjectAreIgnored(t *testing.T) {
	events, _ := newTestEvents(t, "")
	elsewhere := t.TempDir()
	output := handle(t, events, toolEvent("s", elsewhere, "Edit", map[string]any{"file_path": filepath.Join(elsewhere, "a.go")}))
	if output != (hostOutput{}) {
		t.Fatalf("another project's event received %+v", output)
	}
}

func TestMalformedInputIsReportedWithoutBlocking(t *testing.T) {
	events, root := newTestEvents(t, "")
	output, err := events.Handle([]byte("not json"))
	if err != nil || !strings.Contains(string(output), "ArcLint workflow guidance unavailable") {
		t.Fatalf("malformed event: %s %v", output, err)
	}
	unexpected := handle(t, events, toolEvent("s", root, "apply_patch", map[string]any{"command": []string{"*** Update File: a.go"}}))
	if unexpected != (hostOutput{}) {
		t.Fatalf("a non-string patch produced output: %+v", unexpected)
	}
}

func TestUnavailableProgressIsReportedOnceAtSessionStart(t *testing.T) {
	events, root := newTestEvents(t, "")
	if err := os.WriteFile(filepath.Join(root, ".arclint"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if output := handle(t, events, sessionStart("s", root)); !strings.Contains(output.SystemMessage, "ArcLint workflow guidance unavailable for this session") || output.HookSpecificOutput.AdditionalContext == "" {
		t.Fatalf("session start did not report the store: %+v", output)
	}
	for _, event := range []map[string]any{toolEvent("s", root, "Write", map[string]any{"file_path": filepath.Join(root, "a.go")}), stop("s", root)} {
		if output := handle(t, events, event); output != (hostOutput{}) {
			t.Fatalf("the store failure repeated: %+v", output)
		}
	}
}

func TestParallelEventsOfOneSessionNameEachPathOnce(t *testing.T) {
	events, root := newTestEvents(t, "")
	handle(t, events, sessionStart("s", root))
	handle(t, events, toolEvent("s", root, "Write", map[string]any{"file_path": filepath.Join(root, recording)}))
	var group sync.WaitGroup
	outputs := make([]hostOutput, 16)
	for index := range outputs {
		group.Add(1)
		go func() {
			defer group.Done()
			path := filepath.Join(root, fmt.Sprintf("internal/file%d.go", index%4))
			outputs[index] = handle(t, events, toolEvent("s", root, "Edit", map[string]any{"file_path": path}))
		}()
	}
	group.Wait()
	for index := range 4 {
		named := 0
		for _, output := range outputs {
			if strings.Contains(output.HookSpecificOutput.AdditionalContext, fmt.Sprintf("Run `arclint context internal/file%d.go`", index)) {
				named++
			}
		}
		if named != 1 {
			t.Fatalf("file%d.go named %d times", index, named)
		}
	}
}

func TestPatchPaths(t *testing.T) {
	patch := "*** Begin Patch\r\n*** Add File: a.go\r\n+x\r\n*** Update File: b.go\r\n*** Move to: c.go\r\n*** Delete File: d.go\r\n*** End Patch\r\n"
	if got, want := patchPaths(patch), []string{"a.go", "b.go", "c.go", "d.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// A host that lists the hooks in two configuration files runs both with
// the same event; the session hears the advice once.
func TestARepeatedDeliveryOfOneEventIsAnsweredOnce(t *testing.T) {
	events, root := newTestEvents(t, "")
	handle(t, events, sessionStart("s", root))
	edit := map[string]any{"hook_event_name": "PostToolUse", "session_id": "s", "cwd": root, "tool_name": "Edit", "tool_use_id": "toolu_1", "tool_input": map[string]any{"file_path": filepath.Join(root, "internal/app.go")}}
	input, err := json.Marshal(edit)
	if err != nil {
		t.Fatal(err)
	}
	outputs := make([]string, 2)
	var wait sync.WaitGroup
	for index := range outputs {
		wait.Add(1)
		go func() {
			defer wait.Done()
			output, err := events.Handle(input)
			if err != nil {
				t.Error(err)
			}
			outputs[index] = strings.TrimSpace(string(output))
		}()
	}
	wait.Wait()
	answered := 0
	for _, output := range outputs {
		if output != "{}" {
			answered++
		}
	}
	if answered != 1 {
		t.Fatalf("parallel deliveries of one event answered %d times: %q", answered, outputs)
	}
	edit["tool_use_id"] = "toolu_2"
	edit["tool_input"] = map[string]any{"file_path": filepath.Join(root, "internal/other.go")}
	if output := handle(t, events, edit); !strings.Contains(output.HookSpecificOutput.AdditionalContext, "internal/other.go changed before `arclint context`") {
		t.Fatalf("the next event went unanswered: %+v", output)
	}
}

// The committed AGENTS.md block comes from the arclint that generated it;
// when it states another workflow, the session hears which binary differs.
func TestSessionStartNotesAnAgentsBlockStatingAnotherWorkflow(t *testing.T) {
	for name, tc := range map[string]struct {
		content string
		note    bool
	}{
		"no AGENTS.md":        {"", false},
		"no generated block":  {"# Guide\n", false},
		"the same workflow":   {"# Guide\n" + application.AgentsBegin + "\nintro\n\n" + application.AgentWorkflowSection() + "### Commands\n\n- one\n" + application.AgentsEnd + "\n", false},
		"another workflow":    {"# Guide\n" + application.AgentsBegin + "\n### Workflow\n\n1. Write code.\n\n### Commands\n" + application.AgentsEnd + "\n", true},
		"a block without one": {application.AgentsBegin + "\n### Commands\n" + application.AgentsEnd + "\n", true},
	} {
		t.Run(name, func(t *testing.T) {
			events, root := newTestEvents(t, "")
			if tc.content != "" {
				if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(strings.ReplaceAll(tc.content, "\n", "\r\n")), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			text := handle(t, events, sessionStart("s", root)).HookSpecificOutput.AdditionalContext
			noted := strings.Contains(text, "The arclint these hooks run ("+testArclint+") states a different workflow than the AGENTS.md block.")
			if noted != tc.note || !strings.Contains(text, "Work in this order:") {
				t.Fatalf("note %t, want %t:\n%s", noted, tc.note, text)
			}
		})
	}
}
