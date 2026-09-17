package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
)

func TestContextDependenciesEndToEnd(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", "runtime: [go]\nscan:\n  exclude: [\"generated/**\"]\n")
	write(t, root, "go.mod", "module example.test/project\n\ngo 1.23\n")
	write(t, root, "main.go", "package main\nimport _ \"example.test/project/generated\"\nfunc main() {}\n")
	write(t, root, "empty.go", "package main\n")
	write(t, root, "broken.go", "package main\nimport (\n")
	write(t, root, "generated/value.go", "package generated\n")
	stdout, stderr, code := runBin(t, root, os.Environ(), "context", "--dependencies", "--format", "json")
	if code != 0 {
		t.Fatalf("context dependencies: exit %d: %s\n%s", code, stdout, stderr)
	}
	var report struct {
		Dependencies *application.ObservedDependencies `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	d := report.Dependencies
	if d == nil || d.Coverage.Complete || len(d.Edges) != 1 {
		t.Fatalf("dependency report: %s", stdout)
	}
	edge := d.Edges[0]
	if edge.SourcePath != "main.go" || edge.TargetKind != "directory" || edge.TargetPath != "generated" || edge.TargetObserved {
		t.Fatalf("excluded target precision/status: %+v", edge)
	}
	codes := map[string]bool{}
	for _, diagnostic := range d.Diagnostics {
		codes[diagnostic.Code] = true
	}
	if !codes["IMPORTS_UNAVAILABLE"] || !codes["TARGET_NOT_OBSERVED"] {
		t.Fatalf("missing analysis gaps: %+v", d.Diagnostics)
	}
	for _, f := range d.Files {
		if strings.HasPrefix(f.Path, "generated/") {
			t.Fatal("scan exclusion lost")
		}
		if f.Path == "empty.go" && !f.ImportsAvailable {
			t.Fatal("zero imports treated as unavailable")
		}
	}
	stdout, stderr, code = runBin(t, root, os.Environ(), "context", "--format", "json")
	if code != 0 || strings.Contains(stdout, `"dependencies"`) {
		t.Fatalf("default context changed: %d %s %s", code, stdout, stderr)
	}
	stdout, stderr, code = runBin(t, root, os.Environ(), "context", "main.go", "--dependencies", "--format", "json")
	if code == 0 || !strings.Contains(stdout+stderr, "repository scope") {
		t.Fatalf("scoped inspection accepted: %d %s %s", code, stdout, stderr)
	}
}
