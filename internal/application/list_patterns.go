package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// PatternSource supplies available Pattern distribution packages as
// validated domain values.
type PatternSource interface {
	Patterns() ([]rule.Pattern, error)
}

// PatternSummary is the plain result value describing one available
// Pattern.
type PatternSummary struct {
	Namespace string
	Name      string
	Version   string
	Rules     int
	Coverage  []string
}

// ListPatterns lists available Pattern distribution packages through
// the source ports, concatenated in constructor argument order.
type ListPatterns struct {
	sources []PatternSource
}

// NewListPatterns requires at least one Pattern source port.
func NewListPatterns(sources ...PatternSource) (ListPatterns, error) {
	if len(sources) == 0 {
		return ListPatterns{}, fmt.Errorf("list patterns: missing pattern source")
	}
	for _, source := range sources {
		if source == nil {
			return ListPatterns{}, fmt.Errorf("list patterns: missing pattern source")
		}
	}
	return ListPatterns{sources: append([]PatternSource(nil), sources...)}, nil
}

// Execute returns one summary per available Pattern.
func (uc ListPatterns) Execute() ([]PatternSummary, error) {
	var out []PatternSummary
	for _, source := range uc.sources {
		patterns, err := source.Patterns()
		if err != nil {
			return nil, fmt.Errorf("load patterns: %w", err)
		}
		for _, p := range patterns {
			ref := p.Reference()
			coverage := make([]string, 0, len(p.Coverage()))
			for _, l := range p.Coverage() {
				coverage = append(coverage, string(l))
			}
			out = append(out, PatternSummary{
				Namespace: ref.Namespace(),
				Name:      ref.Name(),
				Version:   ref.Version(),
				Rules:     len(p.Rules()),
				Coverage:  coverage,
			})
		}
	}
	return out, nil
}
