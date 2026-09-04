package conformance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// Status is a Violation's reporting state. A Violation remains a
// Violation when suppressed or covered by a Baseline; only its
// reporting and gate effect change.
type Status string

// The three reporting states.
const (
	StatusActive     Status = "active"
	StatusSuppressed Status = "suppressed"
	StatusBaselined  Status = "baselined"
)

// Valid reports whether the value is a defined enum member.
func (s Status) Valid() bool {
	switch s {
	case StatusActive, StatusSuppressed, StatusBaselined:
		return true
	}
	return false
}

// Violation is a Diagnostic value reporting that a Rule Claim is
// violated, or suspected to be violated, for one Rule Subject. It
// carries Severity independently from Assurance.
type Violation struct {
	ruleID            rule.ID
	subject           rule.Subject
	outcome           Outcome
	severity          rule.Severity
	assurance         rule.Assurance
	evidence          rule.EvidenceMethod
	message           string
	remediation       string
	path              string
	line              int
	status            Status
	suppressionReason string
	provenance        *rule.PatternReference
}

// ViolationSpec is the input to validated Violation construction.
type ViolationSpec struct {
	Rule      rule.ID
	Subject   rule.Subject
	Outcome   Outcome // OutcomeViolates or OutcomeSuspectedViolation
	Severity  rule.Severity
	Assurance rule.Assurance
	Evidence  rule.EvidenceMethod
	Message   string
	// Remediation gives the reader context to fix the cause.
	Remediation string
	// Path anchors reporting; defaults to the Subject identity for file
	// and folder Subjects.
	Path string
	// Line is 0 when the Violation is not line-anchored.
	Line int
	// Provenance names the Pattern that distributed the Rule, when any,
	// so a reader can tell a shared Rule from a local one at a glance.
	Provenance *rule.PatternReference
}

// NewViolation constructs a valid, active Violation or rejects the
// spec.
func NewViolation(spec ViolationSpec) (Violation, error) {
	if spec.Rule.IsZero() {
		return Violation{}, fmt.Errorf("violation: missing rule id")
	}
	fail := func(err error) (Violation, error) {
		return Violation{}, fmt.Errorf("violation of %s: %v", spec.Rule, err)
	}
	if spec.Subject.IsZero() {
		return fail(fmt.Errorf("missing subject"))
	}
	if spec.Outcome != OutcomeViolates && spec.Outcome != OutcomeSuspectedViolation {
		return fail(fmt.Errorf("outcome %q is not a violation outcome", spec.Outcome))
	}
	if !spec.Severity.Valid() {
		return fail(fmt.Errorf("severity %q invalid", spec.Severity))
	}
	if !spec.Assurance.Valid() {
		return fail(fmt.Errorf("assurance %q invalid", spec.Assurance))
	}
	if strings.TrimSpace(string(spec.Evidence)) == "" {
		return fail(fmt.Errorf("missing evidence method"))
	}
	if strings.TrimSpace(spec.Message) == "" {
		return fail(fmt.Errorf("missing message"))
	}
	path := spec.Path
	if path == "" {
		if spec.Subject.Kind() == rule.SubjectModule {
			return fail(fmt.Errorf("module-subject violation requires an anchor path"))
		}
		path = spec.Subject.Identity()
	}
	if spec.Line < 0 {
		return fail(fmt.Errorf("negative line"))
	}
	var provenance *rule.PatternReference
	if spec.Provenance != nil {
		if spec.Provenance.IsZero() {
			return fail(fmt.Errorf("unconstructed provenance"))
		}
		if spec.Rule.Qualifier() != spec.Provenance.Qualifier() {
			return fail(fmt.Errorf("provenance %s does not qualify rule %s", spec.Provenance, spec.Rule))
		}
		ref := *spec.Provenance
		provenance = &ref
	}
	return Violation{
		ruleID:      spec.Rule,
		subject:     spec.Subject,
		outcome:     spec.Outcome,
		severity:    spec.Severity,
		assurance:   spec.Assurance,
		evidence:    spec.Evidence,
		message:     spec.Message,
		remediation: spec.Remediation,
		path:        path,
		line:        spec.Line,
		status:      StatusActive,
		provenance:  provenance,
	}, nil
}

// Rule returns the one Rule this Violation references.
func (v Violation) Rule() rule.ID { return v.ruleID }

// Provenance returns the Pattern that distributed the violated Rule,
// when the Rule came from one; a local Rule reports false.
func (v Violation) Provenance() (rule.PatternReference, bool) {
	if v.provenance == nil {
		return rule.PatternReference{}, false
	}
	return *v.provenance, true
}

// Subject returns the one Rule Subject this Violation references.
func (v Violation) Subject() rule.Subject { return v.subject }

// Outcome is violates or suspected_violation.
func (v Violation) Outcome() Outcome { return v.outcome }

// Severity is the configured gate importance.
func (v Violation) Severity() rule.Severity { return v.severity }

// Assurance is the evidence strength, independent from Severity.
func (v Violation) Assurance() rule.Assurance { return v.assurance }

// Evidence names how the conclusion was reached.
func (v Violation) Evidence() rule.EvidenceMethod { return v.evidence }

// Message states what is broken.
func (v Violation) Message() string { return v.message }

// Remediation gives fixing context, possibly empty.
func (v Violation) Remediation() string { return v.remediation }

// Path is the repo-relative reporting anchor.
func (v Violation) Path() string { return v.path }

// Line is the reporting line, 0 when not line-anchored.
func (v Violation) Line() int { return v.line }

// Status reports active, suppressed, or baselined.
func (v Violation) Status() Status { return v.status }

// SuppressionReason returns the suppressing decision's reason, when
// suppressed.
func (v Violation) SuppressionReason() string { return v.suppressionReason }

// WithStatus returns the Violation with a changed reporting state. The
// finding itself is unchanged: suppression and Baseline coverage never
// turn a Violation into conformance.
func (v Violation) WithStatus(status Status, reason string) (Violation, error) {
	if !status.Valid() {
		return Violation{}, fmt.Errorf("violation status %q invalid", status)
	}
	v.status = status
	if status == StatusSuppressed {
		v.suppressionReason = reason
	} else {
		v.suppressionReason = ""
	}
	return v, nil
}

// FailsGate reports whether this Violation, in its current status,
// fails the gate.
func (v Violation) FailsGate() bool {
	return v.status == StatusActive && v.severity.FailsGate()
}

// Fingerprint produces the stable identity used for suppression and
// Baseline comparison. It is line-independent: a finding does not
// change identity when it moves.
func (v Violation) Fingerprint() string {
	return Fingerprint(v.ruleID.Qualified(), v.subject.Identity(), v.message)
}

// Fingerprint derives the stable finding identity from the Rule ID,
// the Subject identity, and the message. Fields are length-prefixed so
// distinct triples can never collide by concatenation.
func Fingerprint(ruleID, subjectIdentity, message string) string {
	h := sha256.New()
	for _, part := range []string{ruleID, subjectIdentity, message} {
		h.Write([]byte(strconv.Itoa(len(part)) + ":"))
		h.Write([]byte(part))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
