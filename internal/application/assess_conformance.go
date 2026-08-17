package application

import (
	"fmt"
	"sort"

	"github.com/wixregiga/arclint/internal/domain/baseline"
	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// ObservationSource is the port through which the use case requests
// normalized Observations for the configured languages and exactly
// the fact classes the Rules' Enforcement declares, without selecting
// parser technology.
type ObservationSource interface {
	Observe(languages []rule.Language, scan rule.Scan, facts []rule.Fact) (conformance.Observations, error)
}

// BaselineSource loads the repository's committed Baseline. Absence is
// a normal result, not an error.
type BaselineSource interface {
	Load() (baseline.Snapshot, bool, error)
}

// AssessConformanceRequest is the plain request value.
type AssessConformanceRequest struct {
	// SkipBaseline evaluates without subtracting the committed Baseline.
	SkipBaseline bool
}

// AssessConformance coordinates observations, configured Rules, the
// Baseline, and the Conformance Check.
type AssessConformance struct {
	rules        rule.Repository
	observations ObservationSource
	baselines    BaselineSource
	extensions   conformance.ExtensionEvaluator
}

// NewAssessConformance requires the repository, observation, and
// baseline ports. The Extension mechanism may be nil: extension Rules
// then evaluate unsupported, honestly, rather than being skipped.
func NewAssessConformance(rules rule.Repository, observations ObservationSource,
	baselines BaselineSource, extensions conformance.ExtensionEvaluator,
) (AssessConformance, error) {
	if rules == nil {
		return AssessConformance{}, fmt.Errorf("assess conformance: missing rule repository")
	}
	if observations == nil {
		return AssessConformance{}, fmt.Errorf("assess conformance: missing observation source")
	}
	if baselines == nil {
		return AssessConformance{}, fmt.Errorf("assess conformance: missing baseline source")
	}
	return AssessConformance{
		rules: rules, observations: observations,
		baselines: baselines, extensions: extensions,
	}, nil
}

// Execute runs one Conformance Check and applies the committed
// Baseline afterwards, surfacing stale Baseline entries as a coverage
// Diagnostic.
func (uc AssessConformance) Execute(req AssessConformanceRequest) (conformance.Assessment, error) {
	cfg, err := uc.rules.ConfiguredRules()
	if err != nil {
		return conformance.Assessment{}, fmt.Errorf("load configured rules: %w", err)
	}
	observations, err := uc.observations.Observe(cfg.Languages, cfg.Scan, requiredFacts(cfg.Rules))
	if err != nil {
		return conformance.Assessment{}, fmt.Errorf("observe repository: %w", err)
	}
	assessment, err := conformance.Run(conformance.Request{
		Rules:          cfg.Rules,
		Modules:        cfg.Modules,
		Observations:   observations,
		UnknownImports: cfg.Scan.UnknownImports,
		Extensions:     uc.extensions,
	})
	if err != nil {
		return conformance.Assessment{}, fmt.Errorf("conformance check: %w", err)
	}
	if req.SkipBaseline {
		return assessment, nil
	}
	snapshot, present, err := uc.baselines.Load()
	if err != nil {
		return conformance.Assessment{}, fmt.Errorf("load baseline: %w", err)
	}
	if !present {
		return assessment, nil
	}
	covered, stale, err := snapshot.Apply(assessment)
	if err != nil {
		return conformance.Assessment{}, fmt.Errorf("apply baseline: %w", err)
	}
	if len(stale) > 0 {
		note, err := conformance.NewCoverage("",
			fmt.Sprintf("baseline: %d adopted finding(s) no longer occur; refresh the baseline", staleCount(stale)))
		if err != nil {
			return conformance.Assessment{}, fmt.Errorf("baseline coverage note: %w", err)
		}
		noted, err := covered.WithDiagnostics(note)
		if err != nil {
			return conformance.Assessment{}, fmt.Errorf("baseline coverage note: %w", err)
		}
		return noted, nil
	}
	return covered, nil
}

func staleCount(entries []baseline.Entry) int {
	total := 0
	for _, e := range entries {
		total += e.Count()
	}
	return total
}

// requiredFacts unions the fact classes every enabled Rule's
// Enforcement declares, so observation gathers exactly what the check
// needs — no more, no less.
func requiredFacts(rules []rule.Rule) []rule.Fact {
	seen := map[rule.Fact]bool{}
	var out []rule.Fact
	for _, r := range rules {
		if r.Disabled() {
			continue
		}
		for _, f := range r.Enforcement().Facts() {
			if !seen[f] {
				seen[f] = true
				out = append(out, f)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
