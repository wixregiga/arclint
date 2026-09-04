package distribution

import (
	"fmt"
	"sort"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// VendoredPattern is an exact copy of one published Pattern version
// with its Manifest: the unit that is fetched from a Registry, written
// under .arclint/patterns/<namespace>/<name>/, exported for
// publication, and verified byte for byte whenever it is read back.
type VendoredPattern struct {
	manifest Manifest
	files    []PatternFile
}

// NewVendoredPattern pairs files with the Manifest that must describe
// them exactly.
func NewVendoredPattern(manifest Manifest, files []PatternFile) (VendoredPattern, error) {
	if manifest.IsZero() {
		return VendoredPattern{}, fmt.Errorf("vendored pattern: manifest required")
	}
	if err := manifest.Verify(files); err != nil {
		return VendoredPattern{}, fmt.Errorf("vendored pattern: %w", err)
	}
	sorted := append([]PatternFile(nil), files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].path < sorted[j].path })
	return VendoredPattern{manifest: manifest, files: sorted}, nil
}

// Vendor computes the Manifest of exact files for one reference and
// pairs them: the constructor for a Pattern leaving its source.
func Vendor(ref rule.PatternReference, files []PatternFile) (VendoredPattern, error) {
	m, err := ManifestOf(ref, files)
	if err != nil {
		return VendoredPattern{}, err
	}
	return NewVendoredPattern(m, files)
}

// Manifest is the record describing the files.
func (v VendoredPattern) Manifest() Manifest { return v.manifest }

// Reference is the published version.
func (v VendoredPattern) Reference() rule.PatternReference { return v.manifest.ref }

// Digest is the whole-Pattern Digest.
func (v VendoredPattern) Digest() Digest { return v.manifest.digest }

// Files returns every shipped file in path order.
func (v VendoredPattern) Files() []PatternFile {
	return append([]PatternFile(nil), v.files...)
}

// File returns one shipped file by relative path.
func (v VendoredPattern) File(filePath string) (PatternFile, bool) {
	for _, f := range v.files {
		if f.path == filePath {
			return f, true
		}
	}
	return PatternFile{}, false
}

// IsZero reports an unconstructed value.
func (v VendoredPattern) IsZero() bool { return v.manifest.IsZero() }
