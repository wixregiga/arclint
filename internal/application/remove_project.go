package application

import (
	"fmt"
	"path/filepath"
	"slices"
)

// RemoveProject takes a project off the list, and deletes its files from
// disk only when asked to and confirmed.
type RemoveProject struct {
	directories ProjectDirectories
	registry    ProjectRegistry
}

// NewRemoveProject requires the port that deletes a project's directory
// and the registry that forgets it.
func NewRemoveProject(directories ProjectDirectories, registry ProjectRegistry) (RemoveProject, error) {
	if directories == nil || registry == nil {
		return RemoveProject{}, fmt.Errorf("remove project: missing project directories or registry")
	}
	return RemoveProject{directories: directories, registry: registry}, nil
}

// RemoveProjectRequest names the project by its directory. DeleteFiles
// asks to delete the directory from disk too; Confirmation must then
// spell the directory's name.
type RemoveProjectRequest struct {
	Directory    string
	DeleteFiles  bool
	Confirmation string
}

// Execute forgets a remembered project. With DeleteFiles it first
// deletes the project's directory, which must be the project's own,
// the one holding its ruleset, and is kept on the list when it cannot
// be deleted.
func (uc RemoveProject) Execute(req RemoveProjectRequest) error {
	remembered, err := uc.registry.Directories()
	if err != nil {
		return fmt.Errorf("remove project: read the remembered projects: %w", err)
	}
	if !slices.Contains(remembered, req.Directory) {
		return fmt.Errorf("remove project: %s is not on the list", req.Directory)
	}
	if req.DeleteFiles {
		if req.Confirmation != filepath.Base(req.Directory) {
			return fmt.Errorf("remove project: type %q to confirm deleting %s", filepath.Base(req.Directory), req.Directory)
		}
		located, err := uc.directories.Locate(req.Directory)
		if err != nil {
			return fmt.Errorf("remove project %s: %w", req.Directory, err)
		}
		if located != req.Directory {
			return fmt.Errorf("remove project: %s is not a project's directory; its project is %s", req.Directory, located)
		}
		if err := uc.directories.Delete(req.Directory); err != nil {
			return fmt.Errorf("remove project %s: %w", req.Directory, err)
		}
	}
	if err := uc.registry.Forget(req.Directory); err != nil {
		return fmt.Errorf("remove project %s: forget it: %w", req.Directory, err)
	}
	return nil
}
