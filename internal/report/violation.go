// Package report defines the violation model shared by every rule provider
// and the output encoders. The JSON shape of Violation is a stable public
// contract: {ruleId, contract, blame, severity, path, line?, message, fixHint}.
package report

import (
	"encoding/json"
	"sort"
)

// Contract names the clause class a rule belongs to.
type Contract string

const (
	ContractConsumes  Contract = "consumes"
	ContractProvides  Contract = "provides"
	ContractInvariant Contract = "invariant"
)

// Blame assigns fault per design-by-contract: a consumes break is the
// importing side's fault (consumer); a provides break is the fault of the
// module that failed its promise (provider). Invariants blame the module
// that holds the property.
type Blame string

const (
	BlameConsumer Blame = "consumer"
	BlameProvider Blame = "provider"
)

// Severity of a violation. Only SeverityError makes `arclint check` exit 1;
// warnings are printed but do not fail the run.
type Severity string

const (
	SeverityError Severity = "error"
	SeverityWarn  Severity = "warn"
	SeverityInfo  Severity = "info"
)

// Violation is one broken contract clause, anchored to a repo-relative path.
type Violation struct {
	RuleID   string   `json:"ruleId"`
	Contract Contract `json:"contract"`
	Blame    Blame    `json:"blame"`
	Severity Severity `json:"severity"`
	// Capability states how the rule enforces its claim:
	// exact | structural | heuristic | advisory.
	Capability string `json:"capability,omitempty"`
	// Path is repo-root-relative, forward slashes on every platform.
	Path    string `json:"path"`
	Line    *int   `json:"line,omitempty"`
	Message string `json:"message"`
	FixHint string `json:"fixHint"`
	// Suppressed marks a finding dropped by an except clause; such
	// findings appear in output only under --show-suppressed and never
	// affect the exit code.
	Suppressed bool `json:"suppressed,omitempty"`
	// SuppressedReason is the except entry's reason.
	SuppressedReason string `json:"suppressedReason,omitempty"`
	// Baselined marks a finding adopted into .arclint/baseline.json;
	// such findings appear in output only under --show-baselined and
	// never affect the exit code.
	Baselined bool `json:"baselined,omitempty"`
}

// LineValue returns the line number or 0 when the violation is not anchored
// to a line.
func (v Violation) LineValue() int {
	if v.Line == nil {
		return 0
	}
	return *v.Line
}

// Sort orders violations deterministically: path, line, ruleId, message.
func Sort(vs []Violation) {
	sort.Slice(vs, func(i, j int) bool {
		a, b := vs[i], vs[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.LineValue() != b.LineValue() {
			return a.LineValue() < b.LineValue()
		}
		if a.RuleID != b.RuleID {
			return a.RuleID < b.RuleID
		}
		return a.Message < b.Message
	})
}

// MarshalJSONList encodes violations as a JSON array (never null).
func MarshalJSONList(vs []Violation) ([]byte, error) {
	if vs == nil {
		vs = []Violation{}
	}
	return json.MarshalIndent(vs, "", "  ")
}

// IntPtr is a helper for building line-anchored violations.
func IntPtr(n int) *int { return &n }
