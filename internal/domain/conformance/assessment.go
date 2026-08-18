package conformance

import (
	"fmt"
	"sort"
)

// Assessment is the immutable complete result of one Conformance
// Check: every Rule Evaluation, every Diagnostic, and the applied Rule
// identities. Unsupported, undetermined, not-applicable, and failed
// evaluations are preserved; passing the gate does not imply every
// Rule conformed. Ordering is deterministic.
type Assessment struct {
	evaluations  []Evaluation
	diagnostics  []Diagnostic // operational and coverage; violations live in evaluations
	appliedRules []string     // qualified Rule identities, sorted unique
}

// NewAssessment validates and orders the complete result.
func NewAssessment(evaluations []Evaluation, diagnostics []Diagnostic, appliedRules []string) (Assessment, error) {
	for _, d := range diagnostics {
		switch d.Kind() {
		case DiagnosticOperational, DiagnosticCoverage:
		case DiagnosticViolation:
			return Assessment{}, fmt.Errorf("assessment: violation diagnostics belong to their evaluations")
		default:
			return Assessment{}, fmt.Errorf("assessment: diagnostic kind %q invalid", d.Kind())
		}
	}
	seen := map[string]bool{}
	applied := make([]string, 0, len(appliedRules))
	for _, id := range appliedRules {
		if id == "" {
			return Assessment{}, fmt.Errorf("assessment: empty applied rule identity")
		}
		if seen[id] {
			return Assessment{}, fmt.Errorf("assessment: duplicate applied rule %q", id)
		}
		seen[id] = true
		applied = append(applied, id)
	}
	sort.Strings(applied)

	evals := append([]Evaluation(nil), evaluations...)
	sort.SliceStable(evals, func(i, j int) bool {
		a, b := evals[i], evals[j]
		if aq, bq := a.Rule().Qualified(), b.Rule().Qualified(); aq != bq {
			return aq < bq
		}
		if a.Subject().Kind() != b.Subject().Kind() {
			return a.Subject().Kind() < b.Subject().Kind()
		}
		return a.Subject().Identity() < b.Subject().Identity()
	})
	diags := append([]Diagnostic(nil), diagnostics...)
	sortDiagnostics(diags)
	return Assessment{evaluations: evals, diagnostics: diags, appliedRules: applied}, nil
}

func sortDiagnostics(ds []Diagnostic) {
	sort.SliceStable(ds, func(i, j int) bool {
		a, b := ds[i], ds[j]
		if a.path != b.path {
			return a.path < b.path
		}
		if a.line != b.line {
			return a.line < b.line
		}
		if a.ruleID != b.ruleID {
			return a.ruleID < b.ruleID
		}
		return a.message < b.message
	})
}

// Evaluations returns every Rule Evaluation in deterministic order.
func (a Assessment) Evaluations() []Evaluation {
	return append([]Evaluation(nil), a.evaluations...)
}

// AppliedRules returns the qualified identities of the Rules that were
// evaluated.
func (a Assessment) AppliedRules() []string {
	return append([]string(nil), a.appliedRules...)
}

// Violations returns every Violation with its status, in deterministic
// reporting order.
func (a Assessment) Violations() []Violation {
	total := 0
	for _, e := range a.evaluations {
		total += len(e.violations)
	}
	out := make([]Violation, 0, total)
	for _, e := range a.evaluations {
		out = append(out, e.violations...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		x, y := out[i], out[j]
		if x.path != y.path {
			return x.path < y.path
		}
		if x.line != y.line {
			return x.line < y.line
		}
		if xq, yq := x.ruleID.Qualified(), y.ruleID.Qualified(); xq != yq {
			return xq < yq
		}
		return x.message < y.message
	})
	return out
}

func (a Assessment) violationsWithStatus(s Status) []Violation {
	var out []Violation
	for _, v := range a.Violations() {
		if v.Status() == s {
			out = append(out, v)
		}
	}
	return out
}

// ActiveViolations returns the Violations that report and gate.
func (a Assessment) ActiveViolations() []Violation { return a.violationsWithStatus(StatusActive) }

// SuppressedViolations returns retained findings whose reporting effect
// a Suppression removed.
func (a Assessment) SuppressedViolations() []Violation {
	return a.violationsWithStatus(StatusSuppressed)
}

// BaselinedViolations returns findings covered by the Baseline.
func (a Assessment) BaselinedViolations() []Violation {
	return a.violationsWithStatus(StatusBaselined)
}

// Diagnostics returns all Diagnostics with their status: Violations
// lifted from evaluations plus operational and coverage notes, in
// deterministic order.
func (a Assessment) Diagnostics() []Diagnostic {
	out := append([]Diagnostic(nil), a.diagnostics...)
	for _, v := range a.Violations() {
		out = append(out, v.Diagnostic())
	}
	sortDiagnostics(out)
	return out
}

// HasErrors decides whether the gate fails: an active error-severity
// Violation, or an error-severity operational Diagnostic (a policy
// like unknown_imports: error).
func (a Assessment) HasErrors() bool {
	for _, v := range a.Violations() {
		if v.FailsGate() {
			return true
		}
	}
	for _, d := range a.diagnostics {
		if d.Kind() == DiagnosticOperational && d.Severity().FailsGate() {
			return true
		}
	}
	return false
}

// RelabelViolations returns a copy whose Violations carry the statuses
// assigned by label; label returns false to leave one unchanged. The
// findings themselves are immutable — only reporting status changes.
func (a Assessment) RelabelViolations(label func(Violation) (Status, string, bool)) (Assessment, error) {
	evals := make([]Evaluation, len(a.evaluations))
	for i, e := range a.evaluations {
		vs := e.Violations()
		changed := false
		for k, v := range vs {
			status, reason, ok := label(v)
			if !ok {
				continue
			}
			relabeled, err := v.WithStatus(status, reason)
			if err != nil {
				return Assessment{}, err
			}
			vs[k] = relabeled
			changed = true
		}
		if changed {
			evals[i] = e.withViolations(vs)
		} else {
			evals[i] = e
		}
	}
	out := a
	out.evaluations = evals
	return out, nil
}

// WithDiagnostics returns a copy carrying additional operational or
// coverage Diagnostics.
func (a Assessment) WithDiagnostics(extra ...Diagnostic) (Assessment, error) {
	for _, d := range extra {
		if d.Kind() != DiagnosticOperational && d.Kind() != DiagnosticCoverage {
			return Assessment{}, fmt.Errorf("assessment: only operational and coverage diagnostics may be added")
		}
	}
	out := a
	out.diagnostics = append(append([]Diagnostic(nil), a.diagnostics...), extra...)
	sortDiagnostics(out.diagnostics)
	return out, nil
}
