package workflow

// Evidence names the supplied work and the limits of what can be assessed.
// Passages contain the actual text; a path alone does not supply its contents.
type Evidence struct {
	Task     string
	Passages map[string]string
	Limits   []string
}
