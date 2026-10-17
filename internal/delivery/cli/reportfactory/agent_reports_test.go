package reportfactory_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/delivery/cli/reportfactory"
	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

func TestAgentReportsPreserveFactsAcrossRenderers(t *testing.T) {
	workflow := cli.HooksStatusReport{}
	workflow.Status.Project = "/project"
	workflow.Status.Binary = "/home/me/.local/bin/arclint"
	workflow.Status.LastEvent = time.Date(2026, 10, 9, 20, 15, 3, 0, time.UTC)
	workflow.Status.Files = []application.HookFileStatus{
		{Host: "claude", Scope: "user", Path: "/home/me/.claude/settings.json", Installation: application.HooksComplete},
		{Host: "claude", Scope: "user", Windows: true, Path: "/mnt/c/Users/me/.claude/settings.json", Installation: application.HooksComplete},
		{Host: "codex", Scope: "project", Path: "/project/.codex/hooks.json", Installation: application.HooksPartial, Listed: []string{"SessionStart", "PostToolUse", "Stop"}, Missing: []string{"UserPromptSubmit"}, Problems: []string{"SessionStart, PostToolUse and Stop are installed and still run; UserPromptSubmit is missing."}},
	}
	cases := []struct {
		name    string
		report  cli.Report
		facts   []string
		jsonKey string
	}{
		{"workflow install", cli.AgentInstallReport{Operation: "hooks", Host: "claude", Paths: []string{"/home/me/.claude/settings.json"}, Removed: []string{"/project/.claude/settings.local.json"}, Orientation: "Work in this order:\n1. Run context.", Activation: "Start a new session."}, []string{"/home/me/.claude/settings.json", "removed the duplicate hooks from /project/.claude/settings.local.json", "Work in this order:\n1. Run context.", "Start a new session."}, "orientation"},
		{"workflow status", workflow, []string{"/project", "Binary: /home/me/.local/bin/arclint", "Last event: 2026-10-09 20:15:03 +0000", "claude user: /home/me/.claude/settings.json (installed: complete)", "claude user (Windows): /mnt/c/Users/me/.claude/settings.json (installed: complete)", "codex project: /project/.codex/hooks.json (installed: partial)", "SessionStart, PostToolUse and Stop are installed and still run; UserPromptSubmit is missing.", "trust them in /hooks"}, "lastEvent"},
	}
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var plain string
			for _, name := range []cli.RendererName{cli.RendererPlain, cli.RendererLipgloss, cli.RendererJSON} {
				render, err := reportfactory.Select(name)
				if err != nil {
					t.Fatal(err)
				}
				var buf bytes.Buffer
				if err := render.Render(&buf, tc.report); err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				if name == cli.RendererJSON {
					var doc map[string]json.RawMessage
					if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
						t.Fatalf("full JSON output: %v", err)
					}
					if _, found := doc[tc.jsonKey]; !found {
						t.Fatalf("missing %s", tc.jsonKey)
					}
				} else {
					output := ansi.ReplaceAllString(buf.String(), "")
					for _, fact := range tc.facts {
						if !strings.Contains(output, fact) {
							t.Fatalf("%s lost %q: %s", name, fact, output)
						}
					}
					if name == cli.RendererPlain {
						plain = output
					} else if output != plain {
						t.Fatalf("human renderers disagree:\nplain: %s\nstyled: %s", plain, output)
					}
				}
				if err := render.Render(&shortWriter{n: 1}, tc.report); !errors.Is(err, io.ErrShortWrite) {
					t.Fatalf("%s lost short-write error: %v", name, err)
				}
			}
		})
	}
}

