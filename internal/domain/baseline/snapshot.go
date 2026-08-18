// Package baseline holds the Baseline: an immutable point-in-time
// record of the Rules applied and the Violations observed during one
// Conformance Check, used to distinguish existing findings from later
// ones. A Baseline never adds later Violations automatically and never
// changes Rule Applicability or architectural truth.
package baseline

import (
	"fmt"
	"sort"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// Snapshot is one captured Baseline.
type Snapshot struct {
	identity      string
	capturedRules []string // qualified Rule identities, sorted
	entries       map[string]Entry
}

// New reconstructs a Baseline from persisted values, rejecting
// ambiguous identities: two entries with the same fingerprint would
// make coverage undecidable.
func New(capturedRules []string, entries []Entry, identity string) (Snapshot, error) {
	rules := append([]string(nil), capturedRules...)
	for _, id := range rules {
		if id == "" {
			return Snapshot{}, fmt.Errorf("baseline: empty captured rule identity")
		}
	}
	sort.Strings(rules)
	byFingerprint := make(map[string]Entry, len(entries))
	for _, e := range entries {
		if e.count < 1 {
			return Snapshot{}, fmt.Errorf("baseline: unconstructed entry")
		}
		fp := e.Fingerprint()
		if _, ok := byFingerprint[fp]; ok {
			return Snapshot{}, fmt.Errorf("baseline: ambiguous finding identity for rule %s at %s", e.ruleID, e.subject)
		}
		byFingerprint[fp] = e
	}
	return Snapshot{identity: identity, capturedRules: rules, entries: byFingerprint}, nil
}

// Capture records the applied Rule identities and the active
// Violations of one completed Conformance Assessment together.
func Capture(a conformance.Assessment, identity string) (Snapshot, error) {
	counts := map[string]int{}
	specs := map[string]conformance.Violation{}
	for _, v := range a.ActiveViolations() {
		fp := v.Fingerprint()
		counts[fp]++
		specs[fp] = v
	}
	entries := make([]Entry, 0, len(counts))
	for fp, n := range counts {
		v := specs[fp]
		e, err := NewEntry(v.Rule().Qualified(), v.Subject().Identity(), v.Message(), n)
		if err != nil {
			return Snapshot{}, err
		}
		entries = append(entries, e)
	}
	return New(a.AppliedRules(), entries, identity)
}

// Identity returns the capture identity, possibly empty when the owner
// prefers fully deterministic snapshots.
func (s Snapshot) Identity() string { return s.identity }

// CapturedRules returns which Rules produced the captured findings.
func (s Snapshot) CapturedRules() []string {
	return append([]string(nil), s.capturedRules...)
}

// Entries returns the captured findings in deterministic order.
func (s Snapshot) Entries() []Entry {
	out := make([]Entry, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.ruleID != b.ruleID {
			return a.ruleID < b.ruleID
		}
		if a.subject != b.subject {
			return a.subject < b.subject
		}
		return a.message < b.message
	})
	return out
}

// Covers decides whether a current Violation was present in this
// snapshot.
func (s Snapshot) Covers(v conformance.Violation) bool {
	_, ok := s.entries[v.Fingerprint()]
	return ok
}

// Apply relabels the assessment's active Violations covered by this
// snapshot as baselined, consuming each entry's count so extra
// occurrences of an identical finding stay reported. It returns the
// new Assessment and the stale entries: captured findings absent from
// this assessment, the signal to refresh the Baseline.
func (s Snapshot) Apply(a conformance.Assessment) (conformance.Assessment, []Entry, error) {
	remaining := make(map[string]int, len(s.entries))
	for fp, e := range s.entries {
		remaining[fp] = e.count
	}
	relabeled, err := a.RelabelViolations(func(v conformance.Violation) (conformance.Status, string, bool) {
		if v.Status() != conformance.StatusActive {
			return "", "", false
		}
		fp := v.Fingerprint()
		if remaining[fp] <= 0 {
			return "", "", false
		}
		remaining[fp]--
		return conformance.StatusBaselined, "", true
	})
	if err != nil {
		return conformance.Assessment{}, nil, fmt.Errorf("apply baseline: %w", err)
	}
	var stale []Entry
	for fp, n := range remaining {
		if n > 0 {
			stale = append(stale, s.entries[fp].withCount(n))
		}
	}
	sort.Slice(stale, func(i, j int) bool {
		a, b := stale[i], stale[j]
		if a.ruleID != b.ruleID {
			return a.ruleID < b.ruleID
		}
		if a.subject != b.subject {
			return a.subject < b.subject
		}
		return a.message < b.message
	})
	return relabeled, stale, nil
}

// StaleEntries returns captured findings absent from a later
// assessment.
func (s Snapshot) StaleEntries(a conformance.Assessment) ([]Entry, error) {
	_, stale, err := s.Apply(a)
	return stale, err
}
