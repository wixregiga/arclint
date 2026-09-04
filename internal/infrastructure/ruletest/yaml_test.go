package ruletest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/infrastructure/ruletest"
)

// writeTest writes one file under root/.arclint/tests.
func writeTest(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, ".arclint", "tests")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestSourceMissingDirectoryYieldsZeroTests(t *testing.T) {
	tests, err := ruletest.NewSource(t.TempDir()).Tests()
	if err != nil {
		t.Fatalf("Tests: %v", err)
	}
	if len(tests) != 0 {
		t.Errorf("tests = %d, want zero for a missing directory", len(tests))
	}
}

func TestSourceRejectsUnknownKey(t *testing.T) {
	root := t.TempDir()
	writeTest(t, root, "bad.yaml", "rule: \"t/p:m/r\"\nfiles:\n  m/a.go: \"package m\"\nbogus: 1\n")
	_, err := ruletest.NewSource(root).Tests()
	if err == nil || !strings.Contains(err.Error(), "bogus") {
		t.Errorf("unknown key: err = %v, want a loud rejection naming the key", err)
	}
}

func TestSourceRejectsMissingRule(t *testing.T) {
	root := t.TempDir()
	writeTest(t, root, "bad.yaml", "files:\n  m/a.go: \"package m\"\n")
	_, err := ruletest.NewSource(root).Tests()
	if err == nil || !strings.Contains(err.Error(), "missing rule") {
		t.Errorf("missing rule: err = %v, want a loud rejection", err)
	}
}

func TestSourceRejectsMissingFiles(t *testing.T) {
	root := t.TempDir()
	writeTest(t, root, "bad.yaml", "rule: \"t/p:m/r\"\n")
	_, err := ruletest.NewSource(root).Tests()
	if err == nil || !strings.Contains(err.Error(), "missing files") {
		t.Errorf("missing files: err = %v, want a loud rejection", err)
	}
}

func TestSourceRejectsInvalidExpectKind(t *testing.T) {
	root := t.TempDir()
	writeTest(t, root, "bad.yaml",
		"rule: \"t/p:m/r\"\nfiles:\n  m/a.go: \"package m\"\nexpect:\n  - kind: warning\n    path: m/a.go\n    message: broken\n")
	_, err := ruletest.NewSource(root).Tests()
	if err == nil || !strings.Contains(err.Error(), `kind "warning"`) ||
		!strings.Contains(err.Error(), "bad.yaml") {
		t.Errorf("invalid kind: err = %v, want the domain rejection wrapped with the file path", err)
	}
}

func TestSourceRejectsEmptyFile(t *testing.T) {
	root := t.TempDir()
	writeTest(t, root, "empty.yaml", "")
	_, err := ruletest.NewSource(root).Tests()
	if err == nil || !strings.Contains(err.Error(), "empty rule test file") {
		t.Errorf("empty file: err = %v, want a loud rejection", err)
	}
}

func TestSourceLoadsTestsInNameOrder(t *testing.T) {
	root := t.TempDir()
	writeTest(t, root, "zz_second.yaml",
		"rule: \"t/p:m/r\"\nfiles:\n  m/b.go: \"package m\"\n  m/a.go: \"package m\"\nexpect:\n  - path: m/a.go\n    line: 2\n    message: broken\n")
	writeTest(t, root, "aa_first.yaml",
		"rule: \"t/p:m/r\"\nfiles:\n  m/a.go: \"package m\"\n")
	writeTest(t, root, "ignored.txt", "not a rule test")
	tests, err := ruletest.NewSource(root).Tests()
	if err != nil {
		t.Fatalf("Tests: %v", err)
	}
	if len(tests) != 2 || tests[0].Name() != "aa_first" || tests[1].Name() != "zz_second" {
		t.Fatalf("tests = %v, want [aa_first zz_second] from file stems", tests)
	}
	second := tests[1]
	if second.RuleID() != "t/p:m/r" {
		t.Errorf("RuleID = %q", second.RuleID())
	}
	files := second.Files()
	if len(files) != 2 || files[0].Path != "m/a.go" || files[1].Path != "m/b.go" {
		t.Errorf("Files = %v, want deterministic path order", files)
	}
	expect := second.Expected()
	if len(expect) != 1 {
		t.Fatalf("Expected = %v, want one entry", expect)
	}
	want := rule.ExpectedFinding{Kind: rule.FindingViolation, Path: "m/a.go", Line: 2, Message: "broken"}
	if expect[0] != want {
		t.Errorf("Expected[0] = %+v, want %+v with kind defaulted to violation", expect[0], want)
	}
}
