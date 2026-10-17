package application

import "time"

// HooksStatus describes the workflow hooks written to each Agent
// Host's configuration files and the last hook event that reached the
// project. A configuration file does not establish that a host trusted or
// ran the hooks; an event establishes that a host ran them here.
type HooksStatus struct {
	Project string
	// Binary is the arclint the hooks run when installed from here.
	Binary string
	Files  []HookFileStatus
	// LastEvent is when a hook event last reached the project, zero when
	// none has.
	LastEvent time.Time
	// Silence says why no hook event has reached the project, empty once
	// one has.
	Silence []string
}

// HookInstallation says how many of the workflow hooks a configuration
// file lists. A host runs each hook the file lists, so a partial
// installation still runs the hooks it lists.
type HookInstallation string

const (
	// HooksComplete is a file that lists a workflow hook for every event.
	HooksComplete HookInstallation = "complete"
	// HooksPartial is a file that lists the workflow hooks of some events
	// and lacks the others.
	HooksPartial HookInstallation = "partial"
	// HooksAbsent is a file that lists no workflow hook, or does not exist.
	HooksAbsent HookInstallation = "absent"
	// HooksUnknown is a file that could not be read or located, so what it
	// lists is unknown.
	HooksUnknown HookInstallation = "unknown"
)

// HookFileStatus is one host configuration file and what is wrong
// with the workflow hooks it lists.
type HookFileStatus struct {
	Host string
	// Scope is "user" for a file the host reads in every project and
	// "project" for one it reads in this project only.
	Scope string
	// Windows marks the Windows profile's file, which the Windows host apps
	// read when they open a project inside the WSL distribution.
	Windows      bool
	Path         string
	Installation HookInstallation
	// Listed names the events the file lists a workflow hook for, and
	// Missing the events it lists none for, both in the order install
	// writes them. Both are empty when the installation is unknown.
	Listed   []string
	Missing  []string
	Problems []string
}

// HooksInstallation names the configuration files an install wrote and
// the files it removed the workflow hooks from.
type HooksInstallation struct {
	Written []string
	Removed []string
}
