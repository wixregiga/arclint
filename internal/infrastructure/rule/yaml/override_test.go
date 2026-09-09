package yamlrule_test

import (
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
	yamlrule "github.com/wixregiga/arclint/internal/infrastructure/rule/yaml"
)

func TestBuiltInOverrideUsesRecordedDomainAndPreservesConstraint(t *testing.T) {
	language, found, err := repoVocabulary(t).RecordedLanguage()
	if err != nil || !found {
		t.Fatalf("recorded language: found=%v error=%v", found, err)
	}
	doc, err := yamlrule.Load([]byte(`runtime: [go]
rules:
  value_object/no-setters:
    severity: info
    disable: adopting the model
    exclude:
      paths: [generated/**]
      reason: generated
    suppress:
      paths: [legacy/**]
      reason: debt
`), "override.yaml", language, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := ruleByID(t, doc.Configured, "value_object/no-setters")
	if !r.BuiltIn() || r.Severity() != rule.SeverityInfo || !r.Disabled() {
		t.Fatal("built-in adoption decisions missing")
	}
	if constraint, ok := r.Constraint().(rule.DomainConstraint); !ok || constraint.Invariant != "value_object/no-setters" {
		t.Fatal("override changed the built-in constraint")
	}
	if r.AppliesToFile("generated/a.go", nil) {
		t.Fatal("built-in exclusion missing")
	}
	if reason, ok := r.SuppressionFor("legacy/a.go"); !ok || reason != "debt" {
		t.Fatal("built-in suppression missing")
	}
}
