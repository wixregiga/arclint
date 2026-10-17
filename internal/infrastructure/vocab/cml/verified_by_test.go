package cmlvocab

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// The tests an invariant or assertion names travel as arclint.verified_by
// hint entries, one a test, escaped like every hint value. An edit of
// the tests replaces those entries alone: an authored entry of the hint
// keeps its place, and clearing the tests leaves the hint as it was.
func TestRecordEditsTheVerifyingTestsOfAHint(t *testing.T) {
	const authored = `Domain shop

ContextMap {
  contains pricing
}

BoundedContext pricing {
  domainVisionStatement = "Pricing orders."
  "Takes a discount off an order's total."
  Service Discounter {
    void Discount();
    "A discount never exceeds the order total."
    void assertNeverExceedsTotal() hint = "team=pricing,arclint.on=Discount";
  }
}
`
	dir := t.TempDir()
	r, err := NewRepository(dir, recording(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "domain.cml"), []byte(authored), 0o600); err != nil {
		t.Fatal(err)
	}
	record := func(tests ...string) string {
		t.Helper()
		lang, _, err := r.RecordedLanguage()
		if err != nil {
			t.Fatal(err)
		}
		lang.Contexts[0].Services[0].Assertions[0].VerifiedBy = tests
		edited, err := vocab.NewUbiquitousLanguage(lang.Project, lang.Description, lang.Contexts, lang.Relations)
		if err != nil {
			t.Fatal(err)
		}
		edited.Recording = lang.Recording
		if err := r.Record(edited); err != nil {
			t.Fatal(err)
		}
		again, _, err := r.RecordedLanguage()
		if err != nil {
			t.Fatal(err)
		}
		if d := edited.Difference(again); d != "" {
			t.Fatalf("language changed across Record: %s", d)
		}
		data, err := os.ReadFile(r.Path())
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	text := record("internal/pricing/discounter_test.go:TestNeverExceedsTotal", "web/discounter.test.ts:caps, = 100%")
	want := `hint = "team=pricing,arclint.on=Discount,arclint.verified_by=internal/pricing/discounter_test.go:TestNeverExceedsTotal,arclint.verified_by=web/discounter.test.ts:caps%2C %3D 100%25"`
	if !strings.Contains(text, want) {
		t.Errorf("recorded hint lacks %s:\n%s", want, text)
	}
	text = record("internal/pricing/discounter_test.go:TestRenamed")
	if want := `hint = "team=pricing,arclint.on=Discount,arclint.verified_by=internal/pricing/discounter_test.go:TestRenamed"`; !strings.Contains(text, want) {
		t.Errorf("renamed test: hint lacks %s:\n%s", want, text)
	}
	if text = record(); !strings.Contains(text, `hint = "team=pricing,arclint.on=Discount";`) {
		t.Errorf("cleared tests: the hint is not restored:\n%s", text)
	}
}
