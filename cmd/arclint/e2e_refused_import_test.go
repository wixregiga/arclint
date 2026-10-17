package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// refusedImportRules restrict storage to app and skill, the shape that
// led an agent to copy a write-access check into storage instead of
// importing it: nothing declares a Zone for the shared mechanism.
const refusedImportRules = `runtime: [go]
zones:
  app: internal/app/**
  skill: internal/skill/**
  clients: internal/infra/clients/**
  storage: internal/infra/skillstore/**
rules:
  storage/imports:
    on: storage
    imports:
      internal: [app, skill]
`

// TestRefusedImportExplainsSharedMechanismZone runs a refused import
// through the binary in both report formats: an import into a Zone the
// allow-list omits and an import of code outside any declared Zone each
// carry the shared-mechanism remediation, and declaring the mechanism a
// Zone of its own and allowing it clears the finding.
func TestRefusedImportExplainsSharedMechanismZone(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "go.mod", "module example.com/skills\n\ngo 1.26\n")
	write(t, dir, "rules.arclint.yaml", refusedImportRules)
	write(t, dir, "internal/app/app.go", "package app\n")
	write(t, dir, "internal/skill/skill.go", "package skill\n")
	write(t, dir, "internal/infra/clients/writeaccess/write_access.go", "package writeaccess\n")
	write(t, dir, "internal/infra/writeaccess/write_access.go", "package writeaccess\n")
	write(t, dir, "internal/infra/skillstore/store.go", `package skillstore

import (
	_ "example.com/skills/internal/infra/clients/writeaccess"
	_ "example.com/skills/internal/infra/writeaccess"
)
`)

	mechanism := `if the imported code is a shared mechanism, declare a Zone of its own for it, ` +
		`allow that Zone in the imports of Zone "storage" and of every other Zone that needs it, ` +
		`and keep it free of product decisions (Eric Evans, Domain-Driven Design Reference: Definitions and Pattern Summaries (2015), p. 44, Cohesive Mechanisms (tier 1, read)); `
	copying := "; never copy the code to get around this rule"
	want := map[string]string{
		`import "example.com/skills/internal/infra/clients/writeaccess" resolves to Zone(s) ["clients"], not in the allow-list of Zone "storage"`: mechanism +
			"otherwise add the target Zone to the allow-list or remove the import" + copying,
		`import "example.com/skills/internal/infra/writeaccess" resolves to code outside any declared Zone, not allowed for Zone "storage"`: mechanism +
			"otherwise declare the Zone that owns the code and add it to the allow-list, or remove the import" + copying,
	}

	stdout, stderr, code := runBin(t, dir, os.Environ(), "check", ".")
	if code != 1 || stderr != "" {
		t.Fatalf("check: exit=%d\n%s\n%s", code, stdout, stderr)
	}
	for message, remediation := range want {
		if !strings.Contains(stdout, message+"\n  remediation: "+remediation+"\n") {
			t.Errorf("human report lacks %q with remediation %q:\n%s", message, remediation, stdout)
		}
	}

	stdout, stderr, code = runBin(t, dir, os.Environ(), "check", ".", "--format", "json")
	if code != 1 || stderr != "" {
		t.Fatalf("JSON check: exit=%d\n%s\n%s", code, stdout, stderr)
	}
	var reports []struct {
		RuleID      string `json:"ruleId"`
		Message     string `json:"message"`
		Remediation string `json:"remediation"`
	}
	if err := json.Unmarshal([]byte(stdout), &reports); err != nil {
		t.Fatalf("JSON report: %v\n%s", err, stdout)
	}
	got := map[string]string{}
	for _, r := range reports {
		if r.RuleID == "storage/imports" {
			got[r.Message] = r.Remediation
		}
	}
	if len(got) != len(want) {
		t.Errorf("JSON findings of storage/imports = %d, want %d: %+v", len(got), len(want), got)
	}
	for message, remediation := range want {
		if got[message] != remediation {
			t.Errorf("JSON remediation for %q:\n got %q\nwant %q", message, got[message], remediation)
		}
	}

	// The remediation's own repair: the mechanism gets a Zone of its own,
	// allowed where it is needed, and storage imports it from there.
	write(t, dir, "rules.arclint.yaml", strings.Replace(
		strings.Replace(refusedImportRules, "  storage: internal/infra/skillstore/**\n",
			"  storage: internal/infra/skillstore/**\n  writeaccess: internal/infra/writeaccess/**\n", 1),
		"internal: [app, skill]", "internal: [app, skill, writeaccess]", 1))
	write(t, dir, "internal/infra/skillstore/store.go",
		"package skillstore\n\nimport _ \"example.com/skills/internal/infra/writeaccess\"\n")
	stdout, stderr, code = runBin(t, dir, os.Environ(), "check", ".")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "0 active finding(s)") {
		t.Fatalf("check after declaring the mechanism Zone: exit=%d\n%s\n%s", code, stdout, stderr)
	}
}
