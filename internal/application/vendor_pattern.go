package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/distribution"
)

// VendorPatternRequest selects the Pattern to vendor and the Registry
// consulted when nothing embedded or local carries it.
type VendorPatternRequest struct {
	// Selection is namespace/name@version, namespace/name, or a name.
	Selection string
	// Registry is the Registry location; empty means offline only.
	Registry string
}

// VendorPatternResult reports what was written.
type VendorPatternResult struct {
	Reference string
	Digest    string
	// Source is where the copy came from: embedded, local, or registry.
	Source distribution.SourceKind
	// Path is the vendored directory, "" when nothing was written.
	Path string
	// Replaced is the version previously vendored under the same
	// namespace and name, "" when none.
	Replaced string
	// Unchanged reports that an identical vendored copy was already in
	// place, so nothing was written.
	Unchanged bool
}

// VendorPattern copies one Pattern version, with its Manifest, under
// .arclint/patterns so the repository resolves it offline and pins the
// exact bytes it adopted. Embedded Patterns vendor from the binary;
// anything else vendors from the Registry.
type VendorPattern struct {
	resolver patternResolver
	store    PatternStore
}

// NewVendorPattern requires the offline sources and the store;
// registry may be nil when only offline vendoring is offered.
func NewVendorPattern(store PatternStore, registry PatternRegistry, sources ...PatternSource) (VendorPattern, error) {
	if store == nil {
		return VendorPattern{}, fmt.Errorf("vendor pattern: missing pattern store")
	}
	if len(sources) == 0 {
		return VendorPattern{}, fmt.Errorf("vendor pattern: at least one pattern source required")
	}
	if err := validSources("vendor pattern", sources); err != nil {
		return VendorPattern{}, err
	}
	return VendorPattern{resolver: patternResolver{sources: sources, registry: registry}, store: store}, nil
}

// Execute resolves the selection and writes the vendored copy.
func (uc VendorPattern) Execute(req VendorPatternRequest) (VendorPatternResult, error) {
	a, err := uc.resolver.resolve(req.Selection, req.Registry)
	if err != nil {
		return VendorPatternResult{}, fmt.Errorf("vendor pattern: %w", err)
	}
	result := VendorPatternResult{
		Reference: a.Reference().String(),
		Digest:    a.Digest().String(),
		Source:    a.Kind,
	}
	if a.vendoredLocally() {
		result.Unchanged = true
		return result, nil
	}
	stored, err := uc.store.Write(a.Vendored)
	if err != nil {
		return VendorPatternResult{}, fmt.Errorf("vendor pattern %s: %w", a.Reference(), err)
	}
	result.Path = stored.Path
	result.Replaced = stored.Replaced
	return result, nil
}
