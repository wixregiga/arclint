package main

// End-to-end coverage of translating the project's domain between
// DomainRecordings against the compiled binary: export writes a copy and
// changes nothing else; convert moves the project to the new file, names
// it in rules.arclint.yaml, and removes the previous one; both prove the
// translation by reading it back.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func requireAbsent(t *testing.T, root, rel string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !os.IsNotExist(err) {
		t.Fatalf("%s still exists (stat: %v)", rel, err)
	}
}

func TestDomainExportWritesACopyAndChangesNothingElse(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", minimalDomainRules)
	write(t, root, "src/ok.go", "package src\n")
	recordOrdering(t, root)
	before := mustRunDomain(t, root, "domain", "overview", "--format", "json")

	stdout := mustRunDomain(t, root, "domain", "export", "model/copy.yml")
	for _, want := range []string{"Translated the domain from domain.arclint.yaml into model/copy.yml (YAML)", "Read back, it records the same language."} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("export said:\n%s\nwant %q", stdout, want)
		}
	}
	if readFile(t, root, "rules.arclint.yaml") != minimalDomainRules {
		t.Fatal("export changed rules.arclint.yaml")
	}
	if got := mustRunDomain(t, root, "domain", "overview", "--format", "json"); got != before {
		t.Fatalf("the project's domain changed after export:\n%s\nwant:\n%s", got, before)
	}
	if !strings.Contains(readFile(t, root, "model/copy.yml"), "project:") {
		t.Fatal("model/copy.yml holds no recorded language")
	}
}

func TestDomainConvertMovesTheProjectAndBack(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", minimalDomainRules)
	write(t, root, "src/ok.go", "package src\n")
	recordOrdering(t, root)
	before := mustRunDomain(t, root, "domain", "overview", "--format", "json")

	stdout := mustRunDomain(t, root, "domain", "convert", "model/domain.yml")
	for _, want := range []string{
		"Moved the domain from domain.arclint.yaml into model/domain.yml (YAML)",
		"rules.arclint.yaml now names model/domain.yml under domain.file.",
		"Removed domain.arclint.yaml.",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("convert said:\n%s\nwant %q", stdout, want)
		}
	}
	requireAbsent(t, root, "domain.arclint.yaml")
	if !strings.Contains(readFile(t, root, "rules.arclint.yaml"), "domain:\n  file: \"model/domain.yml\"\n") {
		t.Fatalf("rules.arclint.yaml does not name the new file:\n%s", readFile(t, root, "rules.arclint.yaml"))
	}
	after := mustRunDomain(t, root, "domain", "overview", "--format", "json")
	if strings.ReplaceAll(after, "model/domain.yml", "domain.arclint.yaml") != before {
		t.Fatalf("the moved domain differs:\n%s\nwant:\n%s", after, before)
	}

	stdout = mustRunDomain(t, root, "domain", "convert", "domain.arclint.yaml", "--format", "json")
	for _, want := range []string{`"converted": true`, `"removed": "model/domain.yml"`, `"same_language": true`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("convert back said:\n%s\nwant %q", stdout, want)
		}
	}
	requireAbsent(t, root, "model/domain.yml")
	if readFile(t, root, "rules.arclint.yaml") != minimalDomainRules {
		t.Fatalf("moving back to the default did not restore rules.arclint.yaml:\n%s", readFile(t, root, "rules.arclint.yaml"))
	}
	if got := mustRunDomain(t, root, "domain", "overview", "--format", "json"); got != before {
		t.Fatalf("the domain moved back differs:\n%s\nwant:\n%s", got, before)
	}
}

func TestDomainExportRefusesMistakes(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", minimalDomainRules)
	write(t, root, "src/ok.go", "package src\n")
	recordOrdering(t, root)
	for _, args := range [][]string{
		{"domain", "export", "domain.arclint.yaml"},
		{"domain", "export", "model.json"},
		{"domain", "convert", "../outside.yaml"},
		{"domain", "export"},
	} {
		_, stderr, code := runBin(t, root, os.Environ(), args...)
		if code != 2 {
			t.Fatalf("%v: exit %d, want 2 (usage)\nstderr: %s", args, code, stderr)
		}
	}
	if readFile(t, root, "rules.arclint.yaml") != minimalDomainRules {
		t.Fatal("a refused command changed rules.arclint.yaml")
	}
}

