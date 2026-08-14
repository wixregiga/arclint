package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/wixregiga/arclint/internal/report"
)

func bv(rule, path, message string, line int) report.Violation {
	v := report.Violation{RuleID: rule, Path: path, Message: message, Severity: report.SeverityError}
	if line > 0 {
		v.Line = report.IntPtr(line)
	}
	return v
}

// TestBaselineRoundTrip proves the write/load/apply cycle, including
// the count semantics: two identical findings baselined once keep one
// reported.
func TestBaselineRoundTrip(t *testing.T) {
	root := t.TempDir()
	adopted := []report.Violation{
		bv("ddd:ARCH-001", "domain/book.go", "domain imports application", 3),
		bv("ddd:ARCH-001", "domain/book.go", "domain imports application", 9), // identical fingerprint
		bv("deps.acyclic", "app/wire.go", "cycle app -> domain -> app", 0),
	}
	if _, err := WriteBaseline(root, adopted); err != nil {
		t.Fatal(err)
	}
	entries, err := loadBaseline(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries: %d, want 2 fingerprints", len(entries))
	}

	// The same three findings: all covered, nothing kept, nothing stale.
	kept, baselined, stale := applyBaseline(entries, adopted)
	if len(kept) != 0 || len(baselined) != 3 || stale != 0 {
		t.Errorf("full coverage: kept %d baselined %d stale %d", len(kept), len(baselined), stale)
	}
	for _, v := range baselined {
		if !v.Baselined {
			t.Errorf("baselined finding not marked: %+v", v)
		}
	}

	// A THIRD identical occurrence exceeds the adopted count of two and
	// must stay reported.
	extra := append(append([]report.Violation{}, adopted...),
		bv("ddd:ARCH-001", "domain/book.go", "domain imports application", 40))
	kept, baselined, stale = applyBaseline(entries, extra)
	if len(kept) != 1 || len(baselined) != 3 || stale != 0 {
		t.Errorf("count exceeded: kept %d baselined %d stale %d", len(kept), len(baselined), stale)
	}

	// One adopted finding fixed: its entry goes stale, everything else
	// still covered, the new finding still reported.
	fixed := []report.Violation{
		bv("ddd:ARCH-001", "domain/book.go", "domain imports application", 3),
		bv("ddd:ARCH-001", "domain/book.go", "domain imports application", 9),
	}
	kept, baselined, stale = applyBaseline(entries, fixed)
	if len(kept) != 0 || len(baselined) != 2 || stale != 1 {
		t.Errorf("after fix: kept %d baselined %d stale %d", len(kept), len(baselined), stale)
	}
}

// TestBaselineDeterministic: identical findings produce byte-identical
// files, so regeneration diffs only when findings change.
func TestBaselineDeterministic(t *testing.T) {
	vs := []report.Violation{
		bv("b-rule", "b/file.go", "second", 2),
		bv("a-rule", "a/file.go", "first", 1),
	}
	rootA, rootB := t.TempDir(), t.TempDir()
	pathA, err := WriteBaseline(rootA, vs)
	if err != nil {
		t.Fatal(err)
	}
	pathB, err := WriteBaseline(rootB, vs)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(pathA)
	b, _ := os.ReadFile(pathB)
	if !bytes.Equal(a, b) {
		t.Errorf("nondeterministic baseline:\n%s\nvs\n%s", a, b)
	}
	if bytes.Contains(a, []byte("time")) {
		t.Errorf("baseline must not contain timestamps:\n%s", a)
	}
}

func TestBaselineLoadFailures(t *testing.T) {
	if entries, err := loadBaseline(t.TempDir()); entries != nil || err != nil {
		t.Errorf("missing file: entries %v err %v", entries, err)
	}

	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(BaselinePath))
	os.MkdirAll(filepath.Dir(path), 0o755)

	os.WriteFile(path, []byte("{broken"), 0o644)
	if _, err := loadBaseline(root); err == nil {
		t.Error("malformed baseline must be a loud error, not a silent no-baseline")
	}

	os.WriteFile(path, []byte(`{"version": 99, "findings": {}}`), 0o644)
	if _, err := loadBaseline(root); err == nil {
		t.Error("future version must be a loud error")
	}
}
