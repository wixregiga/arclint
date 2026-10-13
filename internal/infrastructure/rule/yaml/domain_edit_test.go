package yamlrule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// recordingOf builds the DomainRecording of one file; the empty name is
// the zero DomainRecording, which names no file.
func recordingOf(t *testing.T, file string) vocab.DomainRecording {
	t.Helper()
	if file == "" {
		return vocab.DomainRecording{}
	}
	recording, err := vocab.NewDomainRecording(file)
	if err != nil {
		t.Fatal(err)
	}
	return recording
}

func editRuleset(t *testing.T, content, file string) (string, bool) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rules.arclint.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	editor, err := NewEditor(path)
	if err != nil {
		t.Fatal(err)
	}
	recording := recordingOf(t, file)
	changed, err := nameDomainRecording(editor, recording)
	if err != nil {
		t.Fatalf("name %q: %v", file, err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseRecording(out, path)
	if err != nil {
		t.Fatalf("edited ruleset does not load: %v\n%s", err, out)
	}
	if got != recording {
		t.Fatalf("edited ruleset names %s, want %s\n%s", got.Path(), recording.Path(), out)
	}
	return string(out), changed
}

const rulesetWithoutDomain = `# The project's contracts.
runtime: [go]

# Zones name the folders.
zones:
  domain:
    paths: "internal/domain/**"
rules:
  domain/stdlib-only:
    on: domain
    imports:
      external: forbid
`

func TestNameDomainRecordingInsertsTheSectionBeforeZones(t *testing.T) {
	out, changed := editRuleset(t, rulesetWithoutDomain, "domain.arclint.cml")
	if !changed {
		t.Fatal("changed = false, want true")
	}
	want := "runtime: [go]\n\ndomain:\n  file: \"domain.arclint.cml\"\n\n# Zones name the folders.\nzones:"
	if !strings.Contains(out, want) {
		t.Fatalf("edited ruleset:\n%s\nwant it to contain:\n%s", out, want)
	}
	if !strings.HasPrefix(out, "# The project's contracts.\n") {
		t.Fatalf("the leading comment moved:\n%s", out)
	}
}

func TestNameDomainRecordingReplacesTheFile(t *testing.T) {
	in := strings.Replace(rulesetWithoutDomain, "runtime: [go]\n", "runtime: [go]\ndomain:\n  file: 'docs/model.yaml' # moved here in March\n", 1)
	out, changed := editRuleset(t, in, "docs/model.cml")
	if !changed {
		t.Fatal("changed = false, want true")
	}
	if !strings.Contains(out, "  file: \"docs/model.cml\" # moved here in March\n") {
		t.Fatalf("the file was not replaced in place:\n%s", out)
	}
	if strings.Count(out, "\ndomain:\n") != 1 {
		t.Fatalf("want one domain section:\n%s", out)
	}
}

func TestNameDomainRecordingRemovesTheSectionForNoFile(t *testing.T) {
	in := strings.Replace(rulesetWithoutDomain, "runtime: [go]\n", "runtime: [go]\n\n# Recorded in CML.\ndomain:\n  file: domain.arclint.cml\n", 1)
	out, changed := editRuleset(t, in, "")
	if !changed {
		t.Fatal("changed = false, want true")
	}
	if out != rulesetWithoutDomain {
		t.Fatalf("removing the section left:\n%s\nwant:\n%s", out, rulesetWithoutDomain)
	}
}

func TestNameDomainRecordingLeavesAnUnchangedRuleset(t *testing.T) {
	out, changed := editRuleset(t, rulesetWithoutDomain, "")
	if changed || out != rulesetWithoutDomain {
		t.Fatalf("naming none in a ruleset that names none: changed = %v, ruleset:\n%s", changed, out)
	}
	named := strings.Replace(rulesetWithoutDomain, "runtime: [go]\n", "runtime: [go]\ndomain:\n  file: \"docs/model.cml\"\n", 1)
	out, changed = editRuleset(t, named, "docs/model.cml")
	if changed || out != named {
		t.Fatalf("naming the file the ruleset already names: changed = %v, ruleset:\n%s", changed, out)
	}
}

// A file discovery would find anyway is still a file a ruleset may name:
// nothing substitutes for the setting but the zero DomainRecording.
func TestNameDomainRecordingWritesADefaultNamedFileAsAnyOther(t *testing.T) {
	out, changed := editRuleset(t, rulesetWithoutDomain, "domain.arclint.yaml")
	if !changed {
		t.Fatal("changed = false, want true")
	}
	want := "runtime: [go]\n\ndomain:\n  file: \"domain.arclint.yaml\"\n\n# Zones name the folders.\nzones:"
	if !strings.Contains(out, want) {
		t.Fatalf("edited ruleset:\n%s\nwant it to contain:\n%s", out, want)
	}
	out, changed = editRuleset(t, out, "")
	if !changed || out != rulesetWithoutDomain {
		t.Fatalf("naming none again: changed = %v, ruleset:\n%s\nwant:\n%s", changed, out, rulesetWithoutDomain)
	}
}

func TestNameDomainRecordingRefusesAPatternFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pattern.arclint.yaml")
	content := "pattern:\n  namespace: acme\n  name: layers\n  version: 1.0.0\nzones:\n  domain:\n    description: core\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	editor, err := NewEditor(path)
	if err != nil {
		t.Fatal(err)
	}
	recording, err := vocab.NewDomainRecording("domain.arclint.cml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nameDomainRecording(editor, recording); err == nil || !strings.Contains(err.Error(), "pattern distribution file") {
		t.Fatalf("err = %v, want a refusal naming the pattern distribution file", err)
	}
}

