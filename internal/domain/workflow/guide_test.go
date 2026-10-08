package workflow

import (
	"reflect"
	"testing"
)

// advise runs activities through one session and returns the Guidance
// each activity received.
func advise(t *testing.T, activities ...Activity) [][]Guidance {
	t.Helper()
	var progress Progress
	received := make([][]Guidance, len(activities))
	for index, activity := range activities {
		progress, received[index] = Guide{}.Advise(progress, activity)
	}
	return received
}

// changed is a change of path, owned by zones.
func changed(path string, zones ...string) Activity {
	return Activity{Kind: FilesChanged, Paths: []string{path}, Zones: zones}
}

// shown is context that showed zones for the paths it named.
func shown(paths []string, zones ...string) Activity {
	return Activity{Kind: ContextObtained, Paths: paths, Zones: zones}
}

var (
	everyZone     = Activity{Kind: ContextObtained}
	domainChanged = Activity{Kind: DomainChanged, Paths: []string{"domain.arclint.yaml"}}
	checkRan      = Activity{Kind: CheckRan}
	turnFinished  = Activity{Kind: TurnFinished}
)

func TestWorkflowInOrderReceivesNoGuidance(t *testing.T) {
	for index, guidance := range advise(t,
		shown([]string{"internal/domain/workflow"}, "domain", "workflow"),
		domainChanged,
		changed("internal/domain/workflow/guide.go", "domain", "workflow"),
		checkRan,
		turnFinished,
	) {
		if len(guidance) != 0 {
			t.Fatalf("activity %d received %+v", index, guidance)
		}
	}
}

func TestContextBeforeChangeNamesEachPathOnce(t *testing.T) {
	received := advise(t,
		domainChanged,
		changed("b.go", "app"),
		changed("b.go", "app"),
		changed("c.go", "app"),
	)
	want := [][]Guidance{
		nil,
		{{Step: ContextStep, Paths: []string{"b.go"}}},
		nil,
		{{Step: ContextStep, Paths: []string{"c.go"}}},
	}
	if !reflect.DeepEqual(received, want) {
		t.Fatalf("received %+v, want %+v", received, want)
	}
}

// Context for a directory shows the Zones that own the directory itself,
// not the narrower Zones nested inside it.
func TestContextCoversOnlyTheZonesItShowed(t *testing.T) {
	received := advise(t,
		domainChanged,
		shown([]string{"internal/domain"}, "domain"),
		changed("internal/domain/rule/root.go", "domain"),
		changed("internal/domain/workflow/guide.go", "domain", "workflow"),
	)
	want := [][]Guidance{nil, nil, nil, {{Step: ContextStep, Paths: []string{"internal/domain/workflow/guide.go"}}}}
	if !reflect.DeepEqual(received, want) {
		t.Fatalf("received %+v, want %+v", received, want)
	}
}

func TestContextNamingNothingShowsEveryZone(t *testing.T) {
	received := advise(t, domainChanged, everyZone, changed("cmd/arclint/main.go", "composition"))
	if len(received[2]) != 0 {
		t.Fatalf("whole-project context still advised: %+v", received[2])
	}
}

func TestContextOfAPathNoZoneOwnsShowsNothing(t *testing.T) {
	received := advise(t, domainChanged, shown([]string{"."}), changed("cmd/arclint/main.go", "composition"))
	if want := []Guidance{{Step: ContextStep, Paths: []string{"cmd/arclint/main.go"}}}; !reflect.DeepEqual(received[2], want) {
		t.Fatalf("context of . was credited: %+v", received[2])
	}
}

func TestContextNamingZonesShowsThem(t *testing.T) {
	received := advise(t, domainChanged, shown(nil, "composition"), changed("cmd/arclint/main.go", "composition"))
	if len(received[2]) != 0 {
		t.Fatalf("context named by Zone was not credited: %+v", received[2])
	}
}

func TestAFileNoZoneOwnsNeedsNoContext(t *testing.T) {
	received := advise(t, domainChanged, changed("README.md"))
	if len(received[1]) != 0 {
		t.Fatalf("a file outside every Zone was advised: %+v", received[1])
	}
}

func TestDomainBeforeChangeRemindsOnce(t *testing.T) {
	received := advise(t,
		everyZone,
		changed("internal/a.go"),
		changed("internal/b.go"),
		domainChanged,
		changed("internal/c.go"),
	)
	want := [][]Guidance{
		nil,
		{{Step: DomainStep, Paths: []string{"internal/a.go"}}},
		nil,
		nil,
		nil,
	}
	if !reflect.DeepEqual(received, want) {
		t.Fatalf("received %+v, want %+v", received, want)
	}
}

func TestDomainChangeIsNotAdvisedAsAChange(t *testing.T) {
	received := advise(t, domainChanged)
	if len(received[0]) != 0 {
		t.Fatalf("changing the domain recording first received %+v", received[0])
	}
}

func TestCheckBeforeFinishRepeatsOnlyForNewChanges(t *testing.T) {
	received := advise(t,
		everyZone,
		domainChanged,
		turnFinished,
		turnFinished,
		changed("a.go"),
		turnFinished,
		checkRan,
		turnFinished,
		changed("a.go"),
		turnFinished,
	)
	want := [][]Guidance{
		nil,
		nil,
		{{Step: CheckStep, Paths: []string{"domain.arclint.yaml"}}},
		nil,
		nil,
		{{Step: CheckStep, Paths: []string{"a.go", "domain.arclint.yaml"}}},
		nil,
		nil,
		nil,
		{{Step: CheckStep, Paths: []string{"a.go"}}},
	}
	if !reflect.DeepEqual(received, want) {
		t.Fatalf("received %+v\nwant %+v", received, want)
	}
}

func TestAdviseLeavesTheGivenProgressUnchanged(t *testing.T) {
	progress := Progress{Zones: []string{"a"}, Unverified: []string{"a/x.go"}}
	before := Progress{Zones: []string{"a"}, Unverified: []string{"a/x.go"}}
	next, _ := Guide{}.Advise(progress, changed("b.go", "b"))
	if !reflect.DeepEqual(progress, before) {
		t.Fatalf("Advise changed its input: %+v", progress)
	}
	if !reflect.DeepEqual(next.Unverified, []string{"a/x.go", "b.go"}) {
		t.Fatalf("next Progress lost a change: %+v", next)
	}
}
