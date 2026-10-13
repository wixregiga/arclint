package yamlrule_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	yamlrule "github.com/wixregiga/arclint/internal/infrastructure/rule/yaml"
)

// The ruleset never names itself as the domain file: the recording
// reader and the full load both refuse it at domain.file.
func TestRulesetRejectsItselfAsDomainRecording(t *testing.T) {
	data := []byte("domain:\n  file: ./rules.arclint.yaml\n" + oneZoneRuleset)
	_, parseErr := yamlrule.ParseRecording(data, "rules.arclint.yaml")
	_, loadErr := yamlrule.Load(data, "rules.arclint.yaml", vocab.UbiquitousLanguage{}, nil)
	for _, err := range []error{parseErr, loadErr} {
		if err == nil || !strings.Contains(err.Error(), "domain.file: line 2: DomainFile never-the-ruleset") {
			t.Errorf("want rejection at domain.file, got %v", err)
		}
	}
}
