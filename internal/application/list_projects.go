package application

import "fmt"

// ListProjects lists the projects the page can switch between: the one
// it serves, then every remembered project, each read from its domain
// file.
type ListProjects struct {
	directories ProjectDirectories
	registry    ProjectRegistry
}

// NewListProjects requires the port that opens a project's files and
// the registry of remembered projects.
func NewListProjects(directories ProjectDirectories, registry ProjectRegistry) (ListProjects, error) {
	if directories == nil || registry == nil {
		return ListProjects{}, fmt.Errorf("list projects: missing project directories or registry")
	}
	return ListProjects{directories: directories, registry: registry}, nil
}

// Execute lists served first, whether or not it is remembered, then the
// remembered projects in the registry's order. A registry that cannot
// be read still lists served, with the error.
func (uc ListProjects) Execute(served string) ([]ProjectSummary, error) {
	summaries := []ProjectSummary{summarizeProject(uc.directories, served)}
	remembered, err := uc.registry.Directories()
	if err != nil {
		return summaries, fmt.Errorf("read the remembered projects: %w", err)
	}
	for _, dir := range remembered {
		if dir != served {
			summaries = append(summaries, summarizeProject(uc.directories, dir))
		}
	}
	return summaries, nil
}
