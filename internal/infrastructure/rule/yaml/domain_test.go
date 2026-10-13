package yamlrule_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	yamlrule "github.com/wixregiga/arclint/internal/infrastructure/rule/yaml"
)

const oneZoneRuleset = "zones:\n  core: core/**\n"

func TestLoadAcceptsTheDomainFile(t *testing.T) {
	for _, tc := range []struct {
		name string
		file string
		path string
		form vocab.DomainRecordingFormat
	}{
		{"nested YAML", "docs/model.arclint.yaml", "docs/model.arclint.yaml", vocab.DomainRecordingYAML},
		{"yml", "model.yml", "model.yml", vocab.DomainRecordingYAML},
		{"CML", "docs/model.cml", "docs/model.cml", vocab.DomainRecordingCML},
		{"cleaned", "./docs/../model.cml", "model.cml", vocab.DomainRecordingCML},
		{"the default, spelled", "domain.arclint.yaml", "domain.arclint.yaml", vocab.DomainRecordingYAML},
	} {
		document := "domain:\n  file: " + tc.file + "\n" + oneZoneRuleset
		if _, err := loadString(t, document); err != nil {
			t.Errorf("%s: Load rejected domain.file %q: %v", tc.name, tc.file, err)
		}
		recording, err := yamlrule.ParseRecording([]byte(document), "rules.arclint.yaml")
		if err != nil {
			t.Errorf("%s: ParseRecording: %v", tc.name, err)
			continue
		}
		if recording.Path() != tc.path || recording.Format() != tc.form {
			t.Errorf("%s: recording = %q (%s), want %q (%s)", tc.name, recording.Path(), recording.Format(), tc.path, tc.form)
		}
	}
}

func TestLoadRejectsAnInvalidDomainMapping(t *testing.T) {
	for _, tc := range []struct {
		name     string
		document string
		want     string
	}{
		{"unknown key", "domain:\n  file: model.yaml\n  format: yaml\n" + oneZoneRuleset, `domain: line 3: unknown key "format" (accepted: file)`},
		{"only an unknown key", "domain:\n  path: model.yaml\n" + oneZoneRuleset, `domain: line 2: unknown key "path" (accepted: file)`},
		{"no file", "domain: {}\n" + oneZoneRuleset, "domain: line 1: file is required"},
		{"not a mapping", "domain: model.yaml\n" + oneZoneRuleset, "domain: line 1: expected an object with the key file"},
		{"null", "domain:\n" + oneZoneRuleset, "domain: line 1: expected an object with the key file"},
		{"list", "domain:\n  - model.yaml\n" + oneZoneRuleset, "domain: line 1: expected an object with the key file"},
		{"file twice", "domain:\n  file: a.yaml\n  file: b.yaml\n" + oneZoneRuleset, `key "file" appears twice`},
		{"file not a string", "domain:\n  file: 12\n" + oneZoneRuleset, "domain.file: line 2: expected a string"},
		{"file a list", "domain:\n  file: [model.yaml]\n" + oneZoneRuleset, "domain.file: line 2: expected a string"},
		{"file null", "domain:\n  file:\n" + oneZoneRuleset, "domain.file: line 2: expected a string"},
		{"bad extension", "domain:\n  file: docs/model.json\n" + oneZoneRuleset, "domain.file: line 2: DomainRecording format-named-by-extension"},
		{"upper-case extension", "domain:\n  file: docs/model.YAML\n" + oneZoneRuleset, "DomainRecording format-named-by-extension"},
		{"no extension", "domain:\n  file: docs/model\n" + oneZoneRuleset, "DomainRecording format-named-by-extension"},
		{"only an extension", "domain:\n  file: .yaml\n" + oneZoneRuleset, "DomainRecording format-named-by-extension"},
		{"empty", "domain:\n  file: \"\"\n" + oneZoneRuleset, "domain.file: line 2: DomainRecording inside-the-repository"},
		{"escapes", "domain:\n  file: ../model.yaml\n" + oneZoneRuleset, "domain.file: line 2: DomainRecording inside-the-repository"},
		{"escapes after cleaning", "domain:\n  file: a/../../model.yaml\n" + oneZoneRuleset, "DomainRecording inside-the-repository"},
		{"absolute", "domain:\n  file: /etc/model.yaml\n" + oneZoneRuleset, "DomainRecording inside-the-repository"},
		{"Windows absolute", "domain:\n  file: 'C:\\models\\model.yaml'\n" + oneZoneRuleset, "DomainRecording inside-the-repository"},
		{"drive letter", "domain:\n  file: C:/model.yaml\n" + oneZoneRuleset, "DomainRecording inside-the-repository"},
		{"backslash", "domain:\n  file: 'docs\\model.yaml'\n" + oneZoneRuleset, "DomainRecording inside-the-repository"},
	} {
		_, err := loadString(t, tc.document)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: Load error = %v, want one containing %q", tc.name, err, tc.want)
		}
		_, err = yamlrule.ParseRecording([]byte(tc.document), "rules.arclint.yaml")
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: ParseRecording error = %v, want one containing %q", tc.name, err, tc.want)
		}
	}
}

