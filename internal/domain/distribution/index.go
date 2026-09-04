package distribution

import (
	"fmt"
	"sort"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// IndexEntry is one published version as the Registry index describes
// it: enough to list, select, and verify a Pattern before any of its
// files are fetched.
type IndexEntry struct {
	ref           rule.PatternReference
	digest        Digest
	documentation string
	coverage      []rule.Language
	rules         int
	extensions    int
}

// NewIndexEntry validates one index line.
func NewIndexEntry(ref rule.PatternReference, digest Digest, documentation string, coverage []rule.Language, rules, extensions int) (IndexEntry, error) {
	if ref.IsZero() {
		return IndexEntry{}, fmt.Errorf("index entry: reference required")
	}
	fail := func(err error) (IndexEntry, error) {
		return IndexEntry{}, fmt.Errorf("index entry %s: %v", ref, err)
	}
	if digest.IsZero() {
		return fail(fmt.Errorf("digest required"))
	}
	if rules < 1 {
		return fail(fmt.Errorf("a pattern distributes at least one rule"))
	}
	if extensions < 0 {
		return fail(fmt.Errorf("negative extension count"))
	}
	seen := map[rule.Language]bool{}
	for _, l := range coverage {
		if !l.Valid() {
			return fail(fmt.Errorf("coverage %q is not a supported language", l))
		}
		if seen[l] {
			return fail(fmt.Errorf("coverage lists %s twice", l))
		}
		seen[l] = true
	}
	return IndexEntry{
		ref: ref, digest: digest, documentation: documentation,
		coverage: append([]rule.Language(nil), coverage...), rules: rules, extensions: extensions,
	}, nil
}

// IndexEntryOf describes one Available Pattern as the index would.
func IndexEntryOf(a Available) (IndexEntry, error) {
	return NewIndexEntry(a.Reference(), a.Digest(), a.Pattern.Documentation(),
		a.Pattern.Coverage(), len(a.Pattern.Rules()), len(a.Pattern.Extensions()))
}

// Reference is the published version.
func (e IndexEntry) Reference() rule.PatternReference { return e.ref }

// Digest is the whole-Pattern Digest the Manifest must repeat.
func (e IndexEntry) Digest() Digest { return e.digest }

// Documentation is the Pattern's documentation URL, possibly empty.
func (e IndexEntry) Documentation() string { return e.documentation }

// Coverage lists the languages the Pattern covers.
func (e IndexEntry) Coverage() []rule.Language {
	return append([]rule.Language(nil), e.coverage...)
}

// Rules is the number of Rules the Pattern distributes.
func (e IndexEntry) Rules() int { return e.rules }

// Extensions is the number of Extensions the Pattern ships.
func (e IndexEntry) Extensions() int { return e.extensions }

// IsZero reports an unconstructed entry.
func (e IndexEntry) IsZero() bool { return e.ref.IsZero() }

// Index is a Registry's complete listing of published versions, one
// entry per reference, ordered by namespace, name, then version.
type Index struct {
	entries []IndexEntry
}

// NewIndex validates and orders the listing; one reference twice is an
// error because a published version is immutable.
func NewIndex(entries []IndexEntry) (Index, error) {
	seen := map[string]bool{}
	sorted := make([]IndexEntry, 0, len(entries))
	for _, e := range entries {
		if e.IsZero() {
			return Index{}, fmt.Errorf("index: unconstructed entry")
		}
		key := e.ref.String()
		if seen[key] {
			return Index{}, fmt.Errorf("index: %s is listed twice", key)
		}
		seen[key] = true
		sorted = append(sorted, e)
	}
	sortIndexEntries(sorted)
	return Index{entries: sorted}, nil
}

func sortIndexEntries(entries []IndexEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i].ref, entries[j].ref
		if a.Namespace() != b.Namespace() {
			return a.Namespace() < b.Namespace()
		}
		if a.Name() != b.Name() {
			return a.Name() < b.Name()
		}
		return CompareVersions(a.Version(), b.Version()) < 0
	})
}

// Entries returns the ordered listing.
func (x Index) Entries() []IndexEntry {
	return append([]IndexEntry(nil), x.entries...)
}

// References spells every published reference in order.
func (x Index) References() []rule.PatternReference {
	out := make([]rule.PatternReference, 0, len(x.entries))
	for _, e := range x.entries {
		out = append(out, e.ref)
	}
	return out
}

// Lookup finds one exact reference.
func (x Index) Lookup(ref rule.PatternReference) (IndexEntry, bool) {
	for _, e := range x.entries {
		if e.ref == ref {
			return e, true
		}
	}
	return IndexEntry{}, false
}

// With returns the Index with one entry added or, when its reference is
// already listed, replaced: how an export tree records a publication.
func (x Index) With(entry IndexEntry) (Index, error) {
	if entry.IsZero() {
		return Index{}, fmt.Errorf("index: unconstructed entry")
	}
	entries := make([]IndexEntry, 0, len(x.entries)+1)
	for _, e := range x.entries {
		if e.ref != entry.ref {
			entries = append(entries, e)
		}
	}
	entries = append(entries, entry)
	return NewIndex(entries)
}