// The document's leading comment block is its header (the schema
// modeline and its introduction); the domain section never claims it,
// whether the section is removed or inserted next to it.
func TestNameDomainRecordingKeepsTheDocumentHeader(t *testing.T) {
	const header = "# yaml-language-server: $schema=.arclint/schemas/rules.arclint.schema.json\n# Header text.\n"
	const zones = "zones:\n  a:\n    paths: [\"x/**\"]\nrules: {}\n"
	out, changed := editRuleset(t, header+"domain:\n  file: domain.arclint.cml\n\n"+zones, "")
	if !changed || out != header+zones {
		t.Fatalf("removing a first-key domain section:\n%s\nwant:\n%s", out, header+zones)
	}
	out, changed = editRuleset(t, header+zones, "domain.arclint.cml")
	want := header + "domain:\n  file: \"domain.arclint.cml\"\n\n" + zones
	if !changed || out != want {
		t.Fatalf("inserting below the header:\n%s\nwant:\n%s", out, want)
	}
}

// Ruleset shapes the edit must either handle or refuse, never corrupt.
func TestNameDomainRecordingOnEveryRulesetShape(t *testing.T) {
	const zones = "zones:\n  a:\n    paths: [\"x/**\"]\nrules: {}\n"
	for _, tc := range []struct{ name, src, target string }{
		{"flow", "domain: {file: domain.arclint.cml}\n" + zones, "docs/model.cml"},
		{"single quoted", "domain:\n  file: 'domain.arclint.cml'\n" + zones, "docs/model.cml"},
		{"crlf", strings.ReplaceAll("domain:\n  file: domain.arclint.cml\n"+zones, "\n", "\r\n"), ""},
		{"crlf insert", strings.ReplaceAll(zones, "\n", "\r\n"), "docs/model.cml"},
		{"last key trailing blanks", zones + "domain:\n  file: domain.arclint.cml\n\n\n", ""},
		{"inline comment", "domain:\n  file: domain.arclint.cml # the model\n" + zones, "docs/model.cml"},
		{"document markers", "---\ndomain:\n  file: domain.arclint.cml\n" + zones + "...\n", ""},
		{"no newline at end", strings.TrimSuffix(zones, "\n"), "docs/model.cml"},
		{"anchor", "domain: &d\n  file: domain.arclint.cml\n" + zones, ""},
		{"comment inside domain", "domain:\n  # which file\n  file: domain.arclint.cml\n" + zones, ""},
		{"trailing comment after a last domain key", zones + "domain:\n  file: domain.arclint.cml\n# the end\n", ""},
		{"unicode path", "domain:\n  file: domain.arclint.cml\n" + zones, "docs/modèle.cml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "rules.arclint.yaml")
			if err := os.WriteFile(path, []byte(tc.src), 0o600); err != nil {
				t.Fatal(err)
			}
			editor, err := NewEditor(path)
			if err != nil {
				t.Fatal(err)
			}
			recording := recordingOf(t, tc.target)
			if _, err := nameDomainRecording(editor, recording); err != nil {
				out, _ := os.ReadFile(path)
				if string(out) != tc.src {
					t.Fatalf("a refused edit changed the ruleset: %v\n%s", err, out)
				}
				return
			}
			out, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ParseRecording(out, path)
			if err != nil || got != recording {
				t.Fatalf("the edited ruleset names %q (%v):\n%s", got.Path(), err, out)
			}
		})
	}
}

// nameDomainRecording plans the ruleset edit and writes it, as convert
// does once every other file of the move is planned.
func nameDomainRecording(editor Editor, recording vocab.DomainRecording) (bool, error) {
	change, changed, err := editor.PlanDomainRecordingName(recording)
	if err != nil {
		return false, err
	}
	if err := change.Apply(); err != nil {
		return false, err
	}
	return changed, nil
}

// A domain.file written as an escaped scalar is found by its spelling in
// the ruleset, not its decoded value, and replaced with the comment kept.
func TestNameDomainRecordingReplacesAnEscapedFile(t *testing.T) {
	for _, file := range []string{`'docs/owner''s.yaml'`, `"docs/owner\u0027s.yaml"`, `docs/owners.yaml`} {
		for _, separator := range []string{" ", "\t"} {
			in := strings.Replace(rulesetWithoutDomain, "runtime: [go]\n", "runtime: [go]\ndomain:\n  file: "+file+separator+"# kept\n", 1)
			out, changed := editRuleset(t, in, "docs/model.cml")
			if !changed || !strings.Contains(out, "  file: \"docs/model.cml\""+separator+"# kept\n") {
				t.Fatalf("%s with %q before the comment: the file was not replaced in place:\n%s", file, separator, out)
			}
		}
	}
}
