package application

import (
	"fmt"
	"strings"

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
	project   string
}

// NewInitDomain requires the domain-model repository it initializes and
// the project name recorded when the caller names none: the name of the
// software whose domain the file records, the repository directory's
// by default.
func NewInitDomain(knowledge vocab.Repository, project string) (InitDomain, error) {
	if knowledge == nil {
		return InitDomain{}, fmt.Errorf("init domain: missing knowledge repository")
	}
	project = strings.TrimSpace(project)
	if project == "" {
		return InitDomain{}, fmt.Errorf("init domain: missing project name")
	}
	return InitDomain{knowledge: knowledge, project: project}, nil
}

// Execute records an empty Ubiquitous Language for the project when
// none is present; an empty project name takes the default. Repeated
// execution leaves the existing model unchanged and reports the
// project it records.
func (uc InitDomain) Execute(project string) (InitDomainResult, error) {
	result := InitDomainResult{Source: vocab.UbiquitousLanguageFileName}
	lang, found, err := uc.knowledge.RecordedLanguage()
	if err != nil {
		return InitDomainResult{}, fmt.Errorf("load domain model: %w", err)
	}
	if found {
		result.Project = lang.Project
		return result, nil
	}
	project = strings.TrimSpace(project)
	if project == "" {
		project = uc.project
	}
	fresh, err := vocab.NewUbiquitousLanguage(project, "", nil, nil)
	if err != nil {
		return InitDomainResult{}, fmt.Errorf("new domain model: %w", err)
	}
	if err := uc.knowledge.Record(fresh); err != nil {
		return InitDomainResult{}, fmt.Errorf("save domain model: %w", err)
	}
	result.Project = fresh.Project
	result.Created = true
	return result, nil
}
