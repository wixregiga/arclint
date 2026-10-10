// Package agent holds the order a coding agent follows in a project:
// obtain context for the paths it will change, record new or changed
// meaning in the domain recording, implement, and run the project's
// check before finishing. Guidance names a skipped step; it never
// blocks work.
package agent

// Kind names what an Activity did.
type Kind string

const (
	// ContextObtained obtained ArcLint context. It showed the Activity's
	// Zones; context that named neither paths nor Zones showed every Zone.
	ContextObtained Kind = "context"
	// DomainChanged changed the domain recording at the Activity's paths.
	DomainChanged Kind = "domain"
	// FilesChanged changed the Activity's paths, none of them the domain
	// recording; its Zones are the Zones that own them.
	FilesChanged Kind = "change"
	// CheckRan ran the project's check.
	CheckRan Kind = "check"
	// TurnFinished ended the agent's turn.
	TurnFinished Kind = "finish"
)

// Activity is one observed action of a coding agent. Paths are
// slash-separated and relative to the project root; Zones are named as the
// ruleset declares them.
type Activity struct {
	Kind  Kind
	Paths []string
	Zones []string
}

// showsEveryZone reports whether context showed every Zone because it
// named neither paths nor Zones.
func (activity Activity) showsEveryZone() bool {
	return activity.Kind == ContextObtained && len(activity.Paths) == 0 && len(activity.Zones) == 0
}
