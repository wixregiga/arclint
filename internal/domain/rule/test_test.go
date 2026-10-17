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

func TestRuleTestFailingCaseExpectsAViolation(t *testing.T) {
	violation := []rule.ExpectedFinding{{Kind: rule.FindingViolation, Path: "m/a.go", Line: 1, Message: "m"}}
	suppressed := []rule.ExpectedFinding{{Kind: rule.FindingSuppressed, Path: "m/a.go", Line: 1, Message: "m"}}
	for name, tc := range map[string]struct {
		expect []rule.ExpectedFinding
		want   bool
	}{
		"violation":  {violation, true},
		"suppressed": {suppressed, false},
		"clean":      {nil, false},
	} {
		test, err := rule.NewTest(name, "m/r", validFiles(), tc.expect)
		if err != nil {
			t.Fatalf("%s: NewTest: %v", name, err)
		}
		if got := test.FailingCase(); got != tc.want {
			t.Errorf("%s: FailingCase() = %v, want %v", name, got, tc.want)
		}
	}
}

func TestWithoutFailingCaseNamesEnabledContentRulesOnly(t *testing.T) {
	content := func(id string) rule.Rule {
		r, err := rule.New(rule.Spec{
			ID: id, Type: rule.TypeContent,
			Params: rule.ContentParams{Forbid: `\bpanic\(`},
			Scope:  mustRepoScope(t),
		})
		if err != nil {
			t.Fatalf("New %s: %v", id, err)
		}
		return r
	}
	structure, err := rule.New(rule.Spec{
		ID: "m/structure", Type: rule.TypeStructure,
		Params: rule.StructureParams{Require: []rule.Glob{mustGlob(t, "m/a.go")}},
		Scope:  mustZoneScope(t, "m"),
	})
	if err != nil {
		t.Fatalf("New structure: %v", err)
	}
	disablement, err := rule.NewDisablement("retired here")
	if err != nil {
		t.Fatalf("NewDisablement: %v", err)
	}
	test := func(name, ruleID string, expect []rule.ExpectedFinding) rule.Test {
		tc, err := rule.NewTest(name, ruleID, validFiles(), expect)
		if err != nil {
			t.Fatalf("NewTest %s: %v", name, err)
		}
		return tc
	}
	violation := []rule.ExpectedFinding{{Kind: rule.FindingViolation, Path: "m/a.go", Line: 1, Message: "m"}}
	rules := []rule.Rule{
		content("m/exemplified"),
		content("m/clean-only"),
		content("m/untested"),
		content("m/disabled").Disable(disablement),
		structure,
	}
	tests := []rule.Test{
		test("exemplified-fails", "m/exemplified", violation),
		test("clean-only-clean", "m/clean-only", nil),
	}
	got := rule.WithoutFailingCase(rules, tests)
	var ids []string
	for _, id := range got {
		ids = append(ids, id.Qualified())
	}
	if strings.Join(ids, ",") != "m/clean-only,m/untested" {
		t.Fatalf("WithoutFailingCase = %v, want [m/clean-only m/untested]", ids)
	}
}

func TestContentEnforcementIsExactForItsPatternOnly(t *testing.T) {
	content := rule.BuiltinEnforcement(rule.TypeContent)
	if !content.PatternBound() || content.Assurance() != rule.AssuranceExact {
		t.Fatalf("content enforcement: pattern bound %v, assurance %q", content.PatternBound(), content.Assurance())
	}
	if got := content.AssuranceLabel(); got != "exact for its pattern; rationale not checked" {
		t.Errorf("content AssuranceLabel() = %q", got)
	}
	for _, typ := range []rule.Type{rule.TypeConsumes, rule.TypeStructure, rule.TypeNaming, rule.TypeExtension} {
		e := rule.BuiltinEnforcement(typ)
		if e.PatternBound() || e.AssuranceLabel() != string(e.Assurance()) {
			t.Errorf("%s: pattern bound %v, label %q", typ, e.PatternBound(), e.AssuranceLabel())
		}
	}
}
