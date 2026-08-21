package embeddedpattern_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
	embeddedpattern "github.com/wixregiga/arclint/internal/infrastructure/pattern/embedded"
)

func TestVerticalPatternLoads(t *testing.T) {
	source := embeddedpattern.NewSource()
	patterns, err := source.Patterns()
	if err != nil {
		t.Fatalf("Patterns: %v", err)
	}
	if len(patterns) != 1 {
		t.Fatalf("patterns = %d, want 1", len(patterns))
	}
	p := patterns[0]
	if p.Reference().String() != "arclint/vertical@0.1.0" {
		t.Errorf("reference = %q, want arclint/vertical@0.1.0", p.Reference())
	}
	if len(p.Rules()) != 10 {
		t.Errorf("rules = %d, want 10", len(p.Rules()))
	}
	var sawIndependence bool
	for _, r := range p.Rules() {
		if r.ID().Qualified() == "repo:features/independent" {
			sawIndependence = true
			if r.Type() != rule.TypeIndependence {
				t.Errorf("repo:features/independent type = %q, want independence", r.Type())
			}
		}
	}
	if !sawIndependence {
		t.Errorf("missing repo:features/independent")
	}
	text, ok := source.Ruleset("vertical")
	if !ok {
		t.Fatal("Ruleset(vertical) missing")
	}
	if !strings.Contains(text, "runtime: [go]") {
		t.Errorf("Ruleset text lacks runtime: [go] marker")
	}
}
