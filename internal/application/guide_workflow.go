package application

import (
	"fmt"
	"slices"

	"github.com/wixregiga/arclint/internal/domain/agent"
	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// SessionProgress keeps each session's workflow Progress between the
// activities an Agent Host reports.
type SessionProgress interface {
	// Update replaces the session's Progress with change's result. change
	// also receives, in the order they happened, the activities no host
	// event reported since the session last read them: those ArcLint's own
	// commands recorded, and the files the agent changed through a shell.
	// Updates of one session apply one at a time.
	Update(session string, change func(progress agent.Progress, recorded []agent.Activity) agent.Progress) error
}

// TestFiles tells a project's test files from the rest by the naming
// conventions of the languages they are written in.
type TestFiles interface {
	// Test reports whether the slash-separated, project-relative path is
	// a test or test data.
	Test(path string) bool
}

// GuideWorkflow advises a session on what ran in it.
type GuideWorkflow struct {
	progress  SessionProgress
	rules     rule.Repository
	knowledge vocab.Repository
	tests     TestFiles
}

// NewGuideWorkflow requires the session progress store, the ruleset whose
// Zones own the project's files, the recorded domain whose contexts hold
// code in some of them, and the convention that tells tests apart.
func NewGuideWorkflow(progress SessionProgress, rules rule.Repository, knowledge vocab.Repository, tests TestFiles) (GuideWorkflow, error) {
	if progress == nil || rules == nil || knowledge == nil || tests == nil {
		return GuideWorkflow{}, fmt.Errorf("guide workflow: missing session progress, ruleset, domain recording or test convention")
	}
	return GuideWorkflow{progress: progress, rules: rules, knowledge: knowledge, tests: tests}, nil
}

// Execute advises on the activities ArcLint recorded since the session's
// last update, then on the activities the host observed, and keeps the
// session's Progress so the same reminder is not given twice. Executing
// with no observed activities starts or resumes the session.
func (uc GuideWorkflow) Execute(session string, observed []agent.Activity) ([]agent.Guidance, error) {
	zones := uc.zones()
	model := uc.model(zones)
	var given []agent.Guidance
	err := uc.progress.Update(session, func(progress agent.Progress, recorded []agent.Activity) agent.Progress {
		given = nil
		for _, activity := range owned(append(slices.Clone(recorded), observed...), zones, model, uc.tests) {
			var advice []agent.Guidance
			progress, advice = agent.WorkflowGuide{}.Advise(progress, activity)
			given = append(given, advice...)
		}
		return progress
	})
	if err != nil {
		return nil, fmt.Errorf("guide workflow: %w", err)
	}
	return given, nil
}

// zones returns a function that reads the declared Zones on first use. When
// the ruleset cannot be read, no Zone is known to own a file, so no context
// is advised until it can.
func (uc GuideWorkflow) zones() func() []rule.Zone {
	var declared []rule.Zone
	read := false
	return func() []rule.Zone {
		if !read {
			read = true
			if configured, err := uc.rules.ConfiguredRules(); err == nil {
				declared = configured.Zones
			}
		}
		return declared
	}
}

// modelCode names the Zones that hold a bounded context's code; the
// model's code may lie anywhere when anywhere is true.
type modelCode struct {
	zones    map[string]bool
	anywhere bool
}

// model returns a function that reads, on first use, where the recorded
// model's code lies. When the domain recording cannot be read, records no
// context yet, or records a context no Zone is named for, the model's code
// may lie anywhere, as the check reads it.
func (uc GuideWorkflow) model(zones func() []rule.Zone) func() modelCode {
	var code modelCode
	read := false
	return func() modelCode {
		if read {
			return code
		}
		read = true
		code.zones = map[string]bool{}
		language, _, err := uc.knowledge.RecordedLanguage()
		if err != nil || len(language.Contexts) == 0 {
			code.anywhere = true
			return code
		}
		var names []rule.ZoneName
		for _, zone := range zones() {
			names = append(names, zone.Name())
		}
		for _, context := range language.Contexts {
			holding := conformance.ZonesNamedFor(context.Name, names)
			code.anywhere = code.anywhere || len(holding) == 0
			for _, zone := range holding {
				code.zones[string(zone)] = true
			}
		}
		return code
	}
}

// bearsDomain reports whether a changed file owned by zones bears domain
// meaning: it is not a test and lies in the model's code.
func bearsDomain(path string, zones []string, model modelCode, tests TestFiles) bool {
	if tests.Test(path) {
		return false
	}
	return model.anywhere || slices.ContainsFunc(zones, func(zone string) bool { return model.zones[zone] })
}

// owned gives each activity the Zones it concerns: context shows the Zones
// that own the paths it named besides the Zones it named, and a change is
// split into one Activity for each file, carrying the Zones that own it
// and whether it bears domain meaning.
func owned(activities []agent.Activity, zones func() []rule.Zone, model func() modelCode, tests TestFiles) []agent.Activity {
	var result []agent.Activity
	for _, activity := range activities {
		switch activity.Kind {
		case agent.ContextObtained:
			shown := slices.Clone(activity.Zones)
			for _, path := range activity.Paths {
				shown = append(shown, owners(zones(), path)...)
			}
			slices.Sort(shown)
			result = append(result, agent.Activity{Kind: activity.Kind, Paths: activity.Paths, Zones: slices.Compact(shown)})
		case agent.FilesChanged:
			for _, path := range activity.Paths {
				owning := owners(zones(), path)
				result = append(result, agent.Activity{Kind: agent.FilesChanged, Paths: []string{path}, Zones: owning, BearsDomain: bearsDomain(path, owning, model(), tests)})
			}
		default:
			result = append(result, activity)
		}
	}
	return result
}

// owners names the Zones that own path, as context matches them.
func owners(zones []rule.Zone, path string) []string {
	var names []string
	for _, zone := range zones {
		if zone.Contains(path) {
			names = append(names, string(zone.Name()))
		}
	}
	return names
}
