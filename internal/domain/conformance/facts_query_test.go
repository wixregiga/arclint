package conformance

import (
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

func TestMissingGraphPreparationCannotConform(t *testing.T) {
	scope, err := rule.RepositoryScope()
	if err != nil {
		t.Fatal(err)
	}
	r, err := rule.New(rule.Spec{ID: "graph", Type: rule.TypeAcyclic, Params: rule.AcyclicParams{}, Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := NewFacts(r, nil, Observations{})
	if err != nil {
		t.Fatal(err)
	}
	// A prepared graph with no declared Zones is genuinely empty.
	if edges, err := prepared.zoneEdges(); err != nil || len(edges) != 0 {
		t.Fatalf("observed empty graph: edges=%v, error=%v", edges, err)
	}
	// Simulate a future graph evaluator whose evidence preparation was omitted.
	prepared.graphZones = nil
	if evaluations, err := evaluateGraph(r, prepared); err == nil || len(evaluations) != 0 {
		t.Fatalf("unprepared graph became evaluations %v without an error: %v", evaluations, err)
	}
}

func TestFileFactsDoNotImplicitlySupplyGraphEvidence(t *testing.T) {
	scope, err := rule.RepositoryScope()
	if err != nil {
		t.Fatal(err)
	}
	r, err := rule.New(rule.Spec{ID: "extension", Type: rule.TypeExtension, Params: rule.ExtensionParams{Uses: "probe"}, Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	supplied, err := NewFacts(r, nil, Observations{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := supplied.zoneEdges(); err == nil {
		t.Fatal("import requirements silently granted a Zone graph")
	}
	if _, err := supplied.independentFolders(rule.IndependenceParams{}); err == nil {
		t.Fatal("file facts silently granted sibling Folder evidence")
	}
}
