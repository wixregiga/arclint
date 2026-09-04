package distribution

import (
	"fmt"
	"path"
	"strings"
)

// PatternFileName is the one file every Pattern ships: its
// distribution document.
const PatternFileName = "pattern.yaml"

// ExtensionsDir is the directory, relative to the Pattern's own
// directory, under which every Extension the Pattern distributes lives.
const ExtensionsDir = "extensions"

// PatternFile is one file a Pattern ships: a forward-slash path
// relative to the Pattern's own directory and the file's bytes.
type PatternFile struct {
	path string
	data []byte
}

// NewPatternFile requires a clean relative forward-slash path that
// stays inside the Pattern's directory. Bytes may be empty: an empty
// file is still a file the Manifest must account for.
func NewPatternFile(filePath string, data []byte) (PatternFile, error) {
	if err := validateFilePath(filePath); err != nil {
		return PatternFile{}, err
	}
	return PatternFile{path: filePath, data: append([]byte(nil), data...)}, nil
}

// Path is the relative forward-slash path inside the Pattern.
func (f PatternFile) Path() string { return f.path }

// Data returns a copy of the file bytes.
func (f PatternFile) Data() []byte { return append([]byte(nil), f.data...) }

// Digest hashes the file bytes.
func (f PatternFile) Digest() Digest { return DigestOf(f.data) }

// IsZero reports an unconstructed file.
func (f PatternFile) IsZero() bool { return f.path == "" }

// validateFilePath accepts only a slash-separated relative path that
// is already clean: no leading slash, no drive, no ".", "..", or empty
// segment, and no backslash. Anything else could escape the Pattern's
// directory once written to disk.
func validateFilePath(p string) error {
	if p == "" {
		return fmt.Errorf("pattern file: path required")
	}
	if strings.ContainsAny(p, `\`) || strings.Contains(p, ":") {
		return fmt.Errorf("pattern file %q: path must use forward slashes and stay relative", p)
	}
	if strings.HasPrefix(p, "/") {
		return fmt.Errorf("pattern file %q: path must be relative to the pattern directory", p)
	}
	if path.Clean(p) != p {
		return fmt.Errorf("pattern file %q: path must be clean (no ., .., or repeated slashes)", p)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "." || seg == ".." || seg == "" {
			return fmt.Errorf("pattern file %q: path must stay inside the pattern directory", p)
		}
	}
	return nil
}
