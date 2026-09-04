package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/baseline"
)

// BaselineOutput is the port that persists one complete immutable
// Baseline.
type BaselineOutput interface {
	Write(baseline.Snapshot) error
}

// CaptureBaselineResult is the plain result value.
type CaptureBaselineResult struct {
	// Findings is the number of Violations adopted, duplicates counted.
	Findings int
	// Rules is the number of applied Rules the snapshot records.
	Rules int
}

// CaptureBaseline captures a Baseline from one completed assessment
// (evaluated without subtracting any prior Baseline, since the capture
// must see everything it is about to adopt) and persists it through
// the output port.
type CaptureBaseline struct {
	assess AssessConformance
	output BaselineOutput
}

// NewCaptureBaseline requires the assessment use case and the output
// port.
func NewCaptureBaseline(assess AssessConformance, output BaselineOutput) (CaptureBaseline, error) {
	if assess == (AssessConformance{}) {
		return CaptureBaseline{}, fmt.Errorf("capture baseline: missing assess use case")
	}
	if output == nil {
		return CaptureBaseline{}, fmt.Errorf("capture baseline: missing baseline output")
	}
	return CaptureBaseline{assess: assess, output: output}, nil
}

// Execute assesses, captures, and persists.
func (uc CaptureBaseline) Execute() (CaptureBaselineResult, error) {
	assessment, err := uc.assess.Execute(AssessConformanceRequest{SkipBaseline: true})
	if err != nil {
		return CaptureBaselineResult{}, err
	}
	snapshot, err := baseline.Capture(assessment, "")
	if err != nil {
		return CaptureBaselineResult{}, fmt.Errorf("capture baseline: %w", err)
	}
	if err := uc.output.Write(snapshot); err != nil {
		return CaptureBaselineResult{}, fmt.Errorf("write baseline: %w", err)
	}
	findings := 0
	for _, e := range snapshot.Entries() {
		findings += e.Count()
	}
	return CaptureBaselineResult{Findings: findings, Rules: len(snapshot.CapturedRules())}, nil
}
