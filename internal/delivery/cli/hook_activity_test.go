package cli

import (
	"errors"
	"reflect"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/agent"
)

// activityRecorder fails every record, so the tests also show that a lost
// record never changes a command's outcome.
type activityRecorder struct {
	activities []agent.Activity
	state      string
}

func (r *activityRecorder) Record(activity agent.Activity) error {
	r.activities = append(r.activities, activity)
	return errors.New("cache unavailable")
}

func (r *activityRecorder) RecordingState() string { return r.state }

func commandTree(recorder *activityRecorder, outcome error, changesRecording bool) Command {
	run := func(Context) error { return outcome }
	define := func(Context) error {
		if changesRecording {
			recorder.state += "+"
		}
		return outcome
	}
	return Command{Name: "arclint", Subcommands: []Command{
		{Name: "context", Run: run},
		{Name: "check", Run: run},
		{Name: "rules", Run: run},
		{Name: "domain", Subcommands: []Command{{Name: "define", Run: define}, {Name: "show", Run: run}}},
	}}
}

func runPath(t *testing.T, root Command, ctx Context, names ...string) error {
	t.Helper()
	command := root
	for _, name := range names {
		found := false
		for _, sub := range command.Subcommands {
			if sub.Name == name {
				command, found = sub, true
			}
		}
		if !found {
			t.Fatalf("no command %v", names)
		}
	}
	return command.Run(ctx)
}

func TestRecordHookActivityReportsWhatRan(t *testing.T) {
	recorder := &activityRecorder{}
	root := RecordHookActivity(commandTree(recorder, nil, true), recorder)
	steps := []struct {
		names []string
		ctx   Context
	}{
		{[]string{"context"}, Context{Args: []string{"./internal/domain", ".", "/abs/elsewhere"}}},
		{[]string{"context"}, Context{}},
		{[]string{"context"}, Context{Flags: map[string]string{"zone": "domain, agent"}}},
		{[]string{"check"}, Context{}},
		{[]string{"check"}, Context{Flags: map[string]string{"only": "domain/no-panic"}}},
		{[]string{"rules"}, Context{}},
		{[]string{"domain", "define"}, Context{}},
		{[]string{"domain", "show"}, Context{}},
	}
	for _, step := range steps {
		if err := runPath(t, root, step.ctx, step.names...); err != nil {
			t.Fatalf("%v: a failed record changed the outcome: %v", step.names, err)
		}
	}
	got := make([]agent.Activity, len(recorder.activities))
	for index, activity := range recorder.activities {
		if len(activity.Zones) == 0 {
			activity.Zones = nil
		}
		got[index] = activity
	}
	want := []agent.Activity{
		{Kind: agent.ContextObtained, Paths: []string{"internal/domain", ".", "/abs/elsewhere"}},
		{Kind: agent.ContextObtained},
		{Kind: agent.ContextObtained, Zones: []string{"domain", "agent"}},
		{Kind: agent.CheckRan},
		{Kind: agent.DomainChanged, Paths: []string{"domain.arclint.yaml"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recorded %+v\nwant %+v", got, want)
	}
}

func TestRecordHookActivityCreditsOnlyAChangedRecording(t *testing.T) {
	recorder := &activityRecorder{}
	root := RecordHookActivity(commandTree(recorder, nil, false), recorder)
	if err := runPath(t, root, Context{}, "domain", "define"); err != nil {
		t.Fatal(err)
	}
	if len(recorder.activities) != 0 {
		t.Fatalf("a domain command that changed nothing was recorded: %+v", recorder.activities)
	}
}

func TestRecordHookActivitySkipsFailedCommands(t *testing.T) {
	recorder := &activityRecorder{}
	root := RecordHookActivity(commandTree(recorder, ConfigError(errors.New("bad ruleset")), true), recorder)
	for _, names := range [][]string{{"context"}, {"check"}, {"domain", "define"}} {
		if err := runPath(t, root, Context{}, names...); err == nil {
			t.Fatalf("%v lost its error", names)
		}
	}
	if len(recorder.activities) != 0 {
		t.Fatalf("failed commands were recorded: %+v", recorder.activities)
	}
	findings := RecordHookActivity(commandTree(recorder, ViolationsExit(), true), recorder)
	if err := runPath(t, findings, Context{}, "check"); err == nil {
		t.Fatal("a gating check lost its exit")
	}
	if !reflect.DeepEqual(recorder.activities, []agent.Activity{{Kind: agent.CheckRan}}) {
		t.Fatalf("a check with findings was not recorded as run: %+v", recorder.activities)
	}
}
