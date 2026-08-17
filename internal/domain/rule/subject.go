package rule

import "errors"

// SubjectKind distinguishes the three concrete things a Rule evaluates.
type SubjectKind string

// The three Subject kinds.
const (
	SubjectFile   SubjectKind = "file"
	SubjectFolder SubjectKind = "folder"
	SubjectModule SubjectKind = "module"
)

// Subject is one concrete File, Folder, or Module selected by Rule
// Applicability. A Programming Language filters Applicability but is
// never a Subject.
type Subject struct {
	kind     SubjectKind
	identity string
}

// FileSubject identifies one repo-relative file.
func FileSubject(path string) (Subject, error) {
	if path == "" {
		return Subject{}, errors.New("subject: empty file path")
	}
	return Subject{kind: SubjectFile, identity: path}, nil
}

// FolderSubject identifies one repo-relative folder.
func FolderSubject(path string) (Subject, error) {
	if path == "" {
		return Subject{}, errors.New("subject: empty folder path")
	}
	return Subject{kind: SubjectFolder, identity: path}, nil
}

// ModuleSubject identifies one declared Module.
func ModuleSubject(name ModuleName) (Subject, error) {
	if err := name.validate(); err != nil {
		return Subject{}, err
	}
	return Subject{kind: SubjectModule, identity: string(name)}, nil
}

// Kind reports whether the Subject is a File, Folder, or Module.
func (s Subject) Kind() SubjectKind { return s.kind }

// Identity returns the stable value used in evaluations and
// Diagnostics: a repo-relative path or a Module name.
func (s Subject) Identity() string { return s.identity }

// IsZero reports an unconstructed Subject.
func (s Subject) IsZero() bool { return s.identity == "" }

// Equals compares kind and identity.
func (s Subject) Equals(other Subject) bool {
	return s.kind == other.kind && s.identity == other.identity
}

func (s Subject) String() string { return string(s.kind) + ":" + s.identity }
