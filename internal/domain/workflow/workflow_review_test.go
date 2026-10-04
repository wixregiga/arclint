package workflow

import (
	"errors"
	"reflect"
	"testing"
)

func TestAssessRejectsUnsupportedOrUnexplainedFindings(t *testing.T) {
	evidence := Evidence{Task: "Keep the reservation seat limit enforced through every caller.", Passages: map[string]string{"src/reservation.ts": "addSeats(n) { this.seats += n; }", "": "this.seats += n"}}
	grounded := Finding{Evidence: "src/reservation.ts", Quote: "this.seats += n", Departure: "The reservation mutation accepts excessive additions; the caller check can be bypassed.", Correction: "Reject additions above the recorded limit in this mutation before changing seats."}
	for _, tc := range []struct {
		name   string
		change func(*Finding)
		want   error
	}{
		{"unnamed supplied passage", func(f *Finding) { f.Evidence = "" }, ErrFindingUngrounded},
		{"invented file", func(f *Finding) { f.Evidence = "not-supplied.ts" }, ErrFindingUngrounded},
		{"invented quote", func(f *Finding) { f.Quote = "if (seats > limit)" }, ErrFindingUngrounded},
		{"paraphrase", func(f *Finding) { f.Quote = "increment seat count" }, ErrFindingUngrounded},
		{"empty quote", func(f *Finding) { f.Quote = "" }, ErrFindingUngrounded},
		{"whitespace quote", func(f *Finding) { f.Quote = " " }, ErrFindingUngrounded},
		{"missing departure", func(f *Finding) { f.Departure = " \n" }, ErrFindingUnexplained},
		{"missing correction", func(f *Finding) { f.Correction = "" }, ErrFindingUnexplained},
	} {
		t.Run(tc.name, func(t *testing.T) {
			finding := grounded
			tc.change(&finding)
			result, err := (Review{}).Assess(evidence, Assessment{Findings: []Finding{finding}})
			if !errors.Is(err, tc.want) {
				t.Fatalf("error=%v want %v", err, tc.want)
			}
			if len(result.Findings) != 0 || len(result.Limits) != 0 {
				t.Fatal("invalid candidate returned as an assessment")
			}
		})
	}
}

func TestAssessRequiresTaskEvenWithNoFindings(t *testing.T) {
	for _, task := range []string{"", " \n\t"} {
		_, err := (Review{}).Assess(Evidence{Task: task}, Assessment{})
		if !errors.Is(err, ErrTaskRequired) {
			t.Fatalf("no-task assessment accepted: %v", err)
		}
	}
}

func TestAssessPreservesKnownLimitsWithAndWithoutFindings(t *testing.T) {
	evidence := Evidence{Task: "Review the current checkout change.", Passages: map[string]string{"change": "new Scope recorded after implementation"}, Limits: []string{"Earlier discussion is unavailable.", "Build results were not supplied."}}
	candidate := Assessment{Findings: []Finding{{Evidence: "change", Quote: "recorded after implementation", Departure: "Changed meaning was used before it was recorded.", Correction: "Explain and record the meaning before further implementation."}}, Limits: []string{"Build results were not supplied.", "Caller behavior is not supplied."}}
	expected := []string{"Earlier discussion is unavailable.", "Build results were not supplied.", "Caller behavior is not supplied."}
	for _, findings := range [][]Finding{candidate.Findings, nil} {
		candidate.Findings = findings
		result, err := (Review{}).Assess(evidence, candidate)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(result.Limits, expected) {
			t.Fatalf("limits=%v want %v", result.Limits, expected)
		}
		if len(result.Findings) != len(findings) {
			t.Fatal("findings invented or lost")
		}
	}
}

func TestCoverageAssertionRejectsSuppressedKnownLimits(t *testing.T) {
	review := Review{}
	evidence := Evidence{Limits: []string{"Affected source could not be read."}}
	candidate := Assessment{Limits: []string{"Tests were not supplied."}}
	for _, limits := range [][]string{nil, evidence.Limits, candidate.Limits} {
		err := review.AssertCoverageStated(evidence, candidate, Assessment{Limits: limits})
		if !errors.Is(err, ErrCoverageOmitted) {
			t.Fatalf("suppressed limit accepted: %v", err)
		}
	}
}

func TestAssessDoesNotShareReturnedStorageWithCandidate(t *testing.T) {
	evidence := Evidence{Task: "Review supplied work.", Passages: map[string]string{"task": "Review supplied work."}, Limits: []string{"No source supplied."}}
	candidate := Assessment{Findings: []Finding{{Evidence: "task", Quote: "Review supplied work.", Departure: "A changed definition has not been explained.", Correction: "Explain the proposed definition before implementing it."}}, Limits: []string{"No checks supplied."}}
	result, err := (Review{}).Assess(evidence, candidate)
	if err != nil {
		t.Fatal(err)
	}
	candidate.Findings[0].Correction = "changed afterward"
	candidate.Limits[0] = "changed afterward"
	evidence.Limits[0] = "changed afterward"
	if result.Findings[0].Correction == "changed afterward" || result.Limits[0] == "changed afterward" || result.Limits[1] == "changed afterward" {
		t.Fatal("caller mutation changed accepted assessment")
	}
}
