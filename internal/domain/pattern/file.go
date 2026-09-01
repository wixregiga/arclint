package pattern

import (
	"fmt"
	"path"
	"strings"
)

// File is one file in a Pattern version's full distribution tree.
type File struct {
	path  string
	bytes []byte
}

// NewFile preserves exact bytes and requires an unambiguous canonical path
// relative to the Pattern root.
func NewFile(name string, content []byte) (File, error) {
	if name == "" || strings.Contains(name, `\`) || strings.HasPrefix(name, "/") || path.Clean(name) != name || name == "." || name == ".." || strings.HasPrefix(name, "../") {
		return File{}, fmt.Errorf("pattern file %q: path must be canonical and relative to the pattern root", name)
	}
	return File{path: name, bytes: append([]byte(nil), content...)}, nil
}

// Path is the canonical slash-separated path relative to the Pattern root.
func (f File) Path() string { return f.path }

// Bytes returns an exact defensive copy of the file contents.
func (f File) Bytes() []byte { return append([]byte(nil), f.bytes...) }

// IsZero reports an unconstructed File.
func (f File) IsZero() bool { return f.path == "" }
