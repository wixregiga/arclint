package rule

import (
	"errors"
	"regexp"
)

// SubjectKind distinguishes the four concrete things a Rule evaluates.
type SubjectKind string

// The four Subject kinds.
const (
	SubjectFile    SubjectKind = "file"
	SubjectFolder  SubjectKind = "folder"
	SubjectZone    SubjectKind = "zone"
	SubjectContext SubjectKind = "context"
)

// contextSubjectName is the spelling a recorded bounded context shares
// with the Zone that holds its code.
var contextSubjectName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// Subject is one concrete File, Folder, Zone, or recorded bounded
// Context selected by Rule Scope. A Programming Language
// filters Scope but is never a Subject.
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

// ZoneSubject identifies one declared Zone.
func ZoneSubject(name ZoneName) (Subject, error) {
	if err := name.validate(); err != nil {
		return Subject{}, err
	}
	return Subject{kind: SubjectZone, identity: string(name)}, nil
}

// ContextSubject identifies one bounded context of the recorded domain,
// judged through the Zones that hold its code.
func ContextSubject(name string) (Subject, error) {
	if !contextSubjectName.MatchString(name) {
		return Subject{}, errors.New("subject: context name " + name + " is not spelled like a Zone name")
	}
	return Subject{kind: SubjectContext, identity: name}, nil
}

// Kind reports whether the Subject is a File, Folder, Zone, or
// Context.
func (s Subject) Kind() SubjectKind { return s.kind }

// Identity returns the stable value used in evaluations and
// Diagnostics: a repo-relative path, a Zone name, or a context name.
func (s Subject) Identity() string { return s.identity }

// IsZero reports an unconstructed Subject.
func (s Subject) IsZero() bool { return s.identity == "" }

// Equals compares kind and identity.
func (s Subject) Equals(other Subject) bool {
	return s.kind == other.kind && s.identity == other.identity
}

// IsPath reports whether the identity is a repo-relative path a
// Violation can anchor to; a Zone or a Context is not a path, so a
// Violation of one carries an anchor of its own.
func (s Subject) IsPath() bool {
	return s.kind == SubjectFile || s.kind == SubjectFolder
}

func (s Subject) String() string { return string(s.kind) + ":" + s.identity }
