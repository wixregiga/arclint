package rule_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

func validFiles() []rule.TestFile {
	return []rule.TestFile{{Path: "m/a.go", Content: "package m"}}
}

func TestNewRuleTestConstructionRejections(t *testing.T) {
	files := validFiles()
	expect := []rule.ExpectedFinding{{Kind: rule.FindingViolation, Path: "m/a.go", Line: 1, Message: "broken"}}
	cases := []struct {
		name    string
		build   func() (rule.Test, error)
		wantErr string
	}{
		{"empty name", func() (rule.Test, error) {
			return rule.NewTest("", "t/p:m/r", files, expect)
		}, "missing name"},
		{"empty rule id", func() (rule.Test, error) {
			return rule.NewTest("case", "", files, expect)
		}, "missing rule id"},
		{"no files", func() (rule.Test, error) {
			return rule.NewTest("case", "t/p:m/r", nil, expect)
		}, "no fixture files"},
		{"empty file path", func() (rule.Test, error) {
			return rule.NewTest("case", "t/p:m/r", []rule.TestFile{{Path: "", Content: "x"}}, nil)
		}, "empty path"},
		{"duplicate file path", func() (rule.Test, error) {
			return rule.NewTest("case", "t/p:m/r",
				[]rule.TestFile{{Path: "m/a.go"}, {Path: "m/a.go"}}, nil)
		}, "duplicate fixture path"},
		{"expected missing path", func() (rule.Test, error) {
			return rule.NewTest("case", "t/p:m/r", files,
				[]rule.ExpectedFinding{{Kind: rule.FindingViolation, Message: "broken"}})
		}, "missing path"},
		{"expected missing message", func() (rule.Test, error) {
			return rule.NewTest("case", "t/p:m/r", files,
				[]rule.ExpectedFinding{{Kind: rule.FindingViolation, Path: "m/a.go"}})
		}, "missing message"},
		{"invalid kind", func() (rule.Test, error) {
			return rule.NewTest("case", "t/p:m/r", files,
				[]rule.ExpectedFinding{{Kind: "warning", Path: "m/a.go", Message: "broken"}})
		}, `kind "warning"`},
		{"negative line", func() (rule.Test, error) {
			return rule.NewTest("case", "t/p:m/r", files,
				[]rule.ExpectedFinding{{Path: "m/a.go", Line: -1, Message: "broken"}})
		}, "negative line"},
		{"duplicate expectation", func() (rule.Test, error) {
			return rule.NewTest("case", "t/p:m/r", files,
				[]rule.ExpectedFinding{
					{Kind: rule.FindingViolation, Path: "m/a.go", Line: 1, Message: "broken"},
					{Kind: rule.FindingViolation, Path: "m/a.go", Line: 1, Message: "broken"},
				})
		}, "duplicate expected finding"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.build()
			if err == nil {
				t.Fatalf("construction succeeded, want error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestNewRuleTestDefaultsEmptyKindToViolation(t *testing.T) {
	rt, err := rule.NewTest("case", "t/p:m/r", validFiles(),
		[]rule.ExpectedFinding{{Path: "m/a.go", Line: 2, Message: "broken"}})
	if err != nil {
		t.Fatalf("NewTest: %v", err)
	}
	expected := rt.Expected()
	if len(expected) != 1 || expected[0].Kind != rule.FindingViolation {
		t.Errorf("Expected() = %v, want one entry with kind violation", expected)
	}
	// An empty kind duplicating an explicit violation kind is the same
	// expectation and must be rejected.
	_, err = rule.NewTest("case", "t/p:m/r", validFiles(),
		[]rule.ExpectedFinding{
			{Path: "m/a.go", Line: 2, Message: "broken"},
			{Kind: rule.FindingViolation, Path: "m/a.go", Line: 2, Message: "broken"},
		})
	if err == nil || !strings.Contains(err.Error(), "duplicate expected finding") {
		t.Errorf("kind-defaulted duplicate: err = %v, want duplicate rejection", err)
	}
}

func TestRuleTestAccessors(t *testing.T) {
	files := []rule.TestFile{{Path: "m/a.go", Content: "package m"}}
	rt, err := rule.NewTest("case", "t/p:m/r", files, nil)
	if err != nil {
		t.Fatalf("NewTest: %v", err)
	}
	if rt.Name() != "case" || rt.RuleID() != "t/p:m/r" {
		t.Errorf("identity = %q %q", rt.Name(), rt.RuleID())
	}
	got := rt.Files()
	if len(got) != 1 || got[0] != files[0] {
		t.Errorf("Files() = %v", got)
	}
	got[0].Path = "mutated"
	if rt.Files()[0].Path != "m/a.go" {
		t.Errorf("Files() must return a defensive copy")
	}
}

func compareFixture(t *testing.T, expect []rule.ExpectedFinding) rule.Test {
	t.Helper()
	rt, err := rule.NewTest("case", "t/p:m/r", validFiles(), expect)
	if err != nil {
		t.Fatalf("NewTest: %v", err)
	}
	return rt
}

func TestRuleTestCompareClean(t *testing.T) {
	rt := compareFixture(t, []rule.ExpectedFinding{
		{Kind: rule.FindingViolation, Path: "m/a.go", Line: 3, Message: "broken"},
		{Kind: rule.FindingCoverage, Path: "m/a.go", Line: 0, Message: "not evaluated"},
	})
	c := rt.Compare([]rule.Finding{
		{Kind: rule.FindingCoverage, Path: "m/a.go", Line: 0, Message: "not evaluated"},
		{Kind: rule.FindingViolation, Path: "m/a.go", Line: 3, Message: "broken"},
	})
	if !c.Clean() {
		t.Errorf("Compare = %+v, want clean", c)
	}
}

func TestRuleTestCompareMissing(t *testing.T) {
	rt := compareFixture(t, []rule.ExpectedFinding{
		{Kind: rule.FindingViolation, Path: "m/b.go", Line: 1, Message: "second"},
		{Kind: rule.FindingViolation, Path: "m/a.go", Line: 1, Message: "first"},
	})
	c := rt.Compare(nil)
	if c.Clean() || len(c.Unexpected) != 0 {
		t.Fatalf("Compare = %+v, want two missing and nothing unexpected", c)
	}
	if len(c.Missing) != 2 || c.Missing[0].Path != "m/a.go" || c.Missing[1].Path != "m/b.go" {
		t.Errorf("Missing = %v, want deterministic path order", c.Missing)
	}
}

func TestRuleTestCompareUnexpected(t *testing.T) {
	rt := compareFixture(t, []rule.ExpectedFinding{
		{Kind: rule.FindingViolation, Path: "m/a.go", Line: 3, Message: "broken"},
	})
	c := rt.Compare([]rule.Finding{
		{Kind: rule.FindingViolation, Path: "m/a.go", Line: 3, Message: "broken"},
		// Same anchor and message, different kind: not the expected entry.
		{Kind: rule.FindingSuppressed, Path: "m/a.go", Line: 3, Message: "broken"},
		{Kind: rule.FindingOperational, Path: "m/a.go", Line: 1, Message: "unparsable"},
	})
	if len(c.Missing) != 0 || len(c.Unexpected) != 2 {
		t.Fatalf("Compare = %+v, want zero missing and two unexpected", c)
	}
	if c.Unexpected[0].Line != 1 || c.Unexpected[1].Kind != rule.FindingSuppressed {
		t.Errorf("Unexpected = %v, want line then kind order", c.Unexpected)
	}
}

func TestRuleTestCompareMissingAndUnexpected(t *testing.T) {
	rt := compareFixture(t, []rule.ExpectedFinding{
		{Kind: rule.FindingViolation, Path: "m/a.go", Line: 3, Message: "broken"},
		{Kind: rule.FindingViolation, Path: "m/a.go", Line: 9, Message: "also broken"},
	})
	c := rt.Compare([]rule.Finding{
		{Kind: rule.FindingViolation, Path: "m/a.go", Line: 3, Message: "broken"},
		// A duplicate of an already-matched finding is unexpected: the
		// multiset match consumes each expectation once.
		{Kind: rule.FindingViolation, Path: "m/a.go", Line: 3, Message: "broken"},
	})
	if len(c.Missing) != 1 || c.Missing[0].Line != 9 {
		t.Errorf("Missing = %v, want the line-9 expectation", c.Missing)
	}
	if len(c.Unexpected) != 1 || c.Unexpected[0].Line != 3 {
		t.Errorf("Unexpected = %v, want the duplicated line-3 finding", c.Unexpected)
	}
}

func TestRuleTestCompareEmptyExpectedAssertsConformance(t *testing.T) {
	rt := compareFixture(t, nil)
	if c := rt.Compare(nil); !c.Clean() {
		t.Errorf("Compare(nil) = %+v, want clean", c)
	}
	c := rt.Compare([]rule.Finding{
		{Kind: rule.FindingViolation, Path: "m/a.go", Line: 3, Message: "broken"},
	})
	if len(c.Unexpected) != 1 || len(c.Missing) != 0 || c.Clean() {
		t.Errorf("Compare = %+v, want the one finding unexpected", c)
	}
}
