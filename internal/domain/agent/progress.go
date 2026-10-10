package agent

import "slices"

// Progress is what one session has done in the workflow and the Guidance
// it has already received. The zero Progress is a session that has done
// nothing yet.
type Progress struct {
	// Zones holds the Zones context has shown.
	Zones []string
	// AllZones is true once context showed every Zone.
	AllZones bool
	// DomainRecorded is true once the domain recording changed in the session.
	DomainRecorded bool
	// Unverified holds the files changed since the last check.
	Unverified []string
	// ContextAdvised holds the paths context Guidance has already named.
	ContextAdvised []string
	// DomainAdvised is true once the domain reminder was given.
	DomainAdvised bool
	// CheckAdvised holds the files check Guidance named since the last check.
	CheckAdvised []string
}

// shows reports whether context in the session has shown every one of
// zones. A file that no Zone owns needs no context.
func (progress Progress) shows(zones []string) bool {
	if progress.AllZones {
		return true
	}
	for _, zone := range zones {
		if !slices.Contains(progress.Zones, zone) {
			return false
		}
	}
	return true
}

// record returns the Progress after activity and the Guidance given for it.
func (progress Progress) record(activity Activity, given []Guidance) Progress {
	next := Progress{
		Zones:          slices.Clone(progress.Zones),
		AllZones:       progress.AllZones,
		DomainRecorded: progress.DomainRecorded,
		Unverified:     slices.Clone(progress.Unverified),
		ContextAdvised: slices.Clone(progress.ContextAdvised),
		DomainAdvised:  progress.DomainAdvised,
		CheckAdvised:   slices.Clone(progress.CheckAdvised),
	}
	switch activity.Kind {
	case ContextObtained:
		next.AllZones = next.AllZones || activity.showsEveryZone()
		next.Zones = union(next.Zones, activity.Zones...)
	case DomainChanged:
		next.DomainRecorded = true
		next.Unverified = union(next.Unverified, activity.Paths...)
	case FilesChanged:
		next.Unverified = union(next.Unverified, activity.Paths...)
	case CheckRan:
		next.Unverified = nil
		next.CheckAdvised = nil
	case TurnFinished:
		// Finishing a turn changes nothing the session has done.
	}
	for _, guidance := range given {
		switch guidance.Step {
		case ContextStep:
			next.ContextAdvised = union(next.ContextAdvised, guidance.Paths...)
		case DomainStep:
			next.DomainAdvised = true
		case CheckStep:
			next.CheckAdvised = union(next.CheckAdvised, guidance.Paths...)
		}
	}
	return next
}

// union returns the sorted, duplicate-free paths of base and added.
func union(base []string, added ...string) []string {
	merged := append(slices.Clone(base), added...)
	slices.Sort(merged)
	return slices.Compact(merged)
}
