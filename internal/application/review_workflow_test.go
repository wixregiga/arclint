package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/workflow"
)

type workflowEvaluatorProbe struct {
	candidate workflow.Assessment
	err       error
	calls     int
	evaluate  func(context.Context, workflow.Evidence) (workflow.Assessment, error)
}

func (probe *workflowEvaluatorProbe) EvaluateWorkflow(ctx context.Context, evidence workflow.Evidence) (workflow.Assessment, error) {
	probe.calls++
	if probe.evaluate != nil {
		return probe.evaluate(ctx, evidence)
	}
	return probe.candidate, probe.err
}

func TestReviewWorkflowNeedsEvaluator(t *testing.T) {
	if _, err := NewReviewWorkflow(nil); err == nil {
		t.Fatal("missing evaluator accepted")
	}
}

func TestReviewWorkflowAcceptsGroundedFeedbackAndPreservesLimits(t *testing.T) {
	evidence := workflow.Evidence{Task: "Keep this work within its established domain.", Passages: map[string]string{"proposal": "add a second Scope without explaining its meaning"}, Limits: []string{"Earlier history is unavailable."}}
	probe := &workflowEvaluatorProbe{candidate: workflow.Assessment{Findings: []workflow.Finding{{Evidence: "proposal", Quote: "second Scope", Departure: "The proposed new concept has no explained relationship to existing Scope.", Correction: "Explain whether existing Scope suffices before implementing another concept."}}, Limits: []string{"No behavior checks supplied."}}}
	review, err := NewReviewWorkflow(probe)
	if err != nil {
		t.Fatal(err)
	}
	result, err := review.Execute(context.Background(), evidence)
	if err != nil {
		t.Fatal(err)
	}
	if probe.calls != 1 || len(result.Findings) != 1 || len(result.Limits) != 2 {
		t.Fatalf("feedback=%+v calls=%d", result, probe.calls)
	}
}

func TestReviewWorkflowUnavailableNeverBecomesEmptySuccess(t *testing.T) {
	providerFailure := errors.New("semantic evaluator unavailable")
	for _, tc := range []struct {
		name  string
		probe workflowEvaluatorProbe
		want  error
	}{
		{"provider failure", workflowEvaluatorProbe{err: providerFailure}, providerFailure},
		{"invented finding", workflowEvaluatorProbe{candidate: workflow.Assessment{Findings: []workflow.Finding{{Evidence: "missing", Quote: "invented", Departure: "A departure", Correction: "A correction"}}}}, workflow.ErrFindingUngrounded},
		{"unexplained finding", workflowEvaluatorProbe{candidate: workflow.Assessment{Findings: []workflow.Finding{{Evidence: "supplied", Quote: "actual text", Departure: "A departure"}}}}, workflow.ErrFindingUnexplained},
	} {
		t.Run(tc.name, func(t *testing.T) {
			review, err := NewReviewWorkflow(&tc.probe)
			if err != nil {
				t.Fatal(err)
			}
			result, err := review.Execute(context.Background(), workflow.Evidence{Task: "Review the current work.", Passages: map[string]string{"supplied": "actual text"}})
			if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), "unavailable") {
				t.Fatalf("expected honest unavailable cause: %v", err)
			}
			if len(result.Findings) != 0 || len(result.Limits) != 0 {
				t.Fatal("unavailable review manufactured feedback")
			}
		})
	}
}

func TestReviewWorkflowDoesNotInvokeProviderWithoutTaskOrAfterCancellation(t *testing.T) {
	probe := &workflowEvaluatorProbe{}
	review, err := NewReviewWorkflow(probe)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := review.Execute(context.Background(), workflow.Evidence{}); !errors.Is(err, workflow.ErrTaskRequired) {
		t.Fatalf("missing task: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := review.Execute(ctx, workflow.Evidence{Task: "Review work."}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	if probe.calls != 0 {
		t.Fatal("invalid review invoked provider")
	}
}

func TestReviewWorkflowProtectsSuppliedEvidenceFromProviderMutation(t *testing.T) {
	evidence := workflow.Evidence{Task: "Review actual supplied work.", Passages: map[string]string{"source": "actual text"}, Limits: []string{"No history supplied."}}
	probe := &workflowEvaluatorProbe{evaluate: func(_ context.Context, input workflow.Evidence) (workflow.Assessment, error) {
		input.Passages["source"] = "fabricated text"
		input.Limits[0] = "suppressed history limit"
		return workflow.Assessment{Findings: []workflow.Finding{{Evidence: "source", Quote: "fabricated text", Departure: "An alleged departure", Correction: "An alleged correction"}}}, nil
	}}
	review, err := NewReviewWorkflow(probe)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := review.Execute(context.Background(), evidence); !errors.Is(err, workflow.ErrFindingUngrounded) {
		t.Fatalf("provider mutation supplied its own grounding: %v", err)
	}
	if evidence.Passages["source"] != "actual text" || evidence.Limits[0] != "No history supplied." {
		t.Fatal("provider altered caller evidence")
	}
}

func TestReviewWorkflowDiscardsResultWhenProviderOutlivesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	probe := &workflowEvaluatorProbe{evaluate: func(context.Context, workflow.Evidence) (workflow.Assessment, error) {
		cancel()
		return workflow.Assessment{Limits: []string{"Provider returned after cancellation."}}, nil
	}}
	review, err := NewReviewWorkflow(probe)
	if err != nil {
		t.Fatal(err)
	}
	result, err := review.Execute(ctx, workflow.Evidence{Task: "Review current work."})
	if !errors.Is(err, context.Canceled) || len(result.Limits) != 0 {
		t.Fatalf("cancelled provider result accepted: %+v %v", result, err)
	}
}
