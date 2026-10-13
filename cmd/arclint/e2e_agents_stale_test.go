package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
)

func TestCheckReportsStaleAgentsBlockWithoutFindingOrExitChange(t *testing.T) {
	for _, format := range []string{"yaml", "cml"} {
		t.Run(format, func(t *testing.T) {
			work := instructionProject(t, boxofficeLanguage(t), format)
			check := func(stale bool) string {
				t.Helper()
				stdout, stderr, code := runBin(t, work, os.Environ(), "check", ".")
				// This copy has the real recorded domain without its source code.
				if code != 1 || stderr != "" {
					t.Fatalf("check: exit=%d\n%s\n%s", code, stdout, stderr)
				}
				if got := strings.Contains(stdout, "agents: "+application.AgentsContextStaleMessage); got != stale {
					t.Fatalf("stale report=%v want=%v\n%s", got, stale, stdout)
				}
				return stdout
			}
			before := check(false)
			write(t, work, "AGENTS.md", "# Hand-written guidance\n")
			if got := check(false); got != before {
				t.Fatal("a document without markers changed the report")
			}
			mustRunDomain(t, work, "agents", "md", "--write")
			if got := check(false); got != before {
				t.Fatal("a current block changed the report")
			}
			mustRunDomain(t, work, "domain", "define", "question", "pending-sale", "--context", "catalog", "--text", "When does a pending sale expire?")
			// Questions do not create findings and are not in the block's summary.
			check(false)
			mustRunDomain(t, work, "domain", "define", "bounded_context", "support", "--definition", "Support resolves attendee questions.")
			staleOutput := check(true)
			if !strings.Contains(staleOutput, application.AgentsContextRefreshCommand) {
				t.Fatal("stale report omitted the refresh command")
			}
			stdout, stderr, code := runBin(t, work, os.Environ(), "check", ".", "--format", "json")
			if code != 1 || stderr != "" {
				t.Fatalf("JSON check: exit=%d\n%s\n%s", code, stdout, stderr)
			}
			var reports []map[string]any
			if err := json.Unmarshal([]byte(stdout), &reports); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, report := range reports {
				if report["path"] == "AGENTS.md" {
					found = true
					if report["kind"] != "coverage" || report["remediation"] != application.AgentsContextRefreshCommand || report["status"] != nil || report["severity"] != nil || report["ruleId"] != nil {
						t.Fatalf("stale block became a finding: %+v", report)
					}
				}
			}
			if !found {
				t.Fatal("JSON omitted stale-block report")
			}
			mustRunDomain(t, work, "agents", "md", "--write")
			check(false)
		})
	}
}

func TestStaleAgentsBlockDoesNotFailConformingProject(t *testing.T) {
	work := skillFixture(t)
	mustRunDomain(t, work, "agents", "md", "--write")
	path := filepath.Join(work, "AGENTS.md")
	block, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	write(t, work, "AGENTS.md", strings.Replace(string(block), "Architecture contracts", "Old architecture contracts", 1))
	stdout, stderr, code := runBin(t, work, os.Environ(), "check", ".")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "agents: "+application.AgentsContextStaleMessage) || !strings.Contains(stdout, "0 active finding(s)") {
		t.Fatalf("stale block check: exit=%d\n%s\n%s", code, stdout, stderr)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mustRunDomain(t, work, "check", ".")
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("check changed AGENTS.md: %v", err)
	}
	mustRunDomain(t, work, "agents", "md", "--write")
	stdout = mustRunDomain(t, work, "check", ".")
	if strings.Contains(stdout, "agents:") {
		t.Fatalf("current block reported stale: %s", stdout)
	}
}
