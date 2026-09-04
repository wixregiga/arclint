package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/distribution"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// PatternSummary is one Pattern as the patterns listing shows it:
// its identity, where it resolves from, what the repository carries
// under .arclint/patterns, and its Digest, so a reader can tell an
// embedded Pattern from a vendored copy and match either against what
// a Registry publishes.
type PatternSummary struct {
	Namespace string
	Name      string
	Version   string
	// Source is where the Pattern resolves from: embedded, local, or
	// registry.
	Source distribution.SourceKind
	Digest string
	// Vendored reports a verified copy under .arclint/patterns, with a
	// manifest.json every load checks the files against.
	Vendored bool
	// Authored reports a copy under .arclint/patterns without a
	// manifest: authored in place, loaded as written.
	Authored      bool
	Documentation string
	Rules         int
	Extensions    int
	Coverage      []string
}

// Reference spells the summary's PatternReference.
func (s PatternSummary) Reference() string {
	return s.Namespace + "/" + s.Name + "@" + s.Version
}

// ListPatterns lists every Pattern the resolving sources carry, and on
// request every Pattern a Registry publishes.
type ListPatterns struct {
	sources  []PatternSource
	registry PatternRegistry
}

// NewListPatterns requires at least one source; registry may be nil
// when remote listing is not offered.
func NewListPatterns(registry PatternRegistry, sources ...PatternSource) (ListPatterns, error) {
	if len(sources) == 0 {
		return ListPatterns{}, fmt.Errorf("list patterns: at least one pattern source required")
	}
	if err := validSources("list patterns", sources); err != nil {
		return ListPatterns{}, err
	}
	return ListPatterns{sources: sources, registry: registry}, nil
}

// Execute lists the offline Patterns in resolution order: embedded
// first, then the repository's own.
func (uc ListPatterns) Execute() ([]PatternSummary, error) {
	catalog, err := loadCatalog(uc.sources)
	if err != nil {
		return nil, fmt.Errorf("list patterns: %w", err)
	}
	entries := catalog.Entries()
	out := make([]PatternSummary, 0, len(entries))
	for _, a := range entries {
		out = append(out, summarizeAvailable(a, catalog.Copies(a.Reference())))
	}
	return out, nil
}

// Remote lists what the Registry at location publishes.
func (uc ListPatterns) Remote(location string) ([]PatternSummary, error) {
	if uc.registry == nil {
		return nil, fmt.Errorf("list patterns: no registry client configured")
	}
	reg, err := distribution.NewRegistry(location)
	if err != nil {
		return nil, fmt.Errorf("list patterns: %w", err)
	}
	index, err := uc.registry.Index(reg)
	if err != nil {
		return nil, fmt.Errorf("list patterns: %w", err)
	}
	entries := index.Entries()
	out := make([]PatternSummary, 0, len(entries))
	for _, e := range entries {
		out = append(out, PatternSummary{
			Namespace:     e.Reference().Namespace(),
			Name:          e.Reference().Name(),
			Version:       e.Reference().Version(),
			Source:        distribution.SourceRegistry,
			Digest:        e.Digest().String(),
			Documentation: e.Documentation(),
			Rules:         e.Rules(),
			Extensions:    e.Extensions(),
			Coverage:      runtimeTargetsOf(e.Coverage()),
		})
	}
	return out, nil
}

// summarizeAvailable spells the resolving copy and what the
// repository carries of it under .arclint/patterns.
func summarizeAvailable(a distribution.Available, copies []distribution.Available) PatternSummary {
	ref := a.Reference()
	s := PatternSummary{
		Namespace:     ref.Namespace(),
		Name:          ref.Name(),
		Version:       ref.Version(),
		Source:        a.Kind,
		Digest:        a.Digest().String(),
		Documentation: a.Pattern.Documentation(),
		Rules:         len(a.Pattern.Rules()),
		Extensions:    len(a.Pattern.Extensions()),
		Coverage:      runtimeTargetsOf(a.Pattern.Coverage()),
	}
	for _, c := range copies {
		if c.Kind != distribution.SourceLocal {
			continue
		}
		if c.Authored {
			s.Authored = true
		} else {
			s.Vendored = true
		}
	}
	return s
}

func runtimeTargetsOf(languages []rule.Language) []string {
	out := make([]string, 0, len(languages))
	for _, l := range languages {
		out = append(out, l.RuntimeTarget())
	}
	return out
}
