package workflow

import "slices"

// Guide is the WorkflowGuide domain service. It compares an Activity with
// a session's Progress and returns the Guidance the Activity calls for. A
// session receives each reminder once, so repeated actions do not repeat it.
type Guide struct{}

// Advise returns the Progress that includes activity and the Guidance the
// activity calls for, in workflow order.
func (guide Guide) Advise(progress Progress, activity Activity) (Progress, []Guidance) {
	var given []Guidance
	if guidance, due := guide.AssertContextBeforeChange(progress, activity); due {
		given = append(given, guidance)
	}
	if guidance, due := guide.AssertDomainBeforeChange(progress, activity); due {
		given = append(given, guidance)
	}
	if guidance, due := guide.AssertCheckBeforeFinish(progress, activity); due {
		given = append(given, guidance)
	}
	return progress.record(activity, given), given
}

// AssertContextBeforeChange checks that changed files follow context that
// showed every Zone owning them. It names each such path once in a session.
func (Guide) AssertContextBeforeChange(progress Progress, activity Activity) (Guidance, bool) {
	if activity.Kind != FilesChanged || progress.shows(activity.Zones) {
		return Guidance{}, false
	}
	var uncovered []string
	for _, name := range activity.Paths {
		if !slices.Contains(progress.ContextAdvised, name) {
			uncovered = append(uncovered, name)
		}
	}
	if len(uncovered) == 0 {
		return Guidance{}, false
	}
	return Guidance{Step: ContextStep, Paths: union(nil, uncovered...)}, true
}

// AssertDomainBeforeChange checks that changed files follow a change of
// the domain recording. It reminds once in a session.
func (Guide) AssertDomainBeforeChange(progress Progress, activity Activity) (Guidance, bool) {
	if activity.Kind != FilesChanged || len(activity.Paths) == 0 || progress.DomainRecorded || progress.DomainAdvised {
		return Guidance{}, false
	}
	return Guidance{Step: DomainStep, Paths: union(nil, activity.Paths...)}, true
}

// AssertCheckBeforeFinish checks that a finished turn leaves no change
// unverified. It names the unverified files again only when one of them
// has not been named since the last check.
func (Guide) AssertCheckBeforeFinish(progress Progress, activity Activity) (Guidance, bool) {
	if activity.Kind != TurnFinished {
		return Guidance{}, false
	}
	for _, name := range progress.Unverified {
		if !slices.Contains(progress.CheckAdvised, name) {
			return Guidance{Step: CheckStep, Paths: slices.Clone(progress.Unverified)}, true
		}
	}
	return Guidance{}, false
}
