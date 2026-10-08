package reportfactory_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/delivery/cli/reportfactory"
)

func TestAgentReportsPreserveFactsAcrossRenderers(t *testing.T) {
	reviewer := cli.ReviewerStatusReport{}
	reviewer.Installation.Project = "/project"
	reviewer.Installation.Name = "arclint-domain-reviewer"
	reviewer.Installation.Host = "codex"
	reviewer.Installation.Path = "reviewer.toml"
	reviewer.Installation.Installed = true
	reviewer.Installation.InstalledVersion = "1.0.0"
	reviewer.Installation.AvailableVersion = "1.1.0"
	reviewer.Installation.Problems = []string{"edited reviewer"}
	workflow := cli.WorkflowStatusReport{}
	workflow.Status.Project = "/project"
	workflow.Status.Command = "arclint agents workflow event"
	workflow.Status.Hosts = []application.WorkflowHostStatus{
		{Host: "claude", Path: "/project/.claude/settings.json", Installed: true},
		{Host: "codex", Path: "/project/.codex/hooks.json", Problems: []string{"Missing or changed Stop hook."}},
	}
	cases := []struct {
		name    string
		report  cli.Report
		facts   []string
		jsonKey string
	}{
		{"install", cli.AgentInstallReport{Operation: "reviewer", Host: "codex", Paths: []string{"reviewer.toml"}, Activation: "Restart the host to discover instructions."}, []string{"reviewer.toml", "Restart the host"}, "paths"},
		{"workflow status", workflow, []string{"/project", "arclint agents workflow event", "claude: /project/.claude/settings.json (installed: true)", "codex: /project/.codex/hooks.json (installed: false)", "Missing or changed Stop hook.", "trust them in /hooks"}, "hosts"},
		{"reviewer status", reviewer, []string{"/project", "arclint-domain-reviewer", "reviewer.toml", "1.0.0", "1.1.0", "edited reviewer", "do not prove"}, "problems"},
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
