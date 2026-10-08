package application

import (
	"fmt"
	"slices"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/workflow"
)

// SessionProgress keeps each session's workflow Progress between the
// activities an Agent Host reports.
type SessionProgress interface {
	// Update replaces the session's Progress with change's result. change
	// also receives the activities ArcLint's own commands recorded since
	// the session last read them, in the order they ran. Updates of one
	// session apply one at a time.
	Update(session string, change func(progress workflow.Progress, recorded []workflow.Activity) workflow.Progress) error
}

// GuideWorkflow advises a session on what ran in it.
type GuideWorkflow struct {
	progress SessionProgress
	rules    rule.Repository
}

// NewGuideWorkflow requires the session progress store and the ruleset
// whose Zones own the project's files.
func NewGuideWorkflow(progress SessionProgress, rules rule.Repository) (GuideWorkflow, error) {
	if progress == nil || rules == nil {
		return GuideWorkflow{}, fmt.Errorf("guide workflow: missing session progress or ruleset")
	}
	return GuideWorkflow{progress: progress, rules: rules}, nil
}

// Execute advises on the activities ArcLint recorded since the session's
// last update, then on the activities the host observed, and keeps the
// session's Progress so the same reminder is not given twice. Executing
// with no observed activities starts or resumes the session.
func (uc GuideWorkflow) Execute(session string, observed []workflow.Activity) ([]workflow.Guidance, error) {
	zones := uc.zones()
	var given []workflow.Guidance
	err := uc.progress.Update(session, func(progress workflow.Progress, recorded []workflow.Activity) workflow.Progress {
		given = nil
		for _, activity := range owned(append(slices.Clone(recorded), observed...), zones) {
			var advice []workflow.Guidance
			progress, advice = workflow.Guide{}.Advise(progress, activity)
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

// owned gives each activity the Zones it concerns: context shows the Zones
// that own the paths it named besides the Zones it named, and a change is
// split into one Activity for each file, carrying the Zones that own it.
func owned(activities []workflow.Activity, zones func() []rule.Zone) []workflow.Activity {
	var result []workflow.Activity
	for _, activity := range activities {
		switch activity.Kind {
		case workflow.ContextObtained:
			shown := slices.Clone(activity.Zones)
			for _, path := range activity.Paths {
				shown = append(shown, owners(zones(), path)...)
			}
			slices.Sort(shown)
			result = append(result, workflow.Activity{Kind: activity.Kind, Paths: activity.Paths, Zones: slices.Compact(shown)})
		case workflow.FilesChanged:
			for _, path := range activity.Paths {
				result = append(result, workflow.Activity{Kind: workflow.FilesChanged, Paths: []string{path}, Zones: owners(zones(), path)})
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
