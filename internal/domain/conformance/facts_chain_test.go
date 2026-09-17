package conformance_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// Check every production file, including future helpers regardless of filename.
// Only the Facts implementation and named observation entry points own raw input.
func TestEvaluatorsUsePreparedFacts(t *testing.T) {
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatal("no conformance source files found")
	}
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			for _, problem := range factsBoundaryProblems(name, nil) {
				t.Error(problem)
			}
		})
	}
}

func factsBoundaryProblems(name string, source any) []string {
	switch name {
	case "observation.go", "facts.go", "facts_source.go", "dependency_import.go":
		return nil // These files implement the boundary itself.
	}
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, name, source, 0)
	if err != nil {
		return []string{err.Error()}
	}
	var problems []string
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncDecl:
			if name == "check.go" {
				switch n.Name.Name {
				case "Run", "validRules", "observationDiagnostics":
					return false // Orchestration accepts and validates the collected input.
				}
			}
		case *ast.TypeSpec:
			if name == "check.go" && n.Name.Name == "Request" {
				return false
			}
		case *ast.Ident:
			switch n.Name {
			case "Observations", "NewObservations", "NewInspectionFacts", "NewFacts",
				"membership", "factsSource", "dependencyIndex", "newFactsSource", "newMembership", "newDependencyIndex":
				problems = append(problems, fmt.Sprintf("%s: consumer bypasses prepared Facts through %s", positions.Position(n.Pos()), n.Name))
			}
		case *ast.SelectorExpr:
			switch n.Sel.Name {
			case "source", "index", "observations", "membership":
				problems = append(problems, fmt.Sprintf("%s: consumer accesses backing field %s", positions.Position(n.Pos()), n.Sel.Name))
			}
		}
		return true
	})
	return problems
}

func TestFactsBoundaryCoversNewHelperFiles(t *testing.T) {
	for _, source := range []string{
		"package conformance; func helper(f Facts) { _ = f.source }",
		"package conformance; type leaked = Observations",
		"package conformance; func Run() { _ = newFactsSource }",
	} {
		if len(factsBoundaryProblems("new_helper.go", source)) == 0 {
			t.Errorf("unguarded helper: %s", source)
		}
	}
	if problems := factsBoundaryProblems("new_helper.go", "package conformance; func helper(f Facts) { _ = f.Files() }"); len(problems) != 0 {
		t.Fatalf("ordinary Facts consumer rejected: %v", problems)
	}
}
