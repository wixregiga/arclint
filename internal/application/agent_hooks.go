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
}

// HookFileStatus is one host configuration file and what is wrong
// with the workflow hooks it lists.
type HookFileStatus struct {
	Host string
	// Scope is "user" for a file the host reads in every project and
	// "project" for one it reads in this project only.
	Scope string
	// Windows marks the Windows profile's file, which the Windows host apps
	// read when they open a project inside the WSL distribution.
	Windows   bool
	Path      string
	Installed bool
	Problems  []string
}

// HooksInstallation names the configuration files an install wrote and
// the files it removed the workflow hooks from.
type HooksInstallation struct {
	Written []string
	Removed []string
}