func TestDomainConvertToCMLAndBack(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", minimalDomainRules)
	write(t, root, "src/ok.go", "package src\n")
	recordOrdering(t, root)
	before := mustRunDomain(t, root, "domain", "overview", "--format", "json")
	checkBefore, _, _ := runBin(t, root, os.Environ(), "check", "--format", "json")

	stdout := mustRunDomain(t, root, "domain", "convert", "domain.arclint.cml")
	if !strings.Contains(stdout, "into domain.arclint.cml (CML)") {
		t.Fatalf("convert said:\n%s", stdout)
	}
	requireAbsent(t, root, "domain.arclint.yaml")
	cml := readFile(t, root, "domain.arclint.cml")
	for _, want := range []string{"\nDomain ", "BoundedContext ordering {", "Aggregate Order {", "aggregateRoot", "OrderID id key", "def void ensureCustomerIdentified();"} {
		if !strings.Contains(cml, want) {
			t.Fatalf("domain.arclint.cml lacks %q:\n%s", want, cml)
		}
	}
	if got := mustRunDomain(t, root, "domain", "overview", "--format", "json"); strings.ReplaceAll(got, "domain.arclint.cml", "domain.arclint.yaml") != before {
		t.Fatalf("the CML domain reads differently:\n%s\nwant:\n%s", got, before)
	}
	checkAfter, _, _ := runBin(t, root, os.Environ(), "check", "--format", "json")
	if len(checkAfter) == 0 || strings.Count(checkAfter, `"kind": "violation"`) != strings.Count(checkBefore, `"kind": "violation"`) {
		t.Fatalf("check judges the CML domain differently:\n%s\nwant the findings of:\n%s", checkAfter, checkBefore)
	}

	mustRunDomain(t, root, "domain", "convert", "domain.arclint.yaml")
	requireAbsent(t, root, "domain.arclint.cml")
	if got := mustRunDomain(t, root, "domain", "overview", "--format", "json"); got != before {
		t.Fatalf("the domain moved back from CML differs:\n%s\nwant:\n%s", got, before)
	}
	if readFile(t, root, "rules.arclint.yaml") != minimalDomainRules {
		t.Fatalf("rules.arclint.yaml was not restored:\n%s", readFile(t, root, "rules.arclint.yaml"))
	}
}

// A move is all or nothing on a real file system. The previous domain
// file sits in a read-only directory, so convert writes the new file and
// edits the ruleset, then cannot remove the previous file; it puts back
// what it wrote, and every file is byte for byte as it was.
func TestConvertChangesNothingWhenThePreviousFileCannotBeRemoved(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permissions do not bind the superuser")
	}
	root := t.TempDir()
	rules := "domain:\n  file: docs/model.yaml\n" + minimalDomainRules
	write(t, root, "rules.arclint.yaml", rules)
	write(t, root, "docs/model.yaml", orderingDomain)
	docs := filepath.Join(root, "docs")
	if err := os.Chmod(docs, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(docs, 0o750) })

	_, stderr, code := runBin(t, root, os.Environ(), "domain", "convert", "domain.arclint.cml")
	if code == 0 || !strings.Contains(stderr, "nothing was changed") {
		t.Fatalf("convert: exit %d\nstderr: %s", code, stderr)
	}
	if got := readFile(t, root, "rules.arclint.yaml"); got != rules {
		t.Fatalf("rules.arclint.yaml changed:\n%s", got)
	}
	if got := readFile(t, root, "docs/model.yaml"); got != orderingDomain {
		t.Fatalf("the previous domain file changed:\n%s", got)
	}
	requireAbsent(t, root, "domain.arclint.cml")
}
