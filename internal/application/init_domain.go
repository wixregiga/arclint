package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// InitDomainResult reports whether initialization created the project's
// recorded Ubiquitous Language, and the project it records.
type InitDomainResult struct {
	Source  string
	Project string
	Created bool
}

// InitDomain initializes the project's recorded Ubiquitous Language
// without replacing an existing model.
type InitDomain struct {
	knowledge vocab.Repository
	directory string
}

// NewInitDomain requires the domain-model repository it initializes and
// the directory holding the ruleset, whose Project a fresh model is
// filed under.
func NewInitDomain(knowledge vocab.Repository, directory string) (InitDomain, error) {
	if knowledge == nil {
		return InitDomain{}, fmt.Errorf("init domain: missing knowledge repository")
	}
	if directory == "" {
		return InitDomain{}, fmt.Errorf("init domain: missing repository directory")
	}
	return InitDomain{knowledge: knowledge, directory: directory}, nil
}

// Execute records an empty Ubiquitous Language for the project when
// none is present, named project, or for its directory when project is
// empty. Repeated execution leaves the existing model unchanged and
// reports the project it records.
func (uc InitDomain) Execute(project string) (InitDomainResult, error) {
	lang, found, err := recordedLanguage(uc.knowledge, uc.directory, project)
	if err != nil {
		return InitDomainResult{}, err
	}
	result := InitDomainResult{Source: uc.knowledge.Recording().Path(), Project: lang.Project}
	if found {
		return result, nil
	}
	if err := uc.knowledge.Record(lang); err != nil {
		return InitDomainResult{}, fmt.Errorf("save domain model: %w", err)
	}
	result.Created = true
	return result, nil
}

// recordedLanguage returns the project's recorded Ubiquitous Language
// and whether its file exists. Without one it is a fresh language for
// the Project of directory, named project or for the directory, still
// unrecorded; the Project is consulted only then, so a repository whose
// file names it never depends on its directory's name.
func recordedLanguage(knowledge vocab.Repository, directory, project string) (vocab.UbiquitousLanguage, bool, error) {
	lang, found, err := knowledge.RecordedLanguage()
	if err != nil {
		return vocab.UbiquitousLanguage{}, false, fmt.Errorf("load domain model: %w", err)
	}
	if found {
		return lang, true, nil
	}
	p, err := rule.NewProject(directory, project)
	if err != nil {
		return vocab.UbiquitousLanguage{}, false, fmt.Errorf("%w: %w", ErrDomainUsage, err)
	}
	fresh, err := vocab.NewUbiquitousLanguage(p.Name(), "", nil, nil)
	if err != nil {
		return vocab.UbiquitousLanguage{}, false, fmt.Errorf("new domain model: %w", err)
	}
	fresh.Recording = lang.Recording
	return fresh, false, nil
}
