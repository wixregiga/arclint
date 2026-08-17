package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/baseline"
)

// RefreshBaselineResult is the plain result value.
type RefreshBaselineResult struct {
	// Findings and Rules describe the replacement snapshot.
	Findings int
	Rules    int
	// RemovedStale counts adopted findings that no longer occur and
	// were dropped by the refresh.
	RemovedStale int
}

// RefreshBaseline replaces the committed Baseline after explicit
// comparison with a later assessment: stale entries are dropped,
// current findings are re-adopted, and refusing to refresh a Baseline
// that does not exist keeps capture an explicit first decision.
type RefreshBaseline struct {
	assess    AssessConformance
	baselines BaselineSource
	output    BaselineOutput
}

// NewRefreshBaseline requires the assessment use case and both
// Baseline ports.
func NewRefreshBaseline(assess AssessConformance, baselines BaselineSource,
	output BaselineOutput,
) (RefreshBaseline, error) {
	if assess == (AssessConformance{}) {
		return RefreshBaseline{}, fmt.Errorf("refresh baseline: missing assess use case")
	}
	if baselines == nil {
		return RefreshBaseline{}, fmt.Errorf("refresh baseline: missing baseline source")
	}
	if output == nil {
		return RefreshBaseline{}, fmt.Errorf("refresh baseline: missing baseline output")
	}
	return RefreshBaseline{assess: assess, baselines: baselines, output: output}, nil
}

// Execute compares, replaces, and persists.
func (uc RefreshBaseline) Execute() (RefreshBaselineResult, error) {
	existing, present, err := uc.baselines.Load()
	if err != nil {
		return RefreshBaselineResult{}, fmt.Errorf("load baseline: %w", err)
	}
	if !present {
		return RefreshBaselineResult{}, fmt.Errorf("refresh baseline: no committed baseline exists; capture one first")
	}
	assessment, err := uc.assess.Execute(AssessConformanceRequest{SkipBaseline: true})
	if err != nil {
		return RefreshBaselineResult{}, err
	}
	stale, err := existing.StaleEntries(assessment)
	if err != nil {
		return RefreshBaselineResult{}, fmt.Errorf("compare baseline: %w", err)
	}
	snapshot, err := baseline.Capture(assessment, existing.Identity())
	if err != nil {
		return RefreshBaselineResult{}, fmt.Errorf("capture baseline: %w", err)
	}
	if err := uc.output.Write(snapshot); err != nil {
		return RefreshBaselineResult{}, fmt.Errorf("write baseline: %w", err)
	}
	result := RefreshBaselineResult{Rules: len(snapshot.CapturedRules())}
	for _, e := range snapshot.Entries() {
		result.Findings += e.Count()
	}
	for _, e := range stale {
		result.RemovedStale += e.Count()
	}
	return result, nil
}
