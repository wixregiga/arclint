package application

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// CreateProjectRequest carries the choices a new project starts from.
type CreateProjectRequest struct {
	// Directory is the absolute path the project is created at.
	Directory string
	// Name is the project the domain file records; empty names it for
	// the directory.
	Name string
	// Languages are the runtime targets: go, ts, py. Empty means go.
	Languages []string
}

// CreateProjectResult names the project created and the files it was
// started with.
type CreateProjectResult struct {
	Directory string
	Project   string
	Ruleset   string
	Source    string
}

// CreateProject starts a new project in its own git repository: the
// commented starter ruleset, and a domain file recording an empty
// Ubiquitous Language under the project's name, ready to be built. The
// project is remembered among the projects the page lists.
type CreateProject struct {
	directories ProjectDirectories
	registry    ProjectRegistry
}

// NewCreateProject requires the port that makes the project's directory
// and opens its files, and the registry that remembers it.
func NewCreateProject(directories ProjectDirectories, registry ProjectRegistry) (CreateProject, error) {
	if directories == nil {
		return CreateProject{}, fmt.Errorf("create project: missing project directories")
	}
	if registry == nil {
		return CreateProject{}, fmt.Errorf("create project: missing project registry")
	}
	return CreateProject{directories: directories, registry: registry}, nil
}

// Execute creates the project. Every choice is checked before the
// directory is made, so a refused request leaves nothing behind; a
// project that cannot be started or remembered once the directory is
// made is taken back, so the same request can be made again.
func (uc CreateProject) Execute(req CreateProjectRequest) (CreateProjectResult, error) {
	dir := strings.TrimSpace(req.Directory)
	if dir == "" {
		return CreateProjectResult{}, errors.New("create project: name the directory to create it in")
	}
	if !filepath.IsAbs(dir) {
		return CreateProjectResult{}, fmt.Errorf("create project: %q is not an absolute path", dir)
	}
	dir = filepath.Clean(dir)
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = filepath.Base(dir)
	}
	if _, err := vocab.NewUbiquitousLanguage(name, "", nil, nil); err != nil {
		return CreateProjectResult{}, fmt.Errorf("create project: %w", err)
	}
	for _, l := range req.Languages {
		if !supportsLanguage(l) {
			return CreateProjectResult{}, fmt.Errorf("create project: language %q is not one of %s", l, strings.Join(supportedLanguages, ", "))
		}
	}
	made, err := uc.directories.Make(dir)
	if err != nil {
		return CreateProjectResult{}, fmt.Errorf("create project: %w", err)
	}
	result, err := uc.start(dir, name, req.Languages)
	if err != nil {
		if discardErr := uc.directories.Discard(dir, made); discardErr != nil {
			return CreateProjectResult{}, fmt.Errorf("create project %s: %w; and it could not be taken back: %w", dir, err, discardErr)
		}
		return CreateProjectResult{}, fmt.Errorf("create project %s: %w", dir, err)
	}
	return result, nil
}

// start writes the starter ruleset into the made directory, records the
// empty language in the domain file it names, and remembers the project.
func (uc CreateProject) start(dir, name string, languages []string) (CreateProjectResult, error) {
	scaffold, err := uc.directories.Ruleset(dir)
	if err != nil {
		return CreateProjectResult{}, fmt.Errorf("%w", err)
	}
	initialize, err := NewInitializeRepository(scaffold)
	if err != nil {
		return CreateProjectResult{}, err
	}
	ruleset, err := initialize.Execute(InitializeRepositoryRequest{Languages: languages, Pattern: BarePattern})
	if err != nil {
		return CreateProjectResult{}, err
	}
	knowledge, err := uc.directories.DomainFile(dir)
	if err != nil {
		return CreateProjectResult{}, fmt.Errorf("%w", err)
	}
	initDomain, err := NewInitDomain(knowledge, name)
	if err != nil {
		return CreateProjectResult{}, err
	}
	recorded, err := initDomain.Execute(name)
	if err != nil {
		return CreateProjectResult{}, err
	}
	if err := uc.registry.Remember(dir); err != nil {
		return CreateProjectResult{}, fmt.Errorf("remember the project: %w", err)
	}
	return CreateProjectResult{Directory: dir, Project: recorded.Project, Ruleset: ruleset, Source: recorded.Source}, nil
}
