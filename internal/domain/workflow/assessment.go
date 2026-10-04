// Package workflow accepts grounded review feedback for a project's current work.
package workflow

// Assessment gives feedback and coverage limits without approval or blocking state.
type Assessment struct {
	Findings []Finding
	Limits   []string
}
