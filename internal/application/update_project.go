package application

import "fmt"

// UpdateProject renames a project and replaces its description, both
// recorded in its domain file.
type UpdateProject struct {
	directories ProjectDirectories
}

// NewUpdateProject requires the port that opens a project's domain file.
func NewUpdateProject(directories ProjectDirectories) (UpdateProject, error) {
	if directories == nil {
		return UpdateProject{}, fmt.Errorf("update project: missing project directories")
	}
	return UpdateProject{directories: directories}, nil
}

// UpdateProjectRequest names the project by its directory and carries
// its name and description as they should be recorded; an empty
// description clears it.
type UpdateProjectRequest struct {
	Directory   string
	Name        string
	Description string
}

// Execute records the name and description in one write. The language
// refuses an empty name and a description over its WordLimit, and then
// the domain file stays as it was.
func (uc UpdateProject) Execute(req UpdateProjectRequest) (ProjectSummary, error) {
	knowledge, err := uc.directories.DomainFile(req.Directory)
	if err != nil {
		return ProjectSummary{}, fmt.Errorf("update project %s: %w", req.Directory, err)
	}
	lang, found, err := knowledge.RecordedLanguage()
	if err != nil {
		return ProjectSummary{}, fmt.Errorf("update project %s: %w", req.Directory, err)
	}
	if !found {
		return ProjectSummary{}, fmt.Errorf("update project %s: %s records no language yet", req.Directory, knowledge.Recording().Path())
	}
	renamed, err := lang.Rename(req.Name)
	if err != nil {
		return ProjectSummary{}, fmt.Errorf("update project %s: %w", req.Directory, err)
	}
	next, err := renamed.Describe(req.Description)
	if err != nil {
		return ProjectSummary{}, fmt.Errorf("update project %s: %w", req.Directory, err)
	}
	if next.Project != lang.Project || next.Description != lang.Description {
		if err := knowledge.Record(next); err != nil {
			return ProjectSummary{}, fmt.Errorf("update project %s: save domain model: %w", req.Directory, err)
		}
	}
	return ProjectSummary{Directory: req.Directory, Name: next.Project, Description: next.Description}, nil
}
