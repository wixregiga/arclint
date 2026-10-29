package application

import "github.com/wixregiga/arclint/internal/domain/vocab"

// ProjectDirectories makes, finds and deletes the directories projects
// live in, and opens the files a project holds.
type ProjectDirectories interface {
	// Make creates dir as an empty git repository and says whether it
	// made the directory rather than finding it empty. It refuses a
	// relative path, a parent that does not exist, and a directory that
	// already holds anything, so it never writes into existing work.
	Make(dir string) (made bool, err error)
	// Discard takes back a project that could not be started: it empties
	// dir, which Make found or left empty, and removes dir when Make made
	// it.
	Discard(dir string, made bool) error
	// Ruleset is the scaffold dir's rules.arclint.yaml is written through.
	Ruleset(dir string) (RulesetScaffold, error)
	// DomainFile is the repository of the domain file the ruleset in dir
	// names, or the default one when it names none.
	DomainFile(dir string) (vocab.Repository, error)
	// Locate is the project an absolute path lies in: the directory that
	// holds the rules.arclint.yaml found upward from path.
	Locate(path string) (string, error)
	// Delete removes the project at dir from disk, its git repository
	// included. It refuses a directory that does not hold a ruleset.
	Delete(dir string) error
}

// ProjectRegistry remembers the projects the page has served, created
// or opened, by directory, the most recently used first.
type ProjectRegistry interface {
	Directories() ([]string, error)
	// Remember puts dir first, once.
	Remember(dir string) error
	// Forget removes dir; a directory not remembered is no error.
	Forget(dir string) error
}

// ProjectSummary is one project as the list shows it: its directory and
// the name and description its domain file records, or why the project
// cannot be read.
type ProjectSummary struct {
	Directory   string
	Name        string
	Description string
	Problem     string
}

// summarizeProject reads the project at dir for the list; a project that
// cannot be read is still listed, with the reason.
func summarizeProject(directories ProjectDirectories, dir string) ProjectSummary {
	summary := ProjectSummary{Directory: dir}
	knowledge, err := directories.DomainFile(dir)
	if err != nil {
		summary.Problem = err.Error()
		return summary
	}
	lang, found, err := knowledge.RecordedLanguage()
	switch {
	case err != nil:
		summary.Problem = err.Error()
	case !found:
		summary.Problem = knowledge.Recording().Path() + " records no language yet"
	default:
		summary.Name, summary.Description = lang.Project, lang.Description
	}
	return summary
}
