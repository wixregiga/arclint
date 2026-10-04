package application

// WorkflowHookStatus describes installed files, not native host activation.
type WorkflowHookStatus struct {
	Project   string
	HooksPath string
	Command   string
	Version   string
	Installed bool
	Intact    bool
	Problems  []string
}
