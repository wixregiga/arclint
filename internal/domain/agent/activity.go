// Package agent holds the order a coding agent follows in a project:
// obtain context for the paths it will change, record new or changed
// meaning in the domain recording, implement, and run the project's
// check before finishing. Guidance names a skipped step; it never
// blocks work.
package agent

import "slices"

// Kind names what an Activity did.
type Kind string

const (
	// ContextObtained obtained ArcLint context. It showed the Activity's
	// Zones; context that named neither paths nor Zones showed every Zone.
	ContextObtained Kind = "context"
	// DomainChanged changed the domain recording at the Activity's paths.
	DomainChanged Kind = "domain"
	// FilesChanged changed the Activity's paths, none of them the domain
	// recording, with an edit tool or through a shell command; its Zones
	// are the Zones that own them.
	FilesChanged Kind = "change"
	// CheckRan ran the project's check.
	CheckRan Kind = "check"
	// TurnFinished ended the agent's turn.
	TurnFinished Kind = "finish"
)

// Activity is one observed action of a coding agent. Paths are
// slash-separated and relative to the project root; Zones are named as the
// ruleset declares them. ObtainedContext and ChangedFile build the two
// kinds whose Zones and domain meaning the invariants decide.
type Activity struct {
	Kind  Kind
	Paths []string
	Zones []string
	// BearsDomain marks changed files that bear domain meaning: files
	// other than tests, in a Zone that holds a bounded context's code, or
	// anywhere while the model's code has no Zone of its own.
	BearsDomain bool
}

// showsEveryZone reports whether context showed every Zone because it
// named neither paths nor Zones.
func (activity Activity) showsEveryZone() bool {
	return activity.Kind == ContextObtained && len(activity.Paths) == 0 && len(activity.Zones) == 0
}

// ModelCode is where the recorded model's code lies, as the check reads
// it: the Zones named for a bounded context, or anywhere while no context
// is recorded or a recorded context has no Zone named for it.
type ModelCode struct {
	Zones    []string
	Anywhere bool
}

// holds reports whether a file owned by owners lies in the model's code.
func (code ModelCode) holds(owners []string) bool {
	return code.Anywhere || slices.ContainsFunc(owners, func(zone string) bool { return slices.Contains(code.Zones, zone) })
}

// ObtainedContext is the Activity of obtaining context for paths and for
// the Zones named. It applies context-shows-owning-and-named-zones: it
// shows the Zones named together with the Zones owners gives for each
// path, each once and sorted, and context naming neither shows every Zone.
func ObtainedContext(paths, named []string, owners func(path string) []string) Activity {
	shown := slices.Clone(named)
	for _, path := range paths {
		shown = append(shown, owners(path)...)
	}
	slices.Sort(shown)
	return Activity{Kind: ContextObtained, Paths: paths, Zones: slices.Compact(shown)}
}

// ChangedFile is the Activity of changing one file, owned by owners. It
// applies domain-meaning-in-the-models-code: the change bears domain
// meaning exactly when the file is not a test and lies in the model's
// code.
func ChangedFile(path string, owners []string, test bool, model ModelCode) Activity {
	return Activity{Kind: FilesChanged, Paths: []string{path}, Zones: owners, BearsDomain: !test && model.holds(owners)}
}