// A finding's remediation reaches the reader whichever renderer runs:
// both human renderers print it under the finding, identically once
// styling is removed, and JSON carries the same text.
// A JSON reader tells a partly installed file from a missing one by its
// installation and the hooks it lists and lacks.
func TestHooksStatusJSONNamesListedAndMissingHooks(t *testing.T) {
	report := cli.HooksStatusReport{}
	report.Status.Files = []application.HookFileStatus{
		{Host: "claude", Scope: "user", Path: "/home/me/.claude/settings.json", Installation: application.HooksPartial, Listed: []string{"SessionStart", "PostToolUse", "Stop"}, Missing: []string{"UserPromptSubmit"}},
		{Host: "codex", Scope: "user", Path: "/home/me/.codex/hooks.json", Installation: application.HooksAbsent, Missing: []string{"SessionStart", "UserPromptSubmit", "PostToolUse", "Stop"}},
		{Host: "codex", Scope: "user", Windows: true, Path: `%USERPROFILE%\.codex\hooks.json`, Installation: application.HooksUnknown},
	}
	render, err := reportfactory.Select(cli.RendererJSON)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := render.Render(&buf, report); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Files []struct {
			Installed string   `json:"installed"`
			Listed    []string `json:"listed"`
			Missing   []string `json:"missing"`
		} `json:"files"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	want := []struct {
		installed       string
		listed, missing []string
	}{
		{"partial", []string{"SessionStart", "PostToolUse", "Stop"}, []string{"UserPromptSubmit"}},
		{"absent", []string{}, []string{"SessionStart", "UserPromptSubmit", "PostToolUse", "Stop"}},
		{"unknown", []string{}, []string{}},
	}
	if len(doc.Files) != len(want) {
		t.Fatalf("files: %s", buf.String())
	}
	for index, file := range doc.Files {
		if file.Installed != want[index].installed || !slices.Equal(file.Listed, want[index].listed) || !slices.Equal(file.Missing, want[index].missing) || file.Listed == nil || file.Missing == nil {
			t.Fatalf("file %d: %+v, want %+v", index, file, want[index])
		}
	}
}

func TestCheckRemediationAcrossRenderers(t *testing.T) {
	const remediation = "RoleID includes RoleName, which is not an identity. If RoleName can change while the Role stays the same, it is not part of the identity; change the identity before adding a field. If it cannot change, declare a field of type RoleID on Role."
	id, err := rule.NewID("entity/identity")
	if err != nil {
		t.Fatal(err)
	}
	subject, err := rule.ContextSubject("access")
	if err != nil {
		t.Fatal(err)
	}
	v, err := conformance.NewViolation(conformance.ViolationSpec{
		Rule: id, Subject: subject, Outcome: conformance.OutcomeViolates,
		Severity: rule.SeverityError, Assurance: rule.AssuranceExact, Evidence: "field types",
		Message:     "aggregate Role: Role declares no field holding its identity RoleID",
		Remediation: remediation, Path: "role/role.go", Line: 17,
	})
	if err != nil {
		t.Fatal(err)
	}
	enforcement, err := rule.NewEnforcement([]rule.Language{rule.LanguageGo}, []rule.Fact{rule.FactDeclarations, rule.FactFieldTypes}, "field types", rule.AssuranceExact, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	e, err := conformance.NewEvaluation(id, subject, conformance.OutcomeViolates, enforcement, []conformance.Violation{v})
	if err != nil {
		t.Fatal(err)
	}
	a, err := conformance.NewAssessment([]conformance.Evaluation{e}, nil, []string{"entity/identity"})
	if err != nil {
		t.Fatal(err)
	}
	report := cli.AssessmentReport{Assessment: a}
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	var plain string
	for _, name := range []cli.RendererName{cli.RendererPlain, cli.RendererLipgloss, cli.RendererJSON} {
		render, err := reportfactory.Select(name)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := render.Render(&buf, report); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if name == cli.RendererJSON {
			var docs []struct {
				RuleID      string `json:"ruleId"`
				Remediation string `json:"remediation"`
			}
			if err := json.Unmarshal(buf.Bytes(), &docs); err != nil {
				t.Fatalf("JSON output: %v", err)
			}
			if len(docs) != 1 || docs[0].Remediation != remediation {
				t.Fatalf("JSON remediation %+v, want %q", docs, remediation)
			}
			continue
		}
		output := ansi.ReplaceAllString(buf.String(), "")
		if !strings.Contains(output, "role/role.go:17: [error] entity/identity aggregate Role: Role declares no field holding its identity RoleID\n  remediation: "+remediation+"\n") {
			t.Fatalf("%s does not print the remediation under the finding:\n%s", name, output)
		}
		if name == cli.RendererPlain {
			plain = output
		} else if output != plain {
			t.Fatalf("human renderers disagree:\nplain: %s\nstyled: %s", plain, output)
		}
	}
}
