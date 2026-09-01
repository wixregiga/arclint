package rule

import (
	"fmt"
	"path"
	"strings"
)

// Extension is executable enforcement code a Rule names through uses.
// It may be repository-local or carried by one Pattern version.
type Extension struct {
	name   string
	entry  string
	source []byte
}

// NewExtension requires a declared name, a safe JavaScript or TypeScript
// entry path, and non-blank source bytes.
func NewExtension(name, entry string, source []byte) (Extension, error) {
	if strings.TrimSpace(name) == "" {
		return Extension{}, fmt.Errorf("extension: name required")
	}
	if err := validateExtensionEntry(entry); err != nil {
		return Extension{}, err
	}
	if strings.TrimSpace(string(source)) == "" {
		return Extension{}, fmt.Errorf("extension %q: blank source", name)
	}
	return Extension{name: name, entry: entry, source: append([]byte(nil), source...)}, nil
}

// Name is the name Rules use to select this Extension.
func (e Extension) Name() string { return e.name }

// Entry is the slash-separated path of the executable entry in its owner.
func (e Extension) Entry() string { return e.entry }

// Bytes returns an exact defensive copy of the executable source bytes.
func (e Extension) Bytes() []byte { return append([]byte(nil), e.source...) }

// FileName returns the entry basename for repository-local installation.
func (e Extension) FileName() string { return path.Base(e.entry) }

// Source returns the executable source as text for the existing runtime.
func (e Extension) Source() string { return string(e.source) }

// IsZero reports an unconstructed Extension.
func (e Extension) IsZero() bool { return e.name == "" || e.entry == "" }

func validateExtensionEntry(entry string) error {
	if entry == "" {
		return fmt.Errorf("extension: entry required")
	}
	if strings.Contains(entry, `\`) || strings.HasPrefix(entry, "/") || path.Clean(entry) != entry || entry == "." || entry == ".." || strings.HasPrefix(entry, "../") {
		return fmt.Errorf("extension entry %q: must be a canonical relative path", entry)
	}
	base := path.Base(entry)
	if strings.HasPrefix(base, ".") {
		return fmt.Errorf("extension entry %q: hidden files are not installable entries", entry)
	}
	if strings.HasSuffix(base, ".d.ts") {
		return fmt.Errorf("extension entry %q: declaration files are not installable entries", entry)
	}
	if !strings.HasSuffix(base, ".ts") && !strings.HasSuffix(base, ".js") {
		return fmt.Errorf("extension entry %q: must end in .ts or .js", entry)
	}
	return nil
}

// InstallableExtensionFileName reports whether fileName is an installable
// JavaScript or TypeScript entry basename.
func InstallableExtensionFileName(fileName string) bool {
	return validateExtensionEntry(fileName) == nil && path.Base(fileName) == fileName
}
