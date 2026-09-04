package distribution

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// ManifestEntry names one PatternFile by path and Digest.
type ManifestEntry struct {
	path   string
	digest Digest
}

// NewManifestEntry requires a valid relative path and a constructed
// Digest.
func NewManifestEntry(filePath string, digest Digest) (ManifestEntry, error) {
	if err := validateFilePath(filePath); err != nil {
		return ManifestEntry{}, err
	}
	if digest.IsZero() {
		return ManifestEntry{}, fmt.Errorf("manifest entry %q: digest required", filePath)
	}
	return ManifestEntry{path: filePath, digest: digest}, nil
}

// Path is the relative forward-slash path inside the Pattern.
func (e ManifestEntry) Path() string { return e.path }

// Digest is the file's content hash.
func (e ManifestEntry) Digest() Digest { return e.digest }

// Manifest is the record a published Pattern version travels with:
// its PatternReference, its Digest, and the path and Digest of every
// PatternFile it ships. Entries are kept sorted by path so two
// Manifests of one version are equal value for value.
type Manifest struct {
	ref     rule.PatternReference
	digest  Digest
	entries []ManifestEntry
}

// NewManifest reconstructs a Manifest from its recorded parts and
// validates it as a whole: every entry stays inside the Pattern's
// directory, no path appears twice, pattern.yaml is listed, and the
// recorded Digest is exactly the Digest of the sorted entries.
func NewManifest(ref rule.PatternReference, digest Digest, entries []ManifestEntry) (Manifest, error) {
	if ref.IsZero() {
		return Manifest{}, fmt.Errorf("manifest: pattern reference required")
	}
	fail := func(format string, args ...any) (Manifest, error) {
		return Manifest{}, fmt.Errorf("manifest %s: %s", ref, fmt.Sprintf(format, args...))
	}
	if digest.IsZero() {
		return fail("digest required")
	}
	sorted := append([]ManifestEntry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].path < sorted[j].path })
	seen := map[string]bool{}
	hasDocument := false
	for _, e := range sorted {
		if e.path == "" {
			return fail("unconstructed entry")
		}
		if seen[e.path] {
			return fail("file %q listed twice", e.path)
		}
		seen[e.path] = true
		if e.path == PatternFileName {
			hasDocument = true
		}
	}
	if !hasDocument {
		return fail("%s is not listed", PatternFileName)
	}
	computed := digestOfEntries(sorted)
	if !computed.Equals(digest) {
		return fail("recorded digest %s does not match the digest of the listed files %s", digest, computed)
	}
	return Manifest{ref: ref, digest: digest, entries: sorted}, nil
}

// ManifestOf computes the Manifest of exact files for one reference.
func ManifestOf(ref rule.PatternReference, files []PatternFile) (Manifest, error) {
	entries := make([]ManifestEntry, 0, len(files))
	for _, f := range files {
		if f.IsZero() {
			return Manifest{}, fmt.Errorf("manifest %s: unconstructed file", ref)
		}
		entries = append(entries, ManifestEntry{path: f.path, digest: f.Digest()})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })
	return NewManifest(ref, digestOfEntries(entries), entries)
}

// digestOfEntries hashes the canonical listing: one "<digest>  <path>"
// line per entry in path order. File order, platform, and time never
// enter it.
func digestOfEntries(sorted []ManifestEntry) Digest {
	var b strings.Builder
	for _, e := range sorted {
		b.WriteString(e.digest.String())
		b.WriteString("  ")
		b.WriteString(e.path)
		b.WriteByte('\n')
	}
	return DigestOf([]byte(b.String()))
}

// Reference is the published version the Manifest describes.
func (m Manifest) Reference() rule.PatternReference { return m.ref }

// Digest is the whole-Pattern Digest.
func (m Manifest) Digest() Digest { return m.digest }

// Entries returns every listed file in path order.
func (m Manifest) Entries() []ManifestEntry {
	return append([]ManifestEntry(nil), m.entries...)
}

// IsZero reports an unconstructed Manifest.
func (m Manifest) IsZero() bool { return m.ref.IsZero() }

// Verify checks that files are exactly the listed ones with the listed
// Digests: a missing file, an unlisted file, or changed bytes each
// fail by name.
func (m Manifest) Verify(files []PatternFile) error {
	byPath := map[string]PatternFile{}
	for _, f := range files {
		if f.IsZero() {
			return fmt.Errorf("manifest %s: unconstructed file", m.ref)
		}
		if _, dup := byPath[f.path]; dup {
			return fmt.Errorf("manifest %s: file %q supplied twice", m.ref, f.path)
		}
		byPath[f.path] = f
	}
	for _, e := range m.entries {
		f, ok := byPath[e.path]
		if !ok {
			return fmt.Errorf("manifest %s: listed file %q is missing", m.ref, e.path)
		}
		if got := f.Digest(); !got.Equals(e.digest) {
			return fmt.Errorf("manifest %s: file %q has digest %s, manifest records %s", m.ref, e.path, got, e.digest)
		}
		delete(byPath, e.path)
	}
	if len(byPath) > 0 {
		extra := make([]string, 0, len(byPath))
		for p := range byPath {
			extra = append(extra, p)
		}
		sort.Strings(extra)
		return fmt.Errorf("manifest %s: unlisted file(s) %s", m.ref, strings.Join(extra, ", "))
	}
	return nil
}
