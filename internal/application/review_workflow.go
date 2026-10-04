package application

import (
	"context"
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/workflow"
)

// WorkflowEvaluator proposes semantic findings from the supplied current work.
// Evaluation output remains a candidate until WorkflowReview assesses it.
type WorkflowEvaluator interface {
	EvaluateWorkflow(context.Context, workflow.Evidence) (workflow.Assessment, error)
}

// ReviewWorkflow coordinates evaluation through a port and domain acceptance.
type ReviewWorkflow struct{ evaluator WorkflowEvaluator }

// NewReviewWorkflow requires the semantic evaluator supplied by composition.
func NewReviewWorkflow(evaluator WorkflowEvaluator) (ReviewWorkflow, error) {
	if evaluator == nil {
		return ReviewWorkflow{}, fmt.Errorf("workflow review: missing evaluator")
	}
	return ReviewWorkflow{evaluator: evaluator}, nil
}

// Execute returns grounded feedback or an honest unavailable error.
// An unavailable or incomplete assessment is never represented as approval.
func (review ReviewWorkflow) Execute(ctx context.Context, evidence workflow.Evidence) (workflow.Assessment, error) {
	if err := ctx.Err(); err != nil {
		return workflow.Assessment{}, fmt.Errorf("workflow review unavailable: %w", err)
	}
	if err := (workflow.Review{}).AssertTaskRequired(evidence); err != nil {
		return workflow.Assessment{}, fmt.Errorf("workflow review unavailable: %w", err)
	}
	input := evidence
	input.Passages = make(map[string]string, len(evidence.Passages))
	for name, passage := range evidence.Passages {
		input.Passages[name] = passage
	}
	input.Limits = append([]string{}, evidence.Limits...)
	candidate, err := review.evaluator.EvaluateWorkflow(ctx, input)
	if err != nil {
		return workflow.Assessment{}, fmt.Errorf("workflow review unavailable: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return workflow.Assessment{}, fmt.Errorf("workflow review unavailable: %w", err)
	}
	assessment, err := (workflow.Review{}).Assess(evidence, candidate)
	if err != nil {
		return workflow.Assessment{}, fmt.Errorf("workflow review unavailable: %w", err)
	}
	return assessment, nil
}
