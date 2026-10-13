package vocab

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// DomainRecordingFormat is the notation a DomainRecording is written in.
type DomainRecordingFormat string

// The formats a DomainRecording is written in, each named by the
// extension of the DomainRecording's file.
const (
	// DomainRecordingYAML is the YAML notation, the .yaml and .yml extensions.
	DomainRecordingYAML DomainRecordingFormat = "yaml"
	// DomainRecordingCML is the CML notation, the .cml extension.
	DomainRecordingCML DomainRecordingFormat = "cml"
)

// DefaultDomainRecordingPath is the DomainRecording of a project whose
// ruleset names none and in which no default-named file exists yet: the
// YAML file beside the ruleset. It is also the first of the
// default names, because YAML is the preferred format.
const DefaultDomainRecordingPath = "domain.arclint.yaml"

// domainRecordingFormats maps each extension a DomainRecording may
// carry, exactly as spelled, to the format it names.
var domainRecordingFormats = map[string]DomainRecordingFormat{
	".yaml": DomainRecordingYAML,
	".yml":  DomainRecordingYAML,
	".cml":  DomainRecordingCML,
}

// DomainRecordingExtensions lists the extensions a DomainRecording may
// carry, with their dot, sorted.
func DomainRecordingExtensions() []string {
	out := make([]string, 0, len(domainRecordingFormats))
	for extension := range domainRecordingFormats {
		out = append(out, extension)
	}
	sort.Strings(out)
	return out
}

// DomainRecording is the file a project writes its Ubiquitous Language
// down in: its slash-separated path from the directory that holds the
// ruleset, and its format. The ruleset may name it under domain.file; a
// project that names none records in the first default-named file that
// exists, ResolveDomainRecording. Every command, hook, finding and
// report that names the project's domain file names its DomainRecording.
//
// A DomainRecording is a value: NewDomainRecording is the only door,
// and applies its invariants, so a DomainRecording that
// violates one never exists. The zero DomainRecording names no file.
type DomainRecording struct {
	path   string
	format DomainRecordingFormat
}

// NewDomainRecording builds the DomainRecording of file. It applies
// inside-the-repository: the file is a non-empty relative path written
// with slashes that stays inside the directory holding the ruleset,
// never absolute on any platform, never behind a backslash or a drive
// letter, and never climbing out through "..". It then applies
// format-named-by-extension: the final name carries the exact lowercase
// extension .yaml, .yml or .cml after a non-empty stem. The path is held
// cleaned, so ./docs/../model.yaml is model.yaml. The filesystem
// completes inside-the-repository through links, which a path alone
// cannot show.
func NewDomainRecording(file string) (DomainRecording, error) {
	cleaned, err := insideRepository(file)
	if err != nil {
		return DomainRecording{}, err
	}
	format, err := formatNamedByExtension(file)
	if err != nil {
		return DomainRecording{}, err
	}
	return DomainRecording{path: cleaned, format: format}, nil
}

// DefaultDomainRecording is the DomainRecording of a project that names
// none and in which no default-named file exists: DefaultDomainRecordingPath.
func DefaultDomainRecording() (DomainRecording, error) {
	return NewDomainRecording(DefaultDomainRecordingPath)
}

// DefaultDomainRecordings lists the DomainRecordings a project that
// names none may have, in the order discovery prefers them:
// domain.arclint.yaml, domain.arclint.yml, domain.arclint.cml. YAML
// comes first because YAML is the preferred format.
func DefaultDomainRecordings() []DomainRecording {
	recordings := make([]DomainRecording, 0, 3)
	for _, file := range []string{DefaultDomainRecordingPath, "domain.arclint.yml", "domain.arclint.cml"} {
		recording, err := NewDomainRecording(file)
		if err == nil {
			recordings = append(recordings, recording)
		}
	}
	return recordings
}

