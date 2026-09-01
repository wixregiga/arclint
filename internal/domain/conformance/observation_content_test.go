package conformance_test

import (
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

func TestNewMapContentReadsIndependentCopy(t *testing.T) {
	src := map[string]string{"m/a.go": "package m\n"}
	content := conformance.NewMapContent(src)
	src["m/a.go"] = "mutated"
	got, err := content.Read("m/a.go")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != "package m\n" {
		t.Errorf("Read = %q, want the original map value", got)
	}
	if _, err := content.Read("missing.go"); err == nil {
		t.Errorf("missing path must fail")
	}
}

func TestObservationsWithContent(t *testing.T) {
	obs, err := conformance.NewObservations([]conformance.ObservedFile{
		{Path: "m/a.go", Size: 10},
	}, nil)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	if obs.Content() != nil {
		t.Fatalf("Content() = %v, want nil before WithContent", obs.Content())
	}
	with := obs.WithContent(conformance.NewMapContent(map[string]string{
		"m/a.go": "fixture bytes",
	}))
	if obs.Content() != nil {
		t.Errorf("WithContent must not mutate the original Observations")
	}
	got, err := with.Content().Read("m/a.go")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != "fixture bytes" {
		t.Errorf("Read = %q, want fixture bytes", got)
	}
}
