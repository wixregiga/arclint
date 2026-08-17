package rule

import (
	"fmt"
	"sort"
	"strings"
)

// TestFile is one repo-relative file of a deterministic Rule Test
// fixture.
type TestFile struct {
	Path    string
	Content string
}

// FindingKind classifies one entry of a Rule's complete result as a
// Rule Test asserts it: active and suppressed Violations plus
// operational and coverage Diagnostics.
type FindingKind string

// The closed set of finding kinds.
const (
	// FindingViolation is an active Violation.
	FindingViolation FindingKind = "violation"
	// FindingSuppressed is a retained Violation whose reporting effect a
	// Suppression removed.
	FindingSuppressed FindingKind = "suppressed"
	// FindingOperational is an operational Diagnostic.
	FindingOperational FindingKind = "operational"
	// FindingCoverage is a coverage Diagnostic.
	FindingCoverage FindingKind = "coverage"
)

// Valid reports whether the value is a defined enum member.
func (k FindingKind) Valid() bool {
	switch k {
	case FindingViolation, FindingSuppressed, FindingOperational, FindingCoverage:
		return true
	}
	return false
}

// ExpectedFinding is one asserted entry of a Rule Test's complete
// expected result.
type ExpectedFinding struct {
	Kind    FindingKind
	Path    string
	Line    int
	Message string
}

// Finding is one produced entry of a Rule's complete result,
// identified by kind, anchor path, line, and message.
type Finding struct {
	Kind    FindingKind
	Path    string
	Line    int
	Message string
}

// Test is a deterministic fixture scenario and the complete
// expected Diagnostics for one Rule. It identifies its Rule and
// asserts the complete result, never one convenient Diagnostic.
type Test struct {
	name   string
	ruleID string
	files  []TestFile
	expect []ExpectedFinding
}

// NewTest requires a name, the identity of the Rule under test,
// and at least one fixture file. An expected entry with an empty kind
// defaults to violation; an expectation listed twice is a construction
// error, settling the vocabulary's duplicate-Diagnostics concern at
// construction. An empty expected list is meaningful: the scenario
// must produce no Diagnostics at all.
func NewTest(name, ruleID string, files []TestFile, expect []ExpectedFinding) (Test, error) {
	if strings.TrimSpace(name) == "" {
		return Test{}, fmt.Errorf("rule test: missing name")
	}
	if strings.TrimSpace(ruleID) == "" {
		return Test{}, fmt.Errorf("rule test %q: missing rule id", name)
	}
	if len(files) == 0 {
		return Test{}, fmt.Errorf("rule test %q: no fixture files", name)
	}
	seenPaths := map[string]bool{}
	for _, f := range files {
		if f.Path == "" {
			return Test{}, fmt.Errorf("rule test %q: fixture file with empty path", name)
		}
		if seenPaths[f.Path] {
			return Test{}, fmt.Errorf("rule test %q: duplicate fixture path %q", name, f.Path)
		}
		seenPaths[f.Path] = true
	}
	normalized := make([]ExpectedFinding, 0, len(expect))
	seenExpect := map[ExpectedFinding]bool{}
	for i, e := range expect {
		if e.Kind == "" {
			e.Kind = FindingViolation
		}
		if !e.Kind.Valid() {
			return Test{}, fmt.Errorf("rule test %q: expect[%d]: kind %q is not one of violation, suppressed, operational, coverage", name, i, e.Kind)
		}
		if e.Path == "" {
			return Test{}, fmt.Errorf("rule test %q: expect[%d]: missing path", name, i)
		}
		if strings.TrimSpace(e.Message) == "" {
			return Test{}, fmt.Errorf("rule test %q: expect[%d]: missing message", name, i)
		}
		if e.Line < 0 {
			return Test{}, fmt.Errorf("rule test %q: expect[%d]: negative line", name, i)
		}
		if seenExpect[e] {
			return Test{}, fmt.Errorf("rule test %q: expect[%d]: duplicate expected finding (%s %s:%d %q)", name, i, e.Kind, e.Path, e.Line, e.Message)
		}
		seenExpect[e] = true
		normalized = append(normalized, e)
	}
	return Test{
		name:   name,
		ruleID: ruleID,
		files:  append([]TestFile(nil), files...),
		expect: normalized,
	}, nil
}

// Name identifies the scenario.
func (t Test) Name() string { return t.name }

// RuleID identifies the one Rule this test exercises.
func (t Test) RuleID() string { return t.ruleID }

// Files returns the fixture files.
func (t Test) Files() []TestFile { return append([]TestFile(nil), t.files...) }

// Expected returns the complete expected result, kinds normalized.
func (t Test) Expected() []ExpectedFinding {
	return append([]ExpectedFinding(nil), t.expect...)
}

// IsZero reports an unconstructed Test.
func (t Test) IsZero() bool { return t.name == "" }

// Comparison is the complete outcome of comparing a produced result
// against the expected one: expected entries that never occurred and
// produced entries the test did not expect.
type Comparison struct {
	Missing    []ExpectedFinding
	Unexpected []Finding
}

// Clean reports whether the produced result matched the expected one
// completely.
func (c Comparison) Clean() bool { return len(c.Missing) == 0 && len(c.Unexpected) == 0 }

// findingKey is the exact-equality identity findings are matched on.
type findingKey struct {
	kind    FindingKind
	path    string
	line    int
	message string
}

// Compare detects missing and unexpected Diagnostics by multiset
// comparison on exact (kind, path, line, message) equality; duplicate
// expectations are already unconstructible. With an empty expected
// list the test asserts complete conformance: every produced finding
// is unexpected. Output order is deterministic: path, line, message,
// then kind.
func (t Test) Compare(actual []Finding) Comparison {
	remaining := map[findingKey]int{}
	for _, e := range t.expect {
		remaining[findingKey{e.Kind, e.Path, e.Line, e.Message}]++
	}
	var unexpected []Finding
	for _, f := range actual {
		k := findingKey{f.Kind, f.Path, f.Line, f.Message}
		if remaining[k] > 0 {
			remaining[k]--
			continue
		}
		unexpected = append(unexpected, f)
	}
	var missing []ExpectedFinding
	for _, e := range t.expect {
		k := findingKey{e.Kind, e.Path, e.Line, e.Message}
		if remaining[k] > 0 {
			remaining[k]--
			missing = append(missing, e)
		}
	}
	sort.SliceStable(missing, func(i, j int) bool {
		return lessFinding(
			findingKey{missing[i].Kind, missing[i].Path, missing[i].Line, missing[i].Message},
			findingKey{missing[j].Kind, missing[j].Path, missing[j].Line, missing[j].Message})
	})
	sort.SliceStable(unexpected, func(i, j int) bool {
		return lessFinding(
			findingKey{unexpected[i].Kind, unexpected[i].Path, unexpected[i].Line, unexpected[i].Message},
			findingKey{unexpected[j].Kind, unexpected[j].Path, unexpected[j].Line, unexpected[j].Message})
	})
	return Comparison{Missing: missing, Unexpected: unexpected}
}

// lessFinding orders findings by path, line, message, then kind.
func lessFinding(a, b findingKey) bool {
	if a.path != b.path {
		return a.path < b.path
	}
	if a.line != b.line {
		return a.line < b.line
	}
	if a.message != b.message {
		return a.message < b.message
	}
	return a.kind < b.kind
}
