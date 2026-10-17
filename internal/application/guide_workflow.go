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

// model returns a function that reads, on first use, where the recorded
// model's code lies. When the domain recording cannot be read, records no
// context yet, or records a context no Zone is named for, the model's code
// may lie anywhere, as the check reads it.
func (uc GuideWorkflow) model(zones func() []rule.Zone) func() agent.ModelCode {
	var code agent.ModelCode
	read := false
	return func() agent.ModelCode {
		if read {
			return code
		}
		read = true
		language, _, err := uc.knowledge.RecordedLanguage()
		if err != nil || len(language.Contexts) == 0 {
			code.Anywhere = true
			return code
		}
		var names []rule.ZoneName
		for _, zone := range zones() {
			names = append(names, zone.Name())
		}
		for _, context := range language.Contexts {
			holding := conformance.ZonesNamedFor(context.Name, names)
			code.Anywhere = code.Anywhere || len(holding) == 0
			for _, zone := range holding {
				code.Zones = append(code.Zones, string(zone))
			}
		}
		return code
	}
}

// owned resolves each activity through the agent context's constructors:
// context with the Zones that own the paths it named, and a change split
// into one Activity for each file, with the Zones that own it, whether it
// is a test, and where the model's code lies.
func owned(activities []agent.Activity, zones func() []rule.Zone, model func() agent.ModelCode, tests TestFiles) []agent.Activity {
	var result []agent.Activity
	for _, activity := range activities {
		switch activity.Kind {
		case agent.ContextObtained:
			result = append(result, agent.ObtainedContext(activity.Paths, activity.Zones, func(path string) []string { return owners(zones(), path) }))
		case agent.FilesChanged:
			for _, path := range activity.Paths {
				result = append(result, agent.ChangedFile(path, owners(zones(), path), tests.Test(path), model()))
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
