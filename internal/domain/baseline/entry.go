package baseline

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// Entry is the stable fingerprint of one Violation captured in a
// Baseline: Rule identity, subject identity, and message identity,
// never source line, so a finding does not reopen when it moves.
// Count carries identical duplicate findings so each baselines
// independently.
type Entry struct {
	ruleID  string
	subject string
	message string
	count   int
}

// NewEntry requires the complete finding identity; a partial or
// ambiguous identity is rejected.
func NewEntry(ruleID, subject, message string, count int) (Entry, error) {
	if ruleID == "" || subject == "" || message == "" {
		return Entry{}, fmt.Errorf("baseline entry: rule, subject, and message identity are all required")
	}
	if count < 1 {
		return Entry{}, fmt.Errorf("baseline entry: count %d invalid", count)
	}
	return Entry{ruleID: ruleID, subject: subject, message: message, count: count}, nil
}

// RuleID returns the qualified identity of the Rule that produced the
// captured finding.
func (e Entry) RuleID() string { return e.ruleID }

// Subject returns the captured Rule Subject identity.
func (e Entry) Subject() string { return e.subject }

// Message returns the stable message identity.
func (e Entry) Message() string { return e.message }

// Count returns how many identical findings were captured.
func (e Entry) Count() int { return e.count }

// Fingerprint returns the stable finding identity shared with
// Violation fingerprints.
func (e Entry) Fingerprint() string {
	return conformance.Fingerprint(e.ruleID, e.subject, e.message)
}

// Matches decides whether a current Violation is the captured finding.
func (e Entry) Matches(v conformance.Violation) bool {
	return e.Fingerprint() == v.Fingerprint()
}

// withCount returns the entry with a different count, for stale
// reporting.
func (e Entry) withCount(n int) Entry {
	e.count = n
	return e
}
