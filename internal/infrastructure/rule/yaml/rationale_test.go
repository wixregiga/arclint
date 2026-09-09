package yamlrule_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	yamlrule "github.com/wixregiga/arclint/internal/infrastructure/rule/yaml"
)

func TestRuleRationaleSchemaAndLoader(t *testing.T) {
	schema := compileRuleSchema(t)
	cases := []struct {
		name     string
		fields   string
		accepted bool
		reason   string
	}{
		{"canonical", "    rationale: Keep independent changes independent.\n", true, "Keep independent changes independent."},
		{"removed description", "    description: Keep independent changes independent.\n", false, ""},
		{"absent", "", true, ""},
		{"removed empty description", "    description: ''\n", false, ""},
		{"removed whitespace description", "    description: '   '\n", false, ""},
		{"canonical empty", "    rationale: ''\n", false, ""},
		{"canonical whitespace", "    rationale: '   '\n", false, ""},
		{"canonical null", "    rationale: null\n", false, ""},
		{"description alongside rationale", "    rationale: Reason\n    description: Legacy\n", false, ""},
		{"empty description alongside rationale", "    rationale: Reason\n    description: ''\n", false, ""},
		{"description alongside blank rationale", "    rationale: ''\n    description: Legacy\n", false, ""},
		{"both empty", "    rationale: ''\n    description: ''\n", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := []byte("rules:\n  no-cycles:\n" + tc.fields + "    acyclic: {}\n")
			doc, loadErr := yamlrule.Load(source, "rationale.yaml", vocab.UbiquitousLanguage{}, nil)
			schemaErr := validateAgainstSchema(t, schema, source)
			if (loadErr == nil) != tc.accepted || (schemaErr == nil) != tc.accepted {
				t.Fatalf("accepted=%v: loader=%v schema=%v", tc.accepted, loadErr, schemaErr)
			}
			if !tc.accepted {
				if strings.Contains(tc.fields, "description:") && !strings.Contains(loadErr.Error(), `unknown key "description"`) {
					t.Fatalf("removed field accepted or misdiagnosed: %v", loadErr)
				}
				return
			}
			r := ruleByID(t, doc.Configured, "no-cycles")
			if got := r.Rationale().String(); got != tc.reason {
				t.Fatalf("rationale=%q, want %q", got, tc.reason)
			}
			if r.Proposition() == "" || r.Proposition() == tc.reason {
				t.Fatalf("proposition=%q, rationale=%q", r.Proposition(), tc.reason)
			}
		})
	}
}

func TestOverrideRejectsAuthoredExplanations(t *testing.T) {
	schema := compileRuleSchema(t)
	pattern := loadSamplePattern(t)
	for _, key := range []string{"rationale", "description"} {
		for _, value := range []string{"An alternative reason", ""} {
			t.Run(key+"="+value, func(t *testing.T) {
				source := []byte(`extends:
  - pattern: acme/hexagonal@1.0.0
    bind:
      core: internal/core/**
      ports: internal/ports/**
      adapters: internal/adapters/**
rules:
  acme/hexagonal:core/stdlib-only:
    severity: warning
    ` + key + `: "` + value + `"
`)
				_, loadErr := loadString(t, string(source), pattern)
				if loadErr == nil || !strings.Contains(loadErr.Error(), key) {
					t.Fatalf("override accepted %s or lost field diagnostic: %v", key, loadErr)
				}
				if err := validateAgainstSchema(t, schema, source); err == nil {
					t.Fatalf("schema accepted override %s", key)
				}
			})
		}
	}
}

func TestRationaleDoesNotRenameZoneDescriptions(t *testing.T) {
	schema := compileRuleSchema(t)
	source := []byte(`zones:
  core:
    paths: core/**
    description: Core domain code.
rules:
  no-cycles:
    rationale: Keep changes independent.
    acyclic: {}
`)
	doc, err := yamlrule.Load(source, "zones.yaml", vocab.UbiquitousLanguage{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateAgainstSchema(t, schema, source); err != nil {
		t.Fatal(err)
	}
	if got := doc.Configured.Zones[0].Description(); got != "Core domain code." {
		t.Fatalf("zone description=%q", got)
	}
	bad := []byte(strings.Replace(string(source), "description: Core domain code.", "rationale: Core domain code.", 1))
	if _, err := yamlrule.Load(bad, "zones.yaml", vocab.UbiquitousLanguage{}, nil); err == nil {
		t.Fatal("zone accepted rationale alias")
	}
	if err := validateAgainstSchema(t, schema, bad); err == nil {
		t.Fatal("zone schema accepted rationale alias")
	}
}
