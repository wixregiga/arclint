package markdownagents_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	markdownagents "github.com/wixregiga/arclint/internal/infrastructure/agents/markdown"
)

func block(body string) string {
	return application.AgentsBegin + "\n" + body + "\n" + application.AgentsEnd + "\n"
}

func TestInstallLifecycle(t *testing.T) {
	root := t.TempDir()
	publisher, err := markdownagents.NewPublisher(root)
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}
	target := filepath.Join(root, "AGENTS.md")

	// Missing file: created holding only the block.
	changed, path, err := publisher.Install(block("v1"))
	if err != nil || !changed || path != target {
		t.Fatalf("create = (%v, %q, %v)", changed, path, err)
	}

	// Hand-written content outside the markers survives replacement.
	handWritten := "# My repo\n\ncustom guidance\n\n" + block("v1") + "\ntrailing notes\n"
	if err := os.WriteFile(target, []byte(handWritten), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	changed, _, err = publisher.Install(block("v2"))
	if err != nil || !changed {
		t.Fatalf("replace = (%v, %v)", changed, err)
	}
	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	for _, want := range []string{"custom guidance", "trailing notes", "v2"} {
		if !strings.Contains(string(after), want) {
			t.Errorf("after replacement, file lacks %q:\n%s", want, after)
		}
	}
	if strings.Contains(string(after), "v1") {
		t.Errorf("stale block survived replacement")
	}

	// Unchanged content reports no change.
	changed, _, err = publisher.Install(block("v2"))
	if err != nil || changed {
		t.Errorf("idempotent install = (%v, %v), want no change", changed, err)
	}

	// One marker without the other is corruption, never silent damage.
	if err := os.WriteFile(target, []byte("text\n"+application.AgentsBegin+"\nbroken\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, _, err := publisher.Install(block("v3")); err == nil {
		t.Errorf("a lone marker must be a corruption error")
	}
}

func TestGeneratedBlockReadsTheRangeInstallReplaces(t *testing.T) {
	for _, tc := range []struct {
		name, content, want string
		found, invalid      bool
	}{
		{name: "missing"},
		{name: "hand-written", content: "# Guide\nOnly hand-written guidance.\n"},
		{name: "marked", content: "before\n" + block("snapshot") + "after\n", want: strings.TrimSuffix(block("snapshot"), "\n"), found: true},
		{name: "CRLF", content: strings.ReplaceAll(block("snapshot"), "\n", "\r\n"), want: strings.TrimSuffix(strings.ReplaceAll(block("snapshot"), "\n", "\r\n"), "\r\n"), found: true},
		{name: "begin only", content: application.AgentsBegin, invalid: true},
		{name: "end only", content: application.AgentsEnd, invalid: true},
		{name: "reversed", content: application.AgentsEnd + application.AgentsBegin, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			publisher, err := markdownagents.NewPublisher(root)
			if err != nil {
				t.Fatal(err)
			}
			if tc.content != "" {
				if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(tc.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, found, err := publisher.GeneratedBlock()
			if (err != nil) != tc.invalid || found != tc.found || got != tc.want {
				t.Fatalf("GeneratedBlock = (%q, %v, %v)", got, found, err)
			}
		})
	}
}
