package application

import (
	"errors"
	"reflect"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/workflow"
)

type memoryProgress struct {
	sessions map[string]workflow.Progress
	recorded []workflow.Activity
	err      error
}

func (m *memoryProgress) Update(session string, change func(workflow.Progress, []workflow.Activity) workflow.Progress) error {
	if m.err != nil {
		return m.err
	}
	m.sessions[session] = change(m.sessions[session], m.recorded)
	m.recorded = nil
	return nil
}

type declaredZones struct {
	zones []rule.Zone
	err   error
}

func (d declaredZones) ConfiguredRules() (rule.Configured, error) {
	return rule.Configured{Zones: d.zones}, d.err
}

// nestedZones declares domain over internal/domain and workflow nested in it.
func nestedZones(t *testing.T) declaredZones {
	t.Helper()
	var zones []rule.Zone
	for name, pattern := range map[string]string{"domain": "internal/domain/**", "workflow": "internal/domain/workflow/**"} {
		zoneName, err := rule.NewZoneName(name)
		if err != nil {
			t.Fatal(err)
		}
		glob, err := rule.NewGlob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		zone, err := rule.NewZone(zoneName, "", []rule.Glob{glob})
		if err != nil {
			t.Fatal(err)
		}
		zones = append(zones, zone)
	}
	return declaredZones{zones: zones}
}

func newGuide(t *testing.T, store *memoryProgress, zones declaredZones) GuideWorkflow {
	t.Helper()
	guide, err := NewGuideWorkflow(store, zones)
	if err != nil {
		t.Fatal(err)
	}
	return guide
}

func TestGuideWorkflowKeepsEachSessionsProgress(t *testing.T) {
	guide := newGuide(t, &memoryProgress{sessions: map[string]workflow.Progress{}}, nestedZones(t))
	change := []workflow.Activity{{Kind: workflow.FilesChanged, Paths: []string{"internal/domain/a.go"}}}
	want := []workflow.Guidance{
		{Step: workflow.ContextStep, Paths: []string{"internal/domain/a.go"}},
		{Step: workflow.DomainStep, Paths: []string{"internal/domain/a.go"}},
	}
	if first, err := guide.Execute("one", change); err != nil || !reflect.DeepEqual(first, want) {
		t.Fatalf("first change received %+v, %v", first, err)
	}
	if repeated, err := guide.Execute("one", change); err != nil || len(repeated) != 0 {
		t.Fatalf("repeated change received %+v, %v", repeated, err)
	}
	if other, err := guide.Execute("two", change); err != nil || !reflect.DeepEqual(other, want) {
		t.Fatalf("another session shared progress: %+v, %v", other, err)
	}
}

// Context for internal/domain shows the domain Zone, which owns the
// directory, but not the workflow Zone nested inside it.
func TestGuideWorkflowCreditsTheZonesContextShowed(t *testing.T) {
	store := &memoryProgress{
		sessions: map[string]workflow.Progress{},
		recorded: []workflow.Activity{
			{Kind: workflow.ContextObtained, Paths: []string{"internal/domain"}},
			{Kind: workflow.DomainChanged, Paths: []string{"domain.arclint.yaml"}},
		},
	}
	guide := newGuide(t, store, nestedZones(t))
	given, err := guide.Execute("s", []workflow.Activity{{Kind: workflow.FilesChanged, Paths: []string{"internal/domain/rule.go", "internal/domain/workflow/guide.go", "README.md"}}})
	want := []workflow.Guidance{{Step: workflow.ContextStep, Paths: []string{"internal/domain/workflow/guide.go"}}}
	if err != nil || !reflect.DeepEqual(given, want) {
		t.Fatalf("received %+v, %v; want %+v", given, err, want)
	}
}

func TestGuideWorkflowCreditsNamedZones(t *testing.T) {
	store := &memoryProgress{
		sessions: map[string]workflow.Progress{},
		recorded: []workflow.Activity{
			{Kind: workflow.ContextObtained, Zones: []string{"workflow", "domain"}},
			{Kind: workflow.DomainChanged, Paths: []string{"domain.arclint.yaml"}},
		},
	}
	given, err := newGuide(t, store, nestedZones(t)).Execute("s", []workflow.Activity{{Kind: workflow.FilesChanged, Paths: []string{"internal/domain/workflow/guide.go"}}})
	if err != nil || len(given) != 0 {
		t.Fatalf("context named by Zone was not credited: %+v, %v", given, err)
	}
}

func TestGuideWorkflowWithoutARulesetGivesNoContextReminder(t *testing.T) {
	guide := newGuide(t, &memoryProgress{sessions: map[string]workflow.Progress{}}, declaredZones{err: errors.New("rules.arclint.yaml: invalid")})
	given, err := guide.Execute("s", []workflow.Activity{{Kind: workflow.FilesChanged, Paths: []string{"internal/domain/a.go"}}})
	want := []workflow.Guidance{{Step: workflow.DomainStep, Paths: []string{"internal/domain/a.go"}}}
	if err != nil || !reflect.DeepEqual(given, want) {
		t.Fatalf("received %+v, %v", given, err)
	}
}

func TestGuideWorkflowReportsUnavailableProgress(t *testing.T) {
	guide := newGuide(t, &memoryProgress{err: errors.New("locked")}, nestedZones(t))
	if _, err := guide.Execute("s", []workflow.Activity{{Kind: workflow.TurnFinished}}); err == nil {
		t.Fatal("unavailable progress was not reported")
	}
	if _, err := NewGuideWorkflow(nil, nestedZones(t)); err == nil {
		t.Fatal("missing progress store accepted")
	}
}
