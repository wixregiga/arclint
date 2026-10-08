package application

// WorkflowHookStatus describes the workflow hooks written to each Agent
// Host's configuration. It does not establish that a host trusted or ran
// them.
type WorkflowHookStatus struct {
	Project string
	Command string
	Hosts   []WorkflowHostStatus
}

// WorkflowHostStatus is one host's hook configuration and what is missing
// from it.
type WorkflowHostStatus struct {
	Host      string
	Path      string
	Installed bool
	Problems  []string
}
