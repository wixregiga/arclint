package distribution

import (
	"fmt"
	"strings"

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

// NewCatalog folds source results in resolution order.
func NewCatalog(available ...[]Available) (Catalog, error) {
	var c Catalog
	index := map[string]int{}
	for _, batch := range available {
		for _, a := range batch {
			if a.Pattern.Reference().IsZero() || a.Vendored.IsZero() || !a.Kind.Valid() {
				return Catalog{}, fmt.Errorf("catalog: unconstructed available pattern")
			}
			key := a.Reference().String()
			if i, dup := index[key]; dup {
				if !c.entries[i].Digest().Equals(a.Digest()) {
					return Catalog{}, fmt.Errorf("pattern %s is %s with digest %s but %s with digest %s; a published version is immutable, so one of the copies is not the published one",
						key, c.entries[i].Kind, c.entries[i].Digest().Short(), a.Kind, a.Digest().Short())
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

// Selection resolves one spelling against references: an exact
// namespace/name@version, a namespace/name (its highest version), or a
// bare name carried by exactly one namespace/name. It answers the
// matching references in the order given, so a caller that resolves
// through several sources can prefer the first.
func Selection(spelling string, refs []rule.PatternReference) ([]rule.PatternReference, error) {
	spelling = strings.TrimSpace(spelling)
	if spelling == "" {
		return nil, fmt.Errorf("pattern selection: spelling required")
	}
	if strings.Contains(spelling, "@") {
		ref, err := rule.ParsePatternReference(spelling)
		if err != nil {
			return nil, fmt.Errorf("pattern selection %q: expected namespace/name@version", spelling)
		}
		for _, r := range refs {
			if r == ref {
				return []rule.PatternReference{r}, nil
			}
		}
		return nil, nil
	}
	var out []rule.PatternReference
	if strings.Contains(spelling, "/") {
		parts := strings.Split(spelling, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("pattern selection %q: expected namespace/name[@version] or name", spelling)
		}
		for _, r := range refs {
			if r.Namespace() == parts[0] && r.Name() == parts[1] {
				out = append(out, r)
			}
		}
		return highestVersions(out), nil
	}
	for _, r := range refs {
		if r.Name() == spelling {
			out = append(out, r)
		}
	}
	return highestVersions(out), nil
}

// highestVersions keeps, per namespace/name, the highest version by
// semantic-version ordering, preserving first-seen order of names.
func highestVersions(refs []rule.PatternReference) []rule.PatternReference {
	var order []string
	best := map[string]rule.PatternReference{}
	for _, r := range refs {
		key := r.Namespace() + "/" + r.Name()
		cur, ok := best[key]
		if !ok {
			order = append(order, key)
			best[key] = r
			continue
		}
		if CompareVersions(r.Version(), cur.Version()) > 0 {
			best[key] = r
		}
	}
	out := make([]rule.PatternReference, 0, len(order))
	for _, key := range order {
		out = append(out, best[key])
	}
	return out
}
