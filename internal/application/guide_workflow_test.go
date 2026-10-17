package application

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/agent"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

type memoryProgress struct {
	sessions map[string]agent.Progress
	recorded []agent.Activity
	err      error
}

func (m *memoryProgress) Update(session string, change func(agent.Progress, []agent.Activity) agent.Progress) error {
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
	for name, pattern := range map[string]string{"domain": "internal/domain/**", "agent": "internal/domain/agent/**"} {
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

// recordedDomain is a domain recording read from memory.
type recordedDomain struct {
	lang  vocab.UbiquitousLanguage
	found bool
	err   error
}

func (d *recordedDomain) Recording() vocab.DomainRecording { return vocab.DomainRecording{} }

func (d *recordedDomain) RecordedLanguage() (vocab.UbiquitousLanguage, bool, error) {
	return d.lang, d.found, d.err
}

func (d *recordedDomain) Record(vocab.UbiquitousLanguage) error {
	return errors.New("the guide never records the domain")
}

// goTests tells Go test files apart.
type goTests struct{}

func (goTests) Test(path string) bool { return strings.HasSuffix(path, "_test.go") }

func newGuide(t *testing.T, store *memoryProgress, zones declaredZones) GuideWorkflow {
	t.Helper()
	return newGuideOf(t, store, zones, &recordedDomain{})
}

func newGuideOf(t *testing.T, store *memoryProgress, zones declaredZones, knowledge vocab.Repository) GuideWorkflow {
	t.Helper()
	guide, err := NewGuideWorkflow(store, zones, knowledge, goTests{})
	if err != nil {
		t.Fatal(err)
	}
	return guide
}

// recordedContexts is a domain recording holding the named contexts.
func recordedContexts(t *testing.T, names ...string) *recordedDomain {
	t.Helper()
	contexts := make([]vocab.BoundedContext, len(names))
	for index, name := range names {
		contexts[index] = vocab.BoundedContext{Name: name, Definition: "The " + name + " context."}
	}
	language, err := vocab.NewUbiquitousLanguage("shop", "", contexts, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &recordedDomain{lang: language, found: true}
}

// domainAdvised returns the paths the domain reminder named when each
// change below happened first in a fresh session.
func domainAdvised(t *testing.T, knowledge vocab.Repository, paths ...string) []string {
	t.Helper()
	var advised []string
	for _, path := range paths {
		given, err := newGuideOf(t, &memoryProgress{sessions: map[string]agent.Progress{}}, nestedZones(t), knowledge).Execute("s", []agent.Activity{{Kind: agent.FilesChanged, Paths: []string{path}}})
		if err != nil {
			t.Fatal(err)
		}
		for _, guidance := range given {
			if guidance.Step == agent.DomainStep {
				advised = append(advised, guidance.Paths...)
			}
		}
	}
	return advised
}

// The agent context's code is the agent Zone, the Zone named for it, so
// only a file there that is not a test bears domain meaning.
func TestGuideWorkflowAdvisesDomainOnlyForTheModelsCode(t *testing.T) {
	advised := domainAdvised(t, recordedContexts(t, "agent"),
		"internal/domain/agent/guide.go",
		"internal/domain/agent/guide_test.go",
		"internal/domain/rule.go",
		"tools/gate.sh",
	)
	if want := []string{"internal/domain/agent/guide.go"}; !reflect.DeepEqual(advised, want) {
		t.Fatalf("domain reminder named %v, want %v", advised, want)
	}
}

// While a context has no Zone named for it, or nothing is recorded yet,
// the model's code may lie anywhere, as the check reads it; tests still
// bear no domain meaning.
func TestGuideWorkflowWithoutAZoneForTheModelAdvisesEveryFileButTests(t *testing.T) {
	for name, knowledge := range map[string]vocab.Repository{
		"unanchored context": recordedContexts(t, "agent", "billing"),
		"no context":         &recordedDomain{},
		"unreadable":         &recordedDomain{err: errors.New("domain.arclint.yaml: invalid")},
	} {
		advised := domainAdvised(t, knowledge, "tools/gate.sh", "internal/domain/rule.go", "internal/domain/rule_test.go")
		if want := []string{"tools/gate.sh", "internal/domain/rule.go"}; !reflect.DeepEqual(advised, want) {
			t.Fatalf("%s: domain reminder named %v, want %v", name, advised, want)
		}
	}
}

func TestGuideWorkflowKeepsEachSessionsProgress(t *testing.T) {
	guide := newGuide(t, &memoryProgress{sessions: map[string]agent.Progress{}}, nestedZones(t))
	change := []agent.Activity{{Kind: agent.FilesChanged, Paths: []string{"internal/domain/a.go"}}}
	want := []agent.Guidance{
		{Step: agent.ContextStep, Paths: []string{"internal/domain/a.go"}},
		{Step: agent.DomainStep, Paths: []string{"internal/domain/a.go"}},
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
		sessions: map[string]agent.Progress{},
		recorded: []agent.Activity{
			{Kind: agent.ContextObtained, Paths: []string{"internal/domain"}},
			{Kind: agent.DomainChanged, Paths: []string{"domain.arclint.yaml"}},
		},
	}
	guide := newGuide(t, store, nestedZones(t))
	given, err := guide.Execute("s", []agent.Activity{{Kind: agent.FilesChanged, Paths: []string{"internal/domain/rule.go", "internal/domain/agent/guide.go", "README.md"}}})
	want := []agent.Guidance{{Step: agent.ContextStep, Paths: []string{"internal/domain/agent/guide.go"}}}
	if err != nil || !reflect.DeepEqual(given, want) {
		t.Fatalf("received %+v, %v; want %+v", given, err, want)
	}
}

func TestGuideWorkflowCreditsNamedZones(t *testing.T) {
	store := &memoryProgress{
		sessions: map[string]agent.Progress{},
		recorded: []agent.Activity{
			{Kind: agent.ContextObtained, Zones: []string{"agent", "domain"}},
			{Kind: agent.DomainChanged, Paths: []string{"domain.arclint.yaml"}},
		},
	}
	given, err := newGuide(t, store, nestedZones(t)).Execute("s", []agent.Activity{{Kind: agent.FilesChanged, Paths: []string{"internal/domain/agent/guide.go"}}})
	if err != nil || len(given) != 0 {
		t.Fatalf("context named by Zone was not credited: %+v, %v", given, err)
	}
}

func TestGuideWorkflowWithoutARulesetGivesNoContextReminder(t *testing.T) {
	guide := newGuide(t, &memoryProgress{sessions: map[string]agent.Progress{}}, declaredZones{err: errors.New("rules.arclint.yaml: invalid")})
	given, err := guide.Execute("s", []agent.Activity{{Kind: agent.FilesChanged, Paths: []string{"internal/domain/a.go"}}})
	want := []agent.Guidance{{Step: agent.DomainStep, Paths: []string{"internal/domain/a.go"}}}
	if err != nil || !reflect.DeepEqual(given, want) {
		t.Fatalf("received %+v, %v", given, err)
	}
}

func TestGuideWorkflowReportsUnavailableProgress(t *testing.T) {
	guide := newGuide(t, &memoryProgress{err: errors.New("locked")}, nestedZones(t))
	if _, err := guide.Execute("s", []agent.Activity{{Kind: agent.TurnFinished}}); err == nil {
		t.Fatal("unavailable progress was not reported")
	}
	if _, err := NewGuideWorkflow(nil, nestedZones(t), &recordedDomain{}, goTests{}); err == nil {
		t.Fatal("missing progress store accepted")
	}
}
