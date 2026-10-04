package workflow

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrTaskRequired identifies a review without its current task.
	ErrTaskRequired = errors.New("workflow review requires the current task")
	// ErrFindingUngrounded identifies feedback unsupported by an exact supplied quote.
	ErrFindingUngrounded = errors.New("workflow finding lacks exact supplied evidence")
	// ErrFindingUnexplained identifies feedback without a stated departure or correction.
	ErrFindingUnexplained = errors.New("workflow finding lacks a departure or correction")
	// ErrCoverageOmitted identifies known limits missing from a returned assessment.
	ErrCoverageOmitted = errors.New("workflow assessment omits supplied coverage limits")
)

// Review accepts evidence-grounded feedback from semantic evaluation.
// It does not infer an approval from an empty finding list or hide known limits.
type Review struct{}

// Assess validates candidate feedback and preserves both sources of coverage limits.
// Semantic evaluation must decide whether a quoted passage demonstrates a departure;
// exact quotation by itself does not establish that judgment.
func (review Review) Assess(evidence Evidence, candidate Assessment) (Assessment, error) {
	if err := review.AssertTaskRequired(evidence); err != nil {
		return Assessment{}, err
	}
	if err := review.AssertFindingsGroundedInEvidence(evidence, candidate); err != nil {
		return Assessment{}, err
	}
	if err := review.AssertFindingsExplainDeparture(candidate); err != nil {
		return Assessment{}, err
	}
	result := Assessment{Findings: append([]Finding{}, candidate.Findings...), Limits: []string{}}
	seen := map[string]bool{}
	for _, limits := range [][]string{evidence.Limits, candidate.Limits} {
		for _, limit := range limits {
			if !seen[limit] {
				result.Limits = append(result.Limits, limit)
				seen[limit] = true
			}
		}
	}
	if err := review.AssertCoverageStated(evidence, candidate, result); err != nil {
		return Assessment{}, err
	}
	return result, nil
}

// AssertTaskRequired rejects a review without its actual current task.
func (Review) AssertTaskRequired(evidence Evidence) error {
	if strings.TrimSpace(evidence.Task) == "" {
		return ErrTaskRequired
	}
	return nil
}

// AssertFindingsGroundedInEvidence rejects invented keys, empty quotes and paraphrases.
func (Review) AssertFindingsGroundedInEvidence(evidence Evidence, candidate Assessment) error {
	for index, finding := range candidate.Findings {
		passage, supplied := evidence.Passages[finding.Evidence]
		if strings.TrimSpace(finding.Evidence) == "" || !supplied || strings.TrimSpace(finding.Quote) == "" || !strings.Contains(passage, finding.Quote) {
			return fmt.Errorf("%w: finding %d cites %q", ErrFindingUngrounded, index+1, finding.Evidence)
		}
	}
	return nil
}

// AssertFindingsExplainDeparture requires the stated departure and useful correction.
// Whether their reasoning is sound is semantic evaluation, not a text-length rule.
func (Review) AssertFindingsExplainDeparture(candidate Assessment) error {
	for index, finding := range candidate.Findings {
		if strings.TrimSpace(finding.Departure) == "" || strings.TrimSpace(finding.Correction) == "" {
			return fmt.Errorf("%w: finding %d", ErrFindingUnexplained, index+1)
		}
	}
	return nil
}

// AssertCoverageStated rejects an assessment that drops any known supplied limit.
func (Review) AssertCoverageStated(evidence Evidence, candidate, assessment Assessment) error {
	supplied := map[string]bool{}
	for _, limit := range assessment.Limits {
		supplied[limit] = true
	}
	for _, limits := range [][]string{evidence.Limits, candidate.Limits} {
		for _, limit := range limits {
			if !supplied[limit] {
				return fmt.Errorf("%w: %q", ErrCoverageOmitted, limit)
			}
		}
	}
	return nil
}
