package rule

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ErrProjectNameless is returned when a Project would have no name: a
// rename to nothing, or a default taken from a directory with no name
// of its own, such as the root of a file system.
var ErrProjectNameless = errors.New("project has no name")

// Project is the repository arclint governs: the directory holding the
// ruleset, and the name its domain file records, or that directory's
// name when it records none. Renaming a Project changes its name
// alone; its files stay where they are.
type Project struct {
	directory string
	name      string
}

// NewProject builds the Project of the directory holding the ruleset,
// named by name, or by the directory when name is empty.
func NewProject(directory, name string) (Project, error) {
	if directory == "" {
		return Project{}, errors.New("project: the directory holding the ruleset is empty")
	}
	directory = filepath.Clean(directory)
	name = strings.TrimSpace(name)
	if name == "" {
		name = filepath.Base(directory)
		if name == "." || name == ".." || strings.ContainsRune(name, filepath.Separator) {
			return Project{}, fmt.Errorf("%w: directory %q has no name of its own", ErrProjectNameless, directory)
		}
	}
	return Project{directory: directory, name: name}, nil
}

// Renamed returns the Project under another name, in the same
// directory. A rename to nothing is refused: a Project always has a
// name.
func (p Project) Renamed(name string) (Project, error) {
	if strings.TrimSpace(name) == "" {
		return Project{}, fmt.Errorf("%w: a rename needs a name", ErrProjectNameless)
	}
	return NewProject(p.directory, name)
}

// Name is the Project's name.
func (p Project) Name() string { return p.name }
