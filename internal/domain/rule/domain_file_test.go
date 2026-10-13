package rule

import (
	"strings"
	"testing"
)

// The ruleset never names itself as the domain file, however the path
// spells it; every other path is judged by the DomainRecording alone.
func TestNewDomainFileRefusesTheRuleset(t *testing.T) {
	for _, file := range []string{RulesetFileName, "./" + RulesetFileName, "docs/../" + RulesetFileName, "Rules.arclint.yaml", "RULES.ARCLINT.YAML"} {
		if f, err := NewDomainFile(file); err == nil || !strings.Contains(err.Error(), "DomainFile never-the-ruleset") {
			t.Errorf("NewDomainFile(%q) = %+v, %v; want never-the-ruleset", file, f, err)
		}
	}
	for file, want := range map[string]string{
		"domain.arclint.yaml":     "domain.arclint.yaml",
		"docs/" + RulesetFileName: "docs/" + RulesetFileName,
		"./docs/../model.cml":     "model.cml",
	} {
		f, err := NewDomainFile(file)
		if err != nil {
			t.Errorf("NewDomainFile(%q): %v", file, err)
			continue
		}
		if got := f.Recording().Path(); got != want {
			t.Errorf("NewDomainFile(%q).Recording().Path() = %q, want %q", file, got, want)
		}
	}
	if _, err := NewDomainFile("model.txt"); err == nil || !strings.Contains(err.Error(), "format-named-by-extension") {
		t.Errorf("NewDomainFile(model.txt) = %v; want the DomainRecording's own refusal", err)
	}
}
