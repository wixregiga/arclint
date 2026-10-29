package application

import (
	"errors"
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// DefineProjectRequest names what to record about the project: its
// name, its description, or both. An omitted one stays as recorded; a
// description may be cleared, a name may not.
type DefineProjectRequest struct {
	SetName        bool
	Name           string
	SetDescription bool
	Description    string
}

// ExecuteProject records the project's name and description at the top
// of the domain file. A rename goes through the Project, so a project is
// never left without a name; a description over its WordLimit is
// refused as the recorded language refuses it, never as a usage error.
// Without a domain file the language is created under the name given,
// or the directory's. The model is saved only when something changed,
// so a refusal writes nothing.
func (uc DefineDomainDefinition) ExecuteProject(req DefineProjectRequest) (DomainDefineResult, error) {
	if !req.SetName && !req.SetDescription {
		return DomainDefineResult{}, fmt.Errorf("%w: project: pass a name, --description, or both", ErrDomainUsage)
	}
	name := ""
	if req.SetName {
		name = req.Name
	}
	lang, found, err := recordedLanguage(uc.knowledge, uc.directory, name)
	if err != nil {
		return DomainDefineResult{}, err
	}
	recorded := vocab.UbiquitousLanguage{}
	if found {
		recorded = lang
	}
	out := DomainDefineResult{Target: domainProject, Outcome: vocab.OutcomeUnchanged, Values: map[string]string{}}
	next := lang
	if req.SetName {
		next, err = renamed(uc.directory, next, req.Name)
		if err != nil {
			return DomainDefineResult{}, err
		}
		if next.Project != recorded.Project {
			out.Changed = append(out.Changed, domainName)
			out.Values[domainName] = next.Project
		}
	}
	if req.SetDescription {
		// Describe judges the description before an unchanged one
		// returns, so a description already over its limit is refused
		// when written again, as an entry's definition is.
		next, err = next.Describe(req.Description)
		if err != nil {
			return DomainDefineResult{}, domainDescriptionError(err)
		}
		if next.Description != recorded.Description {
			out.Changed = append(out.Changed, domainDescription)
			out.Values[domainDescription] = next.Description
		}
	}
	out.Name = next.Project
	if len(out.Changed) == 0 {
		return out, nil
	}
	out.Outcome = vocab.OutcomeUpdated
	if !found {
		out.Outcome = vocab.OutcomeCreated
	}
	if err := uc.knowledge.Record(next); err != nil {
		return DomainDefineResult{}, fmt.Errorf("save domain model: %w", err)
	}
	return out, nil
}

// renamed files the language under the name the Project takes when it
// is renamed.
func renamed(directory string, lang vocab.UbiquitousLanguage, name string) (vocab.UbiquitousLanguage, error) {
	project, err := rule.NewProject(directory, lang.Project)
	if err == nil {
		project, err = project.Renamed(name)
	}
	if err == nil {
		lang, err = lang.Rename(project.Name())
	}
	if err != nil {
		return vocab.UbiquitousLanguage{}, fmt.Errorf("%w: %w", ErrDomainUsage, err)
	}
	return lang, nil
}

// domainDescriptionError classifies a refused description: over its
// WordLimit, the recorded language refuses it; otherwise the request
// is unusable as spelled.
func domainDescriptionError(err error) error {
	var over vocab.OverlongText
	if errors.As(err, &over) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrDomainUsage, err)
}
