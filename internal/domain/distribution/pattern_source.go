package distribution

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// SourceKind names one place arclint resolves a PatternReference
// from. A check resolves through embedded and local only; a Registry
// is read when a command asks to vendor or install.
type SourceKind string

// The closed set of PatternSource kinds, in resolution order.
const (
	SourceEmbedded SourceKind = "embedded"
	SourceLocal    SourceKind = "local"
	SourceRegistry SourceKind = "registry"
)

// Valid reports membership in the closed set.
func (k SourceKind) Valid() bool {
	switch k {
	case SourceEmbedded, SourceLocal, SourceRegistry:
		return true
	}
	return false
}

// Available is one Pattern a PatternSource carries: the validated
// Pattern beside the exact files it was loaded from, so its Digest is
// known wherever it is listed and two sources carrying one reference
// can be compared byte for byte. Vendored reports whether a Manifest
// travelled with the files (a Registry copy or a vendored copy) rather
// than being computed from files someone authored in place.
type Available struct {
	Kind     SourceKind
	Pattern  rule.Pattern
	Vendored VendoredPattern
	Authored bool
}

// NewAvailable pairs a Pattern with its files and source kind; the
// Manifest and the Pattern must agree on the reference.
func NewAvailable(kind SourceKind, p rule.Pattern, v VendoredPattern, authored bool) (Available, error) {
	if !kind.Valid() {
		return Available{}, fmt.Errorf("available pattern: source kind %q invalid", kind)
	}
	if p.Reference().IsZero() {
		return Available{}, fmt.Errorf("available pattern: unconstructed pattern")
	}
	if v.IsZero() {
		return Available{}, fmt.Errorf("available pattern %s: files required", p.Reference())
	}
	if v.Reference() != p.Reference() {
		return Available{}, fmt.Errorf("available pattern %s: manifest names %s", p.Reference(), v.Reference())
	}
	return Available{Kind: kind, Pattern: p, Vendored: v, Authored: authored}, nil
}

// Reference is the Pattern's exact identity.
func (a Available) Reference() rule.PatternReference { return a.Pattern.Reference() }

// Digest is the whole-Pattern Digest.
func (a Available) Digest() Digest { return a.Vendored.Digest() }

// PatternSource carries the available Patterns from one resolving place.
// Its contents are immutable; the application loads them through its I/O port.
// Sources that carry the same reference must agree on its published bytes.
type PatternSource struct {
	available []Available
}

// NewPatternSource validates the available Patterns and retains their source
// order. Every Pattern comes from the same kind of place; an empty source
// is valid and carries no Patterns.
func NewPatternSource(available []Available) (PatternSource, error) {
	source := PatternSource{available: append([]Available(nil), available...)}
	seen := map[rule.PatternReference]Available{}
	for _, a := range source.available {
		if _, err := NewAvailable(a.Kind, a.Pattern, a.Vendored, a.Authored); err != nil {
			return PatternSource{}, fmt.Errorf("pattern source: %w", err)
		}
		if a.Kind != source.available[0].Kind {
			return PatternSource{}, fmt.Errorf("pattern source: mixes %s and %s patterns", source.available[0].Kind, a.Kind)
		}
		if previous, ok := seen[a.Reference()]; ok {
			if err := agreeingCopies(previous, a); err != nil {
				return PatternSource{}, err
			}
		}
		seen[a.Reference()] = a
	}
	return source, nil
}

// Available returns the source's Patterns without exposing its stored slice.
func (s PatternSource) Available() []Available {
	return append([]Available(nil), s.available...)
}

func agreeingCopies(first, next Available) error {
	if !first.Digest().Equals(next.Digest()) {
		return fmt.Errorf("pattern %s is %s with digest %s but %s with digest %s; a published version is immutable, so one of the copies is not the published one",
			first.Reference(), first.Kind, first.Digest().Short(), next.Kind, next.Digest().Short())
	}
	return nil
}

// Catalog is every Available Pattern the resolving sources carry,
// deduplicated by reference in source order. Two sources carrying one
// reference must agree on its Digest: a published version is immutable,
// so a disagreement is an error, never a silent choice. The agreeing
// copies stay known, so a listing can say a Pattern is both embedded
// and vendored, and vendoring an already vendored Pattern writes
// nothing.
type Catalog struct {
	entries []Available
	copies  [][]Available
}

// NewCatalog folds validated PatternSources in resolution order.
func NewCatalog(sources ...PatternSource) (Catalog, error) {
	var c Catalog
	index := map[string]int{}
	for _, source := range sources {
		for _, a := range source.available {
			key := a.Reference().String()
			if i, dup := index[key]; dup {
				if err := agreeingCopies(c.entries[i], a); err != nil {
					return Catalog{}, err
				}
				c.copies[i] = append(c.copies[i], a)
				continue
			}
			index[key] = len(c.entries)
			c.entries = append(c.entries, a)
			c.copies = append(c.copies, []Available{a})
		}
	}
	return c, nil
}

// Entries returns every Available Pattern in source order.
func (c Catalog) Entries() []Available {
	return append([]Available(nil), c.entries...)
}

// Copies returns every agreeing copy of one reference in source order,
// the resolving one first; nil when the Catalog does not carry it.
func (c Catalog) Copies(ref rule.PatternReference) []Available {
	for i, a := range c.entries {
		if a.Reference() == ref {
			return append([]Available(nil), c.copies[i]...)
		}
	}
	return nil
}

// Patterns returns the validated Patterns in source order.
func (c Catalog) Patterns() []rule.Pattern {
	out := make([]rule.Pattern, 0, len(c.entries))
	for _, a := range c.entries {
		out = append(out, a.Pattern)
	}
	return out
}

// Lookup finds one exact reference.
func (c Catalog) Lookup(ref rule.PatternReference) (Available, bool) {
	for _, a := range c.entries {
		if a.Reference() == ref {
			return a, true
		}
	}
	return Available{}, false
}

// References lists every carried reference in source order.
func (c Catalog) References() []rule.PatternReference {
	out := make([]rule.PatternReference, 0, len(c.entries))
	for _, a := range c.entries {
		out = append(out, a.Reference())
	}
	return out
}

// Spellings spells every carried reference, for messages.
func (c Catalog) Spellings() []string {
	out := make([]string, 0, len(c.entries))
	for _, a := range c.entries {
		out = append(out, a.Reference().String())
	}
	return out
}
