package agent

// Step names the workflow step a Guidance concerns.
type Step string

const (
	// ContextStep obtains context for the paths about to change.
	ContextStep Step = "context"
	// DomainStep records new or changed meaning before implementing it.
	DomainStep Step = "domain"
	// CheckStep runs the project's check before finishing.
	CheckStep Step = "check"
)

// Guidance names the workflow step an Activity skipped and the paths it
// concerns. It advises; it carries no approval and cannot block work.
type Guidance struct {
	Step  Step
	Paths []string
}