// ResolveDomainRecording chooses the project's DomainRecording from the
// one its ruleset names, the zero DomainRecording when it names none,
// and exists, which reports whether a file is there. A named
// DomainRecording is the project's own choice: it is chosen, no
// discovery takes place, and no file is passed over. Otherwise
// discovery chooses the first of DefaultDomainRecordings that exists,
// and DefaultDomainRecording when none does, and also returns the
// default-named files it passed over, in the order of
// DefaultDomainRecordings: files the project keeps beside its domain
// file that no command reads. exists is asked only during discovery. An
// error from exists is returned with the file it was asked about.
func ResolveDomainRecording(named DomainRecording, exists func(DomainRecording) (bool, error)) (DomainRecording, []DomainRecording, error) {
	if !named.IsZero() {
		return named, nil, nil
	}
	if exists == nil {
		return DomainRecording{}, nil, fmt.Errorf("resolve DomainRecording: no way to tell which files exist")
	}
	var present []DomainRecording
	for _, candidate := range DefaultDomainRecordings() {
		found, err := exists(candidate)
		if err != nil {
			return DomainRecording{}, nil, fmt.Errorf("resolve DomainRecording: %s: %w", candidate.Path(), err)
		}
		if found {
			present = append(present, candidate)
		}
	}
	if len(present) == 0 {
		chosen, err := DefaultDomainRecording()
		if err != nil {
			return DomainRecording{}, nil, err
		}
		return chosen, nil, nil
	}
	return present[0], present[1:], nil
}

// Path returns the DomainRecording's file, slash-separated and
// relative to the directory that holds the ruleset.
func (r DomainRecording) Path() string { return r.path }

// Format returns the notation the DomainRecording is written in.
func (r DomainRecording) Format() DomainRecordingFormat { return r.format }

// IsZero reports the DomainRecording that names no file.
func (r DomainRecording) IsZero() bool { return r.path == "" }

// insideRepository applies the DomainRecording invariant
// inside-the-repository and returns the cleaned path.
func insideRepository(file string) (string, error) {
	const invariant = "DomainRecording inside-the-repository"
	switch {
	case file == "":
		return "", fmt.Errorf("%s: the path is empty; name a file inside the directory that holds the ruleset", invariant)
	case strings.Contains(file, `\`):
		return "", fmt.Errorf("%s: %q contains a backslash; write the path with slashes, relative to the directory that holds the ruleset", invariant, file)
	case strings.HasPrefix(file, "/"):
		return "", fmt.Errorf("%s: %q is absolute; name a file inside the directory that holds the ruleset", invariant, file)
	case hasDriveLetter(file):
		return "", fmt.Errorf("%s: %q starts with a drive letter; name a file inside the directory that holds the ruleset", invariant, file)
	}
	cleaned := path.Clean(file)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("%s: %q leaves the directory that holds the ruleset; name a file inside it", invariant, file)
	}
	return cleaned, nil
}

// hasDriveLetter reports a path that starts with a Windows drive, such
// as C: or c:/, which a platform other than Windows reads as a relative
// name and Windows reads as another volume.
func hasDriveLetter(file string) bool {
	if len(file) < 2 || file[1] != ':' {
		return false
	}
	letter := file[0]
	return (letter >= 'a' && letter <= 'z') || (letter >= 'A' && letter <= 'Z')
}

// formatNamedByExtension applies the DomainRecording invariant
// format-named-by-extension to the final name of the path as written.
// A trailing slash names a directory, not a file, and a name that is
// only an extension has no stem, so neither names a format.
func formatNamedByExtension(file string) (DomainRecordingFormat, error) {
	const invariant = "DomainRecording format-named-by-extension"
	name := file[strings.LastIndexByte(file, '/')+1:]
	dot := strings.LastIndexByte(name, '.')
	if dot > 0 {
		if format, ok := domainRecordingFormats[name[dot:]]; ok {
			return format, nil
		}
	}
	return "", fmt.Errorf("%s: %q does not end in .yaml, .yml or .cml after a file name; the extension names the format the language is written in", invariant, file)
}
