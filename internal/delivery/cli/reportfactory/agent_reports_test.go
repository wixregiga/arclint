package reportfactory_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/delivery/cli/reportfactory"
)

func TestAgentReportsPreserveFactsAcrossRenderers(t *testing.T) {
	workflow := cli.HooksStatusReport{}
	workflow.Status.Project = "/project"
	workflow.Status.Binary = "/home/me/.local/bin/arclint"
	workflow.Status.LastEvent = time.Date(2026, 10, 9, 20, 15, 3, 0, time.UTC)
	workflow.Status.Files = []application.HookFileStatus{
		{Host: "claude", Scope: "user", Path: "/home/me/.claude/settings.json", Installed: true},
		{Host: "claude", Scope: "user", Windows: true, Path: "/mnt/c/Users/me/.claude/settings.json", Installed: true},
		{Host: "codex", Scope: "project", Path: "/project/.codex/hooks.json", Problems: []string{"Missing or changed Stop hook."}},
	}
	cases := []struct {
		name    string
		report  cli.Report
		facts   []string
		jsonKey string
	}{
		{"workflow install", cli.AgentInstallReport{Operation: "hooks", Host: "claude", Paths: []string{"/home/me/.claude/settings.json"}, Removed: []string{"/project/.claude/settings.local.json"}, Orientation: "Work in this order:\n1. Run context.", Activation: "Start a new session."}, []string{"/home/me/.claude/settings.json", "removed the duplicate hooks from /project/.claude/settings.local.json", "Work in this order:\n1. Run context.", "Start a new session."}, "orientation"},
		{"workflow status", workflow, []string{"/project", "Binary: /home/me/.local/bin/arclint", "Last event: 2026-10-09 20:15:03 +0000", "claude user: /home/me/.claude/settings.json (installed: true)", "claude user (Windows): /mnt/c/Users/me/.claude/settings.json (installed: true)", "codex project: /project/.codex/hooks.json (installed: false)", "Missing or changed Stop hook.", "trust them in /hooks"}, "lastEvent"},
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