func TestLoadRejectsADomainInAPatternFile(t *testing.T) {
	const document = "pattern:\n  namespace: acme\n  name: hexagonal\n  version: 1.0.0\ndomain:\n  file: docs/model.yaml\nzones:\n  core: \"The core.\"\n"
	_, err := loadString(t, document)
	if err == nil || !strings.Contains(err.Error(), "domain: line 5: a pattern distribution file does not accept domain") {
		t.Errorf("Load error = %v, want a refusal naming line 5", err)
	}
	_, err = yamlrule.LoadPattern([]byte(document), "pattern.yaml", nil)
	if err == nil || !strings.Contains(err.Error(), "does not accept domain") {
		t.Errorf("LoadPattern error = %v, want the same refusal", err)
	}
	_, err = yamlrule.ParseRecording([]byte(document), "pattern.yaml")
	if err == nil || !strings.Contains(err.Error(), "does not accept domain") {
		t.Errorf("ParseRecording error = %v, want the same refusal", err)
	}
}

func TestParseRecordingReadsOnlyTheDomainKey(t *testing.T) {
	for name, document := range map[string]string{
		"empty":                 "",
		"comments only":         "# nothing yet\n",
		"no domain key":         oneZoneRuleset,
		"keys it does not read": "runtime: [rust]\nnonsense: true\nrules: 3\n",
	} {
		recording, err := yamlrule.ParseRecording([]byte(document), "rules.arclint.yaml")
		if err != nil {
			t.Errorf("%s: ParseRecording: %v", name, err)
			continue
		}
		if !recording.IsZero() {
			t.Errorf("%s: recording = %+v, want none named", name, recording)
		}
	}
	recording, err := yamlrule.ParseRecording([]byte("runtime: [rust]\nrules: 3\ndomain:\n  file: docs/model.cml\n"), "rules.arclint.yaml")
	if err != nil || recording.Path() != "docs/model.cml" {
		t.Errorf("ParseRecording beside invalid keys = %+v, %v; want docs/model.cml", recording, err)
	}
	if _, err := yamlrule.ParseRecording([]byte("domain: [\n"), "rules.arclint.yaml"); err == nil {
		t.Errorf("ParseRecording accepted a document that is not YAML")
	}
	if _, err := yamlrule.ParseRecording([]byte("- domain\n"), "rules.arclint.yaml"); err == nil {
		t.Errorf("ParseRecording accepted a document that is not an object")
	}
}

