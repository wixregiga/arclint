package application

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/distribution"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// PatternSource is one place Patterns resolve from without the
// network: the binary's embedded Patterns or the repository's own
// .arclint/patterns tree. Each Pattern comes with the exact files it
// was loaded from, so its Digest is known wherever it is listed.
type PatternSource interface {
	Available() ([]distribution.Available, error)
}

// PatternRegistry reads a Registry: its index, and one published
// version verified against its Manifest. A check never calls it; only
// the commands that vendor, install, or list remotely do.
type PatternRegistry interface {
	Index(reg distribution.Registry) (distribution.Index, error)
	Fetch(reg distribution.Registry, ref rule.PatternReference) (distribution.Available, error)
}

// PatternStore writes a VendoredPattern under .arclint/patterns,
// replacing any other version of the same namespace and name.
type PatternStore interface {
	Write(v distribution.VendoredPattern) (StoredPattern, error)
}

// StoredPattern reports where a VendoredPattern was written and which
// version it replaced, "" when none.
type StoredPattern struct {
	Path     string
	Replaced string
}

// RulesetEditor records an Installation in the repository ruleset,
// preserving everything else in the document.
type RulesetEditor interface {
	// Exists reports whether the ruleset file is present.
	Exists() (bool, error)
	// Extend adds the extends entry, or replaces the entry of the same
	// namespace and name keeping its bindings. A Module the ruleset
	// already declares under the Pattern Module's name is bound to the
	// declared paths, and the declaration is folded into the Binding.
	Extend(inst rule.Installation) (RulesetChange, error)
}

// RulesetChange reports what the edit wrote.
type RulesetChange struct {
	// Path is the edited ruleset.
	Path string
	// Replaced is the version an existing extends entry of the same
	// Pattern was moved from, "" when the entry is new.
	Replaced string
	// Installation is the extends entry as written, with any Binding
	// taken over from a declared Module.
	Installation rule.Installation
	// Adopted lists the declared Modules folded into Bindings.
	Adopted []rule.ModuleName
}

// PatternPublisher writes one Available Pattern into a Registry tree
// on disk: its version directory and an updated index.
type PatternPublisher interface {
	Publish(dir string, a distribution.Available) (PublishedPattern, error)
}

// PublishedPattern reports the written version directory and index.
type PublishedPattern struct {
	VersionDir string
	IndexPath  string
	Replaced   bool
}

// loadCatalog folds the resolving sources into one Catalog.
func loadCatalog(sources []PatternSource) (distribution.Catalog, error) {
	batches := make([][]distribution.Available, 0, len(sources))
	for _, s := range sources {
		available, err := s.Available()
		if err != nil {
			return distribution.Catalog{}, fmt.Errorf("resolve patterns: %w", err)
		}
		batches = append(batches, available)
	}
	catalog, err := distribution.NewCatalog(batches...)
	if err != nil {
		return distribution.Catalog{}, fmt.Errorf("resolve patterns: %w", err)
	}
	return catalog, nil
}

// patternResolver resolves one selection offline first and through a
// Registry only when nothing embedded or local carries it.
type patternResolver struct {
	sources  []PatternSource
	registry PatternRegistry
}

func validSources(what string, sources []PatternSource) error {
	for _, s := range sources {
		if s == nil {
			return fmt.Errorf("%s: nil pattern source", what)
		}
	}
	return nil
}

// resolved is one resolved Pattern with every agreeing offline copy
// of it, the resolving copy first; a Registry fetch has no copies.
type resolved struct {
	distribution.Available
	copies []distribution.Available
}

// vendoredLocally reports whether a verified copy already lives under
// .arclint/patterns.
func (r resolved) vendoredLocally() bool {
	for _, c := range r.copies {
		if c.Kind == distribution.SourceLocal && !c.Authored {
			return true
		}
	}
	return false
}

// resolve returns the Available Pattern one spelling selects. Location
// names the Registry consulted when the catalog has no match; empty
// means offline only.
func (r patternResolver) resolve(selection, location string) (resolved, error) {
	catalog, err := loadCatalog(r.sources)
	if err != nil {
		return resolved{}, err
	}
	refs, err := distribution.Selection(selection, catalog.References())
	if err != nil {
		return resolved{}, fmt.Errorf("%w; available: %s", err, strings.Join(catalog.Spellings(), ", "))
	}
	switch len(refs) {
	case 1:
		a, _ := catalog.Lookup(refs[0])
		return resolved{Available: a, copies: catalog.Copies(refs[0])}, nil
	case 0:
	default:
		return resolved{}, ambiguous(selection, refs)
	}
	if location == "" || r.registry == nil {
		return resolved{}, fmt.Errorf("pattern %q is not embedded in this binary and not under .arclint/patterns; available: %s",
			selection, strings.Join(catalog.Spellings(), ", "))
	}
	reg, err := distribution.NewRegistry(location)
	if err != nil {
		return resolved{}, fmt.Errorf("pattern %q: %w", selection, err)
	}
	index, err := r.registry.Index(reg)
	if err != nil {
		return resolved{}, fmt.Errorf("pattern %q is not embedded in this binary and not under .arclint/patterns, and the registry could not be read: %w", selection, err)
	}
	refs, err = distribution.Selection(selection, index.References())
	if err != nil {
		return resolved{}, fmt.Errorf("%w; published at %s: %s", err, reg, strings.Join(spellings(index.References()), ", "))
	}
	switch len(refs) {
	case 1:
		entry, _ := index.Lookup(refs[0])
		a, err := r.registry.Fetch(reg, refs[0])
		if err != nil {
			return resolved{}, fmt.Errorf("pattern %q: %w", selection, err)
		}
		if !a.Digest().Equals(entry.Digest()) {
			return resolved{}, fmt.Errorf("pattern %s fetched from %s has digest %s, the registry index records %s; the published files and the index disagree",
				refs[0], reg, a.Digest().Short(), entry.Digest().Short())
		}
		return resolved{Available: a}, nil
	case 0:
		return resolved{}, fmt.Errorf("pattern %q is not embedded, not under .arclint/patterns, and not published at %s; run arclint patterns --remote to list the registry",
			selection, reg)
	default:
		return resolved{}, ambiguous(selection, refs)
	}
}

func ambiguous(selection string, refs []rule.PatternReference) error {
	return fmt.Errorf("pattern name %q is ambiguous; use one of %s", selection, strings.Join(spellings(refs), ", "))
}

func spellings(refs []rule.PatternReference) []string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ref.String())
	}
	return out
}