func TestReadRecording(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		return path
	}

	absent, err := yamlrule.ReadRecording(filepath.Join(dir, "absent", "rules.arclint.yaml"))
	if err != nil || !absent.IsZero() {
		t.Errorf("a missing ruleset file = %+v, %v; want no DomainRecording named", absent, err)
	}
	noKey, err := yamlrule.ReadRecording(write("nokey.yaml", oneZoneRuleset))
	if err != nil || !noKey.IsZero() {
		t.Errorf("a ruleset without domain = %+v, %v; want no DomainRecording named", noKey, err)
	}
	named, err := yamlrule.ReadRecording(write("named.yaml", "domain:\n  file: docs/model.arclint.yaml\n"+oneZoneRuleset))
	if err != nil || named.Path() != "docs/model.arclint.yaml" || named.Format() != vocab.DomainRecordingYAML {
		t.Errorf("a ruleset naming docs/model.arclint.yaml = %+v, %v", named, err)
	}
	cml, err := yamlrule.ReadRecording(write("cml.yaml", "domain:\n  file: model.cml\n"))
	if err != nil || cml.Format() != vocab.DomainRecordingCML {
		t.Errorf("a ruleset naming model.cml = %+v, %v; want CML", cml, err)
	}
	badPath := write("bad.yaml", "domain:\n  file: notes.txt\n")
	if _, err := yamlrule.ReadRecording(badPath); err == nil || !strings.Contains(err.Error(), badPath) || !strings.Contains(err.Error(), "format-named-by-extension") {
		t.Errorf("a ruleset naming notes.txt: error = %v, want one naming the file and the invariant", err)
	}
	if _, err := yamlrule.ReadRecording(dir); err == nil {
		t.Errorf("a directory in place of the ruleset was read")
	}
}

func TestRecordingParserReadsAFixtureRuleset(t *testing.T) {
	recording, err := (yamlrule.RecordingParser{}).ParseRulesetRecording([]byte("domain:\n  file: fixtures/model.yaml\n"))
	if err != nil || recording.Path() != "fixtures/model.yaml" {
		t.Errorf("ParseRulesetRecording = %+v, %v", recording, err)
	}
	if _, err := (yamlrule.RecordingParser{}).ParseRulesetRecording([]byte("domain:\n  file: ../model.yaml\n")); err == nil ||
		!strings.HasPrefix(err.Error(), "rules.arclint.yaml: ") {
		t.Errorf("ParseRulesetRecording error = %v, want one naming rules.arclint.yaml", err)
	}
}

// A built-in rule exists once a domain is recorded, so an override of one
// with nothing recorded tells the author where to record: in the
// project's domain file, whether the ruleset names it or discovery found
// it, not in a file named by default.
func TestLoadNamesTheProjectsDomainFileWhereAnOverrideNeedsADomain(t *testing.T) {
	override := "zones:\n  core: core/**\nrules:\n  aggregate/protects-an-invariant:\n    severity: error\n"
	discovered, err := vocab.NewDomainRecording("domain.arclint.cml")
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		domain string
		lang   vocab.UbiquitousLanguage
		want   string
	}{
		"no file yet":              {"", vocab.UbiquitousLanguage{}, "record the domain in domain.arclint.yaml or remove the override"},
		"named":                    {"domain:\n  file: docs/model.arclint.yaml\n", vocab.UbiquitousLanguage{}, "record the domain in docs/model.arclint.yaml or remove the override"},
		"cleaned CML":              {"domain:\n  file: ./docs/model.cml\n", vocab.UbiquitousLanguage{}, "record the domain in docs/model.cml or remove the override"},
		"discovered":               {"", vocab.UbiquitousLanguage{Recording: discovered}, "record the domain in domain.arclint.cml or remove the override"},
		"named beats the language": {"domain:\n  file: docs/model.arclint.yaml\n", vocab.UbiquitousLanguage{Recording: discovered}, "record the domain in docs/model.arclint.yaml or remove the override"},
	} {
		_, err := yamlrule.Load([]byte(tc.domain+override), "test.yaml", tc.lang, nil)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error = %v, want one containing %q", name, err, tc.want)
		}
	}
}
